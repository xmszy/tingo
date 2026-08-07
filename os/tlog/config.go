package tlog

import (
	"fmt"
	"strings"

	"github.com/xmszy/tingo/os/tcfg"
)

// ConfigFromTree 从 log 命名空间构造日志配置。
func ConfigFromTree(tree tcfg.Reader) (Config, error) {
	cfg := DefaultConfig()
	prefix := "log."
	if tree.Has(prefix + "level") {
		level, err := parseLevel(tree.String(prefix + "level"))
		if err != nil {
			return Config{}, err
		}
		cfg.Level = level
	}
	if tree.Has(prefix + "async") {
		cfg.Async = tree.Bool(prefix + "async")
	}
	if tree.Has(prefix + "async_buffer") {
		cfg.AsyncBuffer = tree.Int(prefix+"async_buffer", cfg.AsyncBuffer)
	}
	if tree.Has(prefix + "time_format") {
		cfg.TimeFormat = tree.String(prefix + "time_format")
	}
	if tree.Has(prefix + "prefix") {
		cfg.Prefix = tree.String(prefix + "prefix")
	}
	if tree.Has(prefix + "flags") {
		flags, err := parseFlags(tree.Strings(prefix + "flags"))
		if err != nil {
			return Config{}, err
		}
		cfg.Flags = flags
	}
	return cfg, nil
}

// NewFromTree 从配置树创建日志实例。
func NewFromTree(tree tcfg.Reader) (*Logger, error) {
	cfg, err := ConfigFromTree(tree)
	if err != nil {
		return nil, err
	}
	return NewWithConfig(cfg), nil
}

func parseLevel(value string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return LevelDebug, nil
	case "info", "":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		return 0, fmt.Errorf("tlog: unsupported level %q", value)
	}
}

func parseFlags(values []string) (Flag, error) {
	var flags Flag
	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "time":
			flags |= FTime
		case "file":
			flags |= FFile
		case "func", "function":
			flags |= FFunc
		case "level":
			flags |= FLevel
		case "":
		default:
			return 0, fmt.Errorf("tlog: unsupported flag %q", value)
		}
	}
	return flags, nil
}
