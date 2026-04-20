// fieldcov walks the Akuity API generated types and emits a sorted inventory
// of every exported struct field. The output is compared against a committed
// baseline (hack/fieldcov/baseline.json) to catch two classes of regression:
//
//  1. A new field appears upstream in akuity-gen without coverage in the
//     provider's CRD types (silent-drop class of bug).
//  2. An existing field disappears upstream, indicating the generated types
//     were refreshed and the provider's converters/overrides need review.
//
// Usage:
//
//	go run ./hack/fieldcov                    # print current inventory
//	go run ./hack/fieldcov -check             # diff against baseline, exit 1 on drift
//	go run ./hack/fieldcov -update-baseline   # rewrite baseline.json
//
// The tool does not attempt to detect "reachability" from the provider's
// converters. That signal comes from round-trip fixtures and from the WS-3
// codegen overrides.yaml coverage report. fieldcov is intentionally simple:
// it only tells you what upstream types exist.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const defaultSourceDir = "internal/types/generated/akuity/v1alpha1"

func main() {
	var (
		source         = flag.String("source", defaultSourceDir, "directory of generated Akuity API types")
		baselinePath   = flag.String("baseline", "hack/fieldcov/baseline.json", "path to the baseline inventory file")
		check          = flag.Bool("check", false, "compare current inventory against baseline; exit 1 on drift")
		updateBaseline = flag.Bool("update-baseline", false, "write the current inventory to the baseline path")
	)
	flag.Parse()

	fields, err := collectFields(*source)
	if err != nil {
		fatalf("collect fields: %v", err)
	}

	switch {
	case *updateBaseline:
		if err := writeJSON(*baselinePath, fields); err != nil {
			fatalf("write baseline: %v", err)
		}
		fmt.Fprintf(os.Stderr, "wrote %d fields to %s\n", len(fields), *baselinePath)
	case *check:
		baseline, err := readJSON(*baselinePath)
		if err != nil {
			fatalf("read baseline: %v", err)
		}
		added, removed := diff(baseline, fields)
		if len(added) == 0 && len(removed) == 0 {
			fmt.Fprintln(os.Stderr, "fieldcov: inventory matches baseline")
			return
		}
		fmt.Fprintf(os.Stderr, "fieldcov: inventory drift vs %s\n", *baselinePath)
		for _, f := range added {
			fmt.Fprintf(os.Stderr, "  + %s\n", f)
		}
		for _, f := range removed {
			fmt.Fprintf(os.Stderr, "  - %s\n", f)
		}
		fmt.Fprintln(os.Stderr, "run `go run ./hack/fieldcov -update-baseline` after auditing coverage.")
		os.Exit(1)
	default:
		if err := json.NewEncoder(os.Stdout).Encode(fields); err != nil {
			fatalf("encode: %v", err)
		}
	}
}

func collectFields(dir string) ([]string, error) {
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

	seen := map[string]struct{}{}
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
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}
				typeName := ts.Name.Name
				for _, field := range st.Fields.List {
					for _, name := range field.Names {
						if !name.IsExported() {
							continue
						}
						seen[typeName+"."+name.Name] = struct{}{}
					}
					// Embedded fields: field.Names is empty; record the type name.
					if len(field.Names) == 0 {
						seen[typeName+"."+embeddedName(field.Type)] = struct{}{}
					}
				}
				return false
			})
		}
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func embeddedName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.StarExpr:
		return embeddedName(t.X)
	default:
		return fmt.Sprintf("<embedded:%T>", expr)
	}
}

func diff(old, new []string) (added, removed []string) {
	oldSet := map[string]struct{}{}
	for _, f := range old {
		oldSet[f] = struct{}{}
	}
	newSet := map[string]struct{}{}
	for _, f := range new {
		newSet[f] = struct{}{}
	}
	for _, f := range new {
		if _, ok := oldSet[f]; !ok {
			added = append(added, f)
		}
	}
	for _, f := range old {
		if _, ok := newSet[f]; !ok {
			removed = append(removed, f)
		}
	}
	return added, removed
}

func readJSON(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func writeJSON(path string, v []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fieldcov: "+format+"\n", args...)
	os.Exit(2)
}
