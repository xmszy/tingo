# 助手函数

Tingo 提供类似 Tingo 的全局助手函数。

## tapp 助手

| 函数 | 说明 |
|---|---|
| `tapp.Req(c)` | 创建请求读取器 |
| `tapp.Abort(code, msg)` | 抛出 HTTP 异常 |
| `tapp.AbortIf(cond, code, msg)` | 条件抛出 |
| `tapp.AbortUnless(cond, code, msg)` | 条件不满足时抛出 |
| `tapp.Snake(s)` | 驼峰 → 蛇形（UserInfo → user_info） |
| `tapp.Camel(s)` | 蛇形 → 驼峰（user_info → UserInfo） |
| `tapp.LowerCamel(s)` | 蛇形 → 小驼峰（user_info → userInfo） |
| `tapp.Halt(code, msg)` | 停止执行 |

## t 门面别名

| 函数 | 说明 |
|---|---|
| `t.Map{}` | `gin.H` 别名 |
| `t.JSON(c, obj)` | JSON 响应 |
| `t.String(c, format, args...)` | 字符串响应 |
| `t.HTML(c, name, obj)` | HTML 响应 |
| `t.XML(c, obj)` | XML 响应 |
| `t.Redirect(c, url)` | 重定向 |
| `t.Data(c, contentType, data)` | 自定义内容类型响应 |
| `t.Status(c, code)` | 状态码响应 |
| `t.File(c, path)` | 文件下载 |
| `t.URL(name, params...)` | URL 生成 |
| `t.FullURL(c, name, params...)` | 完整 URL 生成 |

## 上下文键

| 函数 | 说明 |
|---|---|
| `t.Key[T](name)` | 泛型上下文键 |

## 日志快捷

| 函数 | 说明 |
|---|---|
| `t.LogInfow(msg, kvs...)` | Info 日志 |
| `t.LogDebugw(msg, kvs...)` | Debug 日志 |
| `t.LogWarnw(msg, kvs...)` | Warn 日志 |
| `t.LogErrorw(msg, kvs...)` | Error 日志 |
| `t.LogF(k, v)` | 日志字段对 |

## 环境变量

| 函数 | 说明 |
|---|---|
| `t.Env[T](key, def)` | 泛型读取环境变量 |
| `t.EnvMust[T](key)` | 必须存在，否则 panic |
| `t.EnvLookup(key)` | 返回 (string, bool) |
| `t.EnvHas(key)` | 判断存在 |
| `t.EnvExpand(s)` | 展开 ${VAR} 占位符 |

## 缓存快捷

| 函数 | 说明 |
|---|---|
| `t.CacheSet(cache, key, val, ttl)` | 设置缓存 |
| `t.CacheGet[T](cache, key)` | 泛型获取 |
| `t.CacheGetOrLoad[T](cache, key, ttl, loader)` | 获取或加载 |

## 会话

| 函数 | 说明 |
|---|---|
| `t.SessionGet[T](sess, key)` | 泛型获取会话值 |

## 字符串

| 函数 | 说明 |
|---|---|
| `t.StrHide(s, start, end, char)` | 隐藏中间部分 |
| `t.StrHideEmail(email)` | 隐藏邮箱用户名 |
| `t.StrAddSlashes(s)` | 转义特殊字符 |
| `t.StrStripSlashes(s)` | 去除转义 |
| `t.StrSimilarText(a, b)` | 字符串相似度 |
| `t.StrCompareVersion(a, b)` | 版本号比较 |
| `t.StrRandom(n)` | 随机字母数字串 |

## 文件

| 函数 | 说明 |
|---|---|
| `t.FileHome()` | 用户主目录 |
| `t.FileFormatSize(size)` | 字节数格式化 |
| `t.FileReadableSize(path)` | 文件可读大小 |
| `t.FileReplaceStrIn(path, old, new)` | 字符串内容替换 |
| `t.FileSortFiles(files, by, desc)` | 文件排序 |

## 构建信息

| 函数 | 说明 |
|---|---|
| `t.BuildVersion` | 编译版本号 |
| `t.BuildFullVersion` | 完整版本字符串 |
| `t.BuildInfo` | 构建信息 map |

## Context

| 函数 | 说明 |
|---|---|
| `t.CtxWithValue(ctx, key, val)` | 设置上下文值 |
| `t.CtxValue(ctx, key)` | 泛型读取 |

## 路径

| 函数 | 说明 |
|---|---|
| `t.PathSearch(paths, name)` | 搜索文件 |
| `t.PathSearchGlob(paths, pattern)` | Glob 搜索 |

## 数据库

| 函数 | 说明 |
|---|---|
| `t.Database()` | 默认数据库连接 |
| `t.DatabaseFor(name)` | 指定应用数据库 |
| `t.DatabaseFrom(c)` | 请求上下文数据库 |

如需 Tingo 中的其他助手函数，可通过门面访问或在 `app/common.go` 中封装。
