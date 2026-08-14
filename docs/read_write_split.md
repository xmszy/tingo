# 读写分离

`database/tdb` 内建读写分离能力（对标 Tingo 的多数据库连接），配置后读操作自动走从库、
写操作与主库保持一致。

## 配置

在创建 `DB` 时通过 `Config.ReadDSNs` 指定从库 DSN 列表：

~~~go
db := tdb.NewDB(tdb.Config{
    DSN:      "user:pass@tcp(127.0.0.1:3306)/app",   // 主库
    ReadDSNs: []string{                               // 从库（可选，可多个）
        "user:pass@tcp(127.0.0.1:3307)/app",
        "user:pass@tcp(127.0.0.1:3308)/app",
    },
    MaxOpenConns: 10,
})
~~~

- `ReadDSNs` 为空：读写都走主库（`DSN`）。
- `ReadDSNs` 非空：读操作在从库间**轮询**负载均衡；写操作（Insert/Update/Delete）
  与事务始终走主库。
- 从库不可用时框架按列表顺序尝试，全部失败回退主库（具体策略见 `DB` 实现）。

## 默认行为

~~~go
// 读：自动走从库（配置了 ReadDSNs 时）
users, _ := tdb.Model[User](db).WhereEQ("status", 1).All()

// 写：始终走主库
_, _ = tdb.Model[User](db).WhereEQ("id", 1).Update(map[string]any{"name": "x"})
~~~

## 强制读主库（Master）

刚写入后需要**立即读到最新值**的强一致场景，用 `Master()` 强制本次查询走主库，
避免从库复制延迟导致读不到刚写入的数据：

~~~go
// 写入后立刻读取刚写入的行
_, _ = tdb.Model[User](db).WhereEQ("id", 1).Update(map[string]any{"name": "x"})
u, _ := tdb.Model[User](db).Master().WhereEQ("id", 1).FindOrFail()
~~~

> 经验法则：写后读、计数类校验、库存/余额等强一致读，务必加 `Master()`；
> 普通列表、详情展示等可容忍秒级延迟的读，保持默认走从库以提升吞吐。

## 与事务

事务内所有操作（含查询）都走主库，不受读写分离影响——事务要求强一致，
跨库无法保证：

~~~go
err := db.Tx(func(tx *tdb.Tx) error {
    tm := tdb.NewModelTx[User](tx)
    // 即使不调用 Master()，事务内查询也走主库
    u, err := tm.WhereEQ("id", 1).Scan()
    if err != nil {
        return err
    }
    return tm.WhereEQ("id", 1).Update(map[string]any{"name": u.Name + "_v2"})
})
~~~

## 小结

| 场景 | 走库 | 说明 |
|---|---|---|
| 普通 SELECT | 从库（轮询） | `ReadDSNs` 非空时 |
| Insert/Update/Delete | 主库 | 永远 |
| 事务内任意操作 | 主库 | 强一致 |
| `Master()` 后的 SELECT | 主库 | 写后读强一致 |
