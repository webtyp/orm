package orm

import (
	"webtyp.com/model"
	"webtyp.com/storage"
)

// QB represents a query builder.
// Consumers hold a *QB reference in variables for incremental building.
type QB struct {
	db      *DB
	model   model.Model
	conds   []storage.Condition
	orderBy []storage.Order
	groupBy []string
	limit   int
	offset  int
	nextOr  bool
}

// Clause represents an intermediate state for building a query condition.
type Clause struct {
	qb    *QB
	field string
}

// Where starts a new condition clause for the given column.
func (qb *QB) Where(column string) *Clause {
	return &Clause{qb: qb, field: column}
}

// Or sets the next condition to use OR logic instead of AND.
func (qb *QB) Or() *QB {
	qb.nextOr = true
	return qb
}

func (qb *QB) addCondition(c storage.Condition) *QB {
	if qb.nextOr {
		c = storage.Or(c)
		qb.nextOr = false
	}
	qb.conds = append(qb.conds, c)
	return qb
}

func (c *Clause) Eq(value any) *QB    { return c.qb.addCondition(storage.Eq(c.field, value)) }
func (c *Clause) Neq(value any) *QB   { return c.qb.addCondition(storage.Neq(c.field, value)) }
func (c *Clause) Gt(value any) *QB    { return c.qb.addCondition(storage.Gt(c.field, value)) }
func (c *Clause) Gte(value any) *QB   { return c.qb.addCondition(storage.Gte(c.field, value)) }
func (c *Clause) Lt(value any) *QB    { return c.qb.addCondition(storage.Lt(c.field, value)) }
func (c *Clause) Lte(value any) *QB   { return c.qb.addCondition(storage.Lte(c.field, value)) }
func (c *Clause) Like(value any) *QB  { return c.qb.addCondition(storage.Like(c.field, value)) }
func (c *Clause) In(value any) *QB    { return c.qb.addCondition(storage.In(c.field, value)) }

// Limit sets the limit for the query.
func (qb *QB) Limit(limit int) *QB {
	qb.limit = limit
	return qb
}

// Offset sets the offset for the query.
func (qb *QB) Offset(offset int) *QB {
	qb.offset = offset
	return qb
}

// OrderClause represents an intermediate state for building an order by clause.
type OrderClause struct {
	qb    *QB
	field string
}

// OrderBy starts a new order clause for the given column.
func (qb *QB) OrderBy(column string) *OrderClause {
	return &OrderClause{qb: qb, field: column}
}

// Asc sets the order direction to ascending.
func (o *OrderClause) Asc() *QB {
	o.qb.orderBy = append(o.qb.orderBy, storage.Asc(o.field))
	return o.qb
}

// Desc sets the order direction to descending.
func (o *OrderClause) Desc() *QB {
	o.qb.orderBy = append(o.qb.orderBy, storage.Desc(o.field))
	return o.qb
}

// GroupBy adds a group by clause to the query.
func (qb *QB) GroupBy(columns ...string) *QB {
	qb.groupBy = append(qb.groupBy, columns...)
	return qb
}

// ReadOne executes the query and returns a single result.
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

// ReadAll executes the query and returns all results.
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
