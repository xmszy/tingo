package tdb

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/xmszy/tingo/os/tcache"
	"github.com/xmszy/tingo/os/tevent"
)

// Config 数据库连接配置。驱动名需已在别处 import（如 import _ "github.com/go-sql-driver/mysql"）。
type Config struct {
	Driver   string // 驱动名：mysql/postgres/sqlite/sqlserver
	DSN      string // data source name（驱动特定格式）
	Dialect  string // 方言名，缺省取 Driver
	Schema   string // 默认 schema/库名（反向工程用，mysql=database，postgres=schema）
	Prefix   string // 数据表前缀，模型解析表名时自动追加
	MaxOpen  int    // 最大打开连接数，<=0 表示不限制
	MaxIdle  int    // 最大空闲连接数，<=0 表示不限制
	ReadOnly bool   // 只读模式（禁写，Insert/Update/Delete 直接报错）
	// 读写分离：ReadDSNs 为从库 DSN 列表（可选）。配置后，读操作在从库间轮询，
	// 写操作与主库（DSN）保持一致。为空表示无读写分离（读也走主库）。
	ReadDSNs []string
}

// DB 是一个数据库句柄，封装 *sql.DB 与方言。
// 零外部驱动依赖：具体驱动由调用方 import 注册给 database/sql。
type DB struct {
	sql      *sql.DB
	cfg      Config
	dial     Dialect
	driver   Driver
	mu       sync.RWMutex
	ro       bool
	schema   string        // 默认 schema/库名缓存
	cache    *tcache.Cache // ORM 查询缓存后端（可选）
	eventBus *tevent.Bus   // 模型事件总线（可选）
	slaves   []*sql.DB     // 从库连接池（读写分离），空表示无
	rr       uint64        // 从库轮询计数器
}

// Open 打开一个数据库连接（驱动需已注册）。
func Open(cfg Config) (*DB, error) {
	dialectName := cfg.Dialect
	if dialectName == "" {
		dialectName = cfg.Driver
	}

	driver, hasDriver := DriverFor(dialectName)
	dialect, hasDialect := DialectFor(dialectName)
	if hasDriver {
		dialect = driver.Dialect()
		hasDialect = dialect != nil
	}
	if !hasDialect {
		return nil, fmt.Errorf("tdb: dialect %q is not registered", dialectName)
	}

	var (
		sqlDB *sql.DB
		err   error
	)
	sqlDB, err = openSQL(driver, cfg, dialectName, cfg.DSN)
	if err != nil {
		return nil, err
	}
	db := &DB{sql: sqlDB, cfg: cfg, dial: dialect, driver: driver, ro: cfg.ReadOnly}
	if cfg.MaxOpen > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	}
	if cfg.MaxIdle > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	}
	// 打开从库连接（读写分离）
	for _, rdsn := range cfg.ReadDSNs {
		sdb, e := openSQL(driver, cfg, dialectName, rdsn)
		if e != nil {
			return nil, fmt.Errorf("tdb: open slave: %w", e)
		}
		if cfg.MaxOpen > 0 {
			sdb.SetMaxOpenConns(cfg.MaxOpen)
		}
		if cfg.MaxIdle > 0 {
			sdb.SetMaxIdleConns(cfg.MaxIdle)
		}
		db.slaves = append(db.slaves, sdb)
	}
	return db, nil
}

// openSQL 按驱动能力打开一个 *sql.DB（Connector 优先，否则 database/sql 直连）。
func openSQL(driver Driver, cfg Config, dialectName, dsn string) (*sql.DB, error) {
	if driver != nil && driver.Connector() != nil {
		return driver.Connector().Open(cfg)
	}
	driverName := cfg.Driver
	if driverName == "" {
		driverName = dialectName
	}
	return sql.Open(driverName, dsn)
}

// MustOpen 同 Open，失败 panic（启动期使用）。
func MustOpen(cfg Config) *DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}

// Close 关闭底层连接池（含从库）。
func (db *DB) Close() error {
	var firstErr error
	for _, s := range db.slaves {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := db.sql.Close(); err != nil {
		return err
	}
	return firstErr
}

// masterOrSlave 根据 forceMaster 选择连接：强制主库或事务内返回主库；
// 否则在从库间轮询（无配置从库时回退主库）。
func (db *DB) masterOrSlave(forceMaster bool) *sql.DB {
	if forceMaster || len(db.slaves) == 0 {
		return db.sql
	}
	db.mu.Lock()
	n := db.rr
	db.rr++
	db.mu.Unlock()
	return db.slaves[n%uint64(len(db.slaves))]
}

// SQL 返回底层 *sql.DB（高级用法逃生舱）。
func (db *DB) SQL() *sql.DB { return db.sql }

// Dialect 返回方言。
func (db *DB) Dialect() Dialect { return db.dial }

// Driver 返回统一驱动；兼容 database/sql 直连模式下可能为 nil。
func (db *DB) Driver() Driver { return db.driver }

// Capabilities 返回当前驱动能力。兼容直连模式返回零值。
func (db *DB) Capabilities() Capabilities {
	if db.driver == nil {
		return Capabilities{}
	}
	return db.driver.Capabilities()
}

// RequireCapability 验证当前驱动支持指定能力。
func (db *DB) RequireCapability(capability Capability) error {
	if db.Capabilities().Supports(capability) {
		return nil
	}
	driverName := db.dial.Name()
	if db.driver != nil {
		driverName = db.driver.Name()
	}
	return &CapabilityError{Driver: driverName, Capability: capability}
}

// SetReadOnly 设置只读模式。
func (db *DB) SetReadOnly(ro bool) {
	db.mu.Lock()
	db.ro = ro
	db.mu.Unlock()
}

// readOnly 读取只读标记。
func (db *DB) readOnly() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.ro
}

// Tx 开启一个事务，回调内可读写，返回 error 时自动回滚。
// 用法：err := db.Tx(func(tx *Tx) error { ... return nil })
func (db *DB) Tx(fn func(tx *Tx) error) error {
	sqlTx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	tx := &Tx{tx: sqlTx, db: db, dial: db.dial}
	if err := fn(tx); err != nil {
		if rb := tx.tx.Rollback(); rb != nil {
			return rb
		}
		return err
	}
	return tx.tx.Commit()
}

// TxCtx 等价于 Tx，但支持 context 超时/取消：ctx 结束时若事务仍未提交则自动回滚。
// 用法：
//
//	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
//	defer cancel()
//	err := db.TxCtx(ctx, func(tx *Tx) error { ... })
func (db *DB) TxCtx(ctx context.Context, fn func(tx *Tx) error) error {
	sqlTx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	tx := &Tx{tx: sqlTx, db: db, dial: db.dial}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = sqlTx.Rollback()
		case <-done:
		}
	}()
	defer close(done)

	if err := fn(tx); err != nil {
		if rb := tx.tx.Rollback(); rb != nil {
			return rb
		}
		return err
	}
	return tx.tx.Commit()
}

// SetCache 设置 ORM 查询缓存后端（可选）。
func (db *DB) SetCache(c *tcache.Cache) { db.cache = c }

// Cache 返回缓存后端。
func (db *DB) Cache() *tcache.Cache { return db.cache }

// SetEventBus 设置模型事件总线（可选）。
func (db *DB) SetEventBus(bus *tevent.Bus) { db.eventBus = bus }

// EventBus 返回事件总线。
func (db *DB) EventBus() *tevent.Bus { return db.eventBus }

// query 在 DB 上执行查询（供 Model/Tx 共用）。
func (db *DB) query(sqlStr string, args ...any) (*sql.Rows, error) {
	return db.sql.Query(sqlStr, args...)
}

func (db *DB) exec(sqlStr string, args ...any) (sql.Result, error) {
	if db.readOnly() {
		return nil, ErrReadOnly
	}
	return db.sql.Exec(sqlStr, args...)
}

// queryCtx 支持 context 的查询（ctx 为 nil 时退化为普通 Query）。
// forceMaster 为 true 时强制走主库，否则按读写分离策略选择从库。
func (db *DB) queryCtx(ctx context.Context, forceMaster bool, sqlStr string, args ...any) (*sql.Rows, error) {
	conn := db.masterOrSlave(forceMaster)
	if ctx == nil {
		return conn.Query(sqlStr, args...)
	}
	return conn.QueryContext(ctx, sqlStr, args...)
}

// execCtx 支持 context 的执行（ctx 为 nil 时退化为普通 Exec）。
func (db *DB) execCtx(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	if db.readOnly() {
		return nil, ErrReadOnly
	}
	conn := db.masterOrSlave(true)
	if ctx == nil {
		return conn.Exec(sqlStr, args...)
	}
	return conn.ExecContext(ctx, sqlStr, args...)
}
