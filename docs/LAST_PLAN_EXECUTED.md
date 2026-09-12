---
PLAN: "feat: OmitEmpty omits a zero-valued column from the INSERT"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> **Phase D** of
> [`NULLABLE_COLUMNS_MASTER_PLAN.md`](https://github.com/webtyp/docs/blob/main/NULLABLE_COLUMNS_MASTER_PLAN.md).
> Independent of phases A/B/C — it may run in parallel with them.

# Plan — `webtyp.com/orm`: there is no way to leave a column out of an INSERT

## 0. Context (verified against the repo — do not re-diagnose)

`DB.Create` writes **every** column of `Schema()`:

```go
for i, f := range schema {
    // Skip autoincrement PK fields with zero value — let the DB assign them.
    if f.IsPK() && f.IsAutoInc() {
        if v, ok := allValues[i].(int); ok && v == 0 {
            continue
        }
    }
    columns = append(columns, f.Name)
    values = append(values, allValues[i])
}
```

So a consumer that needs one column left out — to let the database store NULL,
or apply its own default — **cannot express it**. `model.Field.OmitEmpty`
exists and is documented as *"omit from JSON when zero value"*: it guards
`EncodeFields` only, and `Create` never reads it.

The cost of that gap is already visible downstream. `webtyp/auth` needs
email-less LAN users, and its `email` column is `Unique` — writing `""` twice
collides, while NULL is exactly what a unique index allows to repeat. With no
way to omit the column, it had to reach past the ORM:

```go
// auth/authority/users.go — what this plan deletes
q := storage.Query{Action: storage.ActionCreate, Table: u.ModelName(),
    Columns: []string{"id", "name", "phone", "status", "avatar", "created_at"},
    Values:  []any{id, name, phone, "active", "", now}}
conn := db.RawConn()
plan, err := conn.Compile(q, &u)
```

A consumer hand-building a `storage.Query` and compiling it itself is the ORM
leaking: the column list is now duplicated in the consumer and drifts the moment
the model gains a field.

`Create` already has the precedent for the correct behaviour, in the very loop
above: the autoincrement PK is skipped *so the database can supply the value*.
`OmitEmpty` is the same intent, declared by the model author instead of inferred.

**Blast radius, measured before proposing this.** `OmitEmpty` appears in 3 files
across the whole monorepo, and every existing definition is in `mcp/model.go` —
transport-only models with **no `DB:` tag**, never persisted. No persisted model
in the ecosystem changes behaviour.

## Design gate (api-design — five answers)

### 1. Prior art

| Concern | Frameworks | Why we differ |
|---|---|---|
| Omit a column from INSERT | **GORM** (`omitempty` is absent; uses `Omit("col")` per call, or pointer/`sql.Null*` fields for "unset") | A per-call `Omit("email")` puts the rule at every call site, where it is forgotten once and wrong forever. Ours is declared once on the model, where the reason lives. |
| Omit a column from INSERT | **Ent** (`Optional()` on the field → generated builder simply never sets it), **Bun** (`nullzero` struct tag: a zero value is written as NULL) | Same shape as ours, declared at the schema. Bun's `nullzero` is the closest analogue; we reuse the flag the model already has rather than adding a second one. |
| Omit a column from INSERT | **Django** (`null=True` + `blank=True` on the field), **ActiveRecord** (DB default + attribute left unassigned) | Both make it a field-level declaration too. We differ only in not adding new vocabulary: `OmitEmpty` already exists and already means "when zero, leave it out". |

### 2. Novice-name test

No new name. `OmitEmpty` read aloud is "omit it when it is empty" — which is
exactly what this makes it do on the INSERT, and what it already does on the
wire. The current split, where it means that in one direction and nothing in the
other, is the part a junior cannot predict.

### 3. Complexity ledger

```
Concepts the developer must learn   +0 (OmitEmpty already exists) / −1 (nobody learns "the ORM cannot omit a column")
Files they must touch to do X       +0 / −1 (auth deletes its hand-built storage.Query)
Lines at the call site              +0 / −20 (the RawConn bypass in auth)
Ways to do the same thing           +0 / −1 (was: OmitEmpty for the wire, RawConn for the INSERT)
```

### 4. Where it belongs

`orm` owns `Create` and the column list it builds. `model` owns the flag and
keeps owning it — this plan does not change `model.Field`. Putting the
behaviour anywhere else would mean every consumer re-deriving the column list,
which is the defect being removed.

### 5. What it deletes

- The need for `DB.RawConn()` in `webtyp/auth` (deleted there in phase E).
- The split meaning of `OmitEmpty`: one flag, one rule, both directions.

## Quality rules

```
RULE: no stdlib beyond what the repo already imports — webtyp/fmt, webtyp/model,
      webtyp/storage only. Do NOT reach for reflect.
RULE: every repeated string is a named constant; string literals forbidden in logic.
RULE: no silent fallback — the zero test is explicit per storage kind, never a
      catch-all that guesses.
```

## Stage 1 — `Create` honours `OmitEmpty`

**File:** `db.go`.

In the `Create` loop, after the existing autoincrement-PK skip, add the
`OmitEmpty` skip:

```go
// A zero value on an OmitEmpty field is left out of the INSERT entirely, so the
// database applies its own default — NULL for a nullable column. That is the
// only way to store NULL in a UNIQUE column, where "" would collide and NULL is
// the value a unique index allows to repeat.
if f.OmitEmpty && isZeroValue(allValues[i]) {
    continue
}
```

Add the helper in the same file, typed per storage kind — **no `reflect`**:

```go
func isZeroValue(v any) bool {
	switch x := v.(type) {
	case string:
		return x == ""
	case int:
		return x == 0
	case int64:
		return x == 0
	case float64:
		return x == 0
	case bool:
		return !x
	case []byte:
		return len(x) == 0
	case nil:
		return true
	}
	return false
}
```

A PK must never be skipped this way even if someone marks it `OmitEmpty` — a row
with no primary key is not a row. Guard it:

```go
if f.OmitEmpty && !f.IsPK() && isZeroValue(allValues[i]) {
```

## Stage 2 — consumer-shaped test (the publication rule)

**File:** `tests/omitempty_test.go` (new), in the existing `tests` package that
already carries `models_orm.go`.

1. Add a model to `tests/models.go` (or wherever the definitions in that package
   live) with a nullable, unique, `OmitEmpty` column — the shape the real
   consumer has:
   ```go
   {Name: "email", Type: model.Text(), OmitEmpty: true, DB: &model.FieldDB{Unique: true}},
   ```
   Regenerate that package's `_orm.go` with `ormc`.
2. The test, against the in-memory backend the other tests in this package use:
   - creating **two** records with an empty email succeeds — the proof that the
     column was omitted and the unique index saw NULL, not `""` twice;
   - a record with a real email round-trips that email;
   - two records with the **same** real email still fail — omitting the zero
     case must not relax the constraint;
   - a non-`OmitEmpty` zero-valued column is still written (assert a record with
     an empty non-`OmitEmpty` text field reads back as `""`, not a DB default).
3. Assert on `Create`'s emitted column list where the backend exposes it, so the
   test fails for the right reason rather than only via the unique index.

## Acceptance criteria

1. `go build ./...`, `go vet ./...`, `go test ./...` green.
2. `grep -rn "reflect" --include='*.go' .` → empty.
3. The new test fails if the `OmitEmpty` skip is removed from `Create` — verify
   by temporarily reverting stage 1 before finishing.
4. `grep -rn "TODO\|FIXME\|Deprecated" --include='*.go' .` → only hits that
   predate this change.

## Out of scope

- `UpdateFields` and `Update`. `UpdateFields` already takes an explicit column
  list, and making `Update` skip zero values would silently make "clear this
  field" impossible — a different decision, not this one.
- Reading NULL back — phases A/B/C.
- Changing `model.Field` or its documentation beyond what phase E needs.

| Stage | Files | Action |
|---|---|---|
| 1 | `db.go` | `Create` skips zero-valued `OmitEmpty` non-PK columns; `isZeroValue` helper |
| 2 | `tests/omitempty_test.go` + the package's model definitions | consumer-shaped proof, regenerated with `ormc` |
