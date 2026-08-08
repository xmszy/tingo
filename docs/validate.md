# 验证

Tingo 的验证组件 `tingo-contrib/validate`（独立模块）提供 Tingo 风格的参数校验。

## 安装

~~~go
import "github.com/xmszy/tingo-contrib/validate"
~~~

## 基本用法

### 定义规则

~~~go
rules := []validate.Rule{
    {Field: "name", Rule: "require", Msg: "用户名为必填"},
    {Field: "name", Rule: "length:2,20", Msg: "用户名长度 2-20"},
    {Field: "email", Rule: "require|email", Msg: "邮箱格式不正确"},
    {Field: "age", Rule: "require|integer|between:1,120", Msg: "年龄 1-120"},
    {Field: "password", Rule: "require|alphaDash|length:6,20", Msg: "密码 6-20 位"},
    {Field: "repassword", Rule: "require|confirm:password", Msg: "两次密码不一致"},
}
~~~

### 执行验证

~~~go
err := validate.Check(input, rules)
if err != nil {
    // err 包含所有校验错误
    c.JSON(422, t.Map{"code": 422, "message": err.Error()})
    return
}
~~~

## 内置规则

| 规则 | 说明 | 参数 |
|---|---|---|
| `require` | 必填 | - |
| `number` | 纯数字 | - |
| `integer` | 整数 | - |
| `float` | 浮点数 | - |
| `boolean` | 布尔型 | - |
| `email` | 邮箱格式 | - |
| `array` | 数组 | - |
| `accepted` | yes/on/1 之一 | - |
| `date` | 有效日期 | - |
| `alpha` | 纯字母 | - |
| `alphaNum` | 字母和数字 | - |
| `alphaDash` | 字母数字下划线横线 | - |
| `chs` | 纯汉字 | - |
| `chsAlpha` | 汉字和字母 | - |
| `chsAlphaNum` | 汉字字母数字 | - |
| `url` | URL 格式 | - |
| `ip` | IP 地址 | - |
| `json` | JSON 格式 | - |
| `length` | 长度范围 | `min,max` |
| `min` | 最小值/最短长度 | `min` |
| `max` | 最大值/最长长度 | `max` |
| `between` | 值范围 | `min,max` |
| `in` | 值集合 | `v1,v2,v3` |
| `notIn` | 不在集合中 | `v1,v2,v3` |
| `confirm` | 确认字段 | `field` |
| `different` | 不等于字段 | `field` |
| `eq` | 等于 | `val` |
| `regex` | 正则 | `pattern` |
| `unique` | 数据库唯一 | `table,field,except` |
| `exists` | 数据库存在 | `table,field` |

## 提示信息

自定义规则提示：

~~~go
// 单条信息
{Field: "name", Rule: "require", Msg: "请输入用户名"}

// 多条规则
{Field: "name", Rule: "require|length:2,20", Msg: "用户名 2-20 位"}
{Field: "name", Rule: "require|length:2,20", Msg: "{:attr} 为必填"}

// 批量验证
{Field: "name,email", Rule: "require", Msg: "必填项"}
~~~

## 自定义规则

~~~go
// 注册自定义规则
validate.AddRule("mobile", func(value interface{}, rule string, data map[string]interface{}) bool {
    s := fmt.Sprint(value)
    return len(s) == 11 && strings.HasPrefix(s, "1")
})

// 使用自定义规则
rules := []validate.Rule{
    {Field: "phone", Rule: "mobile", Msg: "手机号格式不正确"},
}
~~~

## 控制器集成

通过 `BindValidate` 将验证与参数绑定结合：

~~~go
func (u *User) Save(c *t.Ctx) error {
    var input SaveReq
    rules := []validate.Rule{
        {Field: "Name", Rule: "require|length:2,20", Msg: "用户名 2-20 位"},
        {Field: "Email", Rule: "require|email", Msg: "邮箱格式不正确"},
    }
    if err := u.BindValidate(c, &input, rules); err != nil {
        return err  // 框架自动返回验证错误
    }
    return u.svc.Create(&input)
}
~~~
