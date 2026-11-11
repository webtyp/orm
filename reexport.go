package orm

import "webtyp.com/storage"

// Re-exports of storage's DML value types and condition/order constructors, so that a consumer
// calling Update/Delete (which take a storage.Condition explicitly) doesn't need a second import
// just for Eq/Gt/etc. These are aliases, not new types/wrappers — orm.Condition IS storage.Condition,
// orm.Eq IS storage.Eq. Zero duplication, zero conversion. See docs/PLAN.md §3.
type Condition = storage.Condition
type Order = storage.Order

var (
	Eq        = storage.Eq
	Neq       = storage.Neq
	Gt        = storage.Gt
	Gte       = storage.Gte
	Lt        = storage.Lt
	Lte       = storage.Lte
	Like      = storage.Like
	ILike     = storage.ILike
	In        = storage.In
	Or        = storage.Or
	IsNotNull = storage.IsNotNull
	Asc       = storage.Asc
	Desc      = storage.Desc
)
