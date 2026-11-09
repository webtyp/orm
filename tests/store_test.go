package tests

import (
	"testing"

	"webtyp.com/crud"
	"webtyp.com/crud/conformance"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"
)

type mockIDGen struct {
	next string
}

func (g mockIDGen) NewID() string {
	if g.next == "" {
		return "gen-id"
	}
	return g.next
}

type TenantItem struct {
	ID        string
	TenantID  string
	Name      string
	ExpiresAt int64
}

func (m *TenantItem) IsNil() bool                   { return m == nil }
func (m *TenantItem) EncodeFields(w model.FieldWriter) {}
func (m *TenantItem) DecodeFields(r model.FieldReader) {}
func (m *TenantItem) Pointers() []any {
	return []any{&m.ID, &m.TenantID, &m.Name, &m.ExpiresAt}
}
func (m *TenantItem) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: orm.FieldTenant, Type: model.Text()},
		{Name: "name", Type: model.Text()},
		{Name: "expires_at", Type: model.Int()},
	}
}
func (m *TenantItem) ModelName() string   { return "tenant_items" }
func (m *TenantItem) Allow() model.Action { return model.AllActions }

type CompositeItem struct {
	RoomID      string
	EquipmentID string
	Name        string
}

func (m *CompositeItem) IsNil() bool                   { return m == nil }
func (m *CompositeItem) EncodeFields(w model.FieldWriter) {}
func (m *CompositeItem) DecodeFields(r model.FieldReader) {}
func (m *CompositeItem) Pointers() []any {
	return []any{&m.RoomID, &m.EquipmentID, &m.Name}
}
func (m *CompositeItem) Schema() []model.Field {
	return []model.Field{
		{Name: "room_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "equipment_id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
	}
}
func (m *CompositeItem) ModelName() string   { return "composite_items" }
func (m *CompositeItem) Allow() model.Action { return model.AllActions }

func TestStore_Conformance(t *testing.T) {
	ids := mockIDGen{next: "id-1"}
	conformance.Run(t, func(t *testing.T) crud.Store {
		m := mem.New()
		db := orm.New(m)
		dummy := &conformance.TestModel{ID: "__init__"}
		_ = db.Create(dummy)
		_ = db.Delete(dummy, storage.Eq("id", "__init__"))
		return orm.GlobalStore(db, ids)
	})
}

func TestTenantStore_ReadsOnlyItsTenant(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	storeA := orm.TenantStore(db, "a", ids)
	storeB := orm.TenantStore(db, "b", ids)

	if err := storeA.Create(&TenantItem{ID: "1", Name: "Item A"}); err != nil {
		t.Fatalf("create A: %v", err)
	}
	if err := storeB.Create(&TenantItem{ID: "2", Name: "Item B"}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	var gotA []string
	newRow := func() model.Model { return &TenantItem{} }
	err := storeA.ReadAll(newRow, crud.Query{}, func(m model.Model) {
		gotA = append(gotA, m.(*TenantItem).ID)
	})
	if err != nil {
		t.Fatalf("ReadAll A: %v", err)
	}
	if len(gotA) != 1 || gotA[0] != "1" {
		t.Fatalf("expected [1], got %v", gotA)
	}

	var gotB []string
	err = storeB.ReadAll(newRow, crud.Query{}, func(m model.Model) {
		gotB = append(gotB, m.(*TenantItem).ID)
	})
	if err != nil {
		t.Fatalf("ReadAll B: %v", err)
	}
	if len(gotB) != 1 || gotB[0] != "2" {
		t.Fatalf("expected [2], got %v", gotB)
	}
}

func TestTenantStore_ReadOneOtherTenantIsNotFound(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	storeA := orm.TenantStore(db, "a", ids)
	storeB := orm.TenantStore(db, "b", ids)

	if err := storeB.Create(&TenantItem{ID: "b1", Name: "B's item"}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	var rec TenantItem
	err := storeA.ReadOne(&rec, "b1")
	if err == nil {
		t.Fatal("expected error reading other tenant's item, got nil")
	}
	if model.KindOf(err) != model.ErrorNotFound {
		t.Fatalf("expected ErrorNotFound kind, got %v", err)
	}
	if !orm.IsNotFound(err) {
		t.Fatalf("expected orm.IsNotFound(err) to be true, got %v", err)
	}
}

func TestTenantStore_CreateOverwritesTenant(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	storeA := orm.TenantStore(db, "a", ids)

	item := &TenantItem{ID: "x1", TenantID: "b", Name: "Forged"}
	if err := storeA.Create(item); err != nil {
		t.Fatalf("create: %v", err)
	}

	var stored TenantItem
	if err := storeA.ReadOne(&stored, "x1"); err != nil {
		t.Fatalf("read: %v", err)
	}
	if stored.TenantID != "a" {
		t.Fatalf("expected tenant_id 'a', got %q", stored.TenantID)
	}
}

func TestTenantStore_UpdateOtherTenantChangesNothing(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	storeA := orm.TenantStore(db, "a", ids)
	storeB := orm.TenantStore(db, "b", ids)

	if err := storeB.Create(&TenantItem{ID: "b2", Name: "Original"}); err != nil {
		t.Fatalf("create B: %v", err)
	}

	// storeA attempts to update b's row
	update := &TenantItem{ID: "b2", Name: "Hacked"}
	if err := storeA.UpdateFields(update, "b2", []string{"name"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	var storedB TenantItem
	if err := storeB.ReadOne(&storedB, "b2"); err != nil {
		t.Fatalf("read B: %v", err)
	}
	if storedB.Name != "Original" {
		t.Fatalf("expected name 'Original' unchanged, got %q", storedB.Name)
	}
}

func TestStore_CompositeKey(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	store := orm.GlobalStore(db, ids)

	item := &CompositeItem{RoomID: "r1", EquipmentID: "e7", Name: "Laser"}
	if err := store.Create(item); err != nil {
		t.Fatalf("create: %v", err)
	}

	var found CompositeItem
	if err := store.ReadOne(&found, "r1/e7"); err != nil {
		t.Fatalf("read r1/e7: %v", err)
	}
	if found.Name != "Laser" {
		t.Fatalf("expected 'Laser', got %q", found.Name)
	}

	var missing CompositeItem
	err := store.ReadOne(&missing, "r1")
	if err == nil {
		t.Fatal("expected error reading partial composite key 'r1', got nil")
	}
	if model.KindOf(err) != model.ErrorNotFound {
		t.Fatalf("expected ErrorNotFound for 'r1', got %v", err)
	}

	// ReadAll with After on composite key is rejected
	newRow := func() model.Model { return &CompositeItem{} }
	err = store.ReadAll(newRow, crud.Query{After: "r1/e7"}, func(m model.Model) {})
	if err != crud.ErrAfterCompositeKey {
		t.Fatalf("expected ErrAfterCompositeKey, got %v", err)
	}

	if err := store.Delete(&CompositeItem{}, "r1/e7"); err != nil {
		t.Fatalf("delete r1/e7: %v", err)
	}

	var deleted CompositeItem
	err = store.ReadOne(&deleted, "r1/e7")
	if err == nil {
		t.Fatal("expected ErrNotFound after delete, got nil")
	}
	if model.KindOf(err) != model.ErrorNotFound {
		t.Fatalf("expected ErrorNotFound, got %v", err)
	}
}

func TestStore_ReadAllOrdered(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	store := orm.GlobalStore(db, ids)

	if err := store.Create(&TenantItem{ID: "1", ExpiresAt: 3}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&TenantItem{ID: "2", ExpiresAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(&TenantItem{ID: "3", ExpiresAt: 2}); err != nil {
		t.Fatal(err)
	}

	var orderedIDs []string
	newRow := func() model.Model { return &TenantItem{} }
	q := crud.Query{Order: []storage.Order{storage.Asc("expires_at")}}
	err := store.ReadAll(newRow, q, func(m model.Model) {
		orderedIDs = append(orderedIDs, m.(*TenantItem).ID)
	})
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(orderedIDs) != 3 || orderedIDs[0] != "2" || orderedIDs[1] != "3" || orderedIDs[2] != "1" {
		t.Fatalf("expected [2 3 1], got %v", orderedIDs)
	}

	// After combined with Order must fail
	qAfterOrder := crud.Query{
		After: "2",
		Order: []storage.Order{storage.Asc("expires_at")},
	}
	err = store.ReadAll(newRow, qAfterOrder, func(m model.Model) {})
	if err != crud.ErrAfterWithOrder {
		t.Fatalf("expected ErrAfterWithOrder, got %v", err)
	}
}

func TestTenantStore_OrConditionRejected(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	store := orm.TenantStore(db, "a", ids)

	newRow := func() model.Model { return &TenantItem{} }
	q := crud.Query{
		Conditions: []storage.Condition{storage.Or(storage.Eq("name", "x"))},
	}
	err := store.ReadAll(newRow, q, func(m model.Model) {})
	if err != crud.ErrOrCondition {
		t.Fatalf("expected ErrOrCondition, got %v", err)
	}
}

func TestStore_CheckModel(t *testing.T) {
	db := orm.New(mem.New())
	ids := mockIDGen{}
	tenantStore := orm.TenantStore(db, "a", ids)
	globalStore := orm.GlobalStore(db, ids)

	tenantModel := &TenantItem{}
	globalModel := &conformance.TestModel{}

	if err := tenantStore.CheckModel(globalModel); err != orm.ErrNoTenantField {
		t.Fatalf("expected ErrNoTenantField, got %v", err)
	}
	if err := globalStore.CheckModel(tenantModel); err != orm.ErrTenantOnGlobal {
		t.Fatalf("expected ErrTenantOnGlobal, got %v", err)
	}

	if err := tenantStore.CheckModel(tenantModel); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := globalStore.CheckModel(globalModel); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestTenantStore_EmptyTenantPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on empty tenantID, got none")
		}
		if r != orm.ErrEmptyTenant {
			t.Fatalf("expected ErrEmptyTenant, got %v", r)
		}
	}()
	db := orm.New(mem.New())
	orm.TenantStore(db, "", mockIDGen{})
}
