package tdb

import (
	"database/sql"
	"reflect"
	"time"
)

// SoftDeleter 软删除接口 —— Model embed 该字段后即启用软删除。
//
// 当 Delete() 被调用时，如果目标实体实现了 SoftDeleter 接口，
// 则 tdb 会自动将 DELETE 转换为 UPDATE SET deleted_at = NOW()。
// 同时，所有 SELECT 查询会自动追加 WHERE deleted_at IS NULL。
//
// 用法示例：
//
//	type User struct {
//	    Id        int
//	    Name      string
//	    DeletedAt SoftDelete `tdb:"deleted_at"`
//	}
type SoftDelete struct {
	sql.NullTime
}

// IsDeleted 判断是否已被软删除。
func (s SoftDelete) IsDeleted() bool {
	return s.Valid
}

// Delete 标记为已删除（当前时间）。
func (s *SoftDelete) Delete() {
	s.Time = time.Now()
	s.Valid = true
}

// Restore 恢复软删除。
func (s *SoftDelete) Restore() {
	s.Valid = false
}

// ──────────────── SoftDeleteInt（Unix 时间戳）───────────────

// SoftDeleteInt 是 int64 类型的软删除字段（Unix 时间戳）。
// 用法与 SoftDelete 相同，但列类型为 BIGINT 存储 Unix 秒。
//
//	type User struct {
//	    Id        int
//	    Name      string
//	    DeletedAt SoftDeleteInt `tdb:"deleted_at"`
//	}
type SoftDeleteInt int64

// IsDeleted 判断是否已被软删除。
func (s SoftDeleteInt) IsDeleted() bool { return s != 0 }

// Delete 标记为已删除（当前秒）。
func (s *SoftDeleteInt) Delete() { *s = SoftDeleteInt(time.Now().Unix()) }

// Restore 恢复软删除。
func (s *SoftDeleteInt) Restore() { *s = 0 }

// Time 返回 time.Time。
func (s SoftDeleteInt) Time() time.Time { return time.Unix(int64(s), 0) }

// ──────────────── 通用检测 ────────────────

// softDeleteField 检查值中是否存在 SoftDelete/SoftDeleteInt 字段，返回其列名与类型。
func softDeleteField(t reflect.Type) (string, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		ft := f.Type
		if ft == reflect.TypeFor[SoftDelete]() || ft == reflect.TypeFor[*SoftDelete]() ||
			ft == reflect.TypeFor[SoftDeleteInt]() || ft == reflect.TypeFor[*SoftDeleteInt]() {
			col := columnOf(f)
			if col != "" {
				return col, true
			}
		}
	}
	return "", false
}

// hasSoftDelete 检查指针/结构体是否包含软删除字段。
func hasSoftDelete(v any) (string, bool) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return "", false
	}
	return softDeleteField(rv.Type())
}

// softDeleteWhere 生成软删除过滤条件。
func softDeleteWhere(col string, dialect Dialect) string {
	return dialect.Quote(col) + " IS NULL"
}

// ──────────────── 统一时间戳管理 ────────────────
// 自动检测 created_at/updated_at/deleted_at 三字段，
// 支持 datetime（time.Time）和 int（Unix 秒）两种类型。

// TimestampConfig 描述模型的自动时间戳策略。
type TimestampConfig struct {
	CreateAt  string // 创建时间列名（空=不自动填充）
	UpdateAt  string // 更新时间列名（空=不自动填充）
	DeleteAt  string // 删除时间列名（空=使用结构体标签检测）
	Format    string // 时间格式："datetime"（默认）或 "int"
}

// DefaultTimestampConfig 返回常见约定的时间戳配置。
// 列名自动检测 create_time / update_time / delete_time，
// 类型根据字段具体类型自动适配。
func DefaultTimestampConfig() TimestampConfig {
	return TimestampConfig{Format: "datetime"}
}
// ──────────────── 基于 tag 的时间戳声明 ────────────────

// 通过 struct tag 声明时间戳属性，ORM 自动处理。
// 示例：
//
//	type Article struct {
//	    Id        int        `tdb:"id"`
//	    Title     string     `tdb:"title"`
//	    CreatedAt time.Time  `tdb:"created_at" timestamp:"create"`
//	    UpdatedAt time.Time  `tdb:"updated_at" timestamp:"update"`
//	    DeletedAt SoftDelete `tdb:"deleted_at" timestamp:"delete"`
//	}
//
// Model.Insert/Update/Delete 时会自动处理这些 tag 声明的字段。

// HasTimestampTag 检查结构体的 timestamp tag 配置。
func HasTimestampTag(rt reflect.Type) (create, update string) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("timestamp")
		col := columnOf(f)
		switch tag {
		case "create":
			create = col
		case "update":
			update = col
		}
	}
	return
}
