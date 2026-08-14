package tdb

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// aggKind 关联聚合类型。
type aggKind string

const (
	aggCount aggKind = "COUNT"
	aggSum   aggKind = "SUM"
)

// aggregatePreloader 关联聚合预加载：对每条 T 记录，统计其关联 R 的数量或求和，
// 结果填充到 T 上由 fieldName 指定的字段（通常为 int64 / float64 等数值类型）。
//
// 与 gf 的 WithCount/WithSum 语义一致：一次性 IN 查询聚合所有父记录，避免 N+1。
type aggregatePreloader[T, R any] struct {
	preloaderBase
	relation *Relation[T, R]
	kind     aggKind
	column   string                       // SUM 的列名（COUNT 忽略）
	whereFn  func(*Model[R]) *Model[R]    // 关联侧额外过滤（可选）
}

// load 执行聚合查询并填充到 items（*[]T）的 fieldName 字段。
func (p *aggregatePreloader[T, R]) load(ctx context.Context, db *DB, items any, fieldName string) error {
	rel := p.relation
	if rel == nil {
		return fmt.Errorf("tdb: With%s relation is nil", p.kind)
	}

	itemSlice := reflect.ValueOf(items)
	if itemSlice.Kind() == reflect.Pointer {
		itemSlice = itemSlice.Elem()
	}
	if itemSlice.Kind() != reflect.Slice {
		return fmt.Errorf("tdb: With%s expects a pointer to slice of %T", p.kind, *new(T))
	}

	length := itemSlice.Len()
	if length == 0 {
		return nil
	}

	// 收集父模型外键值（rel.foreignKey 在 T 侧）。
	ids := collectIDs(itemSlice, rel.foreignKey)
	if len(ids) == 0 {
		return nil
	}

	relatedTable := rel.relatedTable
	if relatedTable == "" {
		relatedTable = tableNameOf(reflect.TypeFor[R]())
	}

	// 关联侧额外过滤条件（复用 Model[R] 的 WHERE 构造）。
	rm := newModel[R](db, db.dial, nil)
	if p.whereFn != nil {
		rm = p.whereFn(rm)
	}
	var whereBuf strings.Builder
	var whereArgs []any
	whereArgs = rm.appendWheres(&whereBuf, whereArgs)

	// SELECT rel.relatedFK, AGG(...) FROM relatedTable
	// [WHERE extra] AND rel.relatedFK IN (?) GROUP BY rel.relatedFK
	var aggExpr string
	switch p.kind {
	case aggCount:
		aggExpr = "COUNT(*)"
	case aggSum:
		aggExpr = "SUM(" + db.Dialect().Quote(p.column) + ")"
	default:
		return fmt.Errorf("tdb: unsupported aggregate kind %q", p.kind)
	}

	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(db.Dialect().Quote(rel.relatedFK))
	b.WriteString(", ")
	b.WriteString(aggExpr)
	b.WriteString(" FROM ")
	b.WriteString(db.Dialect().Quote(relatedTable))

	var args []any
	if whereBuf.Len() > 0 {
		b.WriteString(whereBuf.String()) // 已含 " WHERE ..."
		b.WriteString(" AND ")
		args = append(args, whereArgs...)
	} else {
		b.WriteString(" WHERE ")
	}
	b.WriteString(db.Dialect().Quote(rel.relatedFK))
	b.WriteString(" IN (")
	phs := make([]string, len(ids))
	for i, id := range ids {
		phs[i] = db.Dialect().Placeholder(len(args) + i)
		args = append(args, id)
	}
	b.WriteString(strings.Join(phs, ","))
	b.WriteString(")")

	// 软删除过滤（继承父 Model 配置）。
	if sdWhere := p.softDeleteWhere(db.Dialect()); sdWhere != "" {
		b.WriteString(" AND ")
		b.WriteString(sdWhere)
	}

	b.WriteString(" GROUP BY ")
	b.WriteString(db.Dialect().Quote(rel.relatedFK))

	rows, err := db.query(b.String(), args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	fkToAgg := make(map[any]any)
	for rows.Next() {
		ptrs := make([]any, len(cols))
		for i := range cols {
			ptrs[i] = new(any)
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		fk := *(ptrs[0].(*any))
		agg := *(ptrs[1].(*any))
		fkToAgg[fk] = agg
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := 0; i < length; i++ {
		srcElem := itemSlice.Index(i)
		if srcElem.Kind() == reflect.Pointer {
			if srcElem.IsNil() {
				continue
			}
			srcElem = srcElem.Elem()
		}
		fk := findField(srcElem, rel.foreignKey)
		if !fk.IsValid() {
			continue
		}
		key := fk.Interface()

		target := findField(srcElem, fieldName)
		if !target.IsValid() || !target.CanSet() {
			continue
		}
		agg, ok := fkToAgg[key]
		if !ok {
			target.Set(reflect.Zero(target.Type()))
			continue
		}
		av := reflect.ValueOf(agg)
		if av.Type().ConvertibleTo(target.Type()) {
			target.Set(av.Convert(target.Type()))
		} else {
			target.Set(reflect.Zero(target.Type()))
		}
	}

	return nil
}

// WithCount 注册关联数量聚合预加载：对每条 T 记录统计其关联 R 的行数，
// 结果写入 T 上名为 fieldName 的字段（建议 int64）。
//
// 用法：
//
//	users, _ := tdb.WithCount(db.Model[User](), "OrderCount",
//	    tdb.HasMany[User, Order]("id", "user_id")).All()
//
// whereFn 可选，用于给关联侧追加过滤（如仅统计已支付订单）。
func WithCount[T, R any](m *Model[T], fieldName string, relation *Relation[T, R], whereFn ...func(*Model[R]) *Model[R]) *Model[T] {
	model := m.getModel()
	loader := &aggregatePreloader[T, R]{relation: relation, kind: aggCount}
	if len(whereFn) > 0 && whereFn[0] != nil {
		loader.whereFn = whereFn[0]
	}
	loader.setConfig(preloaderConfig{
		WithTrashed:  model.withTrashed,
		OnlyTrashed:  model.onlyTrashed,
		DisableHooks: model.disableHooks,
		CacheEnabled: model.cacheEnabled,
		CacheKey:     model.cacheKey,
		CacheTTL:     model.cacheTTL,
	})
	model.preloads = append(model.preloads, preloadEntry{name: fieldName, preloader: loader})
	return model
}

// WithSum 注册关联求和聚合预加载：对每条 T 记录求关联 R 的 column 字段之和，
// 结果写入 T 上名为 fieldName 的字段（建议 float64 / int64）。
//
// 用法：
//
//	users, _ := tdb.WithSum(db.Model[User](), "OrderAmount", "amount",
//	    tdb.HasMany[User, Order]("id", "user_id")).All()
func WithSum[T, R any](m *Model[T], fieldName, column string, relation *Relation[T, R], whereFn ...func(*Model[R]) *Model[R]) *Model[T] {
	model := m.getModel()
	loader := &aggregatePreloader[T, R]{relation: relation, kind: aggSum, column: column}
	if len(whereFn) > 0 && whereFn[0] != nil {
		loader.whereFn = whereFn[0]
	}
	loader.setConfig(preloaderConfig{
		WithTrashed:  model.withTrashed,
		OnlyTrashed:  model.onlyTrashed,
		DisableHooks: model.disableHooks,
		CacheEnabled: model.cacheEnabled,
		CacheKey:     model.cacheKey,
		CacheTTL:     model.cacheTTL,
	})
	model.preloads = append(model.preloads, preloadEntry{name: fieldName, preloader: loader})
	return model
}
