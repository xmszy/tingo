package tdb

import (
	"fmt"
	"testing"
	"time"
)

// customNoNowDialect 是一个不实现 NowDialect 的自定义方言（独立实现 Dialect 接口）。
// 用于回归验证：软删除不应再硬编码 CURRENT_TIMESTAMP，而应回退为绑定参数，
// 否则第三方自定义方言会在运行时产生错误 SQL。
type customNoNowDialect struct{}

func (customNoNowDialect) Name() string      { return "customdb" }
func (customNoNowDialect) Quote(s string) string { return `"` + s + `"` }
func (customNoNowDialect) Placeholder(int) string { return "?" }
func (customNoNowDialect) Limit(limit, offset int) string {
	if limit <= 0 && offset <= 0 {
		return ""
	}
	if offset > 0 {
		return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}
	return fmt.Sprintf(" LIMIT %d", limit)
}

func init() {
	RegisterDialect(customNoNowDialect{})
}

type softUserInt struct {
	Id        int           `tdb:"id"`
	Name      string        `tdb:"name"`
	DeletedAt SoftDeleteInt `tdb:"deleted_at"`
}

func (softUserInt) TableName() string { return "soft_user_int" }

type softUserTime struct {
	Id        int       `tdb:"id"`
	Name      string    `tdb:"name"`
	DeletedAt SoftDelete `tdb:"deleted_at"`
}

func (softUserTime) TableName() string { return "soft_user_time" }

// storedDeletedAt 读取内存驱动中某行的 deleted_at 列值（仅测试用）。
func storedDeletedAt(t *testing.T, conn, table string, id int) any {
	t.Helper()
	memMu.Lock()
	defer memMu.Unlock()
	rows, ok := memStore[conn][table]
	if !ok {
		t.Fatalf("no table %s in conn %s", table, conn)
	}
	for _, r := range rows {
		if r["id"] == id {
			return r["deleted_at"]
		}
	}
	t.Fatalf("row id=%d not found in %s.%s", id, conn, table)
	return nil
}

// TestSoftDeleteIntUsesIntParam 验证 SoftDeleteInt（BIGINT）软删除使用整数绑定参数，
// 而非 SQL 时间戳函数（否则与整数列类型不匹配）。
func TestSoftDeleteIntUsesIntParam(t *testing.T) {
	db, err := Open(Config{Driver: "tdb_mem", DSN: "sd-int", Dialect: "mysql"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTable("sd-int", "soft_user_int",
		map[string]any{"id": 1, "name": "alice", "deleted_at": nil},
	)

	if _, err := NewModel[softUserInt](db).WhereEQ("id", 1).Delete(); err != nil {
		t.Fatalf("soft delete int: %v", err)
	}
	v := storedDeletedAt(t, "sd-int", "soft_user_int", 1)
	ts, ok := v.(int64)
	if !ok {
		t.Fatalf("soft delete int stored value type=%T, want int64", v)
	}
	if ts <= 0 || time.Now().Unix()-ts > 5 {
		t.Fatalf("soft delete int value wrong: %d", ts)
	}
}

// TestSoftDeleteCustomDialectFallsBackToParam 验证自定义方言（未实现 NowDialect）的
// time 类型软删除回退为绑定 time.Now() 参数，而非写死 CURRENT_TIMESTAMP，避免自定义驱动异常。
func TestSoftDeleteCustomDialectFallsBackToParam(t *testing.T) {
	db, err := Open(Config{Driver: "tdb_mem", DSN: "sd-custom", Dialect: "customdb"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	seedTable("sd-custom", "soft_user_time",
		map[string]any{"id": 1, "name": "bob", "deleted_at": nil},
	)

	if _, err := NewModel[softUserTime](db).WhereEQ("id", 1).Delete(); err != nil {
		t.Fatalf("soft delete custom dialect: %v", err)
	}
	v := storedDeletedAt(t, "sd-custom", "soft_user_time", 1)
	tm, ok := v.(time.Time)
	if !ok {
		t.Fatalf("soft delete custom dialect stored value type=%T, want time.Time", v)
	}
	if tm.IsZero() {
		t.Fatalf("soft delete custom dialect did not set deleted_at time")
	}
}
