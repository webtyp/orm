
`UpdateFieldsIf` performs a conditional update: it updates specified fields only if a designated field matches an expected value, reporting the number of affected rows. This is essential for operations like optimistic concurrency control (e.g., matching a `Revision` field). `orm` imports no driver; each driver proves `orm.GlobalStore` with `crud/conformance` in its own tests.
