package tdb

// RawQuery 是原生 SQL 查询构建器，用于执行未封装到 Model 的 SQL 并直接映射到结构体。
// 绕过 Model 的钩子/事件/软删除过滤——调用方需自行保证 SQL 正确并对用户输入参数化。
//
// 用法（通过包级函数 Raw 构造）：
//
//	var users []User
//	err := tdb.Raw[User](db, "SELECT * FROM user WHERE age > ?", 18).Scan(&users)
type RawQuery[T any] struct {
	db    *DB
	query string
	args  []any
}

// Scan 执行原生 SQL 并将多行结果映射到 dst（*[]T）。
func (r *RawQuery[T]) Scan(dst *[]T) error {
	rows, err := r.db.query(r.query, r.args...)
	if err != nil {
		return err
	}
	res, err := rowsToModels[T](rows)
	if err != nil {
		return err
	}
	*dst = res
	return nil
}

// ScanOne 执行原生 SQL 并将首行映射到 dst（*T），无行返回 ErrNoRows。
func (r *RawQuery[T]) ScanOne(dst *T) error {
	rows, err := r.db.query(r.query, r.args...)
	if err != nil {
		return err
	}
	ok, err := rowToModel(rows, dst)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoRows
	}
	return nil
}
