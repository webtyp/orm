# Tests

| Test | Rule |
|---|---|
| [TestStore_Conformance](../tests/store_test.go) | `conformance.Run` passes against `orm.GlobalStore(orm.New(mem.New()), ids)` |
| [TestTenantStore_ReadsOnlyItsTenant](../tests/store_test.go) | `TenantStore.ReadAll` returns only rows belonging to that tenant |
| [TestTenantStore_ReadOneOtherTenantIsNotFound](../tests/store_test.go) | `TenantStore.ReadOne` on another tenant's row returns kind `model.ErrorNotFound` |
| [TestTenantStore_CreateOverwritesTenant](../tests/store_test.go) | `TenantStore.Create` stamps and overwrites `tenant_id` with store's tenant |
| [TestTenantStore_UpdateOtherTenantChangesNothing](../tests/store_test.go) | `TenantStore.UpdateFields` targeting another tenant's row changes nothing |
| [TestStore_CompositeKey](../tests/store_test.go) | composite primary key splits on `/`, supports ReadOne, Delete, and partial key returns not found |
| [TestStore_ReadAllOrdered](../tests/store_test.go) | `ReadAll` sorts by `q.Order`, and rejects `q.After` when combined with `Order` |
| [TestTenantStore_OrConditionRejected](../tests/store_test.go) | `ReadAll` rejects any condition with `Logic() == "OR"` returning `crud.ErrOrCondition` |
| [TestStore_CheckModel](../tests/store_test.go) | `CheckModel` rejects models without `tenant_id` on tenant store, and models with `tenant_id` on global store |
| [TestTenantStore_EmptyTenantPanics](../tests/store_test.go) | `TenantStore` panics with `ErrEmptyTenant` when given an empty tenant ID |
| [TestErrNotFound_KindIsNotFound](../tests/errors_kind_test.go) | ErrNotFound returns ErrorNotFound kind |
| [TestTx_NestedJoinsTheOuterTransaction](../tests/nested_tx_test.go) | Nested transactions join outer transaction |
| [TestOmitEmptyOmitsZeroColumn](../tests/omitempty_test.go) | OmitEmpty leaves zero-valued columns out of create queries |
| [TestOmitEmptyMemRoundTrip](../tests/omitempty_test.go) | OmitEmpty round trip against mem storage |
| [TestOmitEmptyUpdateWritesNull](../tests/omitempty_test.go) | OmitEmpty writes null when updating zero values |
| [TestCoreLogic_Stlib](../tests/orm_stlib_test.go) | Core logic tests pass on standard library platform |
| [TestCoreLogic_Wasm](../tests/orm_wasm_test.go) | Core logic tests pass on WASM platform |
| [TestBuilderRoundTripAgainstMem](../tests/roundtrip_test.go) | Full CRUD round-trip against in-memory storage engine |
| [TestReadOne_SelectsExplicitColumnsInSchemaOrder](../tests/select_columns_test.go) | ReadOne selects explicit columns in schema order |
| [TestReadAll_SelectsExplicitColumnsInSchemaOrder](../tests/select_columns_test.go) | ReadAll selects explicit columns in schema order |
| [TestUpdateFieldsWritesOnlyTheNamedColumns](../tests/update_fields_test.go) | UpdateFields writes only the named columns |
| [TestUpdateFieldsRejectsAnEmptyFieldList](../tests/update_fields_test.go) | UpdateFields rejects an empty field list |
| [TestUpdateFieldsRejectsAnUnknownField](../tests/update_fields_test.go) | UpdateFields rejects an unknown field |
| [TestUpdateFieldsRejectsADuplicateField](../tests/update_fields_test.go) | UpdateFields rejects a duplicate field |
| [TestUpdateFieldsAppliesToEveryMatchedRow](../tests/update_fields_test.go) | UpdateFields applies to every matched row |
| [TestDeleteRemovesEveryMatchedRow](../tests/update_fields_test.go) | Delete removes every matched row |
| [TestQB_ILikeIgnoresCase](../tests/qb_ilike_test.go) | ILike matches a pattern ignoring upper and lower case |
