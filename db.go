package orm

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

// DB represents an ergonomic handle over a storage backend (a storage.Conn). Consumers instantiate
// it via New(). This type owns no contract — storage.Conn is the contract; DB is the fluent layer
// on top of it (see docs/ARQUITECTURE.md).
type DB struct {
	conn storage.Conn
	log  func(messages ...any)
}

// New wraps a storage.Conn (a backend's Executor+Compiler pair, e.g. sqlt.Open(dsn) or mem.New())
// in the ergonomic DB handle. One argument, not two: storage.Conn already unifies Executor+Compiler
// so an Executor from one backend can never be paired with a Compiler from another.
func New(conn storage.Conn) *DB {
	return &DB{conn: conn}
}

// SetLog sets the log function for warnings and informational messages.
// If not set, messages are silently discarded.
func (d *DB) SetLog(fn func(messages ...any)) {
	d.log = fn
}

func (d *DB) logw(messages ...any) {
	if d.log != nil {
		d.log(messages...)
	}
}

// Create inserts a new model into the database.
func (d *DB) Create(m model.Model) error {
	if err := validateQuery(storage.ActionCreate, m); err != nil {
		return err
	}
	schema := m.Schema()
	ptrs := m.Pointers()
	allValues := model.ReadValues(schema, ptrs)
	var columns []string
	var values []any
	for i, f := range schema {
		// Skip autoincrement PK fields with zero value — let the DB assign them.
		if f.IsPK() && f.IsAutoInc() {
			if v, ok := allValues[i].(int); ok && v == 0 {
				continue
			}
		}
		// A zero value on an OmitEmpty field is left out of the INSERT entirely, so the
		// database applies its own default — NULL for a nullable column. That is the
		// only way to store NULL in a UNIQUE column, where "" would collide and NULL is
		// the value a unique index allows to repeat.
		if f.OmitEmpty && !f.IsPK() && isZeroValue(allValues[i]) {
			continue
		}
		columns = append(columns, f.Name)
		values = append(values, allValues[i])
	}
	q := storage.Query{
		Action:  storage.ActionCreate,
		Table:   m.ModelName(),
		Columns: columns,
		Values:  values,
	}
	plan, err := d.conn.Compile(q, m)
	if err != nil {
		return err
	}
	return d.conn.Exec(plan.Query, plan.Args...)
}

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

// UpdateFields updates ONLY the named columns, leaving every other column of
// the matched rows untouched. It is the PATCH counterpart of Update, which
// writes the whole schema and therefore overwrites columns the caller never
// meant to touch — a lost update whenever anyone else changed one of them in
// the meantime.
//
// fields holds Schema() field names. Order is irrelevant; duplicates are
// rejected. An empty fields slice is an error, not a silent no-op: a caller
// that computed an empty change set has a bug, and a no-op UPDATE would hide
// it.
//
// At least one Condition is required, same as Update — there is no variadic
// fallback, which is what makes an accidental whole-table UPDATE a
// compile-time error rather than a production incident.
func (d *DB) UpdateFields(m model.Model, fields []string, cond storage.Condition, rest ...storage.Condition) error {
	if err := validateQuery(storage.ActionUpdate, m); err != nil {
		return err
	}
	if len(fields) == 0 {
		return fmt.Err("orm: UpdateFields requires at least one field")
	}

	schema := m.Schema()

	// Validate the caller's field list first, in one pass per name: every name
	// must exist in the schema and must not repeat. Nested loops rather than a
	// set — a map would pull TinyGo's hashing machinery in, and a schema holds
	// tens of fields, so the scan is free.
	for i, name := range fields {
		known := false
		for _, f := range schema {
			if f.Name == name {
				known = true
				break
			}
		}
		if !known {
			return fmt.Errf("orm: UpdateFields: unknown field %q", name)
		}
		for _, prev := range fields[:i] {
			if prev == name {
				return fmt.Errf("orm: UpdateFields: duplicate field %q", name)
			}
		}
	}

	// Build in SCHEMA order, not in the caller's order, so columns and values
	// line up with what Update would have produced for the same row.
	allValues := model.ReadValues(schema, m.Pointers())
	columns := make([]string, 0, len(fields))
	values := make([]any, 0, len(fields))
	for i, f := range schema {
		for _, name := range fields {
			if f.Name == name {
				columns = append(columns, f.Name)
				values = append(values, allValues[i])
				break
			}
		}
	}

	conds := append([]storage.Condition{cond}, rest...)
	q := storage.Query{
		Action:     storage.ActionUpdate,
		Table:      m.ModelName(),
		Columns:    columns,
		Values:     values,
		Conditions: conds,
	}
	plan, err := d.conn.Compile(q, m)
	if err != nil {
		return err
	}
	return d.conn.Exec(plan.Query, plan.Args...)
}

// Update modifies an existing row. At least one Condition is required.
// Providing zero conditions is a compile-time error — there is no variadic
// fallback — preventing accidental full-table UPDATE statements.
func (d *DB) Update(m model.Model, cond storage.Condition, rest ...storage.Condition) error {
	if err := validateQuery(storage.ActionUpdate, m); err != nil {
		return err
	}
	conds := append([]storage.Condition{cond}, rest...)
	schema := m.Schema()
	columns := make([]string, len(schema))
	for i, f := range schema {
		columns[i] = f.Name
	}
	q := storage.Query{
		Action:     storage.ActionUpdate,
		Table:      m.ModelName(),
		Columns:    columns,
		Values:     model.ReadValues(schema, m.Pointers()),
		Conditions: conds,
	}
	plan, err := d.conn.Compile(q, m)
	if err != nil {
		return err
	}
	return d.conn.Exec(plan.Query, plan.Args...)
}

// Delete deletes a model from the database.
// At least one Condition is required. Providing zero conditions is a compile-time
// error, preventing accidental full-table DELETE statements.
func (d *DB) Delete(m model.Model, cond storage.Condition, rest ...storage.Condition) error {
	if err := validateQuery(storage.ActionDelete, m); err != nil {
		return err
	}
	conds := append([]storage.Condition{cond}, rest...)
	q := storage.Query{
		Action:     storage.ActionDelete,
		Table:      m.ModelName(),
		Conditions: conds,
	}
	plan, err := d.conn.Compile(q, m)
	if err != nil {
		return err
	}
	return d.conn.Exec(plan.Query, plan.Args...)
}

// Query creates a new QB instance.
func (d *DB) Query(m model.Model) *QB {
	return &QB{db: d, model: m}
}

// Close closes the underlying connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// RawConn returns the underlying storage.Conn. Renamed from RawExecutor: what's underneath is a
// full Conn (Executor+Compiler), not just an Executor.
func (d *DB) RawConn() storage.Conn {
	return d.conn
}
