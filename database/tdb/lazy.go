package tdb

import (
	"database/sql"
	"reflect"
)

// LazyCollection 延迟集合——大数据量场景下的游标式遍历。
//
// 与 All() 不同，LazyCollection 不会一次性将所有行加载到内存，
// 而是通过 sql.Rows 逐行读取，支持 for-range 迭代。
//
// 用法：
//
//	lazy := model.Lazy()
//	for lazy.Next() {
//	    var user User
//	    if err := lazy.Scan(&user); err != nil {
//	        break
//	    }
//	    // 逐行处理
//	}
type LazyCollection[T any] struct {
	model    *Model[T]
	rows     *sql.Rows
	colIndex map[string]int // 列名到结构体字段索引映射
	cols     []string
	err      error
	closed   bool
}

// Lazy 创建延迟集合（游标式遍历）。
func (m *Model[T]) Lazy() (*LazyCollection[T], error) {
	sqlStr, args := m.buildSelect()
	rows, err := m.runQuery(sqlStr, args...)
	if err != nil {
		return nil, err
	}

	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, err
	}

	// 构建列名到字段索引映射
	colIndex := buildColIndex[T]()

	return &LazyCollection[T]{
		model:    m,
		rows:     rows,
		colIndex: colIndex,
		cols:     cols,
	}, nil
}

// buildColIndex 构建列名（tdb/db/json 标签或字段名蛇形）到结构体字段索引的映射。
func buildColIndex[T any]() map[string]int {
	var zero T
	rt := reflect.TypeOf(zero)
	if rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil
	}
	idx := make(map[string]int)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Anonymous {
			continue
		}
		col := columnOf(f)
		if col != "" && col != "-" {
			idx[col] = i
		}
	}
	return idx
}

// Next 移动到下一行。返回 false 表示无更多行或发生错误。
func (l *LazyCollection[T]) Next() bool {
	if l.closed || l.err != nil {
		return false
	}
	ok := l.rows.Next()
	if !ok {
		l.Close()
		return false
	}
	return true
}

// Scan 将当前行扫描到目标结构体。
func (l *LazyCollection[T]) Scan(dst *T) error {
	if l.err != nil {
		return l.err
	}

	ptrs := make([]any, len(l.cols))
	for i := range l.cols {
		ptrs[i] = new(any)
	}
	if err := l.rows.Scan(ptrs...); err != nil {
		l.err = err
		return err
	}

	vv := reflect.ValueOf(dst).Elem()
	for i, c := range l.cols {
		if idx, ok := l.colIndex[c]; ok {
			if vv.Field(idx).CanSet() {
				assignField(vv.Field(idx), *(ptrs[i].(*any)))
			}
		}
	}

	// AfterQuery hook
	if l.err == nil && !l.model.disableHooks {
		if s, ok := any(dst).(AfterQuerier); ok {
			l.err = s.AfterQuery()
		}
	}

	return l.err
}

// Err 返回迭代过程中的错误。
func (l *LazyCollection[T]) Err() error {
	err := l.rows.Err()
	if err != nil {
		return err
	}
	return l.err
}

// Close 关闭游标（释放数据库连接）。
func (l *LazyCollection[T]) Close() error {
	if l.closed {
		return nil
	}
	l.closed = true
	return l.rows.Close()
}

// Count 返回总计数（需要再次查询 DB）。
func (l *LazyCollection[T]) Count() (int64, error) {
	return l.model.Count()
}

// Each 遍历所有行并执行回调。返回处理的条目数和首个错误。
func (l *LazyCollection[T]) Each(fn func(int, *T) error) (int, error) {
	defer l.Close()
	idx := 0
	var item T
	for l.Next() {
		if err := l.Scan(&item); err != nil {
			return idx, err
		}
		if err := fn(idx, &item); err != nil {
			return idx, err
		}
		idx++
	}
	if err := l.Err(); err != nil {
		return idx, err
	}
	return idx, nil
}

// Collect 收集所有行到切片（作用等同 All()，但允许途中处理）。
func (l *LazyCollection[T]) Collect() ([]T, error) {
	defer l.Close()
	var result []T
	for l.Next() {
		var item T
		if err := l.Scan(&item); err != nil {
			return result, err
		}
		result = append(result, item)
	}
	return result, l.Err()
}

// Chunk 分块处理：每 chunkSize 条记录调用一次回调。
func (l *LazyCollection[T]) Chunk(chunkSize int, fn func(int, []T) error) error {
	defer l.Close()
	var chunk []T
	page := 0
	idx := 0
	for l.Next() {
		var item T
		if err := l.Scan(&item); err != nil {
			return err
		}
		chunk = append(chunk, item)
		idx++
		if idx%chunkSize == 0 {
			if err := fn(page, chunk); err != nil {
				return err
			}
			chunk = nil
			page++
		}
	}
	if len(chunk) > 0 {
		if err := fn(page, chunk); err != nil {
			return err
		}
	}
	return l.Err()
}
