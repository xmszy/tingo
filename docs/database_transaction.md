# 数据库事务

Tingo `tdb` 的事务采用**回调模式**：`DB.Tx(fn)` 在回调内自动管理 Begin/Commit/Rollback 和 panic 恢复。

## 基本用法

~~~go
err := t.DB().Tx(func(tx *tdb.Tx) error {
    // 所有操作绑定到该事务
    // 返回 nil → Commit
    // 返回 error → Rollback

    _, err := tdb.NewModelTx[Account](tx).
        WhereEQ("id", 1).
        Update(map[string]any{"balance": "balance - 100"})
    if err != nil {
        return err
    }

    _, err = tdb.NewModelTx[Order](tx).Insert(Order{
        UserId: 1,
        Amount: 100,
    })
    return err
})

if err != nil {
    log.Printf("事务失败: %v", err)
}
~~~

## 完整示例

~~~go
// 用户提现，账户扣款+记录创建，一个事务完成
func Withdraw(userId, amount int) error {
    return t.DB().Tx(func(tx *tdb.Tx) error {
        // 锁定账户行
        acc, err := tdb.NewModelTx[Account](tx).
            WhereEQ("user_id", userId).Scan()
        if err != nil {
            return err
        }
        if acc.Balance < amount {
            return errors.New("insufficient_balance")
        }

        // 扣款
        _, err = tdb.NewModelTx[Account](tx).
            WhereEQ("id", acc.Id).
            Update(map[string]any{"balance": acc.Balance - amount})
        if err != nil {
            return err
        }

        // 创建交易记录
        _, err = tdb.NewModelTx[WithdrawLog](tx).Insert(WithdrawLog{
            UserId: userId,
            Amount: amount,
        })
        return err
    })
}
~~~

## 事务逃生舱

`Tx.SQL()` 返回底层 `*sql.Tx`，可用标准库方式直接操作：

~~~go
err := t.DB().Tx(func(tx *tdb.Tx) error {
    sqlTx := tx.SQL()
    _, err := sqlTx.Exec("UPDATE account SET balance = balance - ? WHERE id = ?", 100, 1)
    return err
})
~~~

## 与 Tingo 事务的差异

| Tingo | tingo |
|---|---|
| `Db::startTrans()` | `DB.Tx(fn)` —— 不支持手动 Begin |
| `Db::commit()` / `Db::rollback()` | 回调内自动处理，根据 error 决定 |
| `Db::transaction(fn)` | `DB.Tx(fn)` —— 行为一致 |
| 嵌套事务（保存点） | 不内置，可在回调内手动通过 `tx.SQL()` 操作 |
