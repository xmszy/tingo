// Package tconsole 提供通用的控制台指令注册与调度系统。
//
// 设计要点：
//   - Command 接口：每个指令实现 Name / Description / Run 三个方法。
//   - 分组支持：指令名使用 "group:name" 格式自动分组（如 make:controller）。
//   - Registry：线程安全的指令注册表，支持注册 / 查找 / 列表 / 帮助。
//   - 全局默认注册表 defaultRegistry，方便简单场景直接使用。
//
// 用法示例：
//
//	type HelloCommand struct{}
//	func (c *HelloCommand) Name() string        { return "hello" }
//	func (c *HelloCommand) Description() string { return "打印问候语" }
//	func (c *HelloCommand) Run(ctx context.Context, args []string) error {
//	    fmt.Println("Hello, Tingo!")
//	    return nil
//	}
//
//	reg := tconsole.NewRegistry()
//	reg.Register(&HelloCommand{})
//	reg.Run(context.Background(), "hello", nil)
//
// 若要在项目配置（config/console.toml）中注册自定义指令，
// 请在应用启动后调用 tconsole.DefaultRegistry().Register(yourCmd)。
package tconsole

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Command 控制台指令接口。
//
// 每个指令通过 Name() 返回唯一标识，支持 "group:name" 分组格式。
// 可选实现 ArgDefiner 接口来声明参数选项。
type Command interface {
	// Name 返回指令名称。支持 "group:name" 分组（如 "make:controller"）。
	Name() string
	// Description 返回简短描述（用于 list 和 help 显示）。
	Description() string
	// Run 执行指令。args 为指令参数（不含指令名称自身）。
	Run(ctx context.Context, args []string) error
}

// ArgDefiner 可选接口：声明指令的参数选项（用于 help 展示和 Parser 解析）。
type ArgDefiner interface {
	// Arguments 返回参数定义列表。
	Arguments() []Arg
	// Usage 返回使用示例（可选）。
	Usage() string
}

// ──────────────── Arg 参数定义 ────────────────

// Arg 定义指令接受的命令行选项。
type Arg struct {
	Name        string // 长参数名（如 "name"）
	Short       string // 短参数名（如 "n"）
	Default     string // 默认值
	Description string // 描述
	Required    bool   // 是否必填
}

// ──────────────── Parser 参数解析器 ────────────────

// Parser 解析命令行参数（--flag value / -f value 和位置参数）。
type Parser struct {
	flags map[string]string // --name → val, -n → val
	pos   []string          // 位置参数
}

// ParseArgs 解析 args 切片，将 --key val / -k val 提取为选项，其余为位置参数。
func ParseArgs(args []string) *Parser {
	p := &Parser{flags: make(map[string]string)}
	i := 0
	for i < len(args) {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			key := arg[2:]
			i++
			if i < len(args) && !strings.HasPrefix(args[i], "-") {
				p.flags[key] = args[i]
				i++
			} else {
				p.flags[key] = "true" // 标记型选项
			}
		} else if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			key := arg[1:] // -k → k
			i++
			if i < len(args) && !strings.HasPrefix(args[i], "-") {
				p.flags[key] = args[i]
				i++
			} else {
				p.flags[key] = "true"
			}
		} else {
			p.pos = append(p.pos, arg)
			i++
		}
	}
	return p
}

// Get 获取选项值，不存在返回空字符串。
func (p *Parser) Get(key string) string { return p.flags[key] }

// GetDefault 获取选项值，不存在返回默认值。
func (p *Parser) GetDefault(key, def string) string {
	if v, ok := p.flags[key]; ok {
		return v
	}
	return def
}

// Has 判断选项是否存在。
func (p *Parser) Has(key string) bool {
	_, ok := p.flags[key]
	return ok
}

// Pos 获取位置参数（按索引）。
func (p *Parser) Pos(index int) string {
	if index < len(p.pos) {
		return p.pos[index]
	}
	return ""
}

// PosAll 返回所有位置参数。
func (p *Parser) PosAll() []string { return p.pos }

// ──────────────── 子命令 ────────────────

// CommandNode 指令树节点，支持父子层级。
type CommandNode struct {
	Command
	parent   *CommandNode
	children map[string]*CommandNode
}

// AddChild 添加子命令节点。
func (n *CommandNode) AddChild(cmd *CommandNode) {
	cmd.parent = n
	n.children[cmd.Name()] = cmd
}

// Children 返回直接子节点列表。
func (n *CommandNode) Children() []*CommandNode {
	list := make([]*CommandNode, 0, len(n.children))
	for _, c := range n.children {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name() < list[j].Name()
	})
	return list
}

// FindChild 按名称查找子节点。
func (n *CommandNode) FindChild(name string) *CommandNode { return n.children[name] }

// NewCommandNode 创建指令树节点。
func NewCommandNode(cmd Command) *CommandNode {
	return &CommandNode{Command: cmd, children: make(map[string]*CommandNode)}
}

// NewRootCommand 创建根节点（无指令）。
func NewRootCommand() *CommandNode {
	return &CommandNode{children: make(map[string]*CommandNode)}
}

// Registry 控制台指令注册表。
//
// 线程安全，支持注册、查找、列表、帮助和执行。
type Registry struct {
	mu       sync.RWMutex
	commands map[string]Command
}

// NewRegistry 创建空的注册表。
func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

// Register 注册指令。若名称已存在则 panic。
func (r *Registry) Register(cmd Command) {
	name := cmd.Name()
	if name == "" {
		panic("tconsole: 指令名不能为空")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.commands[name]; ok {
		panic(fmt.Sprintf("tconsole: 指令 %q 重复注册", name))
	}
	r.commands[name] = cmd
}

// Get 根据名称获取指令，不存在返回 nil。
func (r *Registry) Get(name string) Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.commands[name]
}

// Commands 返回所有已注册的指令（按名称排序）。
func (r *Registry) Commands() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		list = append(list, cmd)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name() < list[j].Name()
	})
	return list
}

// Groups 返回按分组组织好的指令映射（group → []Command）。
func (r *Registry) Groups() map[string][]Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	groups := make(map[string][]Command)
	for _, cmd := range r.commands {
		name := cmd.Name()
		group, _ := splitName(name)
		groups[group] = append(groups[group], cmd)
	}
	for _, cmds := range groups {
		sort.Slice(cmds, func(i, j int) bool {
			return cmds[i].Name() < cmds[j].Name()
		})
	}
	return groups
}

// Run 查找并执行指令。
//
// 指令名支持完整名（如 "make:controller"）和别名查找。
func (r *Registry) Run(ctx context.Context, name string, args []string) error {
	cmd := r.Get(name)
	if cmd == nil {
		return fmt.Errorf("tconsole: 未知指令 %q，运行 \"list\" 查看可用指令", name)
	}
	return cmd.Run(ctx, args)
}

// splitName 拆分 "group:name" 为 (group, name)。
func splitName(full string) (group, name string) {
	idx := strings.Index(full, ":")
	if idx < 0 {
		return "", full
	}
	return full[:idx], full[idx+1:]
}

// ── 全局默认注册表 ────────────────────────────────────────────────

var defaultRegistry = NewRegistry()

// DefaultRegistry 返回全局默认注册表。
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// Register 向全局默认注册表注册指令。
func Register(cmd Command) {
	defaultRegistry.Register(cmd)
}

// ── 内置 help 指令 ────────────────────────────────────────────────

type helpCommand struct{}

func (c *helpCommand) Name() string        { return "help" }
func (c *helpCommand) Description() string { return "显示指令帮助" }
func (c *helpCommand) Run(ctx context.Context, args []string) error {
	reg := DefaultRegistry()
	if len(args) > 0 {
		cmd := reg.Get(args[0])
		if cmd == nil {
			return fmt.Errorf("未知指令 %q", args[0])
		}
		fmt.Printf("指令：%s\n", cmd.Name())
		fmt.Printf("描述：%s\n", cmd.Description())
		if argCmd, ok := cmd.(ArgDefiner); ok {
			if u := argCmd.Usage(); u != "" {
				fmt.Printf("用法：%s\n", u)
			}
			if as := argCmd.Arguments(); len(as) > 0 {
				fmt.Println("选项：")
				for _, a := range as {
					req := ""
					if a.Required {
						req = "（必填）"
					}
					name := "--" + a.Name
					if a.Short != "" {
						name += " / -" + a.Short
					}
					def := ""
					if a.Default != "" {
						def = fmt.Sprintf(" [默认: %s]", a.Default)
					}
					fmt.Printf("  %-20s %s%s%s\n", name, a.Description, req, def)
				}
			}
		}
		return nil
	}
	return (&listCommand{}).Run(ctx, nil)
}

// ── 内置 list 指令 ────────────────────────────────────────────────

type listCommand struct{}

func (c *listCommand) Name() string        { return "list" }
func (c *listCommand) Description() string { return "列出所有可用指令" }
func (c *listCommand) Run(ctx context.Context, args []string) error {
	reg := DefaultRegistry()
	cmds := reg.Commands()
	if len(cmds) == 0 {
		fmt.Println("（无可用指令）")
		return nil
	}
	groups := reg.Groups()
	// 先打印无分组的指令。
	if cmds, ok := groups[""]; ok {
		fmt.Println("可用指令：")
		for _, cmd := range cmds {
			fmt.Printf("  %-24s  %s\n", cmd.Name(), cmd.Description())
		}
		fmt.Println()
	}
	// 再按分组打印。
	groupNames := make([]string, 0, len(groups))
	for g := range groups {
		if g != "" {
			groupNames = append(groupNames, g)
		}
	}
	sort.Strings(groupNames)
	for _, g := range groupNames {
		fmt.Printf(" %s\n", g)
		for _, cmd := range groups[g] {
			fmt.Printf("  %-24s  %s\n", cmd.Name(), cmd.Description())
		}
		fmt.Println()
	}
	return nil
}

func init() {
	Register(&helpCommand{})
	Register(&listCommand{})
}
