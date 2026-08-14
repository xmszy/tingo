package tdb

import (
	"context"
	"testing"
)

// TestTransactionNestedFlatten 验证在不支持 savepoint 的驱动上，
// 嵌套事务退化为扁平执行：内层返回 error 会冒泡到外层，且回调均被调用。
// 注：内存驱动不实现真实回滚语义，故此处仅验证错误传播与回调契约，
// 真实回滚由数据库驱动在 SAVEPOINT 路径（支持 CapabilitySavepoint）保证。
func TestTransactionNestedFlatten(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "a", "age": 1, "email": nil, "created_at": nil},
	)

	innerCalled := false
	err := db.Transaction(context.Background(), func(tx *Tx) error {
		return tx.Transaction(context.Background(), func(sub *Tx) error {
			innerCalled = true
			return errTestNested
		})
	})
	if err == nil {
		t.Fatal("expected nested error to propagate")
	}
	if err != errTestNested {
		t.Fatalf("wrong error: %v", err)
	}
	if !innerCalled {
		t.Fatal("inner nested callback not invoked")
	}
}

// TestRawScan 验证原生 SQL 逃生舱的扫描映射。
func TestRawScan(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "alice", "age": 30, "email": nil, "created_at": nil},
		map[string]any{"id": 2, "name": "bob", "age": 40, "email": nil, "created_at": nil},
	)

	var users []User
	if err := Raw[User](db, "SELECT * FROM user WHERE age > ?", 35).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "bob" {
		t.Fatalf("raw scan wrong: %+v", users)
	}

	var one User
	if err := Raw[User](db, "SELECT * FROM user WHERE id = ?", 1).ScanOne(&one); err != nil {
		t.Fatal(err)
	}
	if one.Name != "alice" {
		t.Fatalf("raw scanone wrong: %+v", one)
	}
	if err := Raw[User](db, "SELECT * FROM user WHERE id = ?", 999).ScanOne(&one); err != ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}

// TestInsertIgnoreBasic 验证 InsertIgnore 调用 buildInsert(ignore=true) 且不崩溃。
func TestInsertIgnoreBasic(t *testing.T) {
	db := openMem(t)
	defer db.Close()
	// id=1 已存在（主键），InsertIgnore 不应返回致命错误（mem 驱动可能直接插入重复，
	// 这里仅验证接口与构建逻辑不 panic）。
	seedTable("test", "user",
		map[string]any{"id": 1, "name": "a", "age": 1, "email": nil, "created_at": nil},
	)
	if _, err := NewModel[User](db).InsertIgnore(User{Id: 2, Name: "b", Age: 2}); err != nil {
		t.Fatalf("InsertIgnore failed: %v", err)
	}
}

var errTestNested = &testErr{"nested"}

type testErr struct{ s string }

func (e *testErr) Error() string { return e.s }
