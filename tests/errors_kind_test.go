package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/orm"
)

// A missing row is a "not found" refusal: rpc.WriteError answers it 404
// without each module translating orm.IsNotFound by hand.
func TestErrNotFound_KindIsNotFound(t *testing.T) {
	if got := model.KindOf(orm.ErrNotFound); got != model.ErrorNotFound {
		t.Fatalf("model.KindOf(orm.ErrNotFound) = %v, want %v", got, model.ErrorNotFound)
	}
}
