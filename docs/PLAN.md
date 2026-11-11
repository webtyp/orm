---
PLAN: "feat: orm QB.ILike and storage.ILike re-export"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 5304311211291987633
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST. Submit after EACH stage that compiles.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.
> **Before you submit:** `git status` must be clean and `git diff --stat main` must list the files of the plan.

# Plan: `orm` exposes `ILike`

`orm/reexport.go` re-exports storage helpers (`Like = storage.Like`, line 19) and `orm/qb.go:53` has `func (c *Clause) Like(value any) *QB { return c.qb.addCondition(storage.Like(c.field, value)) }`. `storage` v0.2.3 (already pinned) has `func ILike(field string, value any) Condition`.

## Tools
```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

## Stage 1
1. `reexport.go`: add `ILike = storage.ILike` right after `Like`.
2. `qb.go`: add right after `Like`:
```go
// ILike matches a pattern ignoring upper and lower case.
//
//	orm.Query(db).Where("name").ILike("%an%")
func (c *Clause) ILike(value any) *QB { return c.qb.addCondition(storage.ILike(c.field, value)) }
```
(Copy the example from the real `Like` comment style; adapt the example line to the call chain the file's other examples use.)

## Stage 2 — test
`TestQB_ILikeIgnoresCase` in `tests/qb_ilike_test.go`: with the repo's in-memory/SQLite test setup (read `tests/` for the helper), rows `Ana`, `ana maría`, `Bob`; `ILike("%AN%")` returns the first two. This needs a backend that honours ILIKE: if the backend of the test setup is `sqlt` and its pinned tag does not yet answer `ILIKE` (that fix is a separate plan), use the `mem` backend. Row in `docs/TESTS.md`.

## Delivery check
`gotest` passes on every destination; a tree that does not compile is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `reexport.go`, `qb.go` |
| 2 | `tests/qb_ilike_test.go`, `docs/TESTS.md` |
