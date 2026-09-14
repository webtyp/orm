---
PLAN: "fix: ReadOne/ReadAll must SELECT explicit columns, never rely on SELECT *"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 6123962758134431652
---

> This plan is dispatched via the CodeJob workflow. See skill: **agents-workflow**.

# Plan — stop `ReadOne`/`ReadAll` from silently corrupting data

You are an agent with **no prior context** and you have **only this repository**
(`webtyp.com/orm`). Everything you need is inline.

**Severity: this is a silent data-corruption defect, confirmed live against a
real Postgres database.** It is not a crash and not an error — every affected
read succeeds and returns a fully-populated struct with the WRONG values in
the WRONG fields. Treat this plan as blocking, not routine.

## 1. The defect, reproduced live

`qb.go`'s `ReadOne` and `ReadAll` build a `storage.Query` with **no `Columns`
set**:

```go
// qb.go — current code, both ReadOne and ReadAll
q := storage.Query{
	Action:     storage.ActionReadOne, // or ActionReadAll
	Table:      qb.model.ModelName(),
	Conditions: qb.conds,
	OrderBy:    qb.orderBy,
	GroupBy:    qb.groupBy,
	Limit:      1, // ReadOne only; ReadAll uses qb.limit
	Offset:     qb.offset,
}
plan, err := qb.db.conn.Compile(q, qb.model)
// ...
row.Scan(qb.model.Pointers()...) // or rows.Scan(...) per row in ReadAll
```

An empty `Columns` makes `webtyp.com/postgres`'s translator emit `SELECT *`
(`translate.go`: `if len(q.Columns) == 0 { sb.Write("*") }`), which returns
columns in the table's **physical** storage order. `Scan` then assigns them
positionally into `qb.model.Pointers()`, which are ordered by the model's
**declared** `Schema()` order. These two orders are the same only by
coincidence — specifically, only as long as no column has ever been added to
the table after its initial `CREATE TABLE`.

**The moment they diverge, this corrupts data without raising an error.**
Postgres's `ALTER TABLE ADD COLUMN` (which `ddl.Sync` uses to add a field to an
existing table — see `webtyp.com/ddl`) always appends the new column to the
**end** of the physical row layout. If the model's own `Schema()` declares the
new field somewhere in the **middle** (a completely ordinary thing to do — a
field is usually declared next to the fields it's conceptually grouped with,
not always last), `SELECT *`'s physical order and `Schema()`'s declared order
permanently disagree from that point on, and every subsequent read scans
every column after the insertion point into the wrong destination.

### Live reproduction (`github.com/veltylabs/staff_manager`, real Postgres)

`StaffMemberModel` originally declared, in this order: `id, tenant_id,
user_id, rut, name, is_active, updated_at` (7 columns). A later change added
`specialty` and `role` **between `name` and `is_active`** — an entirely
reasonable place to put them — migrated onto the live table via `ddl.Sync`
(so physically, Postgres appended them at the end: `..., updated_at,
specialty, role`).

A direct reproduction against the live database, using this library exactly
as `staff_manager.Module.IsTrustedIP` does:

```go
var staff staffmanager.StaffMember
qb := db.Query(&staff).Where("user_id").Eq(userID).
	Where("tenant_id").Eq("mjosefa-cms").Where("is_active").Eq(true)
err := qb.ReadOne()
```

Before this fix, `err` is `nil` (no error — the query has no way to detect the
mismatch) and `staff` comes back as:

```
Specialty: "true"                    // this is is_active's value
Role:      "1789255672827043884"     // this is updated_at's value
IsActive:  false                     // this is specialty's value, coerced
UpdatedAt: 0                         // this is role's value, coerced to zero
```

Every field from the insertion point onward is shifted and silently
type-coerced into the wrong destination. `IsTrustedIP` then denies a
legitimate, correctly-configured login — the actual symptom a human
noticed — but the deeper problem is that **any** field misread this way could
silently corrupt a write, a permission check, or a report, with no exception
to catch.

## 2. Design gate

No exported symbol is added, removed, or renamed — `ReadOne`/`ReadAll`'s
signatures are unchanged; only the `storage.Query` they build internally
gains a populated `Columns` field, which was already part of `storage.Query`
and already honored by `translate.go` when non-empty. The api-design gate
does not apply; skip to stages.

## 3. Decisions already taken — do not revisit

1. **The fix is in `orm`, not in `postgres`, not in `ddl`, and not as a
   consumer-side workaround.** `postgres`'s `SELECT *` fallback is a
   reasonable default for a caller that genuinely has no explicit column
   list; the actual bug is that `orm`'s own `ReadOne`/`ReadAll` — which
   **always** has a `model.Model` with a `Schema()` in hand — never bothers
   to supply one. Every backend behind `storage.Conn` (postgres, sqlite,
   mem, mock) receives the same `storage.Query`; fixing it once in `orm`
   fixes every backend uniformly, forever, with no per-backend patch. This
   is the same reasoning `Create` already applies two functions above in
   this same file — it already builds `Columns` from `Schema()`; `ReadOne`
   and `ReadAll` are the two functions that never adopted that pattern.
2. **Every field in `Schema()` becomes a column, unconditionally — no
   skip logic.** `Create`'s loop skips zero-valued autoincrement PKs and
   `OmitEmpty` zero values, because those are legitimate reasons to omit a
   column from an **INSERT**. None of that applies to a **read**: every
   field `Pointers()` returns a destination for must have a matching column
   in the `SELECT`, in the same order, with no exceptions — that positional
   contract is the entire fix.
3. **Order is `Schema()`'s declared order, not any other ordering.**
   `Pointers()` is documented and generated (by `ormc`) to return pointers in
   exactly the order `Schema()` declares fields — that is the existing,
   already-relied-upon contract this fix restores alignment with. Do not
   sort columns alphabetically, by DB introspection, or any other order.

## 4. Stages

### Stage 1 — `ReadOne`, in `qb.go`

Add a column-list before building the query:

```go
func (qb *QB) ReadOne() error {
	if err := validateQuery(storage.ActionReadOne, qb.model); err != nil {
		return err
	}
	schema := qb.model.Schema()
	columns := make([]string, len(schema))
	for i, f := range schema {
		columns[i] = f.Name
	}
	q := storage.Query{
		Action:     storage.ActionReadOne,
		Table:      qb.model.ModelName(),
		Columns:    columns,
		Conditions: qb.conds,
		OrderBy:    qb.orderBy,
		GroupBy:    qb.groupBy,
		Limit:      1, // Force limit 1
		Offset:     qb.offset,
	}
	plan, err := qb.db.conn.Compile(q, qb.model)
	if err != nil {
		return err
	}

	row := qb.db.conn.QueryRow(plan.Query, plan.Args...)
	if err := row.Scan(qb.model.Pointers()...); err != nil {
		if err == storage.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	return nil
}
```

Only the addition of `schema`/`columns` and the `Columns: columns` line
change; every other line is unchanged.

### Stage 2 — `ReadAll`, in `qb.go`

The identical pattern, in the sibling function immediately below:

```go
func (qb *QB) ReadAll(new func() model.Model, onRow func(model.Model)) error {
	if err := validateQuery(storage.ActionReadAll, qb.model); err != nil {
		return err
	}
	schema := qb.model.Schema()
	columns := make([]string, len(schema))
	for i, f := range schema {
		columns[i] = f.Name
	}
	q := storage.Query{
		Action:     storage.ActionReadAll,
		Table:      qb.model.ModelName(),
		Columns:    columns,
		Conditions: qb.conds,
		OrderBy:    qb.orderBy,
		GroupBy:    qb.groupBy,
		Limit:      qb.limit,
		Offset:     qb.offset,
	}
	plan, err := qb.db.conn.Compile(q, qb.model)
	if err != nil {
		return err
	}

	rows, err := qb.db.conn.Query(plan.Query, plan.Args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		m := new()
		if err := rows.Scan(m.Pointers()...); err != nil {
			return err
		}
		onRow(m)
	}
	return rows.Err()
}
```

**Anti-footgun.** `ReadAll` calls `new()` to build a **fresh** model per row,
and that fresh model's `Schema()` must be used for `columns` — but since every
instance of the same model type returns an identical `Schema()`, computing
`columns` **once**, before the loop, from `qb.model` (the model the `QB` was
built with) is correct and avoids recomputing it per row. Do not move the
`schema`/`columns` computation inside the `for rows.Next()` loop.

### Stage 3 — regression tests, in `tests/`

New file `tests/select_columns_test.go`, `package tests` (matching this
repo's existing convention — `core_test.go` uses `package tests` too, not
`orm_test`). Uses `webtyp.com/storage/mock`, exactly like `core_test.go`
already does — **no real database needed**: `mock.Compiler.Compile` records
the exact `storage.Query` it was asked to compile, so the test asserts
directly on `Columns` without needing any column-order drift to actually
happen against a real backend.

```go
package tests

import (
	"testing"

	mdl "webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage"
	"webtyp.com/storage/mock"
)

func TestReadOne_SelectsExplicitColumnsInSchemaOrder(t *testing.T) {
	schema := []mdl.Field{
		{Name: "id"},
		{Name: "tenant_id"},
		{Name: "name"},
		{Name: "specialty"}, // inserted in the MIDDLE — the exact live-bug shape
		{Name: "role"},
		{Name: "is_active"},
	}
	m := &mock.Model{Table: "staff_member", Sch: schema, Vals: make([]any, len(schema))}

	compiler := &mock.Compiler{}
	exec := &mock.Executor{}
	db := orm.New(mockConn{Executor: exec, Compiler: compiler})

	_ = db.Query(m).Where("id").Eq("x").ReadOne()

	want := []string{"id", "tenant_id", "name", "specialty", "role", "is_active"}
	got := compiler.LastQuery.Columns
	if len(got) != len(want) {
		t.Fatalf("Columns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Columns[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestReadAll_SelectsExplicitColumnsInSchemaOrder(t *testing.T) {
	schema := []mdl.Field{
		{Name: "id"},
		{Name: "extra_middle_field"},
		{Name: "name"},
	}
	m := &mock.Model{Table: "widget", Sch: schema, Vals: make([]any, len(schema))}

	compiler := &mock.Compiler{}
	exec := &mock.Executor{}
	db := orm.New(mockConn{Executor: exec, Compiler: compiler})

	_ = db.Query(m).ReadAll(func() mdl.Model { return m }, func(mdl.Model) {})

	want := []string{"id", "extra_middle_field", "name"}
	got := compiler.LastQuery.Columns
	if len(got) != len(want) {
		t.Fatalf("Columns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Columns[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
```

`mockConn` (a struct embedding `storage.Executor` + `storage.Compiler`) already
exists in `tests/core_test.go` — reuse it, do not redeclare a second one with
a different name.

**Anti-footgun.** `mock.Executor`'s `QueryRow`/`Query` default to a no-op
`Scanner`/empty `Rows` when no `ReturnQueryRow`/`ReturnQueryRows` is set —
`ReadOne`/`ReadAll` will run to completion (ignoring the actual scan) purely
to let `Compile` get called and its argument captured. Do not set
`ReturnQueryRow`/`ReturnQueryRows` in this test; the default no-op is exactly
what makes the test possible without a real backend.

Run `gotest ./...` (never `go test`) — every pre-existing test in `tests/`
must stay green, in addition to the two new ones.

### Stage 4 — documentation

- `ARQUITECTURE.md` (this repo's own spelling of the filename — verify before
  writing): if it describes `ReadOne`/`ReadAll`'s generated SQL anywhere,
  correct it to state that the column list is always explicit, sourced from
  `Schema()`, never `SELECT *`.
- Do **not** link any permanent document to `docs/PLAN.md` — it is deleted
  when this lands.

## 5. Stages table

| # | Stage | Files | Acceptance |
|---|---|---|---|
| 1 | `ReadOne` | `qb.go` | `Columns` populated from `Schema()`, in order |
| 2 | `ReadAll` | `qb.go` | same, computed once before the row loop |
| 3 | Tests | `tests/select_columns_test.go` (new) | both new tests green, reusing `mockConn` |
| 4 | Docs | `ARQUITECTURE.md` (only if it needs correcting) | no doc claims `SELECT *` |

## 6. Acceptance criteria

- `grep -n "Columns:" qb.go` → present in both `ReadOne` and `ReadAll`.
- `grep -n "SELECT \*" .` anywhere describing this library's own read path in
  prose → none (the `postgres` package's own fallback comment, in a
  different repository, is not this repository's concern).
- The new tests prove the exact live-bug shape (a field inserted in the
  middle of `Schema()`) produces a `Columns` list in `Schema()`'s declared
  order — not the case that happened to work before (fields only ever
  appended at the end).
- Every pre-existing test in `tests/` is unmodified and still green.
- `gotest ./...` green.
