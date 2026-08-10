package tdb

import (
	"errors"
	"fmt"
	"strconv"
)

// tdb 包级错误。
var (
	// ErrReadOnly 表示在只读模式实例上尝试写操作。
	ErrReadOnly = errors.New("tdb: database is read-only")
	// ErrNoRows 表示查询无结果但要求至少一行。
	ErrNoRows = errors.New("tdb: no rows in result set")
	// ErrInvalidTable 表示无法推断表名且未显式提供。
	ErrInvalidTable = errors.New("tdb: cannot infer table name; pass table explicitly")
	// ErrInvalidValue 表示传入的参数值类型/结构不合法（如 BatchInsert 非切片、
	// ScanList 目标非切片指针等）。
	ErrInvalidValue = errors.New("tdb: invalid value: type or structure not supported")
	// ErrNoWhere 表示无 WHERE 条件尝试整表写操作（需 AllowAll 显式解除）。
	ErrNoWhere = errors.New("tdb: update/delete without WHERE is forbidden; call AllowAll() to confirm")
	// ErrTableNotFound 表示反向工程时表不存在或无列。
	ErrTableNotFound = errors.New("tdb: table not found or has no columns")
	// ErrUnsupportedCapability 表示当前驱动不支持请求的数据库能力。
	ErrUnsupportedCapability = errors.New("tdb: unsupported database capability")
	// errInvalidInt 内部：非数字字节序列。
	errInvalidInt = errors.New("tdb: invalid int bytes")
)

// CapabilityError 描述哪个驱动缺少哪项能力。
type CapabilityError struct {
	Driver     string
	Capability Capability
}

func (e *CapabilityError) Error() string {
	return fmt.Sprintf("tdb: driver %q does not support capability %q", e.Driver, e.Capability)
}

func (e *CapabilityError) Unwrap() error { return ErrUnsupportedCapability }

// parseFloat 与 strconv.ParseFloat 语义一致，集中到此处减少 import 散落。
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
