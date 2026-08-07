package tdb

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/xmszy/tingo/os/tcfg"
)

// ConventionConfig 是从约定配置文件中解码的数据库配置。
type ConventionConfig struct {
	Default     string                      `json:"default"`
	Connections map[string]ConnectionConfig `json:"connections"`
}

// ConnectionConfig 描述一个命名数据库连接。
type ConnectionConfig struct {
	Type     string `json:"type"`
	Driver   string `json:"driver"`
	DSN      string `json:"dsn"`
	Hostname string `json:"hostname"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	Hostport string `json:"hostport"`
	Charset  string `json:"charset"`
	Prefix   string `json:"prefix"`
	Schema   string `json:"schema"`
	MaxOpen  int    `json:"max_open"`
	MaxIdle  int    `json:"max_idle"`
	ReadOnly bool   `json:"read_only"`
}

// ConfigFromTree 从分层配置树的 database 命名空间解码数据库配置。
func ConfigFromTree(tree tcfg.Reader) (ConventionConfig, error) {
	cfg := ConventionConfig{Default: "mysql", Connections: map[string]ConnectionConfig{}}
	raw, ok := tree.Lookup("database")
	if !ok {
		return cfg, nil
	}
	if err := tcfg.Decode(raw, &cfg); err != nil {
		return ConventionConfig{}, err
	}
	if cfg.Connections == nil {
		cfg.Connections = map[string]ConnectionConfig{}
	}
	return cfg, nil
}

// LoadConfig 读取约定数据库配置。文件不存在表示应用未启用数据库。
func LoadConfig(path string) (ConventionConfig, error) {
	var values map[string]any
	found, err := tcfg.LoadFileInto(path, &values)
	if err != nil {
		return ConventionConfig{}, err
	}
	if !found {
		return ConventionConfig{Default: "mysql", Connections: map[string]ConnectionConfig{}}, nil
	}
	return ConfigFromTree(tcfg.Tree{"database": values})
}

// Resolve 返回默认或指定名称的底层连接配置。
func (c ConventionConfig) Resolve(names ...string) (Config, error) {
	name := c.Default
	if len(names) > 0 && strings.TrimSpace(names[0]) != "" {
		name = names[0]
	}
	connection, ok := c.Connections[name]
	if !ok {
		return Config{}, fmt.Errorf("tdb: connection %q is not configured", name)
	}
	return connection.resolve()
}

func (c ConventionConfig) managerConfig() (ManagerConfig, error) {
	connections := make(map[string]Config, len(c.Connections))
	for name, connection := range c.Connections {
		resolved, err := connection.resolve()
		if err != nil {
			return ManagerConfig{}, fmt.Errorf("tdb: connection %q: %w", name, err)
		}
		connections[name] = resolved
	}
	return ManagerConfig{Default: c.Default, Connections: connections}, nil
}

// Prefix 返回配置的表前缀，供模型生成器和业务约定使用。
func (c ConventionConfig) Prefix(name string) string { return c.Connections[name].Prefix }

func (c ConnectionConfig) resolve() (Config, error) {
	driver := normalizeDriver(c.Type)
	if driver == "" {
		driver = normalizeDriver(c.Driver)
	}
	if driver == "" {
		return Config{}, fmt.Errorf("tdb: database type is required")
	}
	dsn := c.DSN
	if dsn == "" {
		var err error
		dsn, err = c.buildDSN(driver)
		if err != nil {
			return Config{}, err
		}
	}
	return Config{
		Driver: driver, Dialect: driver, DSN: dsn, Schema: c.Schema, Prefix: c.Prefix,
		MaxOpen: c.MaxOpen, MaxIdle: c.MaxIdle, ReadOnly: c.ReadOnly,
	}, nil
}

func (c ConnectionConfig) buildDSN(driver string) (string, error) {
	host := c.Hostname
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.Hostport
	switch driver {
	case "mysql":
		if port == "" {
			port = "3306"
		}
		charset := c.Charset
		if charset == "" {
			charset = "utf8mb4"
		}
		return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=%s&parseTime=true&loc=Local",
			c.Username, c.Password, net.JoinHostPort(host, port), c.Database, url.QueryEscape(charset)), nil
	case "postgres":
		if port == "" {
			port = "5432"
		}
		u := &url.URL{Scheme: "postgres", Host: net.JoinHostPort(host, port), Path: "/" + c.Database}
		if c.Username != "" {
			u.User = url.UserPassword(c.Username, c.Password)
		}
		query := u.Query()
		query.Set("sslmode", "disable")
		u.RawQuery = query.Encode()
		return u.String(), nil
	case "sqlite":
		if c.Database == "" {
			return "runtime/database.sqlite", nil
		}
		return c.Database, nil
	case "sqlserver":
		if port == "" {
			port = "1433"
		}
		u := &url.URL{Scheme: "sqlserver", Host: net.JoinHostPort(host, port)}
		if c.Username != "" {
			u.User = url.UserPassword(c.Username, c.Password)
		}
		query := u.Query()
		query.Set("database", c.Database)
		u.RawQuery = query.Encode()
		return u.String(), nil
	default:
		return "", fmt.Errorf("tdb: cannot build DSN for driver %q", driver)
	}
}

func normalizeDriver(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "pgsql", "postgresql":
		return "postgres"
	case "sqlite3":
		return "sqlite"
	case "mssql", "sqlsrv":
		return "sqlserver"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}
