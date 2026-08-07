# 字符串工具

`tstr` 提供常用字符串处理函数，零外部依赖。

## 脱敏（Hide）

### HideStr —— 隐藏中间部分

用指定字符替换字符串中间部分：

~~~go
import "github.com/xmszy/tingo/frame"

t.StrHide("13812345678", 3, 7, "*")  // "138****5678"
t.StrHide("zhangsan@qq.com", 2, 6, "*") // "zh****an@qq.com"
~~~

参数：`HideStr(s, start, end, char)` —— start/end 为 rune 索引（支持中文）。

### HideEmail —— 隐藏邮箱用户名

~~~go
t.StrHideEmail("zhangsan@example.com") // "zha***san@example.com"
t.StrHideEmail("a@b.com")              // "a***@b.com"
~~~

## 转义（Slashes）

### AddSlashes —— 转义特殊字符

~~~go
t.StrAddSlashes(`It's "ok"`) // `It\'s \"ok\"`
t.StrAddSlashes(`path\to\file`) // `path\\to\\file`
~~~

### StripSlashes —— 去除转义

~~~go
t.StrStripSlashes(`It\'s \"ok\"`) // `It's "ok"`
t.StrStripSlashes(`hello\nworld`) // "hello"+换行+"world"
~~~

支持的转义序列：`\n`、`\r`、`\t`、`\\`、`\'`、`\"`。

## 相似度

### SimilarText —— Levenshtein 距离

计算两个字符串的相似度，返回 `[0, 1]`，1 表示完全相同：

~~~go
t.StrSimilarText("hello", "hallo")     // 0.8
t.StrSimilarText("hello", "world")     // 0.2
t.StrSimilarText("你好世界", "你好")    // 0.5
t.StrSimilarText("abc", "abc")         // 1.0
~~~

使用滚动数组优化空间复杂度为 `O(min(len(a), len(b)))`。

## 版本比较

### CompareVersion —— 版本号比较

~~~go
t.StrCompareVersion("1.2.3", "1.2.4")   // -1（a < b）
t.StrCompareVersion("2.0.0", "1.9.9")   // 1（a > b）
t.StrCompareVersion("1.0.0", "1.0.0")   // 0（a == b）
t.StrCompareVersion("1.2", "1.2.0")     // 0（等长补零）
~~~

## 随机字符串

### Random —— 随机字母数字串

~~~go
t.StrRandom(16)     // "aB3xK9mN2pQ7rT5v"
~~~

字符集：`a-z A-Z 0-9`。

### RandomNum —— 随机数字串

~~~go
t.StrRandomNum(6)   // "384921"
~~~

字符集：`0-9`。

### RandomLetter —— 随机字母串

~~~go
t.StrRandomLetter(8) // "AbCdEfGh"
~~~

字符集：`a-z A-Z`。

## 完整函数表

| 函数 | 说明 |
|---|---|
| `t.StrHide(s, start, end, char)` | 隐藏中间部分 |
| `t.StrHideEmail(email)` | 隐藏邮箱用户名 |
| `t.StrAddSlashes(s)` | 转义 `'` `"` `\` |
| `t.StrStripSlashes(s)` | 去除转义 |
| `t.StrSimilarText(a, b)` | Levenshtein 相似度 |
| `t.StrCompareVersion(a, b)` | 版本号比较 |
| `t.StrRandom(n)` | 随机字母数字串 |
| `t.StrRandomNum(n)` | 随机数字串 |
| `t.StrRandomLetter(n)` | 随机字母串 |
