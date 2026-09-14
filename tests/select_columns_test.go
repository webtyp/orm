package tests

import (
	"testing"

	mdl "webtyp.com/model"
	"webtyp.com/orm"
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
