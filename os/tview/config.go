package tview

import "github.com/xmszy/tingo/os/tcfg"

// Config 描述约定式视图配置。
type Config struct {
	Root       string `json:"root"`
	Extension  string `json:"extension"`
	LeftDelim  string `json:"left_delim"`
	RightDelim string `json:"right_delim"`
}

// ConfigFromTree 从 view 命名空间构造视图配置。
func ConfigFromTree(tree tcfg.Reader) Config {
	return Config{
		Root:       tree.String("view.root", "app/view"),
		Extension:  tree.String("view.extension", ".html"),
		LeftDelim:  tree.String("view.left_delim", "{{"),
		RightDelim: tree.String("view.right_delim", "}}"),
	}
}

// NewWithConfig 从强类型配置创建视图引擎。
func NewWithConfig(cfg Config) *Engine {
	return New(cfg.Root, WithExt(cfg.Extension), WithDelims(cfg.LeftDelim, cfg.RightDelim))
}

// NewFromTree 从配置树创建视图引擎。
func NewFromTree(tree tcfg.Reader) *Engine { return NewWithConfig(ConfigFromTree(tree)) }
