# 结构体工具

`tstructs` 提供结构体反射工具：Tag 解析、字段遍历、Tag 常量。零外部依赖。

## Tag 解析

### ParseTag —— 解析 valid 风格 tag

将 `|` 分隔的 tag 字符串解析为 key→value map：

~~~go
import "github.com/xmszy/tingo/frame"

m := t.StructsParseTag("required|len:3,20|in:1,2,3")
// map[string]string{
//   "required": "",
//   "len":      "3,20",
//   "in":       "1,2,3",
// }
~~~

### ParseTagStruct —— 解析 struct tag 风格

解析标准 Go struct tag 格式（`k:"v"` 形式）：

~~~go
m := t.StructsParseTagStruct(`json:"name" tdb:"user_name" description:"用户名称"`)
// map[string]string{
//   "json":        "name",
//   "tdb":         "user_name",
//   "description": "用户名称",
// }
~~~

## Tag 常量

框架定义了一组标准 Tag 常量，避免硬编码字符串：

| 常量 | 值 | 用途 |
|---|---|---|
| `t.TagJson` | `"json"` | JSON 序列化字段名 |
| `t.TagValid` | `"valid"` | 校验规则 |
| `t.TagTdb` | `"tdb"` | 数据库列名 |
| `t.TagDB` | `"db"` | 数据库列名（备用） |
| `t.TagDescription` | `"description"` | 字段描述 |
| `t.TagDefault` | `"default"` | 默认值 |
| `t.TagParam` | `"param"` | 请求参数名 |
| `t.TagExample` | `"example"` | 示例值 |
| `t.TagIn` | `"in"` | 输入方向标记 |
| `t.TagOut` | `"out"` | 输出方向标记 |
| `t.TagSummary` | `"summary"` | 摘要 |

### 使用示例

~~~go
type User struct {
    Name string `json:"name" tdb:"user_name" valid:"required|len:3,20" description:"用户名称"`
    Age  int    `json:"age"  tdb:"age"       valid:"required|min:1|max:150"   description:"年龄"`
}

// 获取单个字段的 tag 值
name := t.StructsTagValue(user, "Name", t.TagTdb) // "user_name"

// 获取所有字段的 tdb tag
m := t.StructsTagMap(user, t.TagTdb)
// map[string]string{"Name": "user_name", "Age": "age"}

// 按优先级查找 tag值→字段名 映射
m := t.StructsTagMapByName(user, []string{t.TagJson, t.TagTdb, t.TagDB})
// map[string]string{"name": "Name", "age": "Age"}
~~~

## Field 结构体

`t.StructsField` 封装单个字段的完整信息：

~~~go
type StructsField struct {
    Name       string        // Go 字段名
    Index      int           // 字段索引
    Tag        reflect.StructTag
    Type       reflect.Type
    Value      reflect.Value
    IsEmbedded bool          // 是否为匿名嵌入字段
    IsExported bool
}
~~~

### Field 方法

| 方法 | 说明 |
|---|---|
| `TagMap()` | 返回所有 tag 的 key→value map |
| `TagLookup(key)` | 查找 tag 键（返回 `value, ok`） |
| `TagDefault(key, def)` | 查找 tag 键（不存在返回默认值） |
| `TagJsonName()` | 返回 json tag 字段名（去 omitempty） |
| `TagDbName()` | 按优先级返回数据库列名 |
| `HasValidRule(rule)` | 判断是否定义了某校验规则 |

### 使用示例

~~~go
type Base struct {
    ID        int `json:"id" tdb:"id"`
    CreatedAt time.Time `json:"created_at" tdb:"created_at"`
}

type User struct {
    Base
    Name string `json:"name" tdb:"user_name" valid:"required|len:3,20"`
    Age  int    `json:"age" tdb:"age" valid:"required|min:1"`
}
~~~

## FieldsInfo —— 字段信息遍历

获取结构体的所有字段信息，支持递归展开嵌入字段：

~~~go
fields := t.StructsFieldsInfo(t.StructsFieldsInput{
    Object:    User{},
    Recursive: true,  // 递归展开嵌入字段
})

for _, f := range fields {
    fmt.Printf("%s: db=%s, json=%s\n",
        f.Name, f.TagDbName(), f.TagJsonName())

    if f.HasValidRule("required") {
        fmt.Printf("  → 必填字段\n")
    }
}

// 输出：
// ID: db=id, json=id
// CreatedAt: db=created_at, json=created_at
// Name: db=user_name, json=name
//   → 必填字段
// Age: db=age, json=age
//   → 必填字段
~~~

## 完整函数表

| 函数 | 说明 |
|---|---|
| `t.StructsParseTag(tag)` | 解析 valid 风格 tag |
| `t.StructsParseTagStruct(tag)` | 解析 struct tag 风格 |
| `t.StructsFieldsInfo(in)` | 字段信息列表（含嵌入递归） |
| `t.StructsTagMapByName(v, priority)` | tag值→字段名 映射 |
| `t.StructsTagValue(v, field, key)` | 获取字段 tag 值 |
| `t.StructsTagMap(v, key)` | 字段名→tag值 映射 |
| `t.StructsFields(v)` | 导出字段名列表 |
| `t.StructsFieldMap(v)` | 字段名→字段值 映射 |
| `t.StructsSetField(v, name, val)` | 设置字段值 |
| `t.StructsIsStruct(v)` | 判断是否结构体 |
| `t.StructsTypeName(v)` | 类型短名称 |
