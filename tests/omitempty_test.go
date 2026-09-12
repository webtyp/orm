package tests

import (
	"testing"

	"webtyp.com/orm"
	"webtyp.com/storage/mem"
	"webtyp.com/storage/mock"
)

func containsColumn(cols []string, name string) bool {
	for _, c := range cols {
		if c == name {
			return true
		}
	}
	return false
}

func TestOmitEmptyOmitsZeroColumn(t *testing.T) {
	compiler := &mock.Compiler{}
	exec := &mock.Executor{}
	db := orm.New(mockConn{Executor: exec, Compiler: compiler})

	empty := &OmitEmptyDemo{Id: "1", Nickname: "nick"}
	if err := db.Create(empty); err != nil {
		t.Fatalf("Create empty email: %v", err)
	}
	if containsColumn(compiler.LastQuery.Columns, "email") {
		t.Errorf("empty email must be omitted from INSERT, got columns %v", compiler.LastQuery.Columns)
	}
	if !containsColumn(compiler.LastQuery.Columns, "id") {
		t.Errorf("id must always be present, got %v", compiler.LastQuery.Columns)
	}
	if !containsColumn(compiler.LastQuery.Columns, "nickname") {
		t.Errorf("non-OmitEmpty empty nickname must still be written, got %v", compiler.LastQuery.Columns)
	}
	if len(compiler.LastQuery.Columns) != len(compiler.LastQuery.Values) {
		t.Errorf("columns/values length mismatch: %v vs %v", compiler.LastQuery.Columns, compiler.LastQuery.Values)
	}

	full := &OmitEmptyDemo{Id: "2", Email: "a@x.com", Nickname: "nick"}
	if err := db.Create(full); err != nil {
		t.Fatalf("Create real email: %v", err)
	}
	if !containsColumn(compiler.LastQuery.Columns, "email") {
		t.Errorf("real email must be present in INSERT, got %v", compiler.LastQuery.Columns)
	}
}

func TestOmitEmptyMemRoundTrip(t *testing.T) {
	db := orm.New(mem.New())

	first := &OmitEmptyDemo{Id: "1", Nickname: "one"}
	if err := db.Create(first); err != nil {
		t.Fatalf("first empty email rejected: %v", err)
	}
	second := &OmitEmptyDemo{Id: "2", Nickname: "two"}
	if err := db.Create(second); err != nil {
		t.Fatalf("second empty email rejected (column must be omitted, NULL repeats): %v", err)
	}

	for _, id := range []string{"1", "2"} {
		var got OmitEmptyDemo
		if err := db.Query(&got).Where("id").Eq(id).ReadOne(); err != nil {
			t.Fatalf("empty email user %s unreadable: %v", id, err)
		}
		if got.Email != "" {
			t.Errorf("user %s email = %q, want empty", id, got.Email)
		}
	}

	real := &OmitEmptyDemo{Id: "3", Email: "taken@test.com", Nickname: "three"}
	if err := db.Create(real); err != nil {
		t.Fatalf("real email rejected: %v", err)
	}
	var stored OmitEmptyDemo
	if err := db.Query(&stored).Where("id").Eq("3").ReadOne(); err != nil {
		t.Fatalf("real email unreadable: %v", err)
	}
	if stored.Email != "taken@test.com" {
		t.Errorf("real email round-trip = %q, want %q", stored.Email, "taken@test.com")
	}

	bare := &OmitEmptyDemo{Id: "4", Nickname: ""}
	if err := db.Create(bare); err != nil {
		t.Fatalf("bare nickname rejected: %v", err)
	}
	var bareGot OmitEmptyDemo
	if err := db.Query(&bareGot).Where("id").Eq("4").ReadOne(); err != nil {
		t.Fatalf("bare nickname unreadable: %v", err)
	}
	if bareGot.Nickname != "" {
		t.Errorf("non-OmitEmpty nickname = %q, want empty string (still written)", bareGot.Nickname)
	}
}
