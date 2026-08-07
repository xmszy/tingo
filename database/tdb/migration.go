package tdb

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Migration 表示一条数据库迁移记录。
type Migration struct {
	// ID 迁移文件名（不含 .go 后缀）。
	ID string
	// Batch 批次号（同批次一起回滚）。
	Batch int
	// AppliedAt 执行时间。
	AppliedAt time.Time
}

// Migrator 数据库迁移运行器。
//
// 负责在数据库中创建迁移记录表，按文件名排序执行 Up/Down。
//
// 用法：
//
//	db := tdb.Open(...)
//	m := db.Migrator("database/migrations")
//	m.Up()    // 执行所有待执行迁移
//	m.Down()  // 回滚最近一批
//	m.Reset() // 回滚所有并重新执行
type Migrator struct {
	db        *DB
	dir       string
	tableName string
	dryRun    bool
}

// Migrator 创建迁移运行器。
// dir 是存放迁移文件的目录。
func (db *DB) Migrator(dir string) *Migrator {
	return &Migrator{
		db:        db,
		dir:       dir,
		tableName: "migrations",
	}
}

// SetTableName 设置迁移记录表名（默认 "migrations"）。
func (m *Migrator) SetTableName(name string) *Migrator {
	m.tableName = name
	return m
}

// DryRun 设置为演习模式（仅打印 SQL 和日志，不实际执行）。
func (m *Migrator) DryRun() *Migrator {
	m.dryRun = true
	return m
}

// Up 执行所有未执行过的迁移文件。
func (m *Migrator) Up() error {
	entries, err := listMigrationEntries()
	if err != nil {
		return fmt.Errorf("migration: list entries: %w", err)
	}

	if err := m.ensureTable(); err != nil {
		return fmt.Errorf("migration: ensure table: %w", err)
	}

	applied, err := m.getApplied()
	if err != nil {
		return fmt.Errorf("migration: get applied: %w", err)
	}

	batch := m.nextBatch()
	ran := false
	for _, entry := range entries {
		if applied[entry.ID] {
			continue
		}
		ran = true
		fmt.Printf("Migrating: %s\n", entry.ID)
		if !m.dryRun {
			if err := entry.Up(m.db); err != nil {
				return fmt.Errorf("migration: up %s: %w", entry.ID, err)
			}
			if err := m.record(entry.ID, batch); err != nil {
				return fmt.Errorf("migration: record %s: %w", entry.ID, err)
			}
		}
		fmt.Printf("Migrated:  %s\n", entry.ID)
	}

	if !ran {
		fmt.Println("Nothing to migrate.")
	}
	return nil
}

// Down 回滚最近一批迁移。
func (m *Migrator) Down() error {
	if err := m.ensureTable(); err != nil {
		return fmt.Errorf("migration: ensure table: %w", err)
	}

	applied, err := m.getAppliedList()
	if err != nil {
		return fmt.Errorf("migration: get applied: %w", err)
	}

	if len(applied) == 0 {
		fmt.Println("Nothing to rollback.")
		return nil
	}

	// 找到最后一组批次的迁移
	lastBatch := applied[len(applied)-1].Batch
	var toRollback []Migration
	for i := len(applied) - 1; i >= 0 && applied[i].Batch == lastBatch; i-- {
		toRollback = append(toRollback, applied[i])
	}

	entries, err := listMigrationEntries()
	if err != nil {
		return fmt.Errorf("migration: list entries: %w", err)
	}
	entryMap := make(map[string]*migrationEntry, len(entries))
	for i := range entries {
		entryMap[entries[i].ID] = &entries[i]
	}

	for _, mig := range toRollback {
		entry, ok := entryMap[mig.ID]
		if !ok {
			return fmt.Errorf("migration: entry not found for %s", mig.ID)
		}
		fmt.Printf("Rolling back: %s\n", mig.ID)
		if !m.dryRun {
			if err := entry.Down(m.db); err != nil {
				return fmt.Errorf("migration: down %s: %w", mig.ID, err)
			}
			if err := m.remove(mig.ID); err != nil {
				return fmt.Errorf("migration: remove %s: %w", mig.ID, err)
			}
		}
		fmt.Printf("Rolled back:  %s\n", mig.ID)
	}

	return nil
}

// Reset 回滚全部迁移并重新执行。
func (m *Migrator) Reset() error {
	applied, err := m.getAppliedList()
	if err != nil {
		return fmt.Errorf("migration: get applied: %w", err)
	}

	// 回滚全部
	for len(applied) > 0 {
		if err := m.Down(); err != nil {
			return err
		}
		applied, err = m.getAppliedList()
		if err != nil {
			return fmt.Errorf("migration: get applied: %w", err)
		}
	}

	// 重新执行
	return m.Up()
}

// Status 输出迁移状态。
func (m *Migrator) Status() error {
	entries, err := listMigrationEntries()
	if err != nil {
		return fmt.Errorf("migration: list entries: %w", err)
	}

	applied := make(map[string]bool)
	if err := m.ensureTable(); err != nil {
		return err
	}
	appliedList, err := m.getAppliedList()
	if err != nil {
		appliedList = nil
	}
	for _, mig := range appliedList {
		applied[mig.ID] = true
	}

	fmt.Printf("%-30s %-12s %s\n", "Migration", "Batch", "Status")
	fmt.Println(strings.Repeat("-", 60))
	for _, entry := range entries {
		if applied[entry.ID] {
			fmt.Printf("%-30s %-12s Ran\n", entry.ID, "-")
		} else {
			fmt.Printf("%-30s %-12s Pending\n", entry.ID, "-")
		}
	}
	if len(entries) == 0 {
		fmt.Println("(no migrations found)")
	}
	return nil
}

// ---- 内部方法 ----

func (m *Migrator) ensureTable() error {
	d := m.db.Dialect()
	createSQL := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (\n  %s VARCHAR(255) PRIMARY KEY,\n  %s INT NOT NULL,\n  %s DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,\n  %s VARCHAR(255)\n)",
		d.Quote(m.tableName),
		d.Quote("id"),
		d.Quote("batch"),
		d.Quote("applied_at"),
		d.Quote("name"),
	)
	if m.dryRun {
		fmt.Println("[DRY RUN]", createSQL)
		return nil
	}
	_, err := m.db.exec(createSQL)
	return err
}

func (m *Migrator) getApplied() (map[string]bool, error) {
	applied := make(map[string]bool)
	rows, err := m.db.query(fmt.Sprintf("SELECT id FROM %s", m.db.Dialect().Quote(m.tableName)))
	if err != nil {
		return applied, nil // 表可能还不存在
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return applied, err
		}
		applied[id] = true
	}
	return applied, rows.Err()
}

func (m *Migrator) getAppliedList() ([]Migration, error) {
	rows, err := m.db.query(fmt.Sprintf(
		"SELECT id, batch, applied_at FROM %s ORDER BY id ASC",
		m.db.Dialect().Quote(m.tableName),
	))
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()

	var list []Migration
	for rows.Next() {
		var mig Migration
		var appliedAt sql.NullTime
		if err := rows.Scan(&mig.ID, &mig.Batch, &appliedAt); err != nil {
			return nil, err
		}
		if appliedAt.Valid {
			mig.AppliedAt = appliedAt.Time
		}
		list = append(list, mig)
	}
	return list, rows.Err()
}

func (m *Migrator) nextBatch() int {
	maxBatch := 0
	rows, err := m.db.query(fmt.Sprintf("SELECT COALESCE(MAX(batch), 0) FROM %s", m.db.Dialect().Quote(m.tableName)))
	if err != nil {
		return 1
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&maxBatch)
	}
	return maxBatch + 1
}

func (m *Migrator) record(id string, batch int) error {
	_, err := m.db.exec(fmt.Sprintf(
		"INSERT INTO %s (id, batch, applied_at, name) VALUES (?, ?, ?, ?)",
		m.db.Dialect().Quote(m.tableName),
	), id, batch, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

func (m *Migrator) remove(id string) error {
	_, err := m.db.exec(fmt.Sprintf(
		"DELETE FROM %s WHERE id = ?",
		m.db.Dialect().Quote(m.tableName),
	), id)
	return err
}

// ---- 迁移条目列表 ----

type migrationEntry struct {
	ID   string
	Up   func(db *DB) error
	Down func(db *DB) error
}

// Register 注册一个迁移。
// 迁移文件名会自动注册（文件名格式：YYYYMMDDHHmmss_name）。
// 需要在 init() 中调用。
var registeredMigrations []migrationEntry

// RegisterMigration 注册迁移。
func RegisterMigration(id string, up, down func(db *DB) error) {
	registeredMigrations = append(registeredMigrations, migrationEntry{
		ID:   id,
		Up:   up,
		Down: down,
	})
}

func listMigrationEntries() ([]migrationEntry, error) {
	// 按 ID 排序
	sort.Slice(registeredMigrations, func(i, j int) bool {
		return registeredMigrations[i].ID < registeredMigrations[j].ID
	})
	return registeredMigrations, nil
}
