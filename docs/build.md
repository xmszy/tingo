# 构建信息

`tbuild` 提供编译时注入的版本和构建信息，零外部依赖。

## 概念

构建信息通过 `go build -ldflags` 在编译时注入，运行时无需读取任何文件。

## 变量说明

| 变量 | 说明 | 示例值 |
|---|---|---|
| `t.BuildVersion` | 主版本号 | `"1.0.0"` |
| `t.BuildGitCommit` | Git 提交哈希 | `"a1b2c3d"` |
| `t.BuildTimeInfo` | 编译时间 | `"2026-08-05 10:30:00"` |

## 格式化版本

### FullVersion —— 完整版本字符串

~~~go
import "github.com/xmszy/tingo/frame"

fmt.Println(t.BuildFullVersion)
// 输出：1.0.0 (a1b2c3d, 2026-08-05 10:30:00)
~~~

### ShortVersion —— 简短版本号

~~~go
fmt.Println(t.BuildShortVersion)
// 输出：1.0.0
~~~

## 构建信息 Map

~~~go
info := t.BuildInfo
for k, v := range info {
    fmt.Printf("%s: %s\n", k, v)
}
// 输出：
// version: 1.0.0
// git_commit: a1b2c3d
// build_time: 2026-08-05 10:30:00
// full_version: 1.0.0 (a1b2c3d, 2026-08-05 10:30:00)
~~~

## 编译时注入

在构建脚本中通过 `-ldflags` 注入：

~~~bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S')

go build -ldflags "\
  -X 'github.com/xmszy/tingo/os/tbuild.Version=$VERSION' \
  -X 'github.com/xmszy/tingo/os/tbuild.GitCommit=$COMMIT' \
  -X 'github.com/xmszy/tingo/os/tbuild.BuildTime=$BUILD_TIME' \
" -o bin/server .
~~~

如果不注入，变量默认值均为 `"dev"`。

## 在应用中使用

~~~go
package main

import "github.com/xmszy/tingo/frame"

func main() {
    // 启动日志中打印版本
    t.Log().Infow("server starting",
        t.LogF("version", t.BuildFullVersion),
    )
    // ...
}
~~~

也常用于健康检查接口：

~~~go
func (c *HealthController) Version(ctx *t.Ctx) {
    t.JSON(ctx, t.Map{
        "version":   t.BuildVersion,
        "commit":    t.BuildGitCommit,
        "buildTime": t.BuildTimeInfo,
    })
}
~~~
