package orm

import "webtyp.com/fmt"

// notFound is the concrete type of ErrNotFound. IsNotFound recognises it with a
// type assertion: TinyGo compiles that to a type-code comparison, while ==
// between two error values goes through runtime.interfaceEqual and pulls
// internal/reflectlite into the wasm binary.
type notFound struct{}

func (notFound) Error() string { return "record not found" }

// ErrNotFound is returned when ReadOne() finds no matching row. Translates storage.ErrNoRows —
// storage itself has no concept of "not found", only "no rows" (see qb.go's ReadOne). Detect it with IsNotFound, never with ==.
var ErrNotFound error = notFound{}

// IsNotFound reports whether err is ErrNotFound.
func IsNotFound(err error) bool {
	_, ok := err.(notFound)
	return ok
}

// ErrValidation is returned when validate() finds a mismatch.
var ErrValidation = fmt.Err("error", "validation")

// ErrEmptyTable is returned when ModelName() returns an empty string.
var ErrEmptyTable = fmt.Err("name", "table", "empty")

// ErrNoTxSupport is returned by DB.Tx() when the underlying storage.Conn does not implement
// storage.TxExecutor.
var ErrNoTxSupport = fmt.Err("transaction", "not", "supported")
