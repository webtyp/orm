package orm

import "webtyp.com/storage"

// Tx executes a function within a transaction. The underlying storage.Conn must implement
// storage.TxExecutor (type-asserted here) — most backends do; mem.New() also implements it as a
// no-op so tests can exercise this path without a real transactional backend.
//
// Called on a *DB that is already inside a transaction, it joins it: fn runs on the same transaction and the
// outermost Tx commits or rolls back. A module method that opens its own Tx can then be called both alone and by
// code already in a transaction (a data import runs every step in one).
func (d *DB) Tx(fn func(tx *DB) error) error {
	if _, inside := d.conn.(boundConn); inside {
		return fn(d)
	}
	txExec, ok := d.conn.(storage.TxExecutor)
	if !ok {
		return ErrNoTxSupport
	}

	bound, err := txExec.BeginTx()
	if err != nil {
		return err
	}

	// bound is a storage.TxBoundExecutor: Executor + Commit/Rollback. It does NOT satisfy
	// storage.Compiler on its own, so txDB.conn wraps it back together with the original
	// compiler half via boundConn (below) — the same "conn = exec+compile" pairing New()
	// enforces, kept intact across a transaction boundary.
	txDB := &DB{conn: boundConn{TxBoundExecutor: bound, Compiler: d.conn}, log: d.log}

	if err := fn(txDB); err != nil {
		bound.Rollback()
		return err
	}
	return bound.Commit()
}

// boundConn re-pairs a transaction-bound Executor with the original connection's Compiler
// (compiling doesn't depend on being inside a transaction — only executing does), so the
// nested *DB handed to fn still satisfies storage.Conn as a single value.
type boundConn struct {
	storage.TxBoundExecutor
	storage.Compiler
}
