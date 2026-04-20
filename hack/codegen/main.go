// Codegen tool: walks the Akuity wire types in
// internal/types/generated/akuity/v1alpha1 and emits mechanical
// converters between those types and the curated apis/core/v1alpha2
// CRD types.
//
// See hack/codegen/README.md for the emission model and override
// schema.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dave/jennifer/jen"
	"gopkg.in/yaml.v3"
)

const (
	akuitySrcDir     = "internal/types/generated/akuity/v1alpha1"
	convertOutDir    = "internal/convert"
	defaultOverrides = "hack/codegen/overrides.yaml"
	headerFile       = "hack/boilerplate.go.txt"

	akuityImport   = "github.com/akuityio/provider-crossplane-akuity/internal/types/generated/akuity/v1alpha1"
	v1alpha2Import = "github.com/akuityio/provider-crossplane-akuity/apis/core/v1alpha2"
	glueImport     = "github.com/akuityio/provider-crossplane-akuity/internal/convert/glue"
)

// Sections groups the root types per emitted file. Sub-types are
// discovered automatically via reachability analysis.
var Sections = []Section{
	{Name: "cluster", Roots: []string{"ClusterData"}},
	{Name: "instance", Roots: []string{"InstanceSpec", "ArgoCDSpec"}},
	{Name: "kargoinstance", Roots: []string{"KargoSpec", "KargoInstanceSpec"}},
	{Name: "kargoagent", Roots: []string{"KargoAgentSpec", "KargoAgentData"}},
}

type Section struct {
	Name  string
	Roots []string
}

// Overrides describes the per-type code-generation overrides.
type Overrides struct {
	// Keyed by akuity struct type name.
	Types map[string]TypeOverride
	// TypeRenames maps akuity wire type names to their v1alpha2
	// counterparts when the names diverge.
	TypeRenames map[string]string
}

type TypeOverride struct {
	Renames       map[string]string `yaml:"renames"`
	Adapters      []FieldAdapter    `yaml:"adapters"`
	Ignore        []string          `yaml:"ignore"`
	GenerateFalse bool              `yaml:"generate_false"`
}

// FieldAdapter replaces the default assignment for a single field with
// a pair of adapter-function calls. Via is called on the
// curated→wire path; Back on the wire→curated path. Both must be
// fully qualified (e.g. "glue.KustomizationStringToRaw").
type FieldAdapter struct {
	Field string `yaml:"field"`
	Via   string `yaml:"via"`
	Back  string `yaml:"back"`
}

// StructInfo is the subset of go/ast info the emitter needs per struct.
type StructInfo struct {
	Name   string
	Fields []FieldInfo
}

type FieldInfo struct {
	Name    string // Go field name (exported)
	TypeStr string // stringified Go type, e.g. "*bool", "[]*IPAllowListEntry"
	Kind    FieldKind
	Elem    string // for slices/maps/pointers: the element type name
	IsNamed bool   // true when the type is a named type in the akuity package
}

type FieldKind int

const (
	KindPrimitive    FieldKind = iota // string, int32, uint32, int64, bool, float32, float64
	KindNamedString                   // named string type like ClusterSize
	KindPtrPrimitive                  // *bool, *string, *int32, etc.
	KindPtrTime                       // *metav1.Time
	KindRawExtension                  // runtime.RawExtension
	KindStruct                        // nested struct by-value
	KindPtrStruct                     // *<Struct>
	KindSlicePtrStruct                // []*<Struct>
	KindSliceString                   // []string
	KindMapStringString               // map[string]string
	KindMapStringStruct               // map[string]<Struct>
	KindMapStringPtrStruct            // map[string]*<Struct>
	KindUnsupported
)

func main() {
	var (
		outDir        = flag.String("out", convertOutDir, "directory to write zz_generated_*.go files")
		overridesFile = flag.String("overrides", defaultOverrides, "path to overrides YAML")
	)
	flag.Parse()

	structs, err := parseAkuityPackage(akuitySrcDir)
	if err != nil {
		fatalf("parse akuity types: %v", err)
	}

	overrides, err := loadOverrides(*overridesFile)
	if err != nil {
		fatalf("load overrides: %v", err)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("create out dir: %v", err)
	}

	header, err := os.ReadFile(headerFile)
	if err != nil {
		fatalf("read boilerplate header: %v", err)
	}

	// Section ordering determines which file "owns" a shared type.
	// The first section to reach a type emits it; later sections skip
	// it. This keeps cross-MR shared types (e.g. SecretsManagementConfig)
	// declared exactly once.
	alreadyEmitted := map[string]bool{}
	for _, sec := range Sections {
		emitted, err := emitSection(sec, structs, overrides, string(header), *outDir, alreadyEmitted)
		if err != nil {
			fatalf("emit section %q: %v", sec.Name, err)
		}
		fmt.Fprintf(os.Stderr, "codegen: %s → %s (%d types)\n", sec.Name, filepath.Join(*outDir, "zz_generated_"+sec.Name+".go"), emitted)
	}
}

// parseAkuityPackage walks the Akuity source directory and returns a
// catalog of exported struct definitions keyed by type name. Type
// aliases (e.g. `type ClusterSize string`) and interface types are
// ignored — only struct types become converter subjects.
func parseAkuityPackage(dir string) (map[string]*StructInfo, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		name := fi.Name()
		if strings.HasPrefix(name, "zz_generated") {
			return false
		}
		return strings.HasSuffix(name, ".go")
	}, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", dir, err)
	}

	out := map[string]*StructInfo{}
	// Catalog named-string types in a side map so the field classifier
	// can recognise them as NamedString rather than UnsupportedNamedRef.
	namedStrings := map[string]bool{}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				if !ts.Name.IsExported() {
					return true
				}
				// String-named types (e.g. type ClusterSize string).
				if ident, ok := ts.Type.(*ast.Ident); ok && ident.Name == "string" {
					namedStrings[ts.Name.Name] = true
					return false
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}
				info := &StructInfo{Name: ts.Name.Name}
				for _, field := range st.Fields.List {
					for _, name := range field.Names {
						if !name.IsExported() {
							continue
						}
						info.Fields = append(info.Fields, FieldInfo{
							Name:    name.Name,
							TypeStr: typeString(field.Type),
						})
					}
				}
				out[ts.Name.Name] = info
				return false
			})
		}
	}

	// Second pass: classify each field now that we know the full type
	// catalog (so we can tell struct refs from named-string refs).
	for _, s := range out {
		for i := range s.Fields {
			s.Fields[i].classify(out, namedStrings)
		}
	}
	return out, nil
}

// typeString renders an ast.Expr back to the Go source-level string
// used as our canonical type key.
func typeString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.SelectorExpr:
		if pkgIdent, ok := t.X.(*ast.Ident); ok {
			return pkgIdent.Name + "." + t.Sel.Name
		}
		return "<selector>"
	default:
		return fmt.Sprintf("<unsupported:%T>", e)
	}
}

func (f *FieldInfo) classify(structs map[string]*StructInfo, namedStrings map[string]bool) {
	ts := f.TypeStr
	switch ts {
	case "string", "bool", "int32", "int64", "uint32", "uint64", "float32", "float64":
		f.Kind = KindPrimitive
		return
	case "*string", "*bool", "*int32", "*int64", "*uint32", "*uint64", "*float32", "*float64":
		f.Kind = KindPtrPrimitive
		return
	case "*metav1.Time":
		f.Kind = KindPtrTime
		return
	case "runtime.RawExtension":
		f.Kind = KindRawExtension
		return
	case "[]string":
		f.Kind = KindSliceString
		return
	case "map[string]string":
		f.Kind = KindMapStringString
		return
	}
	if namedStrings[ts] {
		f.Kind = KindNamedString
		return
	}
	if strings.HasPrefix(ts, "map[string]") {
		elem := strings.TrimPrefix(ts, "map[string]")
		isPtr := strings.HasPrefix(elem, "*")
		elem = strings.TrimPrefix(elem, "*")
		if _, ok := structs[elem]; ok {
			if isPtr {
				f.Kind = KindMapStringPtrStruct
			} else {
				f.Kind = KindMapStringStruct
			}
			f.Elem = elem
			return
		}
	}
	if strings.HasPrefix(ts, "[]*") {
		elem := strings.TrimPrefix(ts, "[]*")
		if _, ok := structs[elem]; ok {
			f.Kind = KindSlicePtrStruct
			f.Elem = elem
			return
		}
	}
	if strings.HasPrefix(ts, "*") {
		elem := strings.TrimPrefix(ts, "*")
		if _, ok := structs[elem]; ok {
			f.Kind = KindPtrStruct
			f.Elem = elem
			f.IsNamed = true
			return
		}
	}
	if _, ok := structs[ts]; ok {
		f.Kind = KindStruct
		f.Elem = ts
		f.IsNamed = true
		return
	}
	f.Kind = KindUnsupported
}

func loadOverrides(path string) (Overrides, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Overrides{}, err
	}
	// Two passes over the same document: one to pull out _type_renames,
	// one to decode every other key as a TypeOverride.
	var head struct {
		TypeRenames map[string]string `yaml:"_type_renames"`
	}
	if err := yaml.Unmarshal(b, &head); err != nil {
		return Overrides{}, err
	}
	raw := map[string]TypeOverride{}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return Overrides{}, err
	}
	delete(raw, "_type_renames")
	return Overrides{Types: raw, TypeRenames: head.TypeRenames}, nil
}

// v1alpha2Name maps an akuity wire type name to the corresponding
// v1alpha2 type name, applying overrides.TypeRenames when the two
// diverge.
func (o Overrides) v1alpha2Name(akuityName string) string {
	if renamed, ok := o.TypeRenames[akuityName]; ok {
		return renamed
	}
	return akuityName
}

// emitSection walks the section's roots, discovers reachable struct
// types, and writes one zz_generated_<section>.go file with paired
// SpecToAPI / APIToSpec functions for every reachable struct. Types
// with `generate_false: true` are omitted (hand-written in glue).
func emitSection(sec Section, structs map[string]*StructInfo, overrides Overrides, header, outDir string, alreadyEmitted map[string]bool) (int, error) {
	reachable := map[string]bool{}
	var walk func(name string)
	walk = func(name string) {
		if reachable[name] {
			return
		}
		s, ok := structs[name]
		if !ok {
			return
		}
		reachable[name] = true
		for _, f := range s.Fields {
			switch f.Kind {
			case KindStruct, KindPtrStruct, KindSlicePtrStruct, KindMapStringStruct, KindMapStringPtrStruct:
				walk(f.Elem)
			}
		}
	}
	for _, r := range sec.Roots {
		walk(r)
	}

	// Deterministic emission order.
	names := make([]string, 0, len(reachable))
	for n := range reachable {
		names = append(names, n)
	}
	sort.Strings(names)

	f := jen.NewFile("convert")
	f.HeaderComment(strings.TrimSpace(header))
	f.HeaderComment("// Code generated by hack/codegen. DO NOT EDIT.")
	f.ImportAlias(akuityImport, "akuitytypes")
	f.ImportAlias(v1alpha2Import, "v1alpha2")
	f.ImportAlias(glueImport, "glue")

	var emitted int
	for _, name := range names {
		if alreadyEmitted[name] {
			continue
		}
		override := overrides.Types[name]
		if override.GenerateFalse {
			alreadyEmitted[name] = true
			continue
		}
		emitSpecToAPI(f, structs[name], override, overrides)
		emitAPIToSpec(f, structs[name], override, overrides)
		alreadyEmitted[name] = true
		emitted++
	}

	out := filepath.Join(outDir, "zz_generated_"+sec.Name+".go")
	if err := f.Save(out); err != nil {
		return 0, fmt.Errorf("save %s: %w", out, err)
	}
	return emitted, nil
}

func emitSpecToAPI(f *jen.File, s *StructInfo, override TypeOverride, overrides Overrides) {
	fnName := s.Name + "SpecToAPI"
	v1a2Name := overrides.v1alpha2Name(s.Name)
	f.Commentf("%s converts the curated v1alpha2 %s into the Akuity API %s wire type.", fnName, v1a2Name, s.Name)
	f.Func().Id(fnName).
		Params(jen.Id("in").Op("*").Qual(v1alpha2Import, v1a2Name)).
		Op("*").Qual(akuityImport, s.Name).
		BlockFunc(func(g *jen.Group) {
			g.If(jen.Id("in").Op("==").Nil()).Block(jen.Return(jen.Nil()))
			g.Id("out").Op(":=").Op("&").Qual(akuityImport, s.Name).Values()
			for _, field := range s.Fields {
				if shouldSkip(field.Name, override) {
					continue
				}
				g.Add(fieldCopy(field, override, overrides, true))
			}
			g.Return(jen.Id("out"))
		})
	f.Line()
}

func emitAPIToSpec(f *jen.File, s *StructInfo, override TypeOverride, overrides Overrides) {
	fnName := s.Name + "APIToSpec"
	v1a2Name := overrides.v1alpha2Name(s.Name)
	f.Commentf("%s converts the Akuity API %s wire type back into the curated v1alpha2 %s.", fnName, s.Name, v1a2Name)
	f.Func().Id(fnName).
		Params(jen.Id("in").Op("*").Qual(akuityImport, s.Name)).
		Op("*").Qual(v1alpha2Import, v1a2Name).
		BlockFunc(func(g *jen.Group) {
			g.If(jen.Id("in").Op("==").Nil()).Block(jen.Return(jen.Nil()))
			g.Id("out").Op(":=").Op("&").Qual(v1alpha2Import, v1a2Name).Values()
			for _, field := range s.Fields {
				if shouldSkip(field.Name, override) {
					continue
				}
				g.Add(fieldCopy(field, override, overrides, false))
			}
			g.Return(jen.Id("out"))
		})
	f.Line()
}

func shouldSkip(fieldName string, override TypeOverride) bool {
	for _, ig := range override.Ignore {
		if ig == fieldName {
			return true
		}
	}
	return false
}

// fieldCopy builds the single statement that copies a field between
// `in` and `out`. toAPI is true when emitting the curated→wire
// direction and false for wire→curated.
func fieldCopy(field FieldInfo, override TypeOverride, overrides Overrides, toAPI bool) jen.Code {
	adapter := findAdapter(field.Name, override)
	if adapter != nil {
		return adapterCopy(field, *adapter, toAPI)
	}
	switch field.Kind {
	case KindPrimitive, KindPtrPrimitive, KindPtrTime, KindSliceString, KindMapStringString:
		return jen.Id("out").Dot(field.Name).Op("=").Id("in").Dot(field.Name)
	case KindRawExtension:
		// RawExtension with no adapter → direct assign. Normally
		// overridden for Kustomization fields.
		return jen.Id("out").Dot(field.Name).Op("=").Id("in").Dot(field.Name)
	case KindNamedString:
		if toAPI {
			return jen.Id("out").Dot(field.Name).Op("=").Qual(akuityImport, field.TypeStr).Parens(jen.Id("in").Dot(field.Name))
		}
		return jen.Id("out").Dot(field.Name).Op("=").Qual(v1alpha2Import, overrides.v1alpha2Name(field.TypeStr)).Parens(jen.Id("in").Dot(field.Name))
	case KindStruct:
		// By-value nested struct: call converter, deref the returned pointer.
		fn := field.Elem + direction(toAPI)
		return jen.Id("out").Dot(field.Name).Op("=").Op("*").Id(fn).Call(jen.Op("&").Id("in").Dot(field.Name))
	case KindPtrStruct:
		fn := field.Elem + direction(toAPI)
		return jen.Id("out").Dot(field.Name).Op("=").Id(fn).Call(jen.Id("in").Dot(field.Name))
	case KindSlicePtrStruct:
		fn := field.Elem + direction(toAPI)
		elemType := jen.Op("*").Qual(pkgForDirection(toAPI), typeNameForDirection(field.Elem, overrides, toAPI))
		return jen.If(jen.Id("in").Dot(field.Name).Op("!=").Nil()).BlockFunc(func(g *jen.Group) {
			g.Id("out").Dot(field.Name).Op("=").Make(jen.Index().Add(elemType), jen.Lit(0), jen.Len(jen.Id("in").Dot(field.Name)))
			g.For(jen.List(jen.Id("_"), jen.Id("item")).Op(":=").Range().Id("in").Dot(field.Name)).Block(
				jen.Id("out").Dot(field.Name).Op("=").Append(jen.Id("out").Dot(field.Name), jen.Id(fn).Call(jen.Id("item"))),
			)
		})
	case KindMapStringStruct:
		fn := field.Elem + direction(toAPI)
		valueType := jen.Qual(pkgForDirection(toAPI), typeNameForDirection(field.Elem, overrides, toAPI))
		return jen.If(jen.Id("in").Dot(field.Name).Op("!=").Nil()).BlockFunc(func(g *jen.Group) {
			g.Id("out").Dot(field.Name).Op("=").Make(jen.Map(jen.String()).Add(valueType), jen.Len(jen.Id("in").Dot(field.Name)))
			g.For(jen.List(jen.Id("k"), jen.Id("v")).Op(":=").Range().Id("in").Dot(field.Name)).Block(
				jen.Id("v").Op(":=").Id("v"), // shadow so &v captures each loop iteration
				jen.Id("out").Dot(field.Name).Index(jen.Id("k")).Op("=").Op("*").Id(fn).Call(jen.Op("&").Id("v")),
			)
		})
	case KindMapStringPtrStruct:
		fn := field.Elem + direction(toAPI)
		valueType := jen.Op("*").Qual(pkgForDirection(toAPI), typeNameForDirection(field.Elem, overrides, toAPI))
		return jen.If(jen.Id("in").Dot(field.Name).Op("!=").Nil()).BlockFunc(func(g *jen.Group) {
			g.Id("out").Dot(field.Name).Op("=").Make(jen.Map(jen.String()).Add(valueType), jen.Len(jen.Id("in").Dot(field.Name)))
			g.For(jen.List(jen.Id("k"), jen.Id("v")).Op(":=").Range().Id("in").Dot(field.Name)).Block(
				jen.Id("out").Dot(field.Name).Index(jen.Id("k")).Op("=").Id(fn).Call(jen.Id("v")),
			)
		})
	default:
		// Unsupported: emit a compile-time comment so the developer
		// notices. generate_false + glue.go is the escape hatch.
		return jen.Commentf("// codegen: unsupported field %s of type %s — add generate_false override and hand-write in glue", field.Name, field.TypeStr)
	}
}

// typeNameForDirection returns the akuity-side type name unchanged
// when emitting toAPI, or the overridden v1alpha2 name when emitting
// toSpec.
func typeNameForDirection(akuityName string, overrides Overrides, toAPI bool) string {
	if toAPI {
		return akuityName
	}
	return overrides.v1alpha2Name(akuityName)
}

func direction(toAPI bool) string {
	if toAPI {
		return "SpecToAPI"
	}
	return "APIToSpec"
}

func pkgForDirection(toAPI bool) string {
	if toAPI {
		return akuityImport
	}
	return v1alpha2Import
}

func findAdapter(field string, override TypeOverride) *FieldAdapter {
	for i := range override.Adapters {
		if override.Adapters[i].Field == field {
			return &override.Adapters[i]
		}
	}
	return nil
}

func adapterCopy(field FieldInfo, adapter FieldAdapter, toAPI bool) jen.Code {
	fn := adapter.Via
	if !toAPI {
		fn = adapter.Back
	}
	pkg, name := splitQualified(fn)
	return jen.Id("out").Dot(field.Name).Op("=").Qual(qualifiedImport(pkg), name).Call(jen.Id("in").Dot(field.Name))
}

// splitQualified splits "glue.KustomizationStringToRaw" into
// ("glue", "KustomizationStringToRaw"). Unqualified names return ("",
// name) and the emitter falls back to local symbol resolution.
func splitQualified(s string) (string, string) {
	if i := strings.LastIndex(s, "."); i > 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

// qualifiedImport maps a short package identifier (e.g. "glue") to its
// full import path. Any identifier not in the map is returned
// unchanged so callers that already pass a full path keep working.
func qualifiedImport(short string) string {
	switch short {
	case "glue":
		return glueImport
	default:
		return short
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "codegen: "+format+"\n", args...)
	os.Exit(1)
}
