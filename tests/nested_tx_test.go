package tests

import (
	"errors"
	"testing"

	"webtyp.com/orm"
	"webtyp.com/storage/mem"
)

// A module method that opens its own transaction (appointment_booking.SaveDayBlocks) is also called by code
// already inside one (a data import runs every step in one transaction). The inner Tx joins the outer one: it
// runs on the same transaction, and the outer one decides commit or rollback.
func TestTx_NestedJoinsTheOuterTransaction(t *testing.T) {
	db := orm.New(mem.New())
	ran := false
	err := db.Tx(func(outer *orm.DB) error {
		return outer.Tx(func(inner *orm.DB) error {
			ran = true
			return nil
		})
	})
	if err != nil || !ran {
		t.Fatalf("nested Tx: ran=%v err=%v", ran, err)
	}

	boom := errors.New("inner failed")
	err = db.Tx(func(outer *orm.DB) error {
		return outer.Tx(func(inner *orm.DB) error { return boom })
	})
	if !errors.Is(err, boom) {
		t.Errorf("an inner error reaches the outer transaction: got %v", err)
	}
}
