# fieldcov

Walks `internal/types/generated/akuity/v1alpha1/` and emits a sorted
inventory of every exported struct field. The committed `baseline.json`
captures the state of the upstream Akuity API types the provider is built
against.

## Why

During the v2 refactor (see `REFACTOR_PLAN.md`) the codegen-driven converter
in `internal/convert/` relies on knowing every field the upstream API
exposes. Drift between upstream and provider is the root cause of the
current silent-field-drop bugs. This tool catches drift early.

## Usage

```sh
go run ./hack/fieldcov                    # print current inventory to stdout
go run ./hack/fieldcov -check             # diff against baseline.json; exit 1 on drift
go run ./hack/fieldcov -update-baseline   # rewrite baseline.json
```

CI runs `-check` on every PR. A drift — new or removed fields upstream —
blocks merge.

## When to update the baseline

After syncing `internal/types/generated/akuity/v1alpha1/` from
`akuity-platform/crossplane-gen/`, run:

```sh
go run ./hack/fieldcov -update-baseline
```

Audit the diff. New fields must either land in `apis/core/v1alpha2/` or be
explicitly ignored via `hack/codegen/overrides.yaml` (WS-3). Removed fields
mean upstream dropped something; mirror the removal in v1alpha2 or document
why the provider keeps it.

## What this tool does not do

It does not check whether provider converters reach every upstream field.
That signal comes from round-trip fixtures at `internal/types/test/roundtrip/`
and from the WS-3 codegen coverage report. This tool only answers "what
exists upstream right now."
