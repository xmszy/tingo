package tdb

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
)

// Capability 是 ORM、元信息与生成器可检查的稳定能力标识。
type Capability string

const (
	CapabilityReturning             Capability = "returning"
	CapabilityUpsert                Capability = "upsert"
	CapabilitySavepoint             Capability = "savepoint"
	CapabilityLastInsertID          Capability = "last_insert_id"
	CapabilityNamedParameters       Capability = "named_parameters"
	CapabilityMetadata              Capability = "metadata"
	CapabilitySortingKeyMetadata    Capability = "sorting_key_metadata"
	CapabilitySkippingIndexMetadata Capability = "skipping_index_metadata"
)

// Capabilities 声明驱动真实支持的 SQL 能力，供 ORM 与生成器选择路径。
type Capabilities struct {
	Returning             bool
	Upsert                bool
	Savepoint             bool
	LastInsertID          bool
	NamedParameters       bool
	Metadata              bool
	SortingKeyMetadata    bool
	SkippingIndexMetadata bool
}

// Supports 判断驱动是否支持指定能力。未知能力始终返回 false。
func (c Capabilities) Supports(capability Capability) bool {
	switch capability {
	case CapabilityReturning:
		return c.Returning
	case CapabilityUpsert:
		return c.Upsert
	case CapabilitySavepoint:
		return c.Savepoint
	case CapabilityLastInsertID:
		return c.LastInsertID
	case CapabilityNamedParameters:
		return c.NamedParameters
	case CapabilityMetadata:
		return c.Metadata
	case CapabilitySortingKeyMetadata:
		return c.SortingKeyMetadata
	case CapabilitySkippingIndexMetadata:
		return c.SkippingIndexMetadata
	default:
		return false
	}
}

// Connector 负责创建 database/sql 连接池。nil Connector 使用 sql.Open 兼容路径。
type Connector interface {
	Open(Config) (*sql.DB, error)
}

type ConnectorFunc func(Config) (*sql.DB, error)

func (fn ConnectorFunc) Open(config Config) (*sql.DB, error) { return fn(config) }

// SQLConnector 将框架驱动名映射到实际 database/sql 注册名。
// 它适用于 mariadb -> mysql、oracle -> godror 等插件别名场景。
func SQLConnector(driverName string) Connector {
	return ConnectorFunc(func(config Config) (*sql.DB, error) {
		return sql.Open(driverName, config.DSN)
	})
}

// Driver 是 ORM、元信息和 CLI 共同依赖的统一数据库驱动协议。
type Driver interface {
	Name() string
	Connector() Connector
	Dialect() Dialect
	Metadata() SchemaDriver
	Capabilities() Capabilities
}

// DriverDefinition 用组合方式声明驱动，避免同族数据库复制样板实现。
type DriverDefinition struct {
	DriverName         string
	DriverConnector    Connector
	DriverDialect      Dialect
	MetadataDriver     SchemaDriver
	DriverCapabilities Capabilities
}

func (d DriverDefinition) Name() string               { return d.DriverName }
func (d DriverDefinition) Connector() Connector       { return d.DriverConnector }
func (d DriverDefinition) Dialect() Dialect           { return d.DriverDialect }
func (d DriverDefinition) Metadata() SchemaDriver     { return d.MetadataDriver }
func (d DriverDefinition) Capabilities() Capabilities { return d.DriverCapabilities }

var driverRegistry = struct {
	sync.RWMutex
	drivers map[string]Driver
}{drivers: make(map[string]Driver)}

// RegisterDriver 注册统一数据库驱动。重复名称返回错误，避免初始化顺序覆盖实现。
func RegisterDriver(driver Driver) error {
	if driver == nil {
		return fmt.Errorf("tdb: database driver must not be nil")
	}
	name := strings.ToLower(strings.TrimSpace(driver.Name()))
	if name == "" {
		return fmt.Errorf("tdb: database driver name must not be empty")
	}
	if driver.Dialect() == nil {
		return fmt.Errorf("tdb: database driver %q has no dialect", name)
	}
	driverRegistry.Lock()
	defer driverRegistry.Unlock()
	if _, exists := driverRegistry.drivers[name]; exists {
		return fmt.Errorf("tdb: database driver %q already registered", name)
	}
	driverRegistry.drivers[name] = driver
	return nil
}

// MustRegisterDriver 在包初始化期注册驱动，失败时 panic。
func MustRegisterDriver(driver Driver) {
	if err := RegisterDriver(driver); err != nil {
		panic(err)
	}
}

// DriverFor 查找统一数据库驱动。
func DriverFor(name string) (Driver, bool) {
	driverRegistry.RLock()
	driver, ok := driverRegistry.drivers[strings.ToLower(strings.TrimSpace(name))]
	driverRegistry.RUnlock()
	return driver, ok
}

// NewDriver 使用同名已注册方言组合数据库驱动定义。
func NewDriver(name string, metadata SchemaDriver, capabilities Capabilities) DriverDefinition {
	return NewDriverFrom(name, name, metadata, capabilities)
}

// NewDriverWithConnector 使用同名方言和显式连接器组合数据库驱动。
func NewDriverWithConnector(name string, connector Connector, metadata SchemaDriver, capabilities Capabilities) DriverDefinition {
	return NewDriverFromWithConnector(name, name, connector, metadata, capabilities)
}

// NewDriverFrom 使用基础方言组合兼容数据库驱动，同族数据库无需复制方言实现。
func NewDriverFrom(name, dialectName string, metadata SchemaDriver, capabilities Capabilities) DriverDefinition {
	return NewDriverFromWithConnector(name, dialectName, nil, metadata, capabilities)
}

// NewDriverFromWithConnector 使用基础方言和实际 database/sql 连接器组合兼容驱动。
func NewDriverFromWithConnector(name, dialectName string, connector Connector, metadata SchemaDriver, capabilities Capabilities) DriverDefinition {
	dialect, ok := DialectFor(dialectName)
	if !ok {
		panic(fmt.Sprintf("tdb: dialect %q is not registered", dialectName))
	}
	capabilities.Metadata = metadata != nil
	return DriverDefinition{
		DriverName:         name,
		DriverConnector:    connector,
		DriverDialect:      dialect,
		MetadataDriver:     metadata,
		DriverCapabilities: capabilities,
	}
}
