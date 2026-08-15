// Command tingo 是 tingo 框架的命令行工具。
//
// 子命令：
//   - version                  打印版本信息
//   - init <name>              根据项目名创建脚手架项目
//   - gen model [--tables ...] 从约定数据库配置反向生成模型
//   - gen controller <file>    生成资源控制器骨架
//   - make <type> <name>       生成代码骨架（controller/model/middleware/...）
//   - build [--output dir]     编译当前项目为可执行文件
//   - run [--addr :8080]       编译并运行当前项目
//   - route:list               列出所有已注册路由
//   - serve [--addr :8080]     等价于 run（别名）
//   - test [args...]           运行当前项目测试
//   - clean                    清理构建产物（根目录可执行文件、*.test、runtime/log）
//
// 用法示例：
//
//	go run ./cmd/tingo init myapp              # 创建 myapp/ 目录并初始化
//	go run ./cmd/tingo make controller User     # 生成资源控制器
//	go run ./cmd/tingo make controller User --api --force
//	go run ./cmd/tingo route:list               # 列出所有路由
//	go run ./cmd/tingo route:list --method GET  # 按方法筛选
package main

import (
	"fmt"
	"os"

	"github.com/xmszy/tingo"
	"github.com/xmszy/tingo/os/tenv"
)

// version 默认取自 tingo.Version，可由构建期 -ldflags "-X main.version=..." 覆盖。
var version = tingo.Version

func main() {
	// 提前加载 .env / .env.local，使 config/*.toml 中的 ${DB_PASS:-} 等
	// 环境变量占位符可被展开（与 Web 引擎启动时的行为一致）。
	// 文件不存在时忽略；已存在的系统环境变量不会被覆盖。
	_ = tenv.Load(".env", ".env.local")

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("tingo %s\n", version)
	case "init":
		initCmd(args)
	case "gen":
		genCmd(args)
	case "make":
		makeCmd(args)
	case "build":
		buildCmd(args)
	case "run", "serve":
		runCmd(args)
	case "route:list":
		routeListCmd(args)
	case "migrate":
		migrateCmd(args)
	case "test":
		testCmd(args)
	case "clean":
		cleanCmd(args)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`tingo - tingo 框架命令行工具

用法:
  tingo init <name>                    根据项目名创建脚手架项目（<name> 子目录）
  tingo gen model [--tables list]      按 config/database.toml 反向生成 app/model
  tingo gen controller <file>          生成资源控制器骨架
  tingo make <type> <name> [options]   生成代码骨架（见下方 make 类型列表）

  tingo build [--output dir] [--platform GOOS/GOARCH] [--docker]
                                       编译当前项目（支持交叉编译与 Dockerfile）
  tingo run [--addr :8080] [--watch]   编译并运行（--watch 热重载）
  tingo serve [--addr :8080]           run 的别名
  tingo route:list [--method GET]      列出所有已注册路由
  tingo migrate <up|down|reset|status> [--dir PATH] [--connection NAME] [--force]
                                       执行/回滚/查看数据库迁移
  tingo test [go-test args...]         运行当前项目测试
  tingo clean                          清理构建产物

  tingo version                        打印版本
  tingo help                           打印本帮助

make 类型:
  controller  资源控制器（支持 --api / --plain）
  model       数据模型（泛型 Model[T]）
  middleware  中间件
  validate    校验规则
  service     业务服务
  command     控制台指令
  event       事件载荷
  listener    事件监听器
  subscribe   事件订阅者

  选项: --app 应用名  --force 覆盖

更多：https://github.com/xmszy/tingo
`)
}
