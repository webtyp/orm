# ORM Architecture

The `webtyp/orm` package is an ultra-lightweight, strongly-typed, zero-magic (no `reflect`), and isomorphic (WASM/Backend) ORM, strictly following the architectural mandates of the `webtyp` ecosystem.

## Repository Split (2026-07-10)

This repository now contains the **runtime only**. Other components have been moved to their own repositories to keep the dependency graph lean:

| Component | Repository | Role |
|---|---|---|
| **ormc** | [webtyp/ormc](https://github.com/webtyp/ormc) | Build-time code generator and schema synchronization logic. |
| **ddlc** | [webtyp/ddlc](https://github.com/webtyp/ddlc) | SQL Schema (DDL) exporter and topological sorting. |
| **ddl** | [webtyp/ddl](https://github.com/webtyp/ddl) | Runtime DDL: `DropTable`/`CreateDatabase`/`Sync`/`SyncSchema` + `ddl/conformance`. |
| **sqlmcp** | [webtyp/sqlmcp](https://github.com/webtyp/sqlmcp) | MCP tool provider for LLM interaction. |

## Puerto de almacenamiento (2026-07-16, segunda pasada)

`orm` ya no define el contrato de almacenamiento. `webtyp/storage` es el puerto (interfaces, tipos de
valor DML, conformance, mock, mem) — el equivalente de `database/sql/driver`. `orm` es la capa
ergonómica opcional encima (`orm.DB`, query builder), el equivalente de `database/sql`. Un backend
implementa `storage.Conn`; nunca importa `orm`. Ver `webtyp/storage`'s AGENTS.md y
[DB_PORT_PROPOSAL.md](https://github.com/webtyp/app-releases/blob/main/docs/DB_PORT_PROPOSAL.md).

## Background

- [Why use this ORM?](WHY.md)
- [Why package-level schema variables?](WHY_PACKAGE_LEVEL_SCHEMA.md)
- [Why ormc generates so much code (and why it's free)](https://github.com/webtyp/ormc/blob/main/docs/WHY_GENERATED_CODE_IS_FREE.md)


## 1. Primary Architectural Pattern: Inverted Declarative Workflow

Unlike traditional ORMs that use `reflect` or parse structs with tags to generate metadata, `webtyp/orm` inverts the flow. The developer writes a **typed definition** (`model.Definition`), and the generator (`ormc`) produces the **concrete Go struct** and all required interface implementations.

1. **Zero Runtime Reflection:** All metadata is available at compile-time.
2. **O(1) Performance in WASM:** No reflective processing overhead.
3. **Strict Type Safety:** Typos in field names or types are caught by the compiler because definitions use real Go symbols.
4. **Isomorphic by Design:** The same generated code works in Go (backend) and WASM (frontend).

---

## 2. Fundamental Components

### 2.1. `model.Definition` (Source of Truth)

The application developer defines models as package-level variables ending in `Model`.

```go
var UserModel = model.Definition{
    Name: "user",
    Fields: model.Fields{
        {Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
        {Name: "name", Type: model.Text(), NotNull: true},
    },
}
```

### 2.2. `orm.Model` Interface

The generated structs implement this interface, which embeds `fmt.Fielder` and adds `ModelName()`.

```go
type Model interface {
    fmt.Fielder
    ModelName() string
    IsNil() bool
}

// fmt.Fielder provides:
type Fielder interface {
    Schema() []model.Field   // column metadata
    Pointers() []any         // field pointers for DB scanning
    EncodeFields(model.FieldWriter)
    DecodeFields(model.FieldReader)
}
```

### 2.3. Typed Serialization Codec (`Encodable`/`Decodable`)

`ormc` generates reflection-free, 0-allocation methods for serialization (used by `webtyp/json`):

```go
func (m *User) EncodeFields(w model.FieldWriter) {
    w.Int("id", int64(m.ID))
    w.String("name", m.Name)
}

func (m *User) DecodeFields(r model.FieldReader) {
    if v, ok := r.Int("id"); ok { m.ID = v }
    if v, ok := r.String("name"); ok { m.Name = v }
}
```

---

## 3. Code Generation (`ormc`)

See [webtyp/ormc](https://github.com/webtyp/ormc) for detailed documentation.

### 3.1. Role Inference

`ormc` infers the role of each definition based on the presence of `DB` metadata or `Widget` bindings.

### 3.2. Relations

1.  **Composition**: When `Type` is `model.Struct()` or `model.StructSlice()`. The generated struct will embed the referenced type (or a slice of it).
2.  **Scalar Foreign Key**: When `Type` is a scalar and `Ref` is non-nil. This generates `SchemaExt()` metadata for DDL constraints.

---

## 4. Execution Pipeline

`Model` → `Query` → `Compiler` → `Plan` → `Executor`

1. **`Compiler`**: Translates agnostic ORM queries into engine-specific strings (SQL, etc.).
2. **`Executor`**: Standardized interface for running queries and commands, compatible with `database/sql` but engine-independent.

---

## 5. Conformance Testing (`webtyp/storage/conformance`)

The conformance testing suite has moved to `webtyp/storage/conformance`. It provides an executable contract test suite for storage backends to guarantee consistent behavioral standards across different storage engines without compiling or parsing SQL directly.

---

## 6. Mocking and In-Memory Execution (`webtyp/storage/mock` & `webtyp/storage/mem`)

The mock recorders and the in-memory engine have moved to `webtyp/storage/mock` and `webtyp/storage/mem` respectively.
