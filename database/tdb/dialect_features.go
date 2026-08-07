package tdb

import "fmt"

// UpsertSpec 描述冲突目标和发生冲突时需要更新的列。
// MySQL 按唯一键自动识别冲突目标，因此可忽略 ConflictColumns。
type UpsertSpec struct {
	ConflictColumns []string
	UpdateColumns   []string
}

// ReturningPosition 表示返回子句在 INSERT 语句中的插入位置。
type ReturningPosition uint8

const (
	ReturningSuffix ReturningPosition = iota
	ReturningBeforeValues
)

// ReturningClause 是方言格式化后的返回子句。
type ReturningClause struct {
	SQL      string
	Position ReturningPosition
}

// UpsertDialect 是 Dialect 的可选扩展，不破坏已有第三方方言实现。
type UpsertDialect interface {
	UpsertClause(UpsertSpec) (string, error)
}

// ReturningDialect 是 Dialect 的可选扩展。
type ReturningDialect interface {
	ReturningClause(columns []string) (ReturningClause, error)
}

// BuildUpsertClause 通过当前连接的方言生成 Upsert 子句。
func (db *DB) BuildUpsertClause(spec UpsertSpec) (string, error) {
	if err := db.RequireCapability(CapabilityUpsert); err != nil {
		return "", err
	}
	formatter, ok := db.dial.(UpsertDialect)
	if !ok {
		return "", fmt.Errorf("tdb: driver %q advertises %q but its dialect does not implement UpsertDialect", db.dial.Name(), CapabilityUpsert)
	}
	return formatter.UpsertClause(spec)
}

// BuildReturningClause 通过当前连接的方言生成 Returning/Output 子句。
func (db *DB) BuildReturningClause(columns ...string) (ReturningClause, error) {
	if err := db.RequireCapability(CapabilityReturning); err != nil {
		return ReturningClause{}, err
	}
	formatter, ok := db.dial.(ReturningDialect)
	if !ok {
		return ReturningClause{}, fmt.Errorf("tdb: driver %q advertises %q but its dialect does not implement ReturningDialect", db.dial.Name(), CapabilityReturning)
	}
	return formatter.ReturningClause(columns)
}
