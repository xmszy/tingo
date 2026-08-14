package tdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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
	MaxOpen         int           // 最大打开连接数，<=0 表示不限制
	MaxIdle         int           // 最大空闲连接数，<=0 表示不限制
	ConnMaxLifetime time.Duration // 连接最大存活时间，<=0 表示不限制
	ConnMaxIdleTime time.Duration // 连接最大空闲时间，<=0 表示不限制
	ReadOnly        bool          // 只读模式（禁写，Insert/Update/Delete 直接报错）
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
	mu       sync.RWMutex  // 保护 slaves/ro 等运行时字段
	ro       bool
	schema   string        // 默认 schema/库名缓存
	cache    *tcache.Cache // ORM 查询缓存后端（可选）
	eventBus *tevent.Bus   // 模型事件总线（可选）
	slaves   []*sql.DB     // 从库连接池（读写分离），空表示无
	slaveDSNs []string     // 从库 DSN 列表（与 slaves 一一对应，用于断线重连）
	rr       uint64        // 从库轮询计数器
	ctx      context.Context // 查询默认 context（nil 表示 context.Background）
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
	applyPool(sqlDB, cfg)
	// 打开从库连接（读写分离）
	for _, rdsn := range cfg.ReadDSNs {
		sdb, e := openSQL(driver, cfg, dialectName, rdsn)
		if e != nil {
			return nil, fmt.Errorf("tdb: open slave: %w", e)
		}
		applyPool(sdb, cfg)
		db.slaves = append(db.slaves, sdb)
		db.slaveDSNs = append(db.slaveDSNs, rdsn)
	}
	return db, nil
}

// applyPool 将连接池参数应用到 *sql.DB（主库或从库通用）。
func applyPool(sqlDB *sql.DB, cfg Config) {
	if cfg.MaxOpen > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	}
	if cfg.MaxIdle > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
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
// 否则在从库间轮询（无配置从库时回退主库）。返回的 idx 为从库下标，
// 非从库场景 idx 为 -1（主库）。
func (db *DB) masterOrSlave(forceMaster bool) (conn *sql.DB, idx int) {
	if forceMaster || len(db.slaves) == 0 {
		return db.sql, -1
	}
	db.mu.Lock()
	n := db.rr
	db.rr++
	db.mu.Unlock()
	i := int(n % uint64(len(db.slaves)))
	return db.slaves[i], i
}

// isConnectionError 判断错误是否为底层连接断开（需触发从库重连）。
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	// database/sql 在连接失效时会返回 driver.ErrBadConn 或其包装。
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}
	// 常见网络/连接错误特征串（跨驱动兼容）。
	msg := err.Error()
	for _, kw := range []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"bad connection",
		"use of closed network connection",
		"EOF",
		"server closed",
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// reconnectSlave 关闭并重新打开指定下标的从库连接。调用方需持有写锁或确保并发安全。
func (db *DB) reconnectSlave(idx int) {
	dialectName := db.cfg.Dialect
	if dialectName == "" {
		dialectName = db.cfg.Driver
	}
	old := db.slaves[idx]
	if old != nil {
		_ = old.Close()
	}
	if sdb, e := openSQL(db.driver, db.cfg, dialectName, db.slaveDSNs[idx]); e == nil {
		applyPool(sdb, db.cfg)
		db.slaves[idx] = sdb
	}
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

// Transaction 开启一个事务，回调内可读写，返回 error 时自动回滚。
// 支持嵌套：在回调内再次调用 tx.Transaction(...) 会基于 SAVEPOINT 开启子事务
// （方言需支持 CapabilitySavepoint），子事务回滚仅回滚到保存点，不影响外层。
// 用法：
//
//	err := db.Transaction(context.Background(), func(tx *Tx) error {
//	    ...
//	    if err := tx.Transaction(ctx, func(sub *Tx) error { ... }); err != nil {
//	        return err // 仅回滚子事务
//	    }
//	    return nil
//	})
func (db *DB) Transaction(ctx context.Context, fn func(tx *Tx) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	sqlTx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	tx := &Tx{tx: sqlTx, db: db, dial: db.dial}

	// ctx 取消/超时：若事务仍未提交则自动回滚。
	done := make(chan struct{})
	if ctx.Done() != nil {
		go func() {
			select {
			case <-ctx.Done():
				_ = sqlTx.Rollback()
			case <-done:
			}
		}()
	}
	defer close(done)

	if err := fn(tx); err != nil {
		if rb := tx.tx.Rollback(); rb != nil {
			return rb
		}
		return err
	}
	return tx.tx.Commit()
}

// Tx 等价于 Transaction(context.Background(), fn)，保留以兼容旧调用。
func (db *DB) Tx(fn func(tx *Tx) error) error {
	return db.Transaction(context.Background(), fn)
}

// TxCtx 等价于 Transaction(ctx, fn)，保留以兼容旧调用。
func (db *DB) TxCtx(ctx context.Context, fn func(tx *Tx) error) error {
	return db.Transaction(ctx, fn)
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
	ctx := db.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return db.queryCtx(ctx, false, sqlStr, args...)
}

func (db *DB) exec(sqlStr string, args ...any) (sql.Result, error) {
	ctx := db.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return db.execCtx(ctx, sqlStr, args...)
}

// Exec 执行原生 SQL（写操作），返回 sql.Result。会经过连接池与读写分离（强制主库）。
// 这是原生 SQL 逃生舱：绕过 Model 的钩子/事件/软删除过滤，调用方需自行保证 SQL 正确与参数化。
func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.exec(query, args...)
}

// Query 执行原生 SQL（读操作），返回 *sql.Rows，调用方负责关闭。
// 会经过连接池与读写分离（默认从库，DB.Ctx 或 Master 语义不适用，统一走主库读以保证原生查询可控）。
func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.query(query, args...)
}

// Raw 是泛型便捷构造函数，返回一个原生 SQL 构建器，链式调用后可 Scan 到结构体。
// 用法：
//
//	var users []User
//	err := tdb.Raw[User](db, "SELECT * FROM user WHERE age > ?", 18).Scan(&users)
//
// 绕过 Model 的钩子/事件/软删除过滤，仅做结果扫描映射。
// （Go 不支持泛型方法，故以包级函数提供；db.Query/db.Exec 提供更低层的 *sql.Rows 逃生舱。）
func Raw[T any](db *DB, query string, args ...any) *RawQuery[T] {
	return &RawQuery[T]{db: db, query: query, args: args}
}

// Ctx 返回一个绑定了指定 context 的 DB 副本（不修改原 DB）。
// 之后的 query/exec 默认使用该 context（用于超时/取消透传）。
// 注意：仅共享底层连接与配置，副本拥有独立的 ctx 与互斥锁（零值可用）。
func (db *DB) Ctx(ctx context.Context) *DB {
	return &DB{
		sql:      db.sql,
		cfg:      db.cfg,
		dial:     db.dial,
		driver:   db.driver,
		ro:       db.ro,
		schema:   db.schema,
		cache:    db.cache,
		eventBus: db.eventBus,
		slaves:   db.slaves,
		rr:       db.rr,
		ctx:      ctx,
	}
}


// queryCtx 支持 context 的查询（ctx 为 nil 时退化为普通 Query）。
// forceMaster 为 true 时强制走主库，否则按读写分离策略选择从库。
// 若选择从库且查询因连接断开失败，自动重连该从库并重试一次（最多一次，避免死循环）。
func (db *DB) queryCtx(ctx context.Context, forceMaster bool, sqlStr string, args ...any) (*sql.Rows, error) {
	conn, idx := db.masterOrSlave(forceMaster)
	var rows *sql.Rows
	var err error
	if ctx == nil {
		rows, err = conn.Query(sqlStr, args...)
	} else {
		rows, err = conn.QueryContext(ctx, sqlStr, args...)
	}
	if err != nil && idx >= 0 && isConnectionError(err) {
		db.mu.Lock()
		db.reconnectSlave(idx)
		conn = db.slaves[idx]
		db.mu.Unlock()
		if ctx == nil {
			return conn.Query(sqlStr, args...)
		}
		return conn.QueryContext(ctx, sqlStr, args...)
	}
	return rows, err
}

// execCtx 支持 context 的执行（ctx 为 nil 时退化为普通 Exec）。
func (db *DB) execCtx(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	if db.readOnly() {
		return nil, ErrReadOnly
	}
	conn, _ := db.masterOrSlave(true)
	if ctx == nil {
		return conn.Exec(sqlStr, args...)
	}
	return conn.ExecContext(ctx, sqlStr, args...)
}
