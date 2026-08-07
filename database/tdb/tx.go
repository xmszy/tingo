package tdb

import (
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

// query 在事务上执行查询。
func (tx *Tx) query(sqlStr string, args ...any) (*sql.Rows, error) {
	return tx.tx.Query(sqlStr, args...)
}

func (tx *Tx) exec(sqlStr string, args ...any) (sql.Result, error) {
	if tx.db.readOnly() {
		return nil, ErrReadOnly
	}
	return tx.tx.Exec(sqlStr, args...)
}
