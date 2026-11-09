---
PLAN: "refactor!: orm follows storage v0.2.0 (Exec reports rows and duplicates), imports crud/conformance, and its Store updates conditionally (Revision) — proven by crud/conformance"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 9250111747019076818
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage, push and open the pull request. Never ask whether to proceed or finalize — nobody answers, and a session that waits ends without your work.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

## Goal

`crud` renamed its conformance suite `storetest` → `conformance` (crud plan, stage 2). `orm` is its only consumer: `tests/store_test.go` imports it to prove `orm.Store` satisfies `crud.Store`. Nothing else changes.

## Prerequisite

The stages run in this order: (1) the import rename of the first section, (2) the `Exec` stage, (3) the conditional update. `storage` v0.2.1 and `crud` v0.0.9 are published: `go get webtyp.com/crud@v0.0.9 webtyp.com/storage@v0.2.1 && go mod tidy`.

## Change (`tests/store_test.go`)

| Old | New |
|---|---|
| `import "webtyp.com/crud/storetest"` | `import "webtyp.com/crud/conformance"` |
| `storetest.Run(t, func(t *testing.T) crud.Store {…})` | `conformance.Run(t, func(t *testing.T) crud.Store {…})` |
| `storetest.TestModel` (2 uses: `dummy := &storetest.TestModel{…}`, `globalModel := &storetest.TestModel{}`) | `conformance.TestModel` |

`grep -rn "storetest" --include=*.go --include=*.md .` → empty (update any `docs/` mention the same way).

## Delivery check

1. `gotest` passes.
2. `git diff --stat main` lists only `tests/store_test.go`, `go.mod`, `go.sum` and the docs that named `storetest`.

## Stage — `Exec` reports rows affected and a duplicate key (decision D48)

### Goal

`webtyp/storage` v0.2.0 changed the executor contract (decision D48 of the architecture master, so that `crud` can do a conditional update and report a duplicate key):

| Before | After |
|---|---|
| `Exec(query string, args ...any) error` | `Exec(query string, args ...any) (storage.Result, error)` |
| a unique violation = the driver's own error | a unique violation = `storage.Duplicate(driverErr)`, tested with `storage.IsDuplicate(err)` |

`storage.Result` has one method, `RowsAffected() int64`; `storage.Affected(n)` is the value every implementation returns. Nothing else changes in this repo: no new feature, no rename.
Wave 1 of the migration; the repos it depends on are published (only `storage`).

### Ecosystem search

Searched `Exec(` over `webtyp/*` and `veltylabs/*`: the implementers and callers of the old signature are the ones listed here; the compiler reports any site missed. No new library.

### Design gate

1. **Prior art**: `database/sql.Result` (`RowsAffected`), `sql.DB.Exec`.
2. **Novice-name test**: `result.RowsAffected()` = how many rows changed; `storage.IsDuplicate(err)` = the key already exists.
3. **Ledger**: concepts 0 here (the contract is storage's); ways to learn that a write changed nothing: 0 → 1.
4. **Where**: the contract is `webtyp/storage`; this repo adapts to it.
5. **Deletes**: the old single-value `Exec` shape.

### Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/storage@v0.2.1 webtyp.com/crud@v0.0.9
go mod tidy
```

### What to change

`db.go` (and any file that calls `Exec` on a `storage.Executor` or `Conn`): the compiler lists every site. `DB.Create`, `Update`, `UpdateFields`, `Delete` keep their signatures (`error`): inside, `_, err := d.conn.Exec(...)`.
`UpdateFieldsIf` and `ConditionalUpdater` come in the last stage of this plan, after this one is green.

Rules for every call site (the compiler lists them: follow its errors, then `grep -rn "\.Exec(" --include=*.go .`):

| Old | New |
|---|---|
| `err := x.Exec(…)` | `_, err := x.Exec(…)` |
| `if err := x.Exec(…); err != nil {` | `if _, err := x.Exec(…); err != nil {` |
| `return x.Exec(…)` in a function that returns `error` | `_, err := x.Exec(…)` then `return err` |
| a type with `Exec(query string, args ...any) error` | `Exec(query string, args ...any) (storage.Result, error)`; return `storage.Affected(n)` with the real count, or `storage.Affected(0)` when it executes nothing (a test double) |

Never ignore the count where the caller can use it, and never invent a count: a driver that cannot know it fails the `storage/conformance` suite and the delivery.

### Delivery check

1. `gotest` passes (builds, vets, runs the tests; a PR that does not compile is a failed delivery).
2. `gotest` is green in every package of this repo (there is no driver conformance test here).
3. `grep -rnE "\) Exec\(([a-z]+) string, ([a-z]+) \.\.\.any\) error" --include=*.go .` → empty (no implementer with the old shape).
4. `git diff --stat main` lists only the files named above, `go.mod`, `go.sum` and the docs that showed `Exec`.

## Stage — the conditional update (decision D48)

`orm` stays free of any database driver: **its go.mod and its tests import only `storage/mem`**. What a store must show under concurrency (one winner among racing creates, unique values and conditional updates) is `crud/conformance` (v0.0.9, published first): `orm`'s own test runs it over `storage/mem`, and each driver (`sqlite`, `postgres`, …) runs the same suite over `orm.GlobalStore` in its own tests.

1. `db.go`: split `UpdateFields` in two without changing its signature. Move its whole body (everything up to and including `plan, err := d.conn.Compile(q, m)`) into an unexported `func (d *DB) updateFields(m model.Model, fields []string, cond storage.Condition, rest ...storage.Condition) (storage.Result, error)` that ends with `return d.conn.Exec(plan.Query, plan.Args...)`, and make `UpdateFields` call it and drop the result (`_, err := d.updateFields(...)`; `return err`). One implementation, no copy.
2. `store.go`: add

```go
var _ crud.ConditionalUpdater = (*Store)(nil)

// UpdateFieldsIf is crud's conditional update: it writes fields of the row id only if the column field still holds expected, and says how many rows it changed.
//
//	rows, err := store.UpdateFieldsIf(rec, id, []string{"name", "revision"}, "revision", 3)
func (s *Store) UpdateFieldsIf(rec model.Model, id string, fields []string, field string, expected int64) (int64, error) {
	for _, f := range fields {
		if f == FieldTenant {
			return 0, crud.ErrServerOnlyField
		}
	}
	conds, err := keyConds(rec, id)
	if err != nil {
		return 0, err
	}
	scoped, err := s.scope(append(conds, storage.Eq(field, expected)))
	if err != nil {
		return 0, err
	}
	result, err := s.db.updateFields(rec, fields, scoped[0], scoped[1:]...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
```
3. The proof is `tests/store_test.go`'s `TestStore_Conformance` (it already calls `conformance.Run` with `orm.GlobalStore` over `mem.New()`): with `webtyp.com/crud@v0.0.9` and `webtyp.com/storage@v0.2.1` it now also runs the clauses `DuplicatePrimaryKeyIsReported`, `DuplicateUniqueColumnIsReported`, `ConcurrentCreateOfOneKeyHasOneWinner`, `ConcurrentCreateOfOneUniqueValueHasOneWinner`, `ConditionalUpdateAppliesOnlyWhileTheRevisionMatches` and `ConcurrentConditionalUpdatesHaveOneWinner`. The factory of that test creates a table for every model of `conformance.Models()` (`mem` creates them on first write: nothing to add). Run it with `-race -count=3`: it must be stable. If a clause fails, report it in the PR with the output; never weaken the clause or skip it.
4. `docs/ARCHITECTURE.md`: one paragraph on `UpdateFieldsIf` and the sentence «`orm` imports no driver; each driver proves `orm.GlobalStore` with `crud/conformance` in its own tests».

Commit: `feat: conditional update (Revision), proven by crud/conformance`.
## Executor notes

Completed without issues.
