package tdb

import (
	"context"
)

// Seeder 数据填充器——用于在测试或开发环境中批量插入种子数据。
//
// 用法：
//
//	seeder := db.Seeder()
//	seeder.Seed("users", []User{
//	    {Name: "Admin", Age: 30},
//	    {Name: "User1", Age: 25},
//	})
//
// 或使用工厂模式批量生成：
//
//	seeder.Factory("users", 100, func(i int) User {
//	    return User{Name: fmt.Sprintf("User%d", i), Age: 20 + rand.Intn(30)}
//	})
type Seeder struct {
	db     *DB
	ctx    context.Context
	batchSize int
}

// Seeder 创建 Seeder 实例。
func (db *DB) Seeder() *Seeder {
	return &Seeder{
		db:        db,
		ctx:       context.Background(),
		batchSize: 100,
	}
}

// WithContext 设置上下文。
func (s *Seeder) WithContext(ctx context.Context) *Seeder {
	s.ctx = ctx
	return s
}

// WithBatchSize 设置批量插入大小（默认 100）。
func (s *Seeder) WithBatchSize(n int) *Seeder {
	s.batchSize = n
	return s
}

// Seed 插入种子数据。
func Seed[T any](s *Seeder, tableName string, items []T) error {
	m := ModelPrefix[T](s.db, tableName)
	for i := range items {
		if _, err := m.Insert(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

// Factory 使用工厂函数批量生成数据。
func Factory[T any](s *Seeder, tableName string, count int, factory func(int) T) error {
	m := ModelPrefix[T](s.db, tableName)

	batch := make([]T, 0, s.batchSize)
	for i := 0; i < count; i++ {
		item := factory(i)
		batch = append(batch, item)

		if len(batch) >= s.batchSize || i == count-1 {
			for j := range batch {
				if _, err := m.Insert(&batch[j]); err != nil {
					return err
				}
			}
			batch = batch[:0]
		}
	}
	return nil
}

// ModelPrefix 创建带表名的 Model（供 Seed/Factory 使用）。
// 这是包级函数，通过 db 创建指定表名的泛型 Model。
func ModelPrefix[T any](db *DB, tableName string) *Model[T] {
	m := NewModel[T](db, tableName)
	return m
}

// Truncate 清空表并重置自增 ID。
func (s *Seeder) Truncate(tableName string) error {
	sqlStr := "TRUNCATE TABLE " + s.db.Dialect().Quote(tableName)
	if s.db.Dialect().Name() == "sqlite" {
		// SQLite 不支持 TRUNCATE
		sqlStr = "DELETE FROM " + s.db.Dialect().Quote(tableName)
	}
	_, err := s.db.exec(sqlStr)
	return err
}

// TruncateAll 清空所有指定表。
func (s *Seeder) TruncateAll(tableNames ...string) error {
	for _, name := range tableNames {
		if err := s.Truncate(name); err != nil {
			return err
		}
	}
	return nil
}

// DisableForeignKeyChecks 禁用外键检查（MySQL），恢复函数返回。
func (s *Seeder) DisableForeignKeyChecks() func() {
	if s.db.Dialect().Name() == "mysql" {
		s.db.exec("SET FOREIGN_KEY_CHECKS = 0")
		return func() { s.db.exec("SET FOREIGN_KEY_CHECKS = 1") }
	}
	if s.db.Dialect().Name() == "sqlite" {
		s.db.exec("PRAGMA foreign_keys = OFF")
		return func() { s.db.exec("PRAGMA foreign_keys = ON") }
	}
	return func() {}
}
