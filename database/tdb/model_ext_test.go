package tdb

import (
	"testing"
)

type Profile struct {
	Id        int    `tdb:"id"`
	Bio       string `tdb:"bio"`
	ProfileId int    `tdb:"profile_id"` // 外键，指向 User.id
}

type UserWithProfile struct {
	Id      int      `tdb:"id"`
	Name    string   `tdb:"name"`
	Profile *Profile `tdb:"-"`
}

func TestBatchInsert(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db)
	affected, err := m.BatchInsert([]User{
		{Id: 1, Name: "a", Age: 10, Email: "a@x"},
		{Id: 2, Name: "b", Age: 20, Email: "b@x"},
		{Id: 3, Name: "c", Age: 30, Email: "c@x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if affected != 3 {
		t.Fatalf("affected = %d, want 3", affected)
	}
	all, err := m.Order("id ASC").All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Name != "a" || all[2].Name != "c" {
		t.Fatalf("BatchInsert result wrong: %+v", all)
	}
}

func TestBatchInsertChunked(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db)
	list := make([]User, 25)
	for i := range list {
		list[i] = User{Id: i + 1, Name: "n", Age: i}
	}
	affected, err := m.BatchInsert(list, 10)
	if err != nil {
		t.Fatal(err)
	}
	if affected != 25 {
		t.Fatalf("affected = %d, want 25", affected)
	}
}

func TestLockAppendsClause(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	m := NewModel[User](db).Where("age > ?", 18).Lock(LockForUpdate)
	sqlStr, _ := m.buildSelect()
	want := "SELECT * FROM `user` WHERE age > ? FOR UPDATE"
	if sqlStr != want {
		t.Fatalf("Lock: got=%q want=%q", sqlStr, want)
	}
	// 空串清除锁
	noLock, _ := m.Lock("").buildSelect()
	if noLock != "SELECT * FROM `user` WHERE age > ?" {
		t.Fatalf("Lock clear: got=%q", noLock)
	}
	// 不污染原 Model
	orig, _ := m.buildSelect()
	if orig != want {
		t.Fatalf("Lock polluted original: %q", orig)
	}
}

func TestScanListAssemblesNested(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	// 模拟一次联表查询结果（列扁平化）。
	rows := []map[string]any{
		{"id": 1, "bio": "hi", "profile_id": 1},
		{"id": 2, "bio": "yo", "profile_id": 2},
	}
	// 主切片（已有元素，其 id 作为关联父键）
	users := []UserWithProfile{
		{Id: 1, Name: "alice"},
		{Id: 2, Name: "bob"},
	}
	if err := NewModel[UserWithProfile](db).ScanList(rows, &users, "Profile"); err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("len = %d", len(users))
	}
	if users[0].Profile == nil || users[0].Profile.Bio != "hi" {
		t.Fatalf("nested Profile wrong: %+v", users[0])
	}
	if users[1].Profile == nil || users[1].Profile.Bio != "yo" {
		t.Fatalf("nested Profile wrong: %+v", users[1])
	}
}

func TestAllMaps(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
		map[string]any{"id": 2, "name": "bob", "age": 25, "email": "b@x.com"},
	)
	maps, err := NewModel[User](db).Order("id ASC").AllMaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 2 || maps[0]["name"] != "alice" {
		t.Fatalf("AllMaps wrong: %+v", maps)
	}
}

func TestFieldsEx(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
	)
	maps, err := NewModel[User](db).FieldsEx("email").Order("id ASC").AllMaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 1 {
		t.Fatalf("len = %d", len(maps))
	}
	if _, ok := maps[0]["email"]; ok {
		t.Fatalf("FieldsEx did not exclude email: %+v", maps[0])
	}
	if maps[0]["name"] != "alice" {
		t.Fatalf("FieldsEx wrong: %+v", maps[0])
	}
}

func TestValueAndArray(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": "a@x.com"},
		map[string]any{"id": 2, "name": "bob", "age": 25, "email": "b@x.com"},
		map[string]any{"id": 3, "name": "carol", "age": 40, "email": "c@x.com"},
	)
	v, err := NewModel[User](db).Where("id = ?", 2).Value("name")
	if err != nil {
		t.Fatal(err)
	}
	if v != "bob" {
		t.Fatalf("Value = %v, want bob", v)
	}

	arr, err := NewModel[User](db).Order("id ASC").Array("age")
	if err != nil {
		t.Fatal(err)
	}
	if len(arr) != 3 {
		t.Fatalf("Array len = %d, want 3", len(arr))
	}
	if arr[0] != 30 || arr[2] != 40 {
		t.Fatalf("Array wrong: %+v", arr)
	}
}

func TestChunk(t *testing.T) {
	db2 := openMem(t)
	defer db2.Close()
	seedChunk := make([]map[string]any, 0, 25)
	for i := 1; i <= 25; i++ {
		seedChunk = append(seedChunk, map[string]any{"id": i, "name": "n", "age": i, "email": "e"})
	}
	seedTable("test", "user", seedChunk...)

	total := 0
	pages := 0
	err := NewModel[User](db2).Order("id ASC").Chunk(10, func(records []User) error {
		total += len(records)
		pages++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 25 || pages != 3 {
		t.Fatalf("Chunk total=%d pages=%d, want 25/3", total, pages)
	}
}
