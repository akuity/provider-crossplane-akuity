# hack/codegen

Go AST walker that reads the Akuity upstream wire types and emits
mechanical converters between them and the curated `apis/core/v1alpha2`
CRD types. Replaces the 1,381-LOC hand-written converter layer.

## Run

```
make generate-convert    # runs `go run ./hack/codegen`
```

The tool writes `internal/convert/zz_generated_<section>.go` for each
configured root type family. Hand-written adapters live at
`internal/convert/glue/glue.go`.

## Inputs

- `internal/types/generated/akuity/v1alpha1/*.go` — Akuity wire types
  (refreshed from `akuity-platform/api/akuity/v1alpha1`).
- `apis/core/v1alpha2/*.go` — curated CRD types that mirror the wire
  shape 1:1, modulo the adapters in `overrides.yaml`.
- `hack/codegen/overrides.yaml` — declarative field renames, adapters,
  and `generate_false` escape hatches.

## Emission model

For each struct `T` reachable from a configured root, the tool emits:

```go
func TSpecToAPI(in *v1alpha2.T) *akuity.T
func TAPIToSpec(in *akuity.T) *v1alpha2.T
```

Field handling:

| Shape | Strategy |
|---|---|
| Primitive (string, int32, bool, float32, etc.) | direct assign |
| Named primitive (e.g. `ClusterSize string`) | explicit cast |
| `*bool`, `*string`, `*int32`, `*metav1.Time` | direct assign (same type both sides) |
| Nested struct | recurse via `<T>SpecToAPI` / `<T>APIToSpec` |
| `*<Struct>` | nil-check + dispatch |
| `[]<T>` or `[]*<T>` | loop + recurse |
| `map[string]<T>` | loop + recurse |
| Field marked with an adapter | call the override's `via` / `back` function |

## Overrides

See `overrides.yaml` for the schema. The current overrides only cover
the `Kustomization runtime.RawExtension` ↔ `string` transformation.

## Fallback

If the tool cannot mechanically handle a type (polymorphic
`interface{}`, `oneof` union, cyclic type), mark it
`generate_false: true` in `overrides.yaml` and hand-write the pair in
`internal/convert/glue/glue.go` using the expected signatures above.
