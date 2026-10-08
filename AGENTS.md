# AGENTS.md — webtyp/orm

Working notes for AI agents operating in this library. For end-user docs see [README.md](README.md). For tag reference see [docs/](docs/).

## Mission of this package

`webtyp/orm` provides:

1. **An ergonomic optional layer over `webtyp/storage`** (`db.go`, `tx.go`, `qb.go`, `reexport.go`) — `*orm.DB`, query builder, `Create`/`Update`/`Delete`/`Query(ReadOne/ReadAll)`. Isomorphic: compiles for Go and WASM.

The runtime is reflection-free — `Fielder` interface (defined in `webtyp/fmt`) is the only contract. All struct introspection happens at codegen time.

## Architectural rules (do not violate)

### No Go `map` anywhere in this ecosystem

**Never use a built-in `map[K]V`, in any file, wasm-gated or not.** TinyGo's map runtime is heavy and
adds meaningful, unavoidable size to every wasm binary that ends up importing this code — and because
`orm`/`mock` are meant to be imported by wasm frontends (leaf modules use `mock.NewDB()` to test
round-trips without a real driver), there is no "backend-only" escape hatch via `//go:build !wasm`
anymore: the map has to not exist at all, not just be excluded from one build target.

- For a **string→string** pair, use `webtyp.com/fmt.KeyValue{Key, Value string}`.
- For anything else (typed values, non-string keys, or an in-memory "table" of rows), use a small
  local slice-of-structs and scan it linearly — see `storage/mem`'s `dbCell`/`dbRow`/`dbTable`
  for the pattern. These collections are always small (a handful
  of registered adapters, a handful of schema columns), so a linear scan costs nothing in practice and
  it's what the whole ecosystem already does (e.g. `webtyp/fmt.TagPairs` returns `[]KeyValue`, not a
  map).
- If you're tempted to add a map "just for a lookup cache," don't — reach for a linear scan first, and
  only reconsider with a profiler backing you up, never on a hunch.

### Root package (`orm`) — isomorphic, zero dialect

- **No `database/sql` import in the root package.** The root `orm` package compiles for both Go and WASM. Never import `database/sql`, `database/sql/driver`, or any DB driver. Use only `webtyp.com/fmt` and the stdlib.
- **Agnostic API only.** `qb.go`, `db.go` must never contain dialect-specific SQL, driver types, or engine-specific error values (e.g. `sql.ErrNoRows`).
- **Use `orm`-owned sentinels.** `errors.go` defines `ErrNotFound`, `ErrValidation`, `ErrEmptyTable`, `ErrNoTxSupport`. `qb.go` detects `storage.ErrNoRows` (returned by a backend's `Scanner`) with `storage.IsNoRows` and translates it to `ErrNotFound`; callers detect that with `orm.IsNotFound(err)` — never `==` or `errors.Is`, which pull reflection into the TinyGo binary. **Executor adapters** (`webtyp/postgres`, `webtyp/sqlt`) are responsible for mapping their driver-specific errors (e.g. `sql.ErrNoRows`) to `storage.ErrNoRows` inside their `Scanner` implementation.
- **Executor contract.** Adapters implement `storage.Executor` (defined in `webtyp/storage`, not this package): (1) map `sql.ErrNoRows` → `storage.ErrNoRows`, returning the sentinel itself, never wrapped (`storage.IsNoRows` is a type assertion); (2) never leak `database/sql` types into `orm` core.
- **Do not use `tinygo` as a build tag** — it is not a standard Go constraint recognized by the Go toolchain. Use `GOOS=js GOARCH=wasm` to build for wasm, and `gotest -tinygo` to test against the TinyGo compiler specifically.
- **Struct tags are processed at build-time, not runtime.** `fmt.Field` has no `Tag` field. If you ever feel you need to inspect a tag in WASM code, you are wrong — push the work into `ormc`.

## Code layout

| File / Dir | Role |
|------------|------|
| `db.go`, `tx.go` | `*orm.DB`, `*orm.Tx` — DML runtime entry points (`Create`/`Update`/`Delete`/`Query`) |
| `qb.go` | Query builder (`Where`, `Limit`, ...) |
| `validate.go` | Runtime validation glue (delegates to `fmt.ValidateFields`) |
| `reexport.go` | Aliases and value-type re-exports from `storage` |
| `tests/` | Test fixtures + `_test.go` files |
| `docs/` | Architecture, design rationale, tag reference |

> **Nota:** El contrato de almacenamiento vive en `webtyp.com/storage` — este repo no lo redefine.

## Testing

Install once:

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Run:

```bash
gotest              # vet + race + cover + wasm + badges
gotest -no-cache    # force re-run
gotest -run TestX   # filter
```

## Common mistakes to avoid

- Trying to parse struct tags from `fmt.Field` at runtime → impossible, the field has no `Tag`.
- Writing into `model_orm.go` by hand → always run `ormc`.
- Reaching for `map[K]V` for a lookup table or registry → use `fmt.KeyValue` (string/string) or a small
  local slice-of-structs scanned linearly instead. No exceptions, no `//go:build !wasm` escape hatch.
