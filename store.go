package orm

import (
	"webtyp.com/crud"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

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

var (
	_ crud.Store         = (*Store)(nil)
	_ crud.Transactional = (*Store)(nil)
	_ crud.ModelChecker  = (*Store)(nil)
)

// CheckModel validates that the model matches the store type (tenant vs global).
func (s *Store) CheckModel(m model.Model) error {
	idx := tenantIndex(m)
	if s.tenant != "" && idx < 0 {
		return ErrNoTenantField
	}
	if s.tenant == "" && idx >= 0 {
		return ErrTenantOnGlobal
	}
	return nil
}

// NewID mints a new primary-key value using the configured IDGenerator.
func (s *Store) NewID() string {
	if s.ids == nil {
		return ""
	}
	return s.ids.NewID()
}

// Atomic runs fn inside a transaction.
func (s *Store) Atomic(fn func(tx crud.Tx) error) error {
	return s.db.Tx(func(tx *DB) error {
		return fn(&Store{db: tx, tenant: s.tenant, ids: s.ids})
	})
}

func (s *Store) scope(conds []storage.Condition) ([]storage.Condition, error) {
	for _, c := range conds {
		if c.Logic() == "OR" {
			return nil, crud.ErrOrCondition
		}
	}
	if s.tenant != "" {
		out := make([]storage.Condition, 0, len(conds)+1)
		out = append(out, storage.Eq(FieldTenant, s.tenant))
		out = append(out, conds...)
		return out, nil
	}
	return conds, nil
}

func tenantIndex(m model.Model) int {
	for i, f := range m.Schema() {
		if f.Name == FieldTenant {
			return i
		}
	}
	return -1
}

func pkFields(m model.Model) ([]string, []int) {
	var names []string
	var idxs []int
	for i, f := range m.Schema() {
		if f.IsPK() {
			names = append(names, f.Name)
			idxs = append(idxs, i)
		}
	}
	return names, idxs
}

func keyConds(m model.Model, id string) ([]storage.Condition, error) {
	pks, idxs := pkFields(m)
	if len(pks) == 0 {
		return nil, ErrNotFound
	}
	parts, ok := model.SplitKey(id, len(pks))
	if !ok {
		return nil, ErrNotFound
	}
	ptrs := m.Pointers()
	conds := make([]storage.Condition, len(pks))
	for i, name := range pks {
		var val any = parts[i]
		switch ptrs[idxs[i]].(type) {
		case *int64:
			n, err := fmt.Convert(parts[i]).Int64()
			if err != nil {
				return nil, ErrNotFound
			}
			val = n
		case *int:
			n, err := fmt.Convert(parts[i]).Int()
			if err != nil {
				return nil, ErrNotFound
			}
			val = n
		}
		conds[i] = storage.Eq(name, val)
	}
	return conds, nil
}

// ReadOne reads a single record matching the primary key id and tenant scope.
func (s *Store) ReadOne(rec model.Model, id string) error {
	conds, err := keyConds(rec, id)
	if err != nil {
		return err
	}
	scoped, err := s.scope(conds)
	if err != nil {
		return err
	}
	qb := s.db.Query(rec)
	for _, c := range scoped {
		qb.addCondition(c)
	}
	return qb.ReadOne()
}

// ReadAll streams rows matching query q and tenant scope into onRow.
func (s *Store) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	sample := newRow()
	pks, idxs := pkFields(sample)
	if len(pks) > 1 {
		if q.After != "" {
			return crud.ErrAfterCompositeKey
		}
	} else if len(pks) == 1 {
		if q.After != "" {
			if len(q.Order) > 0 {
				return crud.ErrAfterWithOrder
			}
		}
	}

	scoped, err := s.scope(q.Conditions)
	if err != nil {
		return err
	}

	qb := s.db.Query(sample)
	for _, c := range scoped {
		qb.addCondition(c)
	}

	if len(pks) == 1 && q.After != "" {
		var afterVal any = q.After
		switch sample.Pointers()[idxs[0]].(type) {
		case *int64:
			if n, err := fmt.Convert(q.After).Int64(); err == nil {
				afterVal = n
			}
		case *int:
			if n, err := fmt.Convert(q.After).Int(); err == nil {
				afterVal = n
			}
		}
		qb.addCondition(storage.Gt(pks[0], afterVal))
	}

	if len(q.Order) > 0 {
		for _, o := range q.Order {
			if o.Dir() == "DESC" {
				qb.OrderBy(o.Column()).Desc()
			} else {
				qb.OrderBy(o.Column()).Asc()
			}
		}
	} else {
		for _, pk := range pks {
			qb.OrderBy(pk).Asc()
		}
	}

	if q.Limit > 0 {
		qb.Limit(int(q.Limit))
	}

	return qb.ReadAll(newRow, onRow)
}

// Create inserts a new record, stamping the tenant_id when on a TenantStore.
func (s *Store) Create(rec model.Model) error {
	if s.tenant != "" {
		idx := tenantIndex(rec)
		if idx < 0 {
			return ErrNoTenantField
		}
		*rec.Pointers()[idx].(*string) = s.tenant
	}
	return s.db.Create(rec)
}

// UpdateFields updates only the named fields for the record matching id and tenant scope.
func (s *Store) UpdateFields(rec model.Model, id string, fields []string) error {
	for _, f := range fields {
		if f == FieldTenant {
			return crud.ErrServerOnlyField
		}
	}
	conds, err := keyConds(rec, id)
	if err != nil {
		return err
	}
	scoped, err := s.scope(conds)
	if err != nil {
		return err
	}
	return s.db.UpdateFields(rec, fields, scoped[0], scoped[1:]...)
}

// Delete removes the record matching id and tenant scope.
func (s *Store) Delete(rec model.Model, id string) error {
	conds, err := keyConds(rec, id)
	if err != nil {
		return err
	}
	scoped, err := s.scope(conds)
	if err != nil {
		return err
	}
	return s.db.Delete(rec, scoped[0], scoped[1:]...)
}
