package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage/conformance"
	"webtyp.com/storage/mem"
)

func TestBuilderRoundTripAgainstMem(t *testing.T) {
	d := orm.New(mem.New())

	w := &conformance.Widget{Id: "w1", Name: "alpha", Qty: 3, Active: true}
	if err := d.Create(w); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var got conformance.Widget
	if err := d.Query(&got).Where("id").Eq("w1").ReadOne(); err != nil {
		t.Fatalf("ReadOne: %v", err)
	}
	if got.Name != "alpha" || got.Qty != 3 || !got.Active {
		t.Errorf("round-trip mismatch: got %+v", got)
	}

	if err := d.Update(&conformance.Widget{Name: "beta", Qty: 9, Active: false}, orm.Eq("id", "w1")); err != nil {
		t.Fatalf("Update: %v", err)
	}
	var updated conformance.Widget
	if err := d.Query(&updated).Where("id").Eq("w1").ReadOne(); err != nil {
		t.Fatalf("ReadOne after update: %v", err)
	}
	if updated.Name != "beta" || updated.Qty != 9 || updated.Active {
		t.Errorf("update mismatch: got %+v", updated)
	}

	if err := d.Delete(&conformance.Widget{}, orm.Eq("id", "w1")); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	err := d.Query(&conformance.Widget{}).Where("id").Eq("w1").ReadOne()
	if err == nil {
		t.Fatal("expected ErrNotFound after delete")
	}
	if !orm.IsNotFound(err) {
		t.Errorf("expected orm.IsNotFound(err) to be true, got %v", err)
	}

	// ReadAll + Where + OrderBy + Limit, to cover the other half of the builder.
	_ = d.Create(&conformance.Widget{Id: "a", Name: "x", Qty: 1, Active: true})
	_ = d.Create(&conformance.Widget{Id: "b", Name: "x", Qty: 2, Active: true})
	var all []*conformance.Widget
	err = d.Query(&conformance.Widget{}).Where("name").Eq("x").OrderBy("qty").Desc().Limit(1).
		ReadAll(
			func() model.Model { return &conformance.Widget{} },
			func(m model.Model) {
				all = append(all, m.(*conformance.Widget))
			},
		)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 1 || all[0].Id != "b" {
		t.Errorf("expected only b (qty desc, limit 1); got %+v", all)
	}
}
