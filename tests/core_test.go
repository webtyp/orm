package tests

import (
	"errors"
	"testing"

	mdl "webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage"
	"webtyp.com/storage/mock"
)

type mockConn struct {
	storage.Executor
	storage.Compiler
}

type mockTxConn struct {
	storage.TxExecutor
	storage.Compiler
}

func RunCoreTests(t *testing.T) {
	t.Run("IsNotFound", func(t *testing.T) {
		if !orm.IsNotFound(orm.ErrNotFound) {
			t.Error("IsNotFound(ErrNotFound) should be true")
		}
		if orm.IsNotFound(nil) {
			t.Error("IsNotFound(nil) should be false")
		}
		if orm.IsNotFound(storage.ErrNoRows) {
			t.Error("IsNotFound(storage.ErrNoRows) should be false")
		}
		if orm.ErrNotFound.Error() != "record not found" {
			t.Errorf("ErrNotFound.Error() == %q, want \"record not found\"", orm.ErrNotFound.Error())
		}
	})

	// 1. Test Create
	t.Run("Create", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})

		model := &mock.Model{
			Table: "user",
			Sch:   []mdl.Field{{Name: "name"}, {Name: "age"}},
			Vals:  []any{"Alice", 30},
		}

		err := db.Create(model)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if mockCompiler.LastQuery.Action != storage.ActionCreate {
			t.Errorf("Expected ActionCreate, got %v", mockCompiler.LastQuery.Action)
		}
		if mockCompiler.LastQuery.Table != "user" {
			t.Errorf("Expected table 'user', got '%s'", mockCompiler.LastQuery.Table)
		}
		if len(mockCompiler.LastQuery.Columns) != 2 {
			t.Errorf("Expected 2 columns, got %d", len(mockCompiler.LastQuery.Columns))
		}
	})

	// 2. Test Update with Conditions
	t.Run("Update", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})

		model := &mock.Model{
			Table: "user",
			Sch:   []mdl.Field{{Name: "age"}},
			Vals:  []any{31},
		}

		err := db.Update(model, orm.Eq("name", "Alice"))
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		if mockCompiler.LastQuery.Action != storage.ActionUpdate {
			t.Errorf("Expected ActionUpdate, got %v", mockCompiler.LastQuery.Action)
		}
		if len(mockCompiler.LastQuery.Conditions) != 1 {
			t.Errorf("Expected 1 condition, got %d", len(mockCompiler.LastQuery.Conditions))
		}
		if mockCompiler.LastQuery.Conditions[0].Field() != "name" {
			t.Errorf("Expected condition field 'name', got '%s'", mockCompiler.LastQuery.Conditions[0].Field())
		}
	})

	// 3. Test Delete
	t.Run("Delete", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})

		model := &mock.Model{Table: "user"}

		err := db.Delete(model, orm.Gt("age", 100))
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		if mockCompiler.LastQuery.Action != storage.ActionDelete {
			t.Errorf("Expected ActionDelete, got %v", mockCompiler.LastQuery.Action)
		}
		if len(mockCompiler.LastQuery.Conditions) != 1 {
			t.Errorf("Expected 1 condition, got %d", len(mockCompiler.LastQuery.Conditions))
		}
	})

	// 4. Test Query Chain (ReadOne)
	t.Run("ReadOne", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})

		model := &mock.Model{Table: "user"}

		// Setup mock.Executor to return a scanner that succeeds
		mockExec.ReturnQueryRow = &mock.Scanner{}

		err := db.Query(model).
			Where("id").Eq(1).
			OrderBy("created_at").Desc().
			ReadOne()

		if err != nil {
			t.Fatalf("ReadOne failed: %v", err)
		}

		if mockCompiler.LastQuery.Action != storage.ActionReadOne {
			t.Errorf("Expected ActionReadOne, got %v", mockCompiler.LastQuery.Action)
		}
		if mockCompiler.LastQuery.Limit != 1 {
			t.Errorf("Expected Limit 1, got %d", mockCompiler.LastQuery.Limit)
		}
		if len(mockCompiler.LastQuery.OrderBy) != 1 {
			t.Errorf("Expected 1 OrderBy, got %d", len(mockCompiler.LastQuery.OrderBy))
		}
	})

	// Test ReadOne Validation Error
	t.Run("ReadOne Validation Error", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})
		model := &mock.Model{Table: ""} // Empty table

		err := db.Query(model).ReadOne()
		if err == nil || err.Error() != orm.ErrEmptyTable.Error() {
			t.Errorf("Expected ErrEmptyTable, got %v", err)
		}
	})

	t.Run("ReadOne ErrNotFound", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{
			ReturnQueryRow: &mock.Scanner{ScanErr: storage.ErrNoRows},
		}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})
		model := &mock.Model{Table: "user"}
		err := db.Query(model).ReadOne()
		if !orm.IsNotFound(err) {
			t.Errorf("Expected IsNotFound(err) to be true, got err=%v", err)
		}
	})

	// 5. Test ReadAll
	t.Run("ReadAll", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})

		model := &mock.Model{Table: "user"}

		// Simulate 2 rows
		mockRows := &mock.Rows{Count: 2}
		mockExec.ReturnQueryRows = mockRows

		newCalled := 0
		onRowCalled := 0
		newFunc := func() mdl.Model {
			newCalled++
			return &mock.Model{}
		}
		onRow := func(m mdl.Model) {
			onRowCalled++
		}

		err := db.Query(model).ReadAll(newFunc, onRow)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if mockCompiler.LastQuery.Action != storage.ActionReadAll {
			t.Errorf("Expected ActionReadAll, got %v", mockCompiler.LastQuery.Action)
		}
		if newCalled != 2 {
			t.Errorf("Expected new called 2 times, got %d", newCalled)
		}
		if onRowCalled != 2 {
			t.Errorf("Expected onRow called 2 times, got %d", onRowCalled)
		}
	})

	// Test ReadAll Validation Error
	t.Run("ReadAll Validation Error", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})
		model := &mock.Model{Table: ""} // Empty table

		err := db.Query(model).ReadAll(nil, nil)
		if err == nil || err.Error() != orm.ErrEmptyTable.Error() {
			t.Errorf("Expected ErrEmptyTable, got %v", err)
		}
	})

	// 6. Test Validation (Verify db.Create no longer calls Validate)
	t.Run("Create No Longer Calls Validate", func(t *testing.T) {
		db := orm.New(mockConn{Executor: &mock.Executor{}, Compiler: &mock.Compiler{}})
		model := &mock.Model{
			Table:    "user",
			Sch:      []mdl.Field{{Name: "col1"}},
			Vals:     []any{1},
			ValidErr: errors.New("custom validation error"),
		}

		err := db.Create(model)
		if err != nil {
			t.Errorf("Expected no validation error from db.Create, got %v", err)
		}
	})

	// 7. Test Validation (Verify db.Update no longer calls Validate)
	t.Run("Update No Longer Calls Validate", func(t *testing.T) {
		db := orm.New(mockConn{Executor: &mock.Executor{}, Compiler: &mock.Compiler{}})
		model := &mock.Model{
			Table:    "user",
			Sch:      []mdl.Field{{Name: "col1"}},
			Vals:     []any{1},
			ValidErr: errors.New("custom validation error"),
		}

		err := db.Update(model, orm.Eq("id", 1))
		if err != nil {
			t.Errorf("Expected no validation error from db.Update, got %v", err)
		}
	})

	// Test Validation Error (Delete)
	t.Run("Validation Error Delete", func(t *testing.T) {
		db := orm.New(mockConn{Executor: &mock.Executor{}, Compiler: &mock.Compiler{}})
		model := &mock.Model{Table: ""} // Empty table

		err := db.Delete(model, orm.Eq("id", 1))
		if err == nil || err.Error() != orm.ErrEmptyTable.Error() {
			t.Errorf("Expected ErrEmptyTable, got %v", err)
		}
	})

	// Compile-time guarantee: db.Update(&m) with zero conditions no longer compiles.
	// No test case needed — the Go compiler enforces this contract.

	// 8. Test Empty Table Error
	t.Run("Empty Table Error", func(t *testing.T) {
		db := orm.New(mockConn{Executor: &mock.Executor{}, Compiler: &mock.Compiler{}})
		model := &mock.Model{Table: ""}

		err := db.Create(model)
		if err == nil || err.Error() != orm.ErrEmptyTable.Error() {
			t.Errorf("Expected ErrEmptyTable, got %v", err)
		}
	})

	// 9. Test Or Condition
	t.Run("Or Condition", func(t *testing.T) {
		c := orm.Eq("a", 1)
		orC := orm.Or(c)

		if orC.Logic() != "OR" {
			t.Errorf("Expected Logic OR, got %s", orC.Logic())
		}
	})

	// 10. Test Transaction Support
	t.Run("Transaction", func(t *testing.T) {
		mockTxBound := &mock.TxBoundExecutor{}
		mockTxExec := &mock.TxExecutor{Bound: mockTxBound}
		mockCompiler := &mock.Compiler{}
		db := orm.New(mockTxConn{TxExecutor: mockTxExec, Compiler: mockCompiler})

		err := db.Tx(func(tx *orm.DB) error {
			// Perform operations inside tx
			return nil
		})

		if err != nil {
			t.Fatalf("Tx failed: %v", err)
		}

		if !mockTxBound.CommitCalled {
			t.Error("Expected Commit to be called")
		}
		if mockTxBound.RollbackCalled {
			t.Error("Expected Rollback NOT to be called")
		}
	})

	// 11. Test Transaction Rollback
	t.Run("Transaction Rollback", func(t *testing.T) {
		mockTxBound := &mock.TxBoundExecutor{}
		mockTxExec := &mock.TxExecutor{Bound: mockTxBound}
		mockCompiler := &mock.Compiler{}
		db := orm.New(mockTxConn{TxExecutor: mockTxExec, Compiler: mockCompiler})

		expectedErr := errors.New("oops")
		err := db.Tx(func(tx *orm.DB) error {
			return expectedErr
		})

		if err == nil || err.Error() != expectedErr.Error() {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}

		if mockTxBound.CommitCalled {
			t.Error("Expected Commit NOT to be called")
		}
		if !mockTxBound.RollbackCalled {
			t.Error("Expected Rollback to be called")
		}
	})

	// Test Transaction Begin Error
	t.Run("Transaction Begin Error", func(t *testing.T) {
		mockTxExec := &mock.TxExecutor{BeginTxErr: errors.New("begin error")}
		db := orm.New(mockTxConn{TxExecutor: mockTxExec, Compiler: &mock.Compiler{}})

		err := db.Tx(func(tx *orm.DB) error {
			return nil
		})

		if err == nil || err.Error() != "begin error" {
			t.Errorf("Expected 'begin error', got %v", err)
		}
	})

	// 12. Test No Transaction Support
	t.Run("No Tx Support", func(t *testing.T) {
		db := orm.New(mockConn{Executor: &mock.Executor{}, Compiler: &mock.Compiler{}}) // Not a TxExecutor
		err := db.Tx(func(tx *orm.DB) error { return nil })
		if err == nil || err.Error() != orm.ErrNoTxSupport.Error() {
			t.Errorf("Expected ErrNoTxSupport, got %v", err)
		}
	})

	// 14. Test Condition and Order Getters via Builder
	t.Run("Getters", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})
		model := &mock.Model{Table: "user"}
		mockExec.ReturnQueryRow = &mock.Scanner{}

		db.Query(model).OrderBy("col").Asc().ReadOne()

		if len(mockCompiler.LastQuery.OrderBy) != 1 {
			t.Fatalf("Expected 1 OrderBy, got %d", len(mockCompiler.LastQuery.OrderBy))
		}
		o := mockCompiler.LastQuery.OrderBy[0]

		if o.Column() != "col" {
			t.Errorf("Expected Column 'col', got '%s'", o.Column())
		}
		if o.Dir() != "ASC" {
			t.Errorf("Expected Dir 'ASC', got '%s'", o.Dir())
		}
	})

	// 15. Test Builder Chain (Offset, GroupBy, Limit)
	t.Run("Builder Chain", func(t *testing.T) {
		mockCompiler := &mock.Compiler{}
		mockExec := &mock.Executor{}
		db := orm.New(mockConn{Executor: mockExec, Compiler: mockCompiler})
		model := &mock.Model{Table: "user"}
		mockExec.ReturnQueryRow = &mock.Scanner{}

		// Test Offset and GroupBy
		db.Query(model).
			Offset(10).
			GroupBy("a", "b").
			ReadOne()

		if mockCompiler.LastQuery.Offset != 10 {
			t.Errorf("Expected Offset 10, got %d", mockCompiler.LastQuery.Offset)
		}
		if len(mockCompiler.LastQuery.GroupBy) != 2 {
			t.Errorf("Expected 2 GroupBy cols, got %d", len(mockCompiler.LastQuery.GroupBy))
		}

		// Test Limit with ReadAll
		mockExec.ReturnQueryRows = &mock.Rows{Count: 0}
		db.Query(model).
			Limit(5).
			ReadAll(func() mdl.Model { return nil }, func(mdl.Model) {})

		if mockCompiler.LastQuery.Limit != 5 {
			t.Errorf("Expected Limit 5, got %d", mockCompiler.LastQuery.Limit)
		}
	})

	// 16. Errors coverage
	t.Run("Errors", func(t *testing.T) {
		model := &mock.Model{Table: "t", Sch: []mdl.Field{{Name: "a"}}, Vals: []any{1}}

		// Create Plan Error
		db1 := orm.New(mockConn{Executor: &mock.Executor{}, Compiler: &mock.Compiler{ReturnErr: errors.New("plan err")}})
		if err := db1.Create(model); err == nil || err.Error() != "plan err" {
			t.Errorf("Expected plan err, got %v", err)
		}

		// Create Exec Error
		db2 := orm.New(mockConn{Executor: &mock.Executor{ReturnExecErr: errors.New("exec err")}, Compiler: &mock.Compiler{}})
		if err := db2.Create(model); err == nil || err.Error() != "exec err" {
			t.Errorf("Expected exec err, got %v", err)
		}

		// Update Plan Error
		if err := db1.Update(model, orm.Eq("id", 1)); err == nil || err.Error() != "plan err" {
			t.Errorf("Expected plan err, got %v", err)
		}
		// Update Exec Error
		if err := db2.Update(model, orm.Eq("id", 1)); err == nil || err.Error() != "exec err" {
			t.Errorf("Expected exec err, got %v", err)
		}

		// Delete Plan Error
		if err := db1.Delete(model, orm.Eq("id", 1)); err == nil || err.Error() != "plan err" {
			t.Errorf("Expected plan err, got %v", err)
		}
		// Delete Exec Error
		if err := db2.Delete(model, orm.Eq("id", 1)); err == nil || err.Error() != "exec err" {
			t.Errorf("Expected exec err, got %v", err)
		}

		// ReadOne Plan Error
		if err := db1.Query(model).ReadOne(); err == nil || err.Error() != "plan err" {
			t.Errorf("Expected plan err, got %v", err)
		}
		// ReadOne Scan Error
		db3 := orm.New(mockConn{Executor: &mock.Executor{ReturnQueryRow: &mock.Scanner{ScanErr: errors.New("scan err")}}, Compiler: &mock.Compiler{}})
		if err := db3.Query(model).ReadOne(); err == nil || err.Error() != "scan err" {
			t.Errorf("Expected scan err, got %v", err)
		}

		// ReadAll Plan Error
		if err := db1.Query(model).ReadAll(nil, nil); err == nil || err.Error() != "plan err" {
			t.Errorf("Expected plan err, got %v", err)
		}
		// ReadAll Query Error
		db4 := orm.New(mockConn{Executor: &mock.Executor{ReturnQueryErr: errors.New("query err")}, Compiler: &mock.Compiler{}})
		if err := db4.Query(model).ReadAll(nil, nil); err == nil || err.Error() != "query err" {
			t.Errorf("Expected query err, got %v", err)
		}
		// ReadAll Scan Error
		db5 := orm.New(mockConn{Executor: &mock.Executor{ReturnQueryRows: &mock.Rows{Count: 1, ScanErr: errors.New("scan err")}}, Compiler: &mock.Compiler{}})
		f := func() mdl.Model { return &mock.Model{} }
		e := func(m mdl.Model) {}
		if err := db5.Query(model).ReadAll(f, e); err == nil || err.Error() != "scan err" {
			t.Errorf("Expected scan err, got %v", err)
		}
		// ReadAll Rows Err
		db6 := orm.New(mockConn{Executor: &mock.Executor{ReturnQueryRows: &mock.Rows{Count: 0, ErrVal: errors.New("rows err")}}, Compiler: &mock.Compiler{}})
		if err := db6.Query(model).ReadAll(f, e); err == nil || err.Error() != "rows err" {
			t.Errorf("Expected rows err, got %v", err)
		}
	})

	// 17. Test Close and RawConn
	t.Run("Close and RawConn", func(t *testing.T) {
		mockExec := &mock.Executor{ReturnCloseErr: errors.New("close err")}
		db := orm.New(mockConn{Executor: mockExec, Compiler: &mock.Compiler{}})

		// Test RawConn
		if db.RawConn() == nil {
			t.Errorf("Expected RawConn to return the connection")
		}

		// Test Close
		err := db.Close()
		if err == nil || err.Error() != "close err" {
			t.Errorf("Expected close err, got %v", err)
		}
	})
}
