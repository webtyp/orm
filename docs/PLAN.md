---
PLAN: "feat(orm): IsNotFound — detect not-found without == between interfaces; ReadOne uses storage.IsNoRows"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 13414063006642335769
PR: https://github.com/webtyp/orm/pull/58
---

# Plan — `orm.IsNotFound(err)`

> Master: `webtyp/docs/NO_REFLECTION_MASTER_PLAN.md` (ola 2).
> **Prerrequisito:** `webtyp.com/storage` publicado con `IsNoRows` (ola 1). Primer paso:
> `go get webtyp.com/storage@latest` y confirmar que `storage.IsNoRows` existe. Si no existe, parar
> y reportarlo: no implementar un sustituto local.

## 1. El problema

- `qb.go:124`: `if err == storage.ErrNoRows {` — `==` entre dos `error` (interfaces).
- Unos 120 sitios en el ecosistema escriben `err == orm.ErrNotFound` o `switch err { case orm.ErrNotFound: }`.

En TinyGo, `==` entre interfaces compila a `runtime.interfaceEqual` → `reflectValueEqual` y mete
`internal/reflectlite` (~7–9 KB) en el binario wasm. La regla del dueño: cero reflexión en wasm.

## 2. Design gate (api-design)

1. **Antecedentes.** `os.IsNotExist(err)` (biblioteca estándar), `status.Code(err)` de gRPC,
   `apierrors.IsNotFound(err)` de Kubernetes client-go. `database/sql` usa `==`/`errors.Is`, lo que
   aquí cuesta reflexión. Misma decisión que `storage.IsNoRows` (ola 1): función de consulta con
   aserción de tipo.
2. **Nombre.** `orm.IsNotFound(err)`: el mismo que usa client-go para el mismo concepto.
3. **Balance.** Conceptos +1 · formas de detectar "no encontrado": hoy 1 (`==`), después 1
   (`IsNotFound`; el guardia de la ola 4 prohíbe `==` entre interfaces en código wasm) · call site igual.
4. **Dónde va.** `orm`, dueño del centinela.
5. **Qué borra.** El `==` de `qb.go:124`. En la ola 3, los ~120 `== orm.ErrNotFound` de los consumidores.

Solo `ErrNotFound` recibe `IsX`: ningún consumidor del ecosistema compara `ErrValidation`,
`ErrEmptyTable` ni `ErrNoTxSupport` (verificado con grep el 2026-10-08). Superficie mínima.

## 3. La corrección

En `errors.go`, `ErrNotFound` pasa a tener tipo concreto propio:

```go
// notFound is the concrete type of ErrNotFound. IsNotFound recognises it with a
// type assertion: TinyGo compiles that to a type-code comparison, while ==
// between two error values goes through runtime.interfaceEqual and pulls
// internal/reflectlite into the wasm binary.
type notFound struct{}

func (notFound) Error() string { return "<texto actual>" }

// ErrNotFound is returned when ReadOne() finds no matching row. Translates
// storage.ErrNoRows — storage itself has no concept of "not found", only
// "no rows" (see qb.go's ReadOne). Detect it with IsNotFound, never with ==.
var ErrNotFound error = notFound{}

// IsNotFound reports whether err is ErrNotFound.
func IsNotFound(err error) bool {
	_, ok := err.(notFound)
	return ok
}
```

- `<texto actual>`: el string exacto que devuelve hoy `fmt.Err("record", "not", "found").Error()`.
  Medirlo **antes** de cambiar y fijarlo en un test.
- `qb.go:124`: `if err == storage.ErrNoRows {` → `if storage.IsNoRows(err) {`.
- Buscar en el módulo otros `==`/`!=`/`switch` entre interfaces con operandos no nil y migrarlos.

## 4. Tests (rojo primero)

- `tests/`: `IsNotFound(orm.ErrNotFound)` → true; `IsNotFound(nil)` → false;
  `IsNotFound(storage.ErrNoRows)` → false; `orm.ErrNotFound.Error()` == `<texto actual>`.
- `tests/`: `ReadOne` sobre una tabla vacía (con el backend `mem` que ya usan los tests) devuelve un
  error para el que `orm.IsNotFound` es true.
- `gotest` verde (vet, race, tests, wasm).

## 5. Criterios de aceptación

- `grep -rn '== storage.ErrNoRows\|== ErrNotFound\|errors.Is' --include=*.go .` → vacío (fuera de
  comentarios y docs).
- Único símbolo exportado nuevo: `IsNotFound`.
- Docs (`docs/ARQUITECTURE.md`, `README.md`) muestran `orm.IsNotFound(err)` donde hoy muestran
  `err == orm.ErrNotFound`.
- `gotest` verde.

## 6. Restricciones

Las de `AGENTS.md`, más: nada de `reflect`, nada de `unsafe`, ningún `==`/`!=`/`switch` entre valores
de interfaz con operandos no nil. No tocar los consumidores (otros repos): son la ola 3.

## Executor notes
- `errors.go` updated to define `ErrNotFound` as a `notFound` struct instead of `fmt.Err("record", "not", "found")`, with an `IsNotFound` type-assertion check.
- `qb.go` updated to use `storage.IsNoRows(err)`.
- `tests/core_test.go` and `tests/roundtrip_test.go` updated to remove all uses of `errors.Is` and `err == orm.ErrNotFound`. Interface comparisons were changed to string representation matches `err.Error() == expectedErr.Error()` to avoid using standard library reflection tools while checking `err`.
- Added tests to cover `orm.IsNotFound`.
- Documentation: Examined `README.md` and `docs/` as mentioned in step 5 of PLAN.md criteria. However, `orm.ErrNotFound` did not appear in any code blocks showing usage, so no explicit edits to existing docs were required outside of the `PLAN.md` instructions.
