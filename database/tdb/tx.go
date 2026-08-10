package tdb

import (
	"context"
	"database/sql"
)

// Tx 是一个事务句柄，实现与 DB 一致的查询/执行能力，但作用域限定在本次事务。
// 由 DB.Tx 创建，用户不应自行构造。
type Tx struct {
	tx   *sql.Tx
	db   *DB
	dial Dialect
}

// SQL 返回底层 *sql.Tx。
func (tx *Tx) SQL() *sql.Tx { return tx.tx }

// Commit 提交（仅在脱离 DB.Tx 回调手动管理时需要）。
func (tx *Tx) Commit() error { return tx.tx.Commit() }

// Rollback 回滚。
func (tx *Tx) Rollback() error { return tx.tx.Rollback() }

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
