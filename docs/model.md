# 模型

泛型 `Model[T]` 为每个数据表提供类型安全的 ORM 操作。与 Tingo 的 Model 类似，
它自动推断表名与列名，支持链式查询、CRUD 和事务。

## 定义

~~~go
package model

import (
    "time"
    "github.com/xmszy/tingo/database/tdb"
)

type User struct {
    Id        int       `tdb:"id" json:"id"`
    Name      string    `tdb:"name" json:"name"`
    Age       int       `tdb:"age" json:"age"`
    Status    int       `tdb:"status" json:"status"`
    CreatedAt time.Time `tdb:"created_at" json:"created_at"`
    UpdatedAt time.Time `tdb:"updated_at" json:"updated_at"`
}

// TableName 可选：自定义表名
func (User) TableName() string {
    return "user"
}
~~~

## 创建模型实例

~~~go
import (
    "github.com/xmszy/tingo/database/tdb"
    "github.com/xmszy/tingo/frame"
)

// 方式一：门面
m := t.Model[User]()            // 使用 default 连接
m := t.Model[User]("user")      // 指定 connection

// 方式二：直接创建
db := tdb.DB("default")
m := tdb.NewModel[User](db)                // 默认表名
m := tdb.NewModel[User](db, "user")        // 显式表名
~~~

## 查询

### 基本查询

~~~go
// 查询所有
users, err := t.Model[User]().WhereEQ("status", 1).All()

// 条件查询
users, err := t.Model[User]().
    Where("age > ?", 18).
    WhereEQ("status", 1).
    Order("id desc").
    All()

// 查询单条
user, err := t.Model[User]().WhereEQ("id", 1).Scan()

// 查询单条，找不到不报错
user, err := t.Model[User]().WhereEQ("id", 1).Find()

// 查询到目标指针
var u User
err := t.Model[User]().WhereEQ("id", 1).One(&u)
~~~

### 聚合查询

~~~go
count, err := t.Model[User]().WhereEQ("status", 1).Count()
ok, err := t.Model[User]().WhereEQ("id", 1).Exists()
~~~

### 分页查询

~~~go
// 第 1 页，每页 20 条
users, err := t.Model[User]().WhereEQ("status", 1).Page(1, 20).All()

// 或使用 Limit + Offset
users, err := t.Model[User]().WhereEQ("status", 1).Limit(20).Offset(0).All()
~~~

### 连表查询

~~~go
type UserExt struct {
    User
    ProfileCity string `tdb:"city"`
}

users, err := t.Model[UserExt]().
    Fields("user.*", "profile.city").
    LeftJoin("profile", "user.id = profile.user_id").
    Where("user.status = ?", 1).
    All()
~~~

## 新增

~~~go
user := User{Name: "张三", Age: 25}

// 插入单条
result, err := t.Model[User]().Insert(user)
id, _ := result.LastInsertId()

// 忽略唯一键冲突
result, err := t.Model[User]().InsertIgnore(user)

// 冲突时更新
result, err := t.Model[User]().Upsert(user, "name")
~~~

## 更新

> **安全护栏**：无 Where 条件的 Update 会报错，必须调用 `AllowAll()` 确认。

~~~go
// 按结构体更新
_, err := t.Model[User]().
    WhereEQ("id", 1).
    Update(User{Name: "李四", Age: 30})

// 按 map 更新（部分字段）
_, err := t.Model[User]().
    WhereEQ("id", 1).
    Update(map[string]any{"name": "李四", "status": 1})

// 批量更新
_, err := t.Model[User]().WhereEQ("status", 0).Update(map[string]any{"status": 1})
~~~

## 删除

~~~go
// 按条件删除
_, err := t.Model[User]().WhereEQ("id", 1).Delete()

// 批量删除
_, err := t.Model[User]().Where("status < ?", 0).Delete()

// 整表删除（必须 AllowAll）
_, err := t.Model[User]().AllowAll().Delete()
~~~

## 事务

~~~go
err := t.DB().Tx(func(tx *tdb.Tx) error {
    tm := tdb.NewModelTx[User](tx)

    _, err := tm.WhereEQ("id", 1).Update(map[string]any{"balance": balance - 100})
    return err
})
~~~

## 表名推断

1. `NewModel[User](db, "custom")` —— 显式指定
2. `User` 实现 `TableName() string` 接口
3. 类型名 → snake_case + 复数化：`User` → `users`，`UserProfile` → `user_profiles`
4. `tdb:"table:xxx"` 标签

## 完整示例

~~~go
package model

import "github.com/xmszy/tingo/database/tdb"

type Article struct {
    Id      int    `tdb:"id" json:"id"`
    Title   string `tdb:"title" json:"title"`
    Content string `tdb:"content" json:"content"`
    Status  int    `tdb:"status" json:"status"`
}

// 方法封装在 model 层
func ArticleList(page, size int) ([]Article, int64, error) {
    m := tdb.NewModel[Article](tdb.DB("default"))

    total, err := m.WhereEQ("status", 1).Count()
    if err != nil {
        return nil, 0, err
    }

    items, err := m.WhereEQ("status", 1).
        Order("id desc").
        Page(page, size).
        All()
    return items, total, err
}

func ArticleDetail(id int) (Article, error) {
    return tdb.NewModel[Article](tdb.DB("default")).WhereEQ("id", id).Scan()
}
~~~

## 与 Tingo 模型的差异

| Tingo | tingo | 说明 |
|---|---|---|
| `$user = User::find(1)` | `m.WhereEQ("id", 1).Scan()` | 泛型返回，无魔法 __get |
| `$user->name` | `user.Name` | 直接字段访问 |
| 获取器 / 修改器（getter/setter） | 手写方法或 DTO 转换 | Go 类型安全，无需魔术方法 |
| `$user->save()` | `m.Update(user)` | 链式调用独立于实例 |
| `$user->delete()` | `m.WhereEQ("id", 1).Delete()` | 必须显式指定条件 |
| 关联预加载 `with()` | `With("Profile", HasOne(...))` | 7 种关联类型，支持嵌套预加载 |
| 软删除 | 需自行添加 `deleted_at IS NULL` 条件 | 不内置，由业务控制 |
