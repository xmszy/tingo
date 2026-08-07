package tsession

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/xmszy/tingo/database/tdb"
)

// DBStore 是基于 tdb 的会话存储（数据存于 sessions 表：
// id VARCHAR 主键，data TEXT(JSON)，expires BIGINT）。
type DBStore struct {
	db  *tdb.DB
	ttl time.Duration
}

// NewDBStore 创建数据库存储。表结构需由调用方预建：
//
//	CREATE TABLE sessions(id VARCHAR(64) PRIMARY KEY, data TEXT, expires BIGINT);
func NewDBStore(db *tdb.DB) *DBStore {
	return &DBStore{db: db}
}

// Load 实现 Store。
func (d *DBStore) Load(id string) (map[string]any, error) {
	var data sql.NullString
	var expires int64
	row := d.db.SQL().QueryRow("SELECT data, expires FROM sessions WHERE id = ? AND expires > ?", id, time.Now().Unix())
	if err := row.Scan(&data, &expires); err != nil {
		if err == sql.ErrNoRows {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if !data.Valid || data.String == "" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(data.String), &m); err != nil {
		return map[string]any{}, nil
	}
	return m, nil
}

// Save 实现 Store。
func (d *DBStore) Save(id string, data map[string]any, ttl time.Duration) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	expires := time.Now().Add(ttl).Unix()
	// UPSERT：优先 UPDATE，无影响则 INSERT（兼容多数库；sqlite/mysql 语法略有差异，
	// 这里采用通用两步骤，驱动层冲突可忽略）。
	res, err := d.db.SQL().Exec("UPDATE sessions SET data = ?, expires = ? WHERE id = ?", string(b), expires, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		_, err = d.db.SQL().Exec("INSERT INTO sessions(id, data, expires) VALUES(?, ?, ?)", id, string(b), expires)
	}
	return err
}

// Destroy 实现 Store。
func (d *DBStore) Destroy(id string) error {
	_, err := d.db.SQL().Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}
