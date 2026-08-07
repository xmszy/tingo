# 命令行

Tingo 提供 `tingo` CLI 工具，对标 Tingo 的 `php think`。

## 安装

~~~bash
go install github.com/xmszy/tingo/cmd/tingo@latest
~~~

或本地使用：

~~~bash
go run ./cmd/tingo
~~~

## 常用命令

### version —— 版本信息

~~~bash
tingo version
~~~

输出 Tingo 版本号、Go 版本、平台信息。

### init —— 初始化项目

~~~bash
tingo init
tingo init myproject
~~~

生成 TP 风格的项目骨架。

### run —— 启动开发服务器

~~~bash
# 基本启动
tingo run

# 指定端口
tingo run --addr :9090

# 热重载（文件变更自动重启）
tingo run --watch
~~~

`--watch` 通过 mtime 轮询检测文件变化，重新编译并重启。

### route:list —— 路由列表

调试接口时的得力工具，列出应用中注册的全部路由：

~~~bash
# 列出所有路由
tingo route:list

# 按方法筛选
tingo route:list --method GET
tingo route:list --method POST
~~~

输出格式：

~~~
Method   Path                                      Handler
GET      /api/users                                main.(*UserController).Index
GET      /api/users/:id                            main.(*UserController).Show
POST     /api/users                                main.(*UserController).Store
PUT      /api/users/:id                            main.(*UserController).Update
DELETE   /api/users/:id                            main.(*UserController).Destroy
~~~

实现原理：编译项目 → 以 `TINGO_LIST_ROUTES=1` 环境变量运行 → 引擎打印路由表后退出，不启动服务。

### build —— 构建

~~~bash
# 基本构建
tingo build

# 指定输出
tingo build --output bin/server

# 交叉编译
tingo build --platform linux/amd64
tingo build --platform windows/amd64

# 同时生成 Dockerfile
tingo build --docker
~~~

### gen —— 代码生成

#### gen dao —— 逆向生成

连接数据库，反向生成 entity + model + dao 三件套（类似 GoFrame 范式）：

~~~bash
tingo gen dao
~~~

从数据库表生成：

- `app/entity/` —— 实体结构体（纯数据映射）
- `app/model/` —— 模型（带业务方法）
- `app/dao/` —— 数据访问对象（预定义查询）

#### gen model —— 生成模型

~~~bash
tingo gen model User
tingo gen model User --table user --fields id,name,email,age
~~~

#### gen controller —— 生成控制器

~~~bash
tingo gen controller User
~~~

生成标准资源控制器 `app/controller/user.go`。

### make —— 创建文件

根据模板生成骨架代码，支持 9 种类型：

~~~bash
# 控制器（7 个 REST 方法）
tingo make controller User

# 模型（泛型 Model[T]）
tingo make model User

# 中间件
tingo make middleware Auth

# 验证器
tingo make validate User

# 业务服务
tingo make service UserService

# 控制台指令
tingo make command ExportReport

# 事件载荷
tingo make event OrderPaid

# 事件监听器
tingo make listener SendNotification

# 事件订阅者
tingo make subscribe OrderSubscriber
~~~

**控制器变体：**

~~~bash
# API 控制器（5 方法，无 create/edit）
tingo make controller User --api

# 空控制器（仅结构体）
tingo make controller User --plain
~~~

**其他选项：**

~~~bash
# 覆盖已存在文件
tingo make controller User --force

# 指定自定义模板
tingo make controller User --stub my-custom.tpl
~~~

不指定 `--stub` 时自动查找项目 `stubs/<类型>.tpl`，找不到则使用内置模板。
内置模板存放于 `cmd/tingo/stubs/`，可作为自定义模板的参考。

**多应用：`@` 语法与 `app` 类型**

多应用项目用 `@` 表示“应用@名称”（参考 ThinkPHP 的 `php think make:controller admin/User`）：

~~~bash
# 在 admin 应用内生成 User 控制器
tingo make controller admin@User

# 等价于显式 --app
tingo make controller User --app admin

# 新建一个子应用（生成 app/<name>/ 骨架并维护聚合导入）
tingo make app api
~~~

`@` 内联语法优先于 `--app` 参数。生成的控制器位于 `app/<应用名>/controller/<Name>.go`；
`make app <name>` 会生成 `app/<name>/app.go` 与 `app/<name>/controller/index.go`，
并自动在 `app/applications.go` 追加匿名导入。多应用调度靠 `config/app.toml` 的 `[app]` 段，
详见 [多应用](./multi_app.md)。

## 自定义命令

Tingo 的 `tconsole` 包提供参数解析器和指令树，可用于开发自定义 CLI 命令。

### 定义命令

~~~go
import "github.com/xmszy/tingo/os/tconsole"

type GreetCommand struct{}

func (c *GreetCommand) Name() string        { return "greet" }
func (c *GreetCommand) Description() string  { return "打印问候语" }

// 可选：实现 ArgDefiner 接口声明参数
func (c *GreetCommand) Arguments() []tconsole.Arg {
    return []tconsole.Arg{
        {Name: "name", Short: "n", Description: "名称", Required: true},
        {Name: "lang", Short: "l", Default: "zh", Description: "语言"},
    }
}
func (c *GreetCommand) Usage() string { return "greet --name 张三 --lang zh" }

func (c *GreetCommand) Run(ctx context.Context, args []string) error {
    p := tconsole.ParseArgs(args)
    name := p.Get("name")
    lang := p.GetDefault("lang", "zh")

    switch lang {
    case "en":
        fmt.Printf("Hello, %s!\n", name)
    default:
        fmt.Printf("你好，%s！\n", name)
    }
    return nil
}
~~~

### 注册命令

~~~go
reg := tconsole.DefaultRegistry()
reg.Register(&GreetCommand{})

// 执行
ctx := context.Background()
reg.Run(ctx, "greet", "--name", "张三", "--lang", "zh")
~~~

### 指令树（子命令）

对于有父子层级的命令，使用 `CommandNode`：

~~~go
root := tconsole.NewRootCommand()
root.AddChild(tconsole.NewCommandNode(&BuildCommand{}))
root.AddChild(tconsole.NewCommandNode(&ServeCommand{}))

// 嵌套子命令
buildNode := root.FindChild("build")
buildNode.AddChild(tconsole.NewCommandNode(&BuildDockerCommand{}))
~~~

### Parser 参数解析

~~~go
args := []string{"--name", "张三", "--verbose", "extra-arg"}

p := tconsole.ParseArgs(args)

p.Get("name")              // "张三"
p.GetDefault("lang", "zh") // "zh"（未传入，使用默认值）
p.Has("verbose")           // true（标记型选项，值为 "true"）

p.Pos(0)  // "extra-arg"（位置参数）
p.PosAll() // ["extra-arg"]
~~~

### Help 增强

如果命令实现了 `ArgDefiner` 接口，`tingo help` 会自动显示参数选项表：

~~~bash
tingo help greet
# 指令：greet
# 描述：打印问候语
# 用法：greet --name 张三 --lang zh
# 选项：
#   --name / -n           名称（必填）
#   --lang / -l           语言 [默认: zh]
~~~
