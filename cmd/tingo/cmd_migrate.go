package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/xmszy/tingo/database/tdb"
)

// migrateCmd 执行数据库迁移：tingo migrate <up|down|reset|status> [--dir PATH] [--connection NAME]
//   - up      执行所有待执行的迁移
//   - down    回滚最近一批迁移
//   - reset   回滚全部迁移（不重新执行）
//   - status  打印迁移执行状态
func migrateCmd(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dir := fs.String("dir", "database/migrations", "迁移文件目录")
	connection := fs.String("connection", "", "使用指定命名连接（默认主连接）")
	force := fs.Bool("force", false, "down/reset 时跳过确认直接执行")
	_ = fs.Parse(args)

	action := fs.Arg(0)
	switch action {
	case "up", "down", "reset", "status":
	default:
		fmt.Fprintln(os.Stderr, "用法：tingo migrate <up|down|reset|status> [--dir PATH] [--connection NAME] [--force]")
		os.Exit(1)
	}
	if (action == "down" || action == "reset") && !*force {
		fmt.Printf("即将执行 %q，此操作会回滚迁移数据。确认执行？(y/N) ", action)
		var confirm string
		fmt.Scanln(&confirm)
		if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
			fmt.Println("已取消。")
			return
		}
	}

	appConfig, err := tdb.LoadConfig("config/database.toml")
	if err != nil {
		failMigration("读取数据库配置失败", err)
	}
	dbConfig, resolveErr := appConfig.Resolve(*connection)
	if resolveErr != nil {
		failMigration("数据库连接未配置", resolveErr)
	}
	if dbConfig.Driver == "" {
		dbConfig.Driver, dbConfig.Dialect = "mysql", "mysql"
	}

	db, err := tdb.Open(dbConfig)
	if err != nil {
		if strings.Contains(err.Error(), "unknown driver") {
			fmt.Fprintf(os.Stderr, "连接数据库失败：%v\n", err)
			fmt.Fprintln(os.Stderr, "提示：请 import 对应数据库驱动后再运行 tingo migrate。")
			os.Exit(1)
		}
		failMigration("连接数据库失败", err)
	}
	defer db.Close()

	m := db.Migrator(*dir)
	switch action {
	case "up":
		if err := m.Up(); err != nil {
			failMigration("执行迁移失败", err)
		}
		fmt.Println("迁移完成。")
	case "down":
		if err := m.Down(); err != nil {
			failMigration("回滚迁移失败", err)
		}
		fmt.Println("已回滚最近一批迁移。")
	case "reset":
		if err := m.Reset(); err != nil {
			failMigration("重置迁移失败", err)
		}
		fmt.Println("已回滚全部迁移。")
	case "status":
		if err := m.Status(); err != nil {
			failMigration("查询迁移状态失败", err)
		}
	}
}

func failMigration(msg string, err error) {
	fmt.Fprintf(os.Stderr, "错误：%s：%v\n", msg, err)
	os.Exit(1)
}
