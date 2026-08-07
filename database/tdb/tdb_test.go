package tdb

import (
	"errors"
	"fmt"
	"testing"
)

type User struct {
	Id    int    `tdb:"id"`
	Name  string `tdb:"name"`
	Age   int    `tdb:"age"`
	Email string `tdb:"email"`
}

func openMem(t *testing.T) *DB {
	t.Helper()
	db, err := Open(Config{Driver: "tdb_mem", DSN: "test", Dialect: "mysql"})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBuildSelect(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db).Fields("id", "name").
		Where("age > ?", 18).WhereEQ("status", 1).
		Order("created_at DESC").Limit(10).Offset(20)
	sqlStr, args := m.buildSelect()
	want := "SELECT `id`, `name` FROM `user` WHERE age > ? AND `status` = ? ORDER BY created_at DESC LIMIT 10 OFFSET 20"
	if sqlStr != want {
		t.Fatalf("buildSelect:\n got=%q\nwant=%q", sqlStr, want)
	}
	if len(args) != 2 || args[0].(int) != 18 || args[1].(int) != 1 {
		t.Fatalf("args wrong: %v", args)
	}
}

func TestModelAppliesConfiguredTablePrefix(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	db.cfg.Prefix = "tp_"
	model := NewModel[User](db)
	if model.table != "tp_user" {
		t.Fatalf("model table = %q", model.table)
	}
	prefixed := NewModel[User](db, "tp_user")
	if prefixed.table != "tp_user" {
		t.Fatalf("explicit prefixed table = %q", prefixed.table)
	}
}

func TestBuildInsertMapUsesStableColumnOrder(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	model := NewModel[User](db, "users")
	sqlStr, args, columns, err := model.buildInsert(map[string]any{"name": "alice", "id": 7})
	if err != nil {
		t.Fatal(err)
	}
	if sqlStr != "INSERT INTO `users` (`id`, `name`) VALUES (?, ?)" {
		t.Fatalf("buildInsert() = %q", sqlStr)
	}
	if len(args) != 2 || args[0] != 7 || args[1] != "alice" {
		t.Fatalf("args = %#v", args)
	}
	if len(columns) != 2 || columns[0] != "id" || columns[1] != "name" {
		t.Fatalf("columns = %#v", columns)
	}
}

func TestUpsertRejectsUnsupportedDriver(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	_, err := NewModel[User](db).Upsert(map[string]any{"id": 1, "name": "alice"}, "id")
	if !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("Upsert() error = %v", err)
	}
}

func TestWithoutColumns(t *testing.T) {
	got := withoutColumns([]string{"id", "tenant_id", "name"}, []string{"tenant_id", "id"})
	if len(got) != 1 || got[0] != "name" {
		t.Fatalf("withoutColumns() = %#v", got)
	}
}

func TestBuildInsert(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db)
	res, err := m.Insert(User{Name: "bob", Age: 20})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
}

func TestSelectScan(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
		map[string]any{"id": 2, "name": "bob", "age": 25, "email": "b@x.com"},
		map[string]any{"id": 3, "name": "carol", "age": 40, "email": "c@x.com"},
	)
	m := NewModel[User](db)
	all, err := m.Where("age > ?", 26).Order("age ASC").All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(all), all)
	}
	if all[0].Name != "alice" || all[1].Name != "carol" {
		t.Fatalf("order/scan wrong: %+v", all)
	}
	if all[1].Email != "c@x.com" {
		t.Fatalf("email scan wrong: %+v", all)
	}
}

func TestOneCount(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
	)
	m := NewModel[User](db)
	var u User
	if err := m.WhereEQ("name", "alice").One(&u); err != nil {
		t.Fatal(err)
	}
	if u.Id != 1 || u.Name != "alice" {
		t.Fatalf("One wrong: %+v", u)
	}
	n, err := m.Count()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("Count wrong: %d", n)
	}
}

func TestUpdateDelete(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
		map[string]any{"id": 2, "name": "bob", "age": 25, "email": "b@x.com"},
	)
	m := NewModel[User](db)
	res, err := m.WhereEQ("id", 1).Update(map[string]any{"age": 31})
	if err != nil {
		t.Fatal(err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		t.Fatalf("update affected=%d", affected)
	}
	// 无 WHERE 应被护栏拦截（用全新 Model 实例，避免复用上方 Where 条件）
	if _, err := NewModel[User](db).Update(map[string]any{"age": 0}); err != ErrNoWhere {
		t.Fatalf("expected ErrNoWhere, got %v", err)
	}
	// AllowAll 放行
	if _, err := NewModel[User](db).AllowAll().Update(map[string]any{"age": 0}); err != nil {
		t.Fatal(err)
	}
	// 删除
	if _, err := NewModel[User](db).WhereEQ("id", 2).Delete(); err != nil {
		t.Fatal(err)
	}
	n, _ := NewModel[User](db).Count()
	if n != 1 {
		t.Fatalf("after delete count=%d", n)
	}
}

func TestTransaction(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
	)
	// 验证事务提交路径：在 Tx 内更新并提交后，数据应落库。
	err := db.Tx(func(tx *Tx) error {
		m := NewModelTx[User](tx)
		if _, e := m.WhereEQ("id", 1).Update(map[string]any{"age": 99}); e != nil {
			return e
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// 提交后 age 应为 99（直接读内存验证；用字符串比较避免 interface 类型差异）
	memMu.Lock()
	committed := false
	for _, r := range memStore["test"]["user"] {
		if fmt.Sprintf("%v", r["id"]) == "1" && fmt.Sprintf("%v", r["age"]) == "99" {
			committed = true
		}
	}
	memMu.Unlock()
	if !committed {
		t.Fatalf("tx commit failed, age not updated")
	}
}

func TestInsertScanRoundTrip(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db)
	if _, err := m.Insert(User{Name: "dave", Age: 22, Email: "d@x.com"}); err != nil {
		t.Fatal(err)
	}
	u, err := m.WhereEQ("name", "dave").Scan()
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "dave" || u.Age != 22 {
		t.Fatalf("roundtrip wrong: %+v", u)
	}
}

// TestSafeMode 验证 Safe 模式下链式调用不污染原 Model。
func TestSafeMode(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
		map[string]any{"id": 2, "name": "bob", "age": 25, "email": "b@x.com"},
		map[string]any{"id": 3, "name": "carol", "age": 40, "email": "c@x.com"},
	)

	// 创建查询模板
	tmpl := NewModel[User](db).Safe().Where("age > ?", 0)

	// 从模板派生不同查询，原模板不受污染
	young, err := tmpl.Fields("id", "name").Where("age < ?", 30).Order("age ASC").All()
	if err != nil {
		t.Fatal(err)
	}
	if len(young) != 1 || young[0].Name != "bob" {
		t.Fatalf("young query wrong: %+v", young)
	}

	// 原模板的 wheres 仍只有 age > 0 一条
	sqlStr, _ := tmpl.buildSelect()
	if sqlStr != "SELECT * FROM `user` WHERE age > ?" {
		t.Fatalf("template polluted: got %q", sqlStr)
	}

	// 从同一模板派生另一个查询
	old, err := tmpl.Where("age > ?", 35).All()
	if err != nil {
		t.Fatal(err)
	}
	if len(old) != 1 || old[0].Name != "carol" {
		t.Fatalf("old query wrong: %+v", old)
	}
}

// TestSafeClone 验证 Clone 返回独立副本。
func TestSafeClone(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m1 := NewModel[User](db).Where("id = ?", 1).Fields("name")
	m2 := m1.Clone()

	// m2 修改不影响 m1
	m2.Where("age = ?", 18).Fields("email")
	sql1, _ := m1.buildSelect()
	sql2, _ := m2.buildSelect()
	if sql1 != "SELECT `name` FROM `user` WHERE id = ?" {
		t.Fatalf("m1 polluted: %q", sql1)
	}
	if sql2 != "SELECT `name`, `email` FROM `user` WHERE id = ? AND age = ?" {
		t.Fatalf("m2 wrong: %q", sql2)
	}
}

// TestSafeUnsafeDefault 确认默认 unsafe 行为不变。
func TestSafeUnsafeDefault(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db).Where("id = ?", 1)
	m.Fields("name") // 原地修改
	sql, _ := m.buildSelect()
	if sql != "SELECT `name` FROM `user` WHERE id = ?" {
		t.Fatalf("unsafe default should modify in place: %q", sql)
	}
}
