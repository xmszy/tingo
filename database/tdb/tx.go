package tdb

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
)

// Tx 是一个事务句柄，实现与 DB 一致的查询/执行能力，但作用域限定在本次事务。
// 由 DB.Transaction 创建，用户不应自行构造。
type Tx struct {
	tx   *sql.Tx
	db   *DB
	dial Dialect
	sp   int64 // 嵌套 savepoint 序号计数（原子），用于生成唯一保存点名
}

// SQL 返回底层 *sql.Tx。
func (tx *Tx) SQL() *sql.Tx { return tx.tx }

// Commit 提交（仅在脱离 DB.Transaction 回调手动管理时需要）。
func (tx *Tx) Commit() error { return tx.tx.Commit() }

// Rollback 回滚。
func (tx *Tx) Rollback() error { return tx.tx.Rollback() }

// Transaction 在已有事务内开启嵌套事务（基于 SAVEPOINT）。
//   - 若方言支持 CapabilitySavepoint：通过 SAVEPOINT/ROLLBACK TO/RELEASE 提供子回滚隔离，
//     回调返回 error 仅回滚到该保存点，不影响外层事务；回调成功则 RELEASE 保存点。
//   - 若不支持 savepoint：退化为在当前事务内直接执行（扁平化，无子回滚隔离）。
//
// 嵌套事务仍使用同一个底层 *sql.Tx，因此对所有写操作可见。
func (tx *Tx) Transaction(ctx context.Context, fn func(tx *Tx) error) error {
	if !tx.db.Capabilities().Supports(CapabilitySavepoint) {
		// 不支持 savepoint：扁平化执行（与外层共享提交/回滚）。
		return fn(tx)
	}
	n := atomic.AddInt64(&tx.sp, 1)
	name := fmt.Sprintf("sp_%d", n)
	q := tx.dial.Quote(name)
	if _, err := tx.tx.ExecContext(ctx, "SAVEPOINT "+q); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if _, rbErr := tx.tx.ExecContext(ctx, "ROLLBACK TO "+q); rbErr != nil {
			return rbErr
		}
		return err
	}
	if _, err := tx.tx.ExecContext(ctx, "RELEASE SAVEPOINT "+q); err != nil {
		return err
	}
	return nil
}

func (tx *Tx) exec(sqlStr string, args ...any) (sql.Result, error) {
	if tx.db.readOnly() {
		return nil, ErrReadOnly
	}
	return tx.tx.Exec(sqlStr, args...)
}

// queryCtx 支持 context 的查询（ctx 为 nil 时退化为普通 Query）。
func (tx *Tx) queryCtx(ctx context.Context, sqlStr string, args ...any) (*sql.Rows, error) {
	if ctx == nil {
		return tx.tx.Query(sqlStr, args...)
	}
	return tx.tx.QueryContext(ctx, sqlStr, args...)
}

// execCtx 支持 context 的执行（ctx 为 nil 时退化为普通 Exec）。
func (tx *Tx) execCtx(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	if tx.db.readOnly() {
		return nil, ErrReadOnly
	}
	if ctx == nil {
		return tx.tx.Exec(sqlStr, args...)
	}
	return tx.tx.ExecContext(ctx, sqlStr, args...)
}
