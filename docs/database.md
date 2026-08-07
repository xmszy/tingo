# 数据库

Tingo `tdb` 是基于 `database/sql` 的泛型 ORM 引擎。方言抽象支持 MySQL / PostgreSQL / SQLite / SQL Server，
驱动由调用方通过 `database/sql` 的 `import _` 注册。

~~~go
import (
    "github.com/xmszy/tingo/database/tdb"
    _ "github.com/go-sql-driver/mysql"
)
~~~

## 连接

在 `config/database.toml` 中配置连接：

~~~toml
[default]
driver = "mysql"
dsn = "user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True"
debug = false
~~~

~~~go
db := tdb.DB("default")        // 获取 default 连接
userDB := tdb.DB("user")       // 获取 user 连接
~~~

## 查询构造器

泛型查询构造器 `tdb.NewModel[T]` 返回 `*Model[T]`，所有方法支持链式调用：

~~~go
type User struct {
    Id   int    `tdb:"id"`
    Name string `tdb:"name"`
    Age  int    `tdb:"age"`
}

users, err := tdb.NewModel[User](db).
    Fields("id", "name", "age").
    Where("age > ?", 18).
    Order("id desc").
    Limit(10).
    All()
~~~

### Fields —— 指定查询字段

~~~go
m.Fields("id", "name", "age")    // 多个字段
m.Fields("*")                     // 全字段（默认）
// 不调用 Fields 时自动 SELECT tdb 标签对应的列
~~~

### Where —— 条件查询

~~~go
// 参数化条件
m.Where("age > ?", 18)
m.Where("name LIKE ?", "%张%")
m.Where("status IN (?, ?, ?)", 1, 2, 3)
m.Where("age BETWEEN ? AND ?", 18, 60)
m.Where("deleted_at IS NULL")

// WhereEQ 等于条件
m.WhereEQ("name", "张三")

// 多次调用 And 连接
m.WhereEQ("status", 1).Where("age >= ?", 18)
// → WHERE status = ? AND age >= ?
~~~

### Order —— 排序

~~~go
m.Order("id desc")
m.Order("name asc").Order("id desc")
~~~

### Group / Having —— 分组

~~~go
m.Group("status").Having("COUNT(*) > ?", 5)
~~~

### Limit / Offset / Page —— 分页

~~~go
m.Limit(20).Offset(0)       // LIMIT 20 OFFSET 0
m.Page(1, 20)               // 等价：page 从 1 开始
m.Page(2, 10)               // LIMIT 10 OFFSET 10
~~~

### Distinct —— 去重

~~~go
m.Distinct().Fields("city").All()
~~~

### Join —— 连表

~~~go
m.Join("profile", "user.id = profile.user_id")
m.LeftJoin("profile", "user.id = profile.user_id")
m.RightJoin("profile", "user.id = profile.user_id")
~~~

## CRUD 操作

### 查询

~~~go
m := tdb.NewModel[User](db)

// 查询所有
users, err := m.WhereEQ("status", 1).All()

// 单条记录
user, err := m.WhereEQ("id", 1).Scan()

// 单条记录，找不到时返回零值（no rows 不报错）
user, err := m.WhereEQ("id", 1).Find()

// 查询一行到目标指针
var u User
err := m.WhereEQ("id", 1).One(&u)
~~~

### 聚合

~~~go
count, err := m.WhereEQ("status", 1).Count()   // SELECT COUNT(*)
ok, err := m.WhereEQ("id", 1).Exists()          // SELECT EXISTS(...)
~~~

### 新增

~~~go
// 插入单条
result, err := tdb.NewModel[User](db).Insert(User{Name: "张三", Age: 25})

// 获取自增 ID
id, err := result.LastInsertId()

// 批量插入（逐条）
for _, u := range users {
    tdb.NewModel[User](db).Insert(u)
}

// InsertIgnore —— 忽略唯一键冲突
result, err := tdb.NewModel[User](db).InsertIgnore(User{Name: "张三", Age: 25})

// Upsert —— 冲突时更新
result, err := tdb.NewModel[User](db).Upsert(User{Name: "张三", Age: 25}, "name")
~~~

### 更新

> **安全护栏**：更新/删除操作必须有 `Where` 条件，否则必须调用 `AllowAll()` 解除限制。

~~~go
// 按结构体更新
result, err := tdb.NewModel[User](db).
    WhereEQ("id", 1).
    Update(User{Name: "李四", Age: 30})

// 按 map 更新
result, err := tdb.NewModel[User](db).
    WhereEQ("id", 1).
    Update(map[string]any{"name": "李四", "age": 30})

// 整表更新（需 AllowAll）
result, err := tdb.NewModel[User](db).AllowAll().Update(map[string]any{"status": 0})
~~~

### 删除

~~~go
// 按条件删除
result, err := tdb.NewModel[User](db).
    WhereEQ("id", 1).
    Delete()

// 整表删除（需 AllowAll）
result, err := tdb.NewModel[User](db).AllowAll().Delete()
~~~

## 事务

~~~go
err := db.Tx(func(tx *tdb.Tx) error {
    // 事务内的 Model 使用 NewModelTx
    m := tdb.NewModelTx[User](tx)

    // 一系列操作
    _, err := m.WhereEQ("id", 1).Update(map[string]any{"name": "事务更新"})
    return err
    // 返回 nil → 提交，返回 error → 回滚
})
~~~

## 原生 SQL 逃生舱

~~~go
sqlDB := db.SQL() // *sql.DB
rows, err := sqlDB.Query("SELECT * FROM users WHERE age > ?", 18)
~~~

## 表名推断

表名按以下优先级确定：

1. `NewModel[User](db, "custom_table")` —— 显式指定
2. `User` 实现 `TableName() string` 接口
3. `User` 的 `tdb:"table:xxx"` 标签
4. 默认：类型名 → snake_case，复数化 → `users`

## 列名标签

`tdb` 结构体标签指定列名、JSON 标签作为后备：

~~~go
type User struct {
    Id   int    `tdb:"id" json:"id"`
    Name string `tdb:"user_name" json:"name"`
}
~~~

## 关联预加载

通过 `With()` 声明关联并在查询时自动加载，现有 7 种关联类型：

### HasOne（一对一）

~~~go
type User struct {
    Id      int      `tdb:"id"`
    Profile *Profile `tdb:"-"`       // 关联字段，标签 tdb:"-" 排除扫描
}

rel := tdb.HasOne[User, Profile]("id", "user_id")
users, err := tdb.NewModel[User](db).With("Profile", rel).All()
// SELECT * FROM users
// SELECT * FROM profiles WHERE user_id IN (...)
~~~

### HasMany（一对多）

~~~go
type User struct {
    Id     int      `tdb:"id"`
    Orders []*Order `tdb:"-"`
}

rel := tdb.HasMany[User, Order]("id", "user_id")
users, err := tdb.NewModel[User](db).With("Orders", rel).All()
~~~

### BelongsTo（属于）

~~~go
type Order struct {
    Id     int  `tdb:"id"`
    UserId int  `tdb:"user_id"`
    User   *User `tdb:"-"`
}

rel := tdb.BelongsTo[Order, User]("user_id", "id")
orders, err := tdb.NewModel[Order](db).With("User", rel).All()
~~~

### BelongsToMany（多对多）

~~~go
type User struct {
    Id    int    `tdb:"id"`
    Roles []*Role `tdb:"-"`
}

rel := tdb.BelongsToMany[User, Role]("user_role", "user_id", "role_id", "id", "id")
users, err := tdb.NewModel[User](db).With("Roles", rel).All()
// SELECT * FROM user_role WHERE user_id IN (...)
// SELECT * FROM roles WHERE id IN (...)
~~~

### HasOneThrough（穿透关联）

通过中间表获取远端关联，不需要在中间模型定义：

~~~go
// User → (经 user_history) → History
rel := tdb.HasOneThrough[User, History]("id", "user_id", "history_id", "id")
rel.SetPivot("user_history")
users, err := tdb.NewModel[User](db).With("LatestHistory", rel).All()
// SELECT h.* FROM history h
// INNER JOIN user_history p ON p.history_id = h.id
// WHERE p.user_id IN (...)
~~~

### MorphOne / MorphMany（多态关联）

一条关联同时属于多种模型（如评论可关联文章或视频）：

~~~go
type Comment struct {
    Id               int    `tdb:"id"`
    CommentableType  string `tdb:"commentable_type"`  // "article" / "video"
    CommentableId    int    `tdb:"commentable_id"`
}

type Article struct {
    Id       int        `tdb:"id"`
    Comments []*Comment `tdb:"-"`
}

rel := tdb.MorphMany[Article, Comment]("article", "commentable_type", "commentable_id")
articles, err := tdb.NewModel[Article](db).With("Comments", rel).All()
// SELECT * FROM comments WHERE commentable_type='article' AND commentable_id IN (...)
~~~

### 嵌套预加载

`With()` 支持点号路径，实现层级预加载：

~~~go
// 用户 → 资料 → 头像（三层嵌套）
users, err := tdb.NewModel[User](db).
    With("Profile", tdb.HasOne[User, Profile]("id", "user_id")).
    With("Profile.Avatar", tdb.HasOne[Profile, Avatar]("id", "profile_id")).
    All()
// 第一步：加载 Profile 到 user.Profile
// 第二步：在 Profile 上加载 Avatar 到 profile.Avatar
~~~

嵌套预加载按层级顺序执行，第二层在第一层完成之后才运行，确保数据完整。

### 懒加载

也可以使用 `Load` / `LoadAll` 按需加载已查出记录的关联：

~~~go
users, _ := tdb.NewModel[User](db).All()

// 懒加载关联
rel := tdb.HasOne[User, Profile]("id", "user_id")
tdb.LoadAll(db, &users, rel, "Profile")
~~~

## Schema DDL（表结构管理）

通过 `SchemaTool()` 获取 schema 管理器，执行 DDL 操作：

~~~go
sch := db.SchemaTool()

// 创建表
sch.CreateTable("users", func(table *tdb.Blueprint) {
    table.AddColumn("id", "int", tdb.AutoIncrement(true), tdb.PrimaryKey(true))
    table.AddColumn("name", "varchar(100)", tdb.NotNull(true))
    table.AddColumn("age", "int", tdb.Default("0"))
})

// 添加/修改/删除列
sch.AddColumn("users", "email", "varchar(255)")
sch.ModifyColumn("users", "age", "int", tdb.Default("18"))
sch.DropColumn("users", "age")

// 添加/删除索引
sch.AddIndex("users", "idx_name", "name")
sch.DropIndex("users", "idx_name")

// 添加/删除外键
sch.AddForeignKey("orders", "fk_orders_user", "user_id", "users", "id", "CASCADE", "RESTRICT")
sch.DropForeignKey("orders", "fk_orders_user")
~~~

外键约束参数说明：

| 参数 | 说明 |
|------|------|
| tableName | 子表 |
| constraintName | 约束名称 |
| column | 子表字段 |
| refTable | 父表 |
| refColumn | 父表字段 |
| onDelete | 删除级联（CASCADE / RESTRICT / SET NULL） |
| onUpdate | 更新级联 |

更多 Schema 操作（迁移、种子填充等）见 [数据库迁移](./database_transaction.md)。
