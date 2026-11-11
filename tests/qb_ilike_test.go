package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage/conformance"
	"webtyp.com/storage/mem"
)

func TestQB_ILikeIgnoresCase(t *testing.T) {
	db := orm.New(mem.New())

	// Insert rows
	widgets := []conformance.Widget{
		{Id: "1", Name: "Ana"},
		{Id: "2", Name: "ana maría"},
		{Id: "3", Name: "Bob"},
	}

	for _, w := range widgets {
		if err := db.Create(&w); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// Query using ILike
	var results []conformance.Widget
	if err := db.Query(&conformance.Widget{}).Where("name").ILike("%AN%").ReadAll(
		func() model.Model { return &conformance.Widget{} },
		func(m model.Model) {
			results = append(results, *m.(*conformance.Widget))
		},
	); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].Name != "Ana" || results[1].Name != "ana maría" {
		t.Errorf("unexpected results: %+v", results)
	}
}
