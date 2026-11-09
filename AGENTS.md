# AGENTS.md — webtyp/orm

<!-- webtyp:base — copied from devskills skill agents-md; do not edit here -->
## Ecosystem rules — read before any change

`docs/PLAN.md` is the spec: no design judgment; anything unclear is a question in the PR, not a guess.

- Public API: typed (no `any`/`interface{}`), explicit, sequential, minimal; no generics (`[T any]`) or `reflect` without benchmarked allocation/performance justification and owner approval; never export a symbol only a test uses.
- One intent, one path: the old path is deleted in the same change. No silent fallback, `TODO`, commented code or stub.
- Fail at compile time, else a loud diagnostic — never silently. Zero values deny.
- A defect in a dependency is reported, never wrapped or patched here.
- No string literals in logic: named constants. The same logic never lives in two repos: import the package the plan names; if you find an existing one the plan does not name that already does what you are about to write, say so in the PR instead of writing it.
- `cmd/*/main.go`: parse flags, inject, print/exit only. stdout = data, stderr = logs, exit codes are the contract.
- WASM code: `webtyp/fmt`, `webtyp/time`, `webtyp/json` instead of stdlib (imported without alias as `fmt`, `time`, `json` — never `tinytime`); no `map`; `dom.Element` embedded by value.
- `css.go`, `svg.go`, `js.go`, `html.go` carry `//go:build !wasm`; no `ssr.go`, no `front.go`.
- Build tags: `wasm`/`!wasm` only when the API does not exist on that platform; `browser`/`!browser` when an app's file belongs to another binary; a library carries no destination tag (the linker drops what a binary never calls). One main per folder, no destination tag: `web/browser/main.go` → `web/public/browser.wasm`, `web/server/main.go` → `web/server/server`, `web/edge/main.go` → `web/edge/edge.wasm`.
- Code and keys in English; UI text is `lang.Text`, translations live in `lang.json`. Comments and docs: English in `webtyp/*`; in `veltylabs/*` **every README, `docs/*` file, diagram and code comment is in Spanish**, written for a reader with little technical background, and each exported symbol's comment shows a short usage example.
- A translation key is reused, never duplicated as a near-identical phrase (`"not a valid integer"` + `"not a valid decimal number"` repeats `"not a valid"` three ways instead of composing it once) — every library ships to WASM/TinyGo, so each unreused phrase is dictionary rows AND binary bytes, in every consumer, forever. A token that doesn't need translating (a kind/type name like `int`) is passed as data (a variable), never as a string literal — a literal becomes a dictionary key whether or not it needed one.
- Tests cover behaviour and domain rules, never the look (CSS, colours, markup order, class strings). In `tests/` (external package), real stack with fakes only at the edge; run `gotest`.
- Docs (`README.md`, `docs/*`) updated with the API; permanent docs never link `docs/PLAN.md`.
- Before the PR: `git diff --stat main` lists every file of the plan's stages, and nothing the plan did not order is deleted.
<!-- /webtyp:base -->


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
- **Struct tags are processed at build-time, not runtime.** `fmt.Field` has no `Tag` field. If you ever feel you need to inspect a tag in WASM code, you are wrong — push the work into `definitionc`.

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
- Writing into `definition_gen.go` by hand → always run `definitionc`.
- Reaching for `map[K]V` for a lookup table or registry → use `fmt.KeyValue` (string/string) or a small
  local slice-of-structs scanned linearly instead. No exceptions, no `//go:build !wasm` escape hatch.
