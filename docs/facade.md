# 门面（Facade）

Tingo 的门面包 **`t`**（一个字母）是框架的统一入口，汇聚了所有核心能力。

## 使用门面

~~~go
import "github.com/xmszy/tingo/frame"

func handler(c *t.Ctx) {
    // 配置
    debug := t.Config().Bool("app.debug", false)

    // 日志
    t.Log().Infow("request", t.LogF("path", c.Path()))

    // 缓存
    t.CacheSet(cache, "key", value, time.Hour)

    // 数据库
    db := t.Database()
    db := t.DatabaseFor("admin")

    // 请求
    req := t.Req(c)
    name := req.Param("name")

    // 视图
    html, _ := t.View().Render("index", data)

    // 会话
    mgr := t.Session()
}
~~~

## 完整功能表

| 门面函数 | 说明 |
|---|---|
| **应用** | |
| `t.App(name, app)` | 注册应用 |
| `t.BaseApp` | 基类应用 |
| **配置** | |
| `t.Config()` | 全局配置 |
| `t.ConfigFor(name)` | 指定应用配置 |
| `t.ConfigFrom(c)` | 请求上下文配置 |
| `t.ConfigWithBytes(f, b)` | 从字节加载 |
| `t.ConfigLoader[T]` | 泛型配置绑定 |
| **日志** | |
| `t.Log()` | 默认 Logger |
| `t.LogNew(cfg)` | 新建 Logger |
| `t.LogInfow(msg, kvs...)` | 包级 Info |
| `t.LogF(k, v)` | 日志字段 |
| **缓存** | |
| `t.Cache()` | 默认缓存 |
| `t.CacheNew(opts)` | 新建缓存 |
| `t.CacheSet(c, k, v, ttl)` | 设置 |
| `t.CacheGet[T](c, k)` | 泛型获取 |
| **数据库** | |
| `t.Database()` | 默认数据库 |
| `t.DatabaseFor(name)` | 指定应用数据库 |
| `t.DatabaseFrom(c)` | 请求上下文数据库 |
| **请求** | |
| `t.Req(c)` | 请求读取器 |
| **响应** | |
| `t.Map{}` | gin.H 别名 |
| `t.JSON(c, data)` | JSON 响应 |
| `t.Redirect(c, url)` | 重定向 |
| **视图** | |
| `t.ViewNew(root, opts...)` | 新建视图引擎 |
| `t.View()` | 默认视图引擎 |
| **会话** | |
| `t.SessionNew(cfg)` | 新建会话管理器 |
| `t.SessionGet[T](sess, key)` | 泛型读取 |
| **事件** | |
| `t.BusNew(async)` | 新建事件总线 |
| `t.EventNew[T](name)` | 新建事件 |
| `t.BusSubscribe(bus, ev, fn)` | 订阅 |
| `t.BusDispatch(bus, ctx, ev, payload)` | 分发 |
| **队列** | |
| `t.QueueNew[T](async, retries)` | 新建任务队列 |
| **定时任务** | |
| `t.CronNew(logger)` | 新建调度器 |
| **环境变量** | |
| `t.Env[T](key, def)` | 泛型读取 |
| `t.EnvMust[T](key)` | 必须存在 |
| `t.EnvLookup(key)` | 检测存在 |
| `t.EnvHas(key)` | 判断存在 |
| **容器** | |
| `t.Bind[T](impl)` | 接口绑定 |
| `t.Make[T]()` | 泛型获取 |
| `t.Key[T](name)` | 上下文键 |
| **注册** | |
| `t.RegisterController(p, c)` | 注册控制器 |
| `t.AutoRoute(r)` | 自动路由 |
| `t.Provide(svc)` | 注册服务 |
| **字符串** | |
| `t.StrHide(s, start, end, char)` | 隐藏中间部分 |
| `t.StrHideEmail(email)` | 隐藏邮箱用户名 |
| `t.StrAddSlashes(s)` | 转义特殊字符 |
| `t.StrStripSlashes(s)` | 去除转义 |
| `t.StrSimilarText(a, b)` | 字符串相似度 [0,1] |
| `t.StrCompareVersion(a, b)` | 版本号比较（-1/0/1） |
| `t.StrRandom(n)` | 随机字母数字串 |
| `t.StrRandomNum(n)` | 随机数字串 |
| `t.StrRandomLetter(n)` | 随机字母串 |
| **文件** | |
| `t.FileHome()` | 用户主目录 |
| `t.FileHomeDir()` | 主目录（含 fallback） |
| `t.FileFormatSize(size)` | 字节数格式化 |
| `t.FileReadableSize(path)` | 文件可读大小 |
| `t.FileReplaceIn(path, old, new)` | 正则内容替换 |
| `t.FileReplaceStrIn(path, old, new)` | 字符串替换 |
| `t.FileSortFiles(files, by, desc)` | 文件排序 |
| `t.FileSortByName` | 按名称排序常量 |
| `t.FileSortByTime` | 按时间排序常量 |
| `t.FileSortBySize` | 按大小排序常量 |
| **结构体** | |
| `t.StructsParseTag(tag)` | 解析 valid 风格 tag |
| `t.StructsParseTagStruct(tag)` | 解析 struct tag 风格 |
| `t.StructsFieldsInfo(in)` | 字段信息列表（含嵌入递归） |
| `t.StructsTagMapByName(v, prio)` | tag值→字段名 映射 |
| `t.TagJson / t.TagValid / t.TagTdb / ...` | Tag 常量 |
| `t.StructsField` | 字段信息类型 |
| **命令行** | |
| `t.ConsoleParseArgs(args)` | 解析命令行参数 |
| `t.ConsoleCommandNodeNew(cmd)` | 创建指令树节点 |
| `t.ConsoleRootCommandNew()` | 创建根节点 |
| `t.ConsoleArg` | 参数定义类型 |
| `t.ConsoleParser` | 解析器类型 |
| **构建信息** | |
| `t.BuildVersion` | 编译版本号 |
| `t.BuildGitCommit` | Git 提交哈希 |
| `t.BuildTimeInfo` | 编译时间 |
| `t.BuildFullVersion` | 完整版本字符串 |
| `t.BuildShortVersion` | 简短版本号 |
| `t.BuildInfo` | 构建信息 map |
| **Context** | |
| `t.CtxWithValue(ctx, key, val)` | 设置上下文值 |
| `t.CtxValue(ctx, key)` | 泛型读取 |
| `t.CtxMustValue(ctx, key)` | 读取或零值 |
| `t.CtxWithValues(ctx, m)` | 批量设置 |
| **路径** | |
| `t.PathSearch(paths, name)` | 搜索文件 |
| `t.PathSearchGlob(paths, pattern)` | glob 搜索 |
| **助手** | |
| `tapp.Abort(code, msg)` | 抛出异常 |
| `tapp.Snake(s)` | 驼峰转蛇形 |
| `tapp.Camel(s)` | 蛇形转驼峰 |
