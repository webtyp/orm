---
PLAN: "feat: orm.TenantStore and orm.GlobalStore implement crud.Store — the tenant is stamped and filtered on every call"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally** and is **published as soon as the delivery check passes** (no `replace` anywhere).
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: `orm` implements `crud.Store`

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/crud@latest webtyp.com/model@latest   # model must contain model.SplitKey (A6.1)
```
`crud@latest` must contain `crud.Store`, `crud.Tx`, `crud.Query`, `crud.ModelChecker`, `crud/storetest` (plan A6.2); if
`go doc webtyp.com/crud Store` fails, stop. `orm` imports `crud` (the contract); `crud` never imports `orm`.

## Why

Decision D20 of `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md`: `crud` persists tables through a `crud.Store`. The tenant must be
impossible to forget: a module passes `orm.TenantStore(m.db, m.tenantID, m.deps.IDs)` once, and every read and write of
that table carries `tenant_id` — stamped on insert, filtered on read, update and delete.

## APIs used

| Symbol | Signature |
|---|---|
| `crud.Tx` | `ReadOne(rec model.Model, id string) error`; `ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error`; `Create(rec model.Model) error`; `UpdateFields(rec model.Model, id string, fields []string) error`; `Delete(rec model.Model, id string) error` |
| `crud.Store` | `Tx` + `NewID() string` |
| `crud.Transactional` | `Atomic(fn func(tx crud.Tx) error) error` (optional capability; orm's stores implement it) |
| `crud.Query` | `struct{ Conditions []storage.Condition; Order []storage.Order; After string; Limit int64 }` |
| `crud.ModelChecker` | `CheckModel(m model.Model) error` |
| `crud.ErrOrCondition` | `model.Invalid` |
| `storetest.Run` | `func Run(t *testing.T, newStore func(t *testing.T) crud.Store)` (`webtyp.com/crud/storetest`) |

## Stage 1 — `store.go` (new)

```go
// FieldTenant is the column a TenantStore stamps and filters.
const FieldTenant = "tenant_id"

var (
	ErrNoTenantField  = fmt.Err("orm: TenantStore on a table without a tenant_id field; add the field or use orm.GlobalStore")
	ErrTenantOnGlobal = fmt.Err("orm: GlobalStore on a table with a tenant_id field; use orm.TenantStore(db, tenantID, ids)")
	ErrEmptyTenant    = fmt.Err("orm: TenantStore with an empty tenant id; pass the installation's tenant from Deps")
)

// TenantStore is the crud.Store of one tenant's rows: every insert gets tenant_id = tenantID and every read,
// update and delete adds tenant_id = tenantID. An empty tenantID panics at construction (a wiring mistake).
//
//	store := orm.TenantStore(m.db, m.tenantID, m.deps.IDs)
func TenantStore(db *DB, tenantID string, ids model.IDGenerator) *Store {
	if tenantID == "" {
		panic(ErrEmptyTenant)
	}
	return &Store{db: db, tenant: tenantID, ids: ids}
}

// GlobalStore is the crud.Store of a table without tenants (a catalog shared by every installation).
//
//	store := orm.GlobalStore(m.db, m.deps.IDs)
func GlobalStore(db *DB, ids model.IDGenerator) *Store { return &Store{db: db, ids: ids} }

// Store is orm's crud.Store: it also implements crud.Transactional (Atomic) and crud.ModelChecker.
// A module keeps the concrete *orm.Store when it needs Atomic outside crud (readers, imports).
type Store struct {
	db     *DB
	tenant string // "" = global
	ids    model.IDGenerator
}
```

Composite keys (D22): `keyConds(m, id) ([]storage.Condition, error)` = one `storage.Eq(<pk field>, part)` per primary-key field in schema order, from `model.SplitKey(id, <number of pk fields>)` (a single key is one part); not ok → an error of kind `model.ErrorNotFound`. `ReadOne`, `UpdateFields` and `Delete` use `keyConds` instead of `Eq(pk, id)`. `ReadAll` on a composite key: `q.After != ""` → `crud.ErrAfterCompositeKey`; default order = every pk column ascending.

Methods (all on `*Store`; `pk(m)` = name of the field with `IsPK()`; `tenantIdx(m)` = index of `FieldTenant` in `m.Schema()` or -1):

- `CheckModel(m)`: tenant store and `tenantIdx(m) < 0` → `ErrNoTenantField`; global store and `tenantIdx(m) >= 0` → `ErrTenantOnGlobal`.
- `scope(conds)`: rejects any condition with `Logic() == "OR"` (`crud.ErrOrCondition`); returns `conds` with
  `storage.Eq(FieldTenant, s.tenant)` **first** when tenant store.
- `ReadOne(rec, id)`: `s.db.Query(rec)` with the scoped conditions `[Eq(pk, id)]` (use the internal `addCondition`), `ReadOne()`.
- `ReadAll(newRow, q, onRow)`: `q.After != ""` with `len(q.Order) > 0` → `crud.ErrAfterWithOrder`; `s.db.Query(newRow())`, scoped `q.Conditions`, `Gt(pk, q.After)` when `q.After != ""`,
  each `q.Order` applied in order (or `OrderBy(pk).Asc()` when empty), `Limit(int(q.Limit))` when > 0, `ReadAll(newRow, onRow)`.
  If the query builder has no way to take a `storage.Order` value, add an unexported helper in this repo that maps it (same package).
- `Create(rec)`: tenant store → set `*rec.Pointers()[tenantIdx].(*string) = s.tenant` (overwrites whatever came); `s.db.Create(rec)`.
- `UpdateFields(rec, id, fields)`: `fields` containing `FieldTenant` → `crud.ErrServerOnlyField`; `s.db.UpdateFields(rec, fields, scoped Eq(pk,id)...)`.
- `Delete(rec, id)`: `s.db.Delete(rec, scoped Eq(pk, id)...)`.
- `Atomic(fn)`: `s.db.Tx(func(tx *DB) error { return fn(&Store{db: tx, tenant: s.tenant, ids: s.ids}) })`.
- `NewID()`: `s.ids.NewID()`.

`var _ crud.Store = (*Store)(nil)`, `var _ crud.Transactional = (*Store)(nil)` and `var _ crud.ModelChecker = (*Store)(nil)`.

## Stage 2 — tests (`tests/store_test.go`, rows in `docs/TESTS.md`), real storage `storage/mem`

| Test | Setup → expected |
|---|---|
| `TestStore_Conformance` | `storetest.Run(t, func(t *testing.T) crud.Store { return orm.GlobalStore(orm.New(mem.New()), ids) })` passes (migrate the storetest model first if `mem` needs it) |
| `TestTenantStore_ReadsOnlyItsTenant` | rows of tenants a and b; `TenantStore(db,"a",ids).ReadAll` → only a's |
| `TestTenantStore_ReadOneOtherTenantIsNotFound` | row of b; store a `ReadOne(id)` → kind `model.ErrorNotFound` |
| `TestTenantStore_CreateOverwritesTenant` | create with `tenant_id:"b"` through store a → stored `tenant_id == "a"` |
| `TestTenantStore_UpdateOtherTenantChangesNothing` | store a updates b's id → b's row unchanged |
| `TestStore_CompositeKey` | table PK (room_id, equipment_id); Create r1/e7; `ReadOne(rec, "r1/e7")` → found; `Delete(rec, "r1/e7")` → gone; `ReadOne(rec, "r1")` → not found |
| `TestStore_ReadAllOrdered` | rows with expires_at 3,1,2; `Order: [storage.Asc("expires_at")]` → 1,2,3; with `After` too → `crud.ErrAfterWithOrder` |
| `TestTenantStore_OrConditionRejected` | `ReadAll` with `storage.Or(storage.Eq("name","x"))` → `crud.ErrOrCondition` |
| `TestStore_CheckModel` | tenant store + model without tenant_id → `ErrNoTenantField`; global + with → `ErrTenantOnGlobal` |

## Stage 3 — documentation

`README.md`: «Persist a table through crud» with `orm.TenantStore(m.db, m.tenantID, m.deps.IDs)`. `docs/ARQUITECTURE.md`: tenant
scoping and why OR is rejected (conditions are not grouped).

## Delivery check

`gotest`. A tree that does not compile or whose tests did not run is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `store.go`, `go.mod` |
| 2 | `tests/store_test.go`, `docs/TESTS.md` |
| 3 | `README.md`, `docs/ARQUITECTURE.md` |
