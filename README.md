# webtyp/orm
<img src="docs/img/badges.svg">

**Ultra-lightweight, strongly-typed ORM engineered for WebAssembly and backend environments.**

## Features

- **Declarative Source of Truth**: Hand-written `model.Definition` literals as the source for code generation.
- **Zero Reflection**: Interface-driven schema via `webtyp.com/model` with generated code.
- **Isomorphic**: Same generated code works in Go (backend) and WASM (frontend).
- **0-alloc Codec**: Symmetric, reflection-free serialization/deserialization methods generated for every model.
- **Query Builder**: Reflection-free, type-safe query building.
- **Backend-Agnostic**: `orm.DB` wraps a `storage.Conn` — no storage contract is defined here, so any backend implementing `webtyp/storage` (Postgres, SQLite, in-memory, IndexedDB, ...) works unchanged.

## Ecosystem

This repository is the **ergonomic layer only** — the equivalent of `database/sql`. It defines no storage contract of its own. Other parts of the ecosystem:

| Component | Repository | Role |
|---|---|---|
| **storage** | [webtyp/storage](https://github.com/webtyp/storage) | Storage port (`Executor`/`Compiler`/`Query`/`Condition`/`Plan`, `mock`, `mem`) — the equivalent of `database/sql/driver`. A backend implements `storage.Conn`; `orm.New(conn)` takes one. |
| **ormc** | [webtyp/ormc](https://github.com/webtyp/ormc) | Build-time code generator. |
| **sqlmcp** | [webtyp/sqlmcp](https://github.com/webtyp/sqlmcp) | MCP tool provider for LLM interaction. |

## Installation

```bash
go get webtyp.com/orm
```

To install the code generator:

```bash
go install webtyp.com/ormc/cmd/ormc@latest
```

## Declarative Workflow

In this ORM, you write the **typed definition** of your model, and `ormc` generates the **concrete struct** and the rest of the code. This ensures your schema is a compile-time checked symbol, eliminating errors from fragile string tags.

### 1. Define your models

Create a `model.go` or `models.go` file. Define your models as `model.Definition` variables ending in `Model`.

```go
package user

import (
    "webtyp.com/model"
)

var UserModel = model.Definition{
    Name: "user",
    Fields: model.Fields{
        {Name: "id",    Type: model.Int(),  DB: &model.FieldDB{PK: true, AutoInc: true}},
        {Name: "name",  Type: model.Text(), NotNull: true, Permitted: model.Permitted{Minimum: 2}},
        {Name: "email", Type: model.Text(), NotNull: true, DB: &model.FieldDB{Unique: true}},
        {Name: "bio",   Type: model.Text(), OmitEmpty: true},
        {Name: "addr",  Type: model.Struct(), Ref: &AddressModel},
    },
}

var AddressModel = model.Definition{
    Name: "address",
    Fields: model.Fields{
        {Name: "street", Type: model.Text()},
        {Name: "city",   Type: model.Text()},
    },
}
```

### 2. Generate

```bash
ormc
```

Generates `model_orm.go` next to each source file, containing the `type User struct`, `type Address struct`, and all interface implementations.

### 3. Use it

```go
func GetUser(db *orm.DB, id int64) (*user.User, error) {
    return user.ReadOneUser(
        db.Query(&user.User{}).Where("id").Eq(id),
        &user.User{},
    )
}
```

## Definition Reference

### `model.Field` Options

| Field | Meaning |
|-------|---------|
| `Name` | wire/DB column name (snake_case). Becomes PascalCase in the generated struct. |
| `Type` | `model.Kind` (e.g. `model.Text()`, `model.Int()`, `model.Struct()`). |
| `NotNull` | `NOT NULL` constraint in DB. |
| `OmitEmpty` | Skips field if zero-value in codecs. |
| `Exclude` | Field exists in generated struct but is skipped in `Schema()`, `Pointers()`, and codecs (e.g. for `password_hash`). |
| `Widget` | Binding for UI rendering (see `webtyp/webtyp/input`). |
| `DB` | Database-specific metadata (`PK`, `Unique`, `AutoInc`, `RefColumn`, `OnDelete`). |
| `Ref` | Points to another `*model.Definition`. Used for composition or scalar Foreign Keys. |
| `Permitted` | Validation rules (Minimum, Maximum, Letters, Numbers, etc.). |

### Relations

1.  **Composition**: When `Type` is `model.Struct()` or `model.StructSlice()`. The field becomes a nested struct or slice of structs part of the same row/JSON.
2.  **Scalar Foreign Key**: When `Type` is a scalar (e.g. `model.Int()`) and `Ref` is non-nil. Adds DDL metadata for `FOREIGN KEY` constraints.

```go
{Name: "user_id", Type: model.Int(), Ref: &UserModel, DB: &model.FieldDB{RefColumn: "id", OnDelete: "CASCADE"}}
```

## Schema Types Mapping

| `model.Kind` | Generated Go Type | Codec Method |
|---|---|---|
| `model.Text()`, `model.Raw()` | `string` | `String` / `Raw` |
| `model.Int()` | `int64` | `Int` |
| `model.Float()` | `float64` | `Float` |
| `model.Bool()` | `bool` | `Bool` |
| `model.Blob()` | `[]byte` | `Bytes` |
| `model.IntSlice()` | `[]int` | `Array` |
| `model.Struct()` | `T` (from `Ref`) | `Object` |
| `model.StructSlice()` | `[]*T` (from `Ref`) | `Array` |

## Generated Code

The `ormc` generator produces:

- `type T struct` declaration.
- `Schema() []model.Field`, `Pointers() []any` (parallel, zero-alloc).
- `EncodeFields(model.FieldWriter)`, `DecodeFields(model.FieldReader)` (symmetric codec).
- `IsNil() bool`.
- `Validate(action byte) error` (calling `model.ValidateFields`). Always generated.
- `ModelName() string`.
- `ReadOneT()`, `ReadAllT()`, `TList` type (for DB models).
- `SchemaExt() []model.FieldExt` (foreign keys; `model.FieldExt` lives in [webtyp/model](https://github.com/webtyp/model)).

## Errors

`ReadOne` returns `orm.ErrNotFound` when no row matches; detect it with `orm.IsNotFound(err)`. It is a "not found" refusal (`model.KindOf(err) == model.ErrorNotFound`), so `rpc.WriteError` answers it 404 without the module translating it.

## More Documentation

- [Architecture](docs/ARQUITECTURE.md)
- [Improvement Roadmap](docs/IMPROVE.md)
