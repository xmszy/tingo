package tdb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/xmszy/tingo/os/tevent"
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
	sqlStr, args, columns, err := model.buildInsert(map[string]any{"name": "alice", "id": 7}, false)
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

// saveProbe 字段对齐 User，并实现 AfterSaver，用于观测 Save 的 AfterSave 钩子触发次数。
type saveProbe struct {
	Id        int    `tdb:"id"`
	Name      string `tdb:"name"`
	Age       int    `tdb:"age"`
	Email     string `tdb:"email"`
	afterSave int    // 钩子触发计数
}

// AfterSave 实现 AfterSaver 接口，累计触发次数。
func (p *saveProbe) AfterSave() error { p.afterSave++; return nil }

// TableName 让 tdb 能推断表名（与 User 同表）。
func (saveProbe) TableName() string { return "user" }

// TestSaveRoutesAndFiresAfterSave 验证 Save：主键零值走 Insert、非零走 Update，
// 且两种路径都触发 AfterSave 钩子。
func TestSaveRoutesAndFiresAfterSave(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[saveProbe](db, "user")

	// 主键为零值 → Insert（给一个非零字段，便于分解出插入列）
	sp := &saveProbe{Name: "new"}
	if _, err := m.Save(sp); err != nil {
		t.Fatal(err)
	}
	// 主键非零值 → Update（需先有该记录）
	seedTable("test", "user", map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"})
	sp.Id = 1
	sp.Name = "alice2"
	if _, err := m.Save(sp); err != nil {
		t.Fatal(err)
	}

	if sp.afterSave != 2 {
		t.Fatalf("AfterSave should fire on both Insert and Update paths, got %d", sp.afterSave)
	}

	// 验证更新生效：id=1 的 name 应为 alice2
	memMu.Lock()
	var updatedName string
	for _, r := range memStore["test"]["user"] {
		if fmt.Sprintf("%v", r["id"]) == "1" {
			updatedName = fmt.Sprintf("%v", r["name"])
		}
	}
	memMu.Unlock()
	if updatedName != "alice2" {
		t.Fatalf("Save->Update did not apply, name=%q", updatedName)
	}
}

// ---- 查询作用域（scope）与模型事件测试 ----

// OnlyAdult 是一个查询作用域：仅查询成年人。
func onlyAdult(m *Model[User]) *Model[User] {
	return m.Where("age >= ?", 18)
}

// OrderByAge 是一个查询作用域：按年龄升序。
func orderByAge(m *Model[User]) *Model[User] {
	return m.Order("age ASC")
}

// TestScopeAppliesAtExecution 验证 Scopes 在查询执行前按注册顺序应用。
func TestScopeAppliesAtExecution(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
		map[string]any{"id": 2, "name": "kid", "age": 10, "email": "k@x.com"},
		map[string]any{"id": 3, "name": "bob", "age": 25, "email": "b@x.com"},
	)
	// 应用作用域后，scope 条件与额外条件叠加，且作用域内部 Order 生效。
	m := NewModel[User](db).Scopes(onlyAdult, orderByAge)
	// 验证 scope 已正确编译进 SQL（过滤条件与排序）。
	applied := m.applyScopes()
	sqlStr, _ := applied.buildSelect()
	want := "SELECT * FROM `user` WHERE age >= ? ORDER BY age ASC"
	if sqlStr != want {
		t.Fatalf("scope compiled SQL:\n got=%q\nwant=%q", sqlStr, want)
	}
	all, err := m.All()
	if err != nil {
		t.Fatal(err)
	}
	// 注意：memdriver 测试驱动仅解析 LIMIT/OFFSET，不对内存结果排序，
	// 故此处仅断言过滤（age>=18）生效，排序由 SQL 编译断言保证。
	if len(all) != 2 {
		t.Fatalf("scope should filter out age<18, got %d rows: %+v", len(all), all)
	}
}

// TestScopeAppliedOnce 验证作用域只应用一次（多次查询不重复叠加）。
func TestScopeAppliedOnce(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
		map[string]any{"id": 2, "name": "kid", "age": 10, "email": "k@x.com"},
	)
	m := NewModel[User](db).Scope(onlyAdult)
	_ = m // 复用同一 model 多次查询
	first, err := m.All()
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("scope should apply exactly once per query, got %d/%d", len(first), len(second))
	}
}

// scopeProbe 用于观测模型事件触发。
type scopeProbe struct {
	Id    int    `tdb:"id"`
	Name  string `tdb:"name"`
	Age   int    `tdb:"age"`
	Email string `tdb:"email"`
}

func (scopeProbe) TableName() string { return "user" }

// TestModelEventFires 验证 Insert/Update/Delete 触发对应的前后模型事件。
func TestModelEventFires(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	bus := tevent.NewBus(true)
	db.EnableEvents(bus)

	var mu sync.Mutex
	var fired []string
	m := NewModel[scopeProbe](db, "user")
	m.OnBeforeInsert(func(ctx context.Context, d ModelEventData) error {
		mu.Lock()
		fired = append(fired, "before_insert")
		mu.Unlock()
		return nil
	})
	m.OnAfterInsert(func(ctx context.Context, d ModelEventData) error {
		mu.Lock()
		fired = append(fired, "after_insert")
		mu.Unlock()
		if _, ok := d.Model.(scopeProbe); !ok {
			t.Errorf("after_insert model type wrong: %T", d.Model)
		}
		return nil
	})

	// 触发 Insert 事件（before_insert / after_insert）。
	if _, err := m.Insert(scopeProbe{Id: 1, Name: "alice", Age: 30, Email: "a@x.com"}); err != nil {
		t.Fatal(err)
	}

	// 注册 update/delete 监听，并对已存在记录执行更新/删除。
	m.OnBeforeUpdate(func(ctx context.Context, d ModelEventData) error {
		mu.Lock()
		fired = append(fired, "before_update")
		mu.Unlock()
		return nil
	})
	m.OnBeforeDelete(func(ctx context.Context, d ModelEventData) error {
		mu.Lock()
		fired = append(fired, "before_delete")
		mu.Unlock()
		return nil
	})
	m.OnAfterDelete(func(ctx context.Context, d ModelEventData) error {
		mu.Lock()
		fired = append(fired, "after_delete")
		mu.Unlock()
		return nil
	})

	if _, err := m.WhereEQ("id", 1).Update(map[string]any{"age": 31}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.WhereEQ("id", 1).Delete(); err != nil {
		t.Fatal(err)
	}

	want := []string{"before_insert", "after_insert", "before_update", "before_delete", "after_delete"}
	mu.Lock()
	got := append([]string(nil), fired...)
	mu.Unlock()
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("model events fired = %v, want %v", got, want)
	}
}

// TestModelEventCanAbort 验证 Before 事件返回错误可中断写操作。
func TestModelEventCanAbort(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	bus := tevent.NewBus(true)
	db.EnableEvents(bus)

	m := NewModel[scopeProbe](db, "user")
	m.OnBeforeInsert(func(ctx context.Context, d ModelEventData) error {
		return errors.New("aborted by listener")
	})
	if _, err := m.Insert(scopeProbe{Name: "x"}); err == nil {
		t.Fatal("expected before_insert error to abort insert")
	}
}

// TestFindOrFail 验证查不到时返回 ErrNoRows。
func TestFindOrFail(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db)
	if _, err := m.FindOrFail(); !errors.Is(err, ErrNoRows) {
		t.Fatalf("FindOrFail empty want ErrNoRows, got %v", err)
	}
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
	)
	u, err := m.WhereEQ("id", 1).FindOrFail()
	if err != nil {
		t.Fatal(err)
	}
	if u.Name != "alice" {
		t.Fatalf("FindOrFail name = %q", u.Name)
	}
}

// TestFirstOrCreate 验证命中返回且不重复插入。
//
// 注：memdriver 的 INSERT 走事务连接、与查询连接按连接名隔离，
// 不保证写后即可在同连接读到（真实数据库无此限制）。因此本测试仅验证
// 「命中现有记录」路径：不重复插入、返回已存在行。插入路径（写后读）由
// 真实数据库集成测试覆盖。
func TestFirstOrCreate(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db)

	// 预置一条 name=carol 的记录，FirstOrCreate 应命中而不重复插入。
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "carol", "age": 18, "email": "c@x"},
	)
	got, err := m.FirstOrCreate(func(q *Model[User]) *Model[User] {
		return q.WhereEQ("name", "carol")
	}, User{Name: "carol", Age: 99})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "carol" || got.Age != 18 {
		t.Fatalf("FirstOrCreate should return existing row, got %+v", got)
	}
	if all, _ := m.All(); len(all) != 1 {
		t.Fatalf("FirstOrCreate should not duplicate existing, rows=%d", len(all))
	}

	// 不存在条件：FirstOrCreate 应进入插入分支且不 panic（memdriver 不持久化，
	// 此处仅验证调用路径可达，真实 DB 下会写入并返回新行）。
	if _, err := m.FirstOrCreate(func(q *Model[User]) *Model[User] {
		return q.WhereEQ("name", "dave")
	}, User{Name: "dave", Age: 21}); err != nil {
		t.Fatalf("FirstOrCreate insert branch should not error: %v", err)
	}
}

// TestChunkById 验证按主键游标分批遍历。
// 注：memdriver 支持 WHERE/LIMIT/ORDER BY 解析，可验证分批逻辑；
// 分批的去重完整性依赖主键游标推进，size=1 时每批 1 行。
func TestChunkById(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db)
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "a", "age": 1, "email": "a@x"},
		map[string]any{"id": 2, "name": "b", "age": 2, "email": "b@x"},
		map[string]any{"id": 3, "name": "c", "age": 3, "email": "c@x"},
		map[string]any{"id": 4, "name": "d", "age": 4, "email": "d@x"},
	)
	// size=1：每批 1 行，应分 4 批，游标按主键推进。
	var total int
	batches := 0
	err := m.ChunkById(1, func(items []User) (bool, error) {
		t.Logf("batch %d: %+v", batches+1, items)
		batches++
		total += len(items)
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || batches != 4 {
		t.Fatalf("ChunkById(size=1) total=%d batches=%d, want 4/4", total, batches)
	}

	// size=4：一次取完，1 批。
	total = 0
	batches = 0
	err = m.ChunkById(4, func(items []User) (bool, error) {
		batches++
		total += len(items)
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || batches != 1 {
		t.Fatalf("ChunkById(size=4) total=%d batches=%d, want 4/1", total, batches)
	}
}
