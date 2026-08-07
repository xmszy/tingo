# 控制台开发

`tconsole` 是命令行开发工具包，提供参数解析器、指令树和自定义命令注册机制。

## 参数解析器（Parser）

### ParseArgs —— 解析命令行参数

~~~go
import "github.com/xmszy/tingo/frame"

args := []string{"--name", "张三", "--verbose", "-c", "config.toml", "extra"}

p := t.ConsoleParseArgs(args)
~~~

支持两种选项格式：

- **长选项**：`--key value` 或 `--key`（标记型，值为 `"true"`）
- **短选项**：`-k value` 或 `-k`（标记型，值为 `"true"`）
- **位置参数**：不以 `-` 开头的独立值

### 读取选项

~~~go
p.Get("name")                  // "张三"
p.Get("verbose")               // "true"（标记型）
p.Get("c")                     // "config.toml"（短选项）

p.GetDefault("lang", "zh")     // "zh"（未传入，使用默认值）
p.Has("verbose")               // true（判断选项是否存在）
~~~

### 读取位置参数

~~~go
p.Pos(0)      // "extra"
p.Pos(1)      // ""（超出范围返回空）
p.PosAll()    // ["extra"]
~~~

## 自定义命令

### 基础命令

实现 `tconsole.Command` 接口：

~~~go
import (
    "context"
    "fmt"
    "github.com/xmszy/tingo/os/tconsole"
)

type GreetCommand struct{}

func (c *GreetCommand) Name() string        { return "greet" }
func (c *GreetCommand) Description() string  { return "打印问候语" }

func (c *GreetCommand) Run(ctx context.Context, args []string) error {
    p := tconsole.ParseArgs(args)
    name := p.GetDefault("name", "World")
    fmt.Printf("Hello, %s!\n", name)
    return nil
}
~~~

### 声明参数选项

可选实现 `ArgDefiner` 接口，声明命令接受的参数选项：

~~~go
// ArgDefiner 接口
type ArgDefiner interface {
    Arguments() []Arg  // 参数定义列表
    Usage() string      // 使用示例（可选）
}
~~~

实现后 `tingo help` 会自动展示参数表：

~~~go
func (c *GreetCommand) Arguments() []tconsole.Arg {
    return []tconsole.Arg{
        {
            Name:        "name",
            Short:       "n",
            Description: "名称",
            Required:    true,
        },
        {
            Name:        "lang",
            Short:       "l",
            Default:     "zh",
            Description: "语言（zh/en）",
        },
    }
}

func (c *GreetCommand) Usage() string {
    return "greet --name 张三 --lang zh"
}
~~~

### 注册命令

~~~go
reg := tconsole.DefaultRegistry()
reg.Register(&GreetCommand{})

// 通过注册表执行
ctx := context.Background()
reg.Run(ctx, "greet", "--name", "张三", "--lang", "en")
// 输出：Hello, 张三!
~~~

## 指令树（CommandNode）

对于有父子层级关系的命令，使用指令树：

~~~go
root := tconsole.NewRootCommand()

// 构建子命令
root.AddChild(tconsole.NewCommandNode(&BuildCommand{}))
root.AddChild(tconsole.NewCommandNode(&ServeCommand{}))

// 嵌套子命令
buildNode := root.FindChild("build")
buildNode.AddChild(tconsole.NewCommandNode(&BuildDockerCommand{}))
buildNode.AddChild(tconsole.NewCommandNode(&BuildBinaryCommand{}))
~~~

### CommandNode 方法

| 方法 | 说明 |
|---|---|
| `AddChild(cmd)` | 添加子命令节点 |
| `Children()` | 获取子节点列表（按名称排序） |
| `FindChild(name)` | 按名称查找子节点 |

## Help 增强

实现 `ArgDefiner` 后，`tingo help 命令名` 会显示完整的参数说明：

~~~bash
tingo help greet
~~~

输出：

~~~
指令：greet
描述：打印问候语
用法：greet --name 张三 --lang zh
选项：
  --name / -n           名称（必填）
  --lang / -l           语言（zh/en） [默认: zh]
~~~

## 完整类型表

| 类型/函数 | 说明 |
|---|---|
| `tconsole.Command` | 命令接口（Name/Description/Run） |
| `tconsole.ArgDefiner` | 参数定义接口（Arguments/Usage） |
| `tconsole.Arg` | 参数定义（Name/Short/Default/Description/Required） |
| `tconsole.Parser` | 参数解析器 |
| `tconsole.ParseArgs(args)` | 解析命令行参数 |
| `tconsole.CommandNode` | 指令树节点 |
| `tconsole.NewCommandNode(cmd)` | 创建指令树节点 |
| `tconsole.NewRootCommand()` | 创建根节点 |
| `tconsole.DefaultRegistry()` | 获取默认注册表 |
