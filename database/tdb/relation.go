package tdb

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Relation 描述两个模型之间的关联关系。
//
// 使用泛型 Relation[T, R] 来描述当前模型 T 与关联模型 R 的关系。
// 关联在注册期声明，查询期通过 With()/Load() 触发预加载。
type Relation[T, R any] struct {
	// typ 关联类型
	typ string // "hasOne", "hasMany", "belongsTo", "belongsToMany", "hasOneThrough", "morphOne", "morphMany", "morphTo"
	// foreignKey 当前模型侧的外键字段名
	foreignKey string
	// localKey 当前模型侧的本地键字段名
	localKey string
	// relatedFK 关联模型侧的外键（对于 BelongsTo 指 R 的主键）
	relatedFK string
	// pivot 多对多中间表名
	pivot string
	// pivotFK 中间表中指向当前模型的列
	pivotFK string
	// pivotRelatedFK 中间表中指向关联模型的列
	pivotRelatedFK string
	// relatedTable 关联表名
	relatedTable string
	// relatedPK 关联模型主键
	relatedPK string
	// morphType 多态类型标识（如 "user"、"post"）
	morphType string
	// morphTypeField 多态类型字段（如 "commentable_type"）
	morphTypeField string
	// morphIDField 多态 ID 字段（如 "commentable_id"）
	morphIDField string
}

// HasOne 声明当前模型 T 有一条关联模型 R（T.foreignKey = R.relatedFK）。
//
// 例如：User{Id} HasOne Profile{UserId} → User.Id = Profile.UserId
func HasOne[T, R any](foreignKey, relatedFK string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:       "hasOne",
		foreignKey: foreignKey,
		relatedFK:  relatedFK,
	}
}

// HasMany 声明当前模型 T 有多条关联模型 R（T.foreignKey = R.relatedFK）。
//
// 例如：User{Id} HasMany Order{UserId} → User.Id = Order.UserId
func HasMany[T, R any](foreignKey, relatedFK string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:       "hasMany",
		foreignKey: foreignKey,
		relatedFK:  relatedFK,
	}
}

// BelongsTo 声明当前模型 T 属于某个关联模型 R（T.relatedFK = R.pk）。
//
// 例如：Order{UserId} BelongsTo User{Id} → Order.UserId = User.Id
func BelongsTo[T, R any](relatedFK, pk string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:       "belongsTo",
		foreignKey: pk,
		relatedFK:  relatedFK,
	}
}

// BelongsToMany 声明多对多关联。
//
// pivot 是中间表名，pivotFK 指向当前模型，pivotRelatedFK 指向关联模型。
// 例如：User{Id} BelongsToMany Role{Id} 经 user_role(user_id, role_id)
func BelongsToMany[T, R any](pivot, pivotFK, pivotRelatedFK, localKey, relatedKey string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:            "belongsToMany",
		foreignKey:      localKey,
		localKey:        localKey,
		relatedFK:       pivotRelatedFK,
		pivot:           pivot,
		pivotFK:         pivotFK,
		pivotRelatedFK: pivotRelatedFK,
		relatedPK:       relatedKey,
	}
}

// HasOneThrough 通过中间表获取一条远端关联。
//
// 例如：Supplier → History (经 user 表)：
//
//	HasOneThrough[User, History]("id", "user_id", "id", "history")
//
// 对应 SQL：SELECT h.* FROM history h
// INNER JOIN user_history uh ON uh.history_id = h.id
// WHERE uh.user_id IN (1,2,3)
//
// 参数含义：
//   - localKey        T 上用于匹配中间表的键（通常是主键）
//   - pivotFK        中间表中指向 T 的列
//   - pivotRelatedFK 中间表中指向 R 的列
//   - relatedKey      R 上用于匹配中间表的键（通常是主键）
func HasOneThrough[T, R any](localKey, pivotFK, pivotRelatedFK, relatedKey string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:            "hasOneThrough",
		localKey:       localKey,
		foreignKey:     localKey,
		pivotFK:        pivotFK,
		pivotRelatedFK: pivotRelatedFK,
		relatedPK:      relatedKey,
		relatedFK:      relatedKey,
	}
}

// MorphOne 声明一条多态一对一关联。
//
// 例如：User{MorphOne(Image: "imageable_type", "imageable_id")}
// 对应 SQL：SELECT * FROM images WHERE imageable_type='user' AND imageable_id IN (...)
func MorphOne[T, R any](morphType string, morphTypeField, morphIDField string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:            "morphOne",
		morphType:      morphType,
		morphTypeField: morphTypeField,
		morphIDField:   morphIDField,
	}
}

// MorphMany 声明多条多态一对多关联。
//
// 例如：Post{MorphMany(Comment: "commentable_type", "commentable_id")}
// 对应 SQL：SELECT * FROM comments WHERE commentable_type='post' AND commentable_id IN (...)
func MorphMany[T, R any](morphType string, morphTypeField, morphIDField string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:            "morphMany",
		morphType:      morphType,
		morphTypeField: morphTypeField,
		morphIDField:   morphIDField,
	}
}

// MorphTo 声明多态反向关联（属于）。
//
// R 的具体类型由源记录中的 type 字段运行时确定。
// 例如：Comment{CommentableType, CommentableId} MorphTo(User or Post)
func MorphTo[T, R any](morphTypeField, morphIDField string) *Relation[T, R] {
	return &Relation[T, R]{
		typ:            "morphTo",
		morphTypeField: morphTypeField,
		morphIDField:   morphIDField,
	}
}

// SetPivot 设置 hasOneThrough 使用的中间表名。
func (r *Relation[T, R]) SetPivot(pivot string) *Relation[T, R] {
	r.pivot = pivot
	return r
}

// Type 返回关联类型。
func (r *Relation[T, R]) Type() string { return r.typ }

// preloaderConfig 携带父 Model 的设置，供预加载器继承。
// 确保关联查询也遵循软删除、缓存等父 Model 的配置。
type preloaderConfig struct {
	WithTrashed  bool
	OnlyTrashed  bool
	DisableHooks bool
	// 缓存配置（未来可由父 Model 的 Cache() 方法继承）
	CacheEnabled bool
	CacheKey     string
	CacheTTL     time.Duration
}

// preloader 是内部预加载抽象，统一处理不同类型的关联。
type preloader interface {
	// load 查询关联数据并填充到 items 中 fieldName 指定的字段。
	load(ctx context.Context, db *DB, items any, fieldName string) error
	// setConfig 注入父 Model 的配置（由 With() 调用）。
	setConfig(cfg preloaderConfig)
	// config 返回当前配置。
	config() preloaderConfig
}

// preloaderBase 嵌入所有预加载器，提供配置继承。
type preloaderBase struct {
	cfg preloaderConfig
}

func (b *preloaderBase) setConfig(cfg preloaderConfig) { b.cfg = cfg }
func (b *preloaderBase) config() preloaderConfig        { return b.cfg }

// softDeleteWhere 根据配置返回软删除 WHERE 子句。
// 返回空字符串表示不需要软删除过滤。
// 使用约定列名 "deleted_at"（与 tdb 默认约定一致）。
func (b *preloaderBase) softDeleteWhere(dial Dialect) string {
	const defaultSDColumn = "deleted_at"
	if b.cfg.OnlyTrashed {
		return dial.Quote(defaultSDColumn) + " IS NOT NULL"
	}
	if !b.cfg.WithTrashed {
		return dial.Quote(defaultSDColumn) + " IS NULL"
	}
	return ""
}

// MakePreloader 根据关联类型创建对应的 preloader。
func MakePreloader[T, R any](relation *Relation[T, R]) preloader {
	switch relation.typ {
	case "hasOne", "hasMany":
		return &hasOneManyPreloader[T, R]{relation: relation}
	case "belongsTo":
		return &belongsToPreloader[T, R]{relation: relation}
	case "belongsToMany":
		return &belongsToManyPreloader[T, R]{relation: relation}
	case "hasOneThrough":
		return &hasOneThroughPreloader[T, R]{relation: relation}
	case "morphOne", "morphMany":
		return &morphOneManyPreloader[T, R]{relation: relation}
	default:
		return nil
	}
}

// ---- hasOne / hasMany preloader ----

type hasOneManyPreloader[T, R any] struct {
	preloaderBase
	relation *Relation[T, R]
}

func (p *hasOneManyPreloader[T, R]) load(ctx context.Context, db *DB, items any, fieldName string) error {
	rel := p.relation

	// 收集当前模型的主键值
	itemSlice := reflect.ValueOf(items)
	if itemSlice.Kind() == reflect.Pointer {
		itemSlice = itemSlice.Elem()
	}
	if itemSlice.Kind() != reflect.Slice {
		return fmt.Errorf("tdb: With() expects a pointer to slice of %T", *new(T))
	}

	length := itemSlice.Len()
	if length == 0 {
		return nil
	}

	ids := collectIDs(itemSlice, rel.foreignKey)
	if len(ids) == 0 {
		return nil
	}

	// 查询关联模型
	relatedTable := rel.relatedTable
	if relatedTable == "" {
		relatedTable = tableNameOf(reflect.TypeFor[R]())
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = db.Dialect().Placeholder(i)
		args[i] = id
	}

	sqlStr := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s IN (%s)",
		db.Dialect().Quote(relatedTable),
		db.Dialect().Quote(rel.relatedFK),
		strings.Join(placeholders, ","),
	)

	// Apply soft-delete filtering from parent Model config
	if sdWhere := p.softDeleteWhere(db.Dialect()); sdWhere != "" {
		sqlStr += " AND " + sdWhere
	}

	rows, err := db.query(sqlStr, args...)
	if err != nil {
		return err
	}

	relatedItems, err := rowsToModels[R](rows)
	if err != nil {
		return err
	}

	// 构建 relatedFK → []*R 映射（指针切片，用于填充）
	relatedPtrs := make([]*R, len(relatedItems))
	for i := range relatedItems {
		relatedPtrs[i] = &relatedItems[i]
	}

	fkToRelated := make(map[any][]*R, len(relatedPtrs))
	for _, rp := range relatedPtrs {
		rv := reflect.ValueOf(rp).Elem()
		fk := findField(rv, rel.relatedFK)
		if !fk.IsValid() {
			continue
		}
		key := fk.Interface()
		fkToRelated[key] = append(fkToRelated[key], rp)
	}

	// 填充每个源条目
	isHasOne := rel.typ == "hasOne"
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

		related, ok := fkToRelated[key]
		if !ok || len(related) == 0 {
			continue
		}

		target := findField(srcElem, fieldName)
		if !target.IsValid() || !target.CanSet() {
			continue
		}

		if isHasOne {
			target.Set(reflect.ValueOf(related[0]))
		} else {
			// hasMany：创建切片并批量设置
			sliceType := target.Type()
			newSlice := reflect.MakeSlice(sliceType, len(related), len(related))
			for mi := range related {
				newSlice.Index(mi).Set(reflect.ValueOf(related[mi]))
			}
			target.Set(newSlice)
		}

	}

	return nil
}

// ---- belongsTo preloader ----

type belongsToPreloader[T, R any] struct {
	preloaderBase
	relation *Relation[T, R]
}

func (p *belongsToPreloader[T, R]) load(ctx context.Context, db *DB, items any, fieldName string) error {
	rel := p.relation

	itemSlice := reflect.ValueOf(items)
	if itemSlice.Kind() == reflect.Pointer {
		itemSlice = itemSlice.Elem()
	}
	if itemSlice.Kind() != reflect.Slice {
		return fmt.Errorf("tdb: With() expects a pointer to slice")
	}

	length := itemSlice.Len()
	if length == 0 {
		return nil
	}

	ids := collectIDs(itemSlice, rel.relatedFK)
	if len(ids) == 0 {
		return nil
	}

	relatedTable := rel.relatedTable
	if relatedTable == "" {
		relatedTable = tableNameOf(reflect.TypeFor[R]())
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = db.Dialect().Placeholder(i)
		args[i] = id
	}

	// 按 R 的主键（rel.foreignKey）查询
	sqlStr := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s IN (%s)",
		db.Dialect().Quote(relatedTable),
		db.Dialect().Quote(rel.foreignKey),
		strings.Join(placeholders, ","),
	)
	if sdWhere := p.softDeleteWhere(db.Dialect()); sdWhere != "" {
		sqlStr += " AND " + sdWhere
	}

	rows, err := db.query(sqlStr, args...)
	if err != nil {
		return err
	}

	relatedItems, err := rowsToModels[R](rows)
	if err != nil {
		return err
	}

	// 构建 foreignKey(R的主键) → *R 映射
	fkToRelated := make(map[any]*R, len(relatedItems))
	for i := range relatedItems {
		rp := &relatedItems[i]
		rv := reflect.ValueOf(rp).Elem()
		fk := findField(rv, rel.foreignKey)
		if !fk.IsValid() {
			continue
		}
		key := fk.Interface()
		fkToRelated[key] = rp
	}

	// 填充每个源条目（按 T.relatedFK 匹配 R.foreignKey）
	for i := 0; i < length; i++ {
		srcElem := itemSlice.Index(i)
		if srcElem.Kind() == reflect.Pointer {
			if srcElem.IsNil() {
				continue
			}
			srcElem = srcElem.Elem()
		}

		fk := findField(srcElem, rel.relatedFK)
		if !fk.IsValid() {
			continue
		}
		key := fk.Interface()

		rp, ok := fkToRelated[key]
		if !ok {
			continue
		}

		target := findField(srcElem, fieldName)
		if !target.IsValid() || !target.CanSet() {
			continue
		}
		target.Set(reflect.ValueOf(rp))
	}

	return nil
}

// ---- belongsToMany preloader ----

type belongsToManyPreloader[T, R any] struct {
	preloaderBase
	relation *Relation[T, R]
}

func (p *belongsToManyPreloader[T, R]) load(ctx context.Context, db *DB, items any, fieldName string) error {
	rel := p.relation

	itemSlice := reflect.ValueOf(items)
	if itemSlice.Kind() == reflect.Pointer {
		itemSlice = itemSlice.Elem()
	}
	if itemSlice.Kind() != reflect.Slice {
		return fmt.Errorf("tdb: With() expects a pointer to slice")
	}

	length := itemSlice.Len()
	if length == 0 {
		return nil
	}

	ids := collectIDs(itemSlice, rel.localKey)
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = db.Dialect().Placeholder(i)
		args[i] = id
	}

	// 两阶段查询：
	// 1. 查中间表获取 (pivotFK, pivotRelatedFK) 映射
	// 2. 查关联表获取 R 记录，按 relatedPK 去重和映射

	// Phase 1: pivot mapping
	pivotQuery := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE %s IN (%s)",
		db.Dialect().Quote(rel.pivotFK),
		db.Dialect().Quote(rel.pivotRelatedFK),
		db.Dialect().Quote(rel.pivot),
		db.Dialect().Quote(rel.pivotFK),
		strings.Join(placeholders, ","),
	)

	pivotRows, err := db.query(pivotQuery, args...)
	if err != nil {
		return err
	}

	pivotMap, relatedIDs := scanPivotMap(pivotRows)
	if len(relatedIDs) == 0 {
		return nil
	}

	// Phase 2: query related models
	relatedTable := rel.relatedTable
	if relatedTable == "" {
		relatedTable = tableNameOf(reflect.TypeFor[R]())
	}

	relatedPlaces := make([]string, len(relatedIDs))
	relatedArgs := make([]any, len(relatedIDs))
	for i, id := range relatedIDs {
		relatedPlaces[i] = db.Dialect().Placeholder(i)
		relatedArgs[i] = id
	}

	relatedQuery := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s IN (%s)",
		db.Dialect().Quote(relatedTable),
		db.Dialect().Quote(rel.relatedPK),
		strings.Join(relatedPlaces, ","),
	)
	if sdWhere := p.softDeleteWhere(db.Dialect()); sdWhere != "" {
		relatedQuery += " AND " + sdWhere
	}

	relRows, err := db.query(relatedQuery, relatedArgs...)
	if err != nil {
		return err
	}

	relatedItems, err := rowsToModels[R](relRows)
	if err != nil {
		return err
	}

	// Build relatedPK → *R map
	relatedByPK := make(map[any]*R, len(relatedItems))
	for i := range relatedItems {
		rp := &relatedItems[i]
		rv := reflect.ValueOf(rp).Elem()
		pk := findField(rv, rel.relatedPK)
		if !pk.IsValid() {
			continue
		}
		relatedByPK[pk.Interface()] = rp
	}

	// Fill each source item: collect all *R via pivotMap[localKey]
	for i := 0; i < length; i++ {
		srcElem := itemSlice.Index(i)
		if srcElem.Kind() == reflect.Pointer {
			if srcElem.IsNil() {
				continue
			}
			srcElem = srcElem.Elem()
		}

		fk := findField(srcElem, rel.localKey)
		if !fk.IsValid() {
			continue
		}
		key := fk.Interface()

		relatedFKValues, ok := pivotMap[key]
		if !ok || len(relatedFKValues) == 0 {
			continue
		}

		target := findField(srcElem, fieldName)
		if !target.IsValid() || !target.CanSet() {
			continue
		}

		// 收集所有关联的 *R
		var matched []reflect.Value
		for _, rfk := range relatedFKValues {
			if rp, ok := relatedByPK[rfk]; ok {
				matched = append(matched, reflect.ValueOf(rp))
			}
		}

		// 设置切片
		sliceType := target.Type()
		newSlice := reflect.MakeSlice(sliceType, len(matched), len(matched))
		for mi := range matched {
			newSlice.Index(mi).Set(matched[mi])
		}
		target.Set(newSlice)
	}

	return nil
}

// ---- hasOneThrough preloader ----

type hasOneThroughPreloader[T, R any] struct {
	preloaderBase
	relation *Relation[T, R]
}

func (p *hasOneThroughPreloader[T, R]) load(ctx context.Context, db *DB, items any, fieldName string) error {
	rel := p.relation

	itemSlice := reflect.ValueOf(items)
	if itemSlice.Kind() == reflect.Pointer {
		itemSlice = itemSlice.Elem()
	}
	if itemSlice.Kind() != reflect.Slice {
		return fmt.Errorf("tdb: With() expects a pointer to slice of %T", *new(T))
	}

	length := itemSlice.Len()
	if length == 0 {
		return nil
	}

	ids := collectIDs(itemSlice, rel.localKey)
	if len(ids) == 0 {
		return nil
	}

	relatedTable := rel.relatedTable
	if relatedTable == "" {
		relatedTable = tableNameOf(reflect.TypeFor[R]())
	}
	pivotTable := rel.pivot
	if pivotTable == "" {
		pivotTable = tableNameOf(reflect.TypeOf(struct{}{})) // won't match; user must set pivot
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = db.Dialect().Placeholder(i)
		args[i] = id
	}

	// SELECT r.* FROM related_table r
	// INNER JOIN pivot_table p ON p.pivotRelatedFK = r.relatedPK
	// WHERE p.pivotFK IN (...)
	sqlStr := fmt.Sprintf(
		"SELECT %s.* FROM %s %s INNER JOIN %s %s ON %s.%s = %s.%s WHERE %s.%s IN (%s)",
		db.Dialect().Quote(relatedTable),
		db.Dialect().Quote(relatedTable), "r",
		db.Dialect().Quote(pivotTable), "p",
		"p", db.Dialect().Quote(rel.pivotRelatedFK),
		"r", db.Dialect().Quote(rel.relatedPK),
		"p", db.Dialect().Quote(rel.pivotFK),
		strings.Join(placeholders, ","),
	)

	rows, err := db.query(sqlStr, args...)
	if err != nil {
		return err
	}

	relatedItems, err := rowsToModels[R](rows)
	if err != nil {
		return err
	}

	// Build pivotFK → *R map via pivot
	// We need to query pivot to get (pivotFK, pivotRelatedFK) mapping
	pivotQuery := fmt.Sprintf(
		"SELECT %s, %s FROM %s WHERE %s IN (%s)",
		db.Dialect().Quote(rel.pivotFK),
		db.Dialect().Quote(rel.pivotRelatedFK),
		db.Dialect().Quote(pivotTable),
		db.Dialect().Quote(rel.pivotFK),
		strings.Join(placeholders, ","),
	)

	// Re-execute with same args
	pivotRows, err := db.query(pivotQuery, args...)
	if err != nil {
		return err
	}
	pivotMap, _ := scanPivotMap(pivotRows)

	// Build relatedPK → *R map
	relatedByPK := make(map[any]*R, len(relatedItems))
	for i := range relatedItems {
		rp := &relatedItems[i]
		rv := reflect.ValueOf(rp).Elem()
		pkf := findField(rv, rel.relatedPK)
		if !pkf.IsValid() {
			continue
		}
		relatedByPK[pkf.Interface()] = rp
	}

	// Fill each source item
	for i := 0; i < length; i++ {
		srcElem := itemSlice.Index(i)
		if srcElem.Kind() == reflect.Pointer {
			if srcElem.IsNil() {
				continue
			}
			srcElem = srcElem.Elem()
		}

		fk := findField(srcElem, rel.localKey)
		if !fk.IsValid() {
			continue
		}
		key := fk.Interface()

		relatedFKValues, ok := pivotMap[key]
		if !ok || len(relatedFKValues) == 0 {
			continue
		}

		target := findField(srcElem, fieldName)
		if !target.IsValid() || !target.CanSet() {
			continue
		}

		// HasOneThrough 只取第一条
		if rp, ok := relatedByPK[relatedFKValues[0]]; ok {
			target.Set(reflect.ValueOf(rp))
		}
	}

	return nil
}

// ---- morphOne / morphMany preloader ----

type morphOneManyPreloader[T, R any] struct {
	preloaderBase
	relation *Relation[T, R]
}

func (p *morphOneManyPreloader[T, R]) load(ctx context.Context, db *DB, items any, fieldName string) error {
	rel := p.relation

	itemSlice := reflect.ValueOf(items)
	if itemSlice.Kind() == reflect.Pointer {
		itemSlice = itemSlice.Elem()
	}
	if itemSlice.Kind() != reflect.Slice {
		return fmt.Errorf("tdb: With() expects a pointer to slice of %T", *new(T))
	}

	length := itemSlice.Len()
	if length == 0 {
		return nil
	}

	// 收集源模型的主键值。
	// MorphOne/MorphMany 默认用 "id" 字段匹配 morphIDField。
	// 如果显式设置了 foreignKey/localKey，则用指定的字段。
	idField := "id"
	if rel.foreignKey != "" {
		idField = rel.foreignKey
	}
	if rel.localKey != "" {
		idField = rel.localKey
	}
	pkIDs := collectIDs(itemSlice, idField)

	if len(pkIDs) == 0 {
		return nil
	}

	relatedTable := rel.relatedTable
	if relatedTable == "" {
		relatedTable = tableNameOf(reflect.TypeFor[R]())
	}

	placeholders := make([]string, len(pkIDs))
	args := make([]any, len(pkIDs)*2+2)

	// args layout: [type_val, id1, id2, ...] for MySQL-style
	for i, id := range pkIDs {
		placeholders[i] = db.Dialect().Placeholder(i + 1)
		args[i+1] = id
	}
	args[0] = rel.morphType

	morphTypePlaceholder := db.Dialect().Placeholder(0)

	// SELECT * FROM related_table WHERE morphTypeField = ? AND morphIDField IN (...)
	sqlStr := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s = %s AND %s IN (%s)",
		db.Dialect().Quote(relatedTable),
		db.Dialect().Quote(rel.morphTypeField),
		morphTypePlaceholder,
		db.Dialect().Quote(rel.morphIDField),
		strings.Join(placeholders, ","),
	)
	if sdWhere := p.softDeleteWhere(db.Dialect()); sdWhere != "" {
		sqlStr += " AND " + sdWhere
	}

	// Trim unused trailing args
	args = args[:len(pkIDs)+1]

	rows, err := db.query(sqlStr, args...)
	if err != nil {
		return err
	}

	relatedItems, err := rowsToModels[R](rows)
	if err != nil {
		return err
	}

	// Build morphID → []*R map
	relatedPtrs := make([]*R, len(relatedItems))
	for i := range relatedItems {
		relatedPtrs[i] = &relatedItems[i]
	}

	idToRelated := make(map[any][]*R, len(relatedPtrs))
	for _, rp := range relatedPtrs {
		rv := reflect.ValueOf(rp).Elem()
		idv := findField(rv, rel.morphIDField)
		if !idv.IsValid() {
			continue
		}
		key := idv.Interface()
		idToRelated[key] = append(idToRelated[key], rp)
	}

	isMorphOne := rel.typ == "morphOne"
	for i := 0; i < length; i++ {
		srcElem := itemSlice.Index(i)
		if srcElem.Kind() == reflect.Pointer {
			if srcElem.IsNil() {
				continue
			}
			srcElem = srcElem.Elem()
		}

		pk := findField(srcElem, idField)
		if !pk.IsValid() {
			continue
		}
		key := pk.Interface()

		related, ok := idToRelated[key]
		if !ok || len(related) == 0 {
			continue
		}

		target := findField(srcElem, fieldName)
		if !target.IsValid() || !target.CanSet() {
			continue
		}

		if isMorphOne {
			target.Set(reflect.ValueOf(related[0]))
		} else {
			sliceType := target.Type()
			newSlice := reflect.MakeSlice(sliceType, len(related), len(related))
			for mi := range related {
				newSlice.Index(mi).Set(reflect.ValueOf(related[mi]))
			}
			target.Set(newSlice)
		}
	}

	return nil
}

// scanPivotMap 扫描 pivot 查询结果，返回 (pivotFK → []pivotRelatedFK) 映射和去重的 relatedFK 集合。
func scanPivotMap(rows *sql.Rows) (map[any][]any, []any) {
	defer rows.Close()
	result := make(map[any][]any)
	seen := make(map[any]bool)
	var uniqueIDs []any

	cols, err := rows.Columns()
	if err != nil {
		return result, uniqueIDs
	}

	for rows.Next() {
		ptrs := make([]any, len(cols))
		for i := range cols {
			ptrs[i] = new(any)
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}
		pivotFK := *(ptrs[0].(*any))
		pivotRelatedFK := *(ptrs[1].(*any))
		result[pivotFK] = append(result[pivotFK], pivotRelatedFK)
		if !seen[pivotRelatedFK] {
			seen[pivotRelatedFK] = true
			uniqueIDs = append(uniqueIDs, pivotRelatedFK)
		}
	}
	return result, uniqueIDs
}

// ---- 工具函数 ----

// collectIDs 从 slice 中收集指定字段的值（去重）。
func collectIDs(slice reflect.Value, fieldName string) []any {
	length := slice.Len()
	var ids []any
	seen := make(map[any]bool)
	for i := 0; i < length; i++ {
		elem := slice.Index(i)
		if elem.Kind() == reflect.Pointer {
			if elem.IsNil() {
				continue
			}
			elem = elem.Elem()
		}
		fv := findField(elem, fieldName)
		if !fv.IsValid() || isZero(fv) {
			continue
		}
		val := fv.Interface()
		if !seen[val] {
			seen[val] = true
			ids = append(ids, val)
		}
	}
	return ids
}

// findField 递归查找字段（支持嵌入结构体），按 tdb/db/json 标签或字段名匹配。
// 优先用 structMeta 的字段/列索引查表（缓存，免逐字段反射），查不到再退化递归。
func findField(v reflect.Value, name string) reflect.Value {
	t := v.Type()
	if m := metaFor(t); m != nil {
		if idx, ok := m.colIndex[name]; ok {
			return v.Field(idx)
		}
		if idx, ok := m.fieldIndex[name]; ok {
			return v.Field(idx)
		}
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if f.Anonymous {
			if fv := findField(v.Field(i), name); fv.IsValid() {
				return fv
			}
			continue
		}
		col := columnOf(f)
		if col == name || f.Name == name {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// Load 加载单条记录的关联（懒加载）。
// fieldName 是 T 结构体中用于存放关联数据的字段名。
func Load[T, R any](db *DB, model *T, relation *Relation[T, R], fieldName string) error {
	// 创建单元素切片以复用 preload 逻辑
	slice := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(model).Elem()), 1, 1)
	slice.Index(0).Set(reflect.ValueOf(model).Elem())

	preloader := MakePreloader(relation)
	if preloader == nil {
		return fmt.Errorf("tdb: unknown relation type")
	}
	return preloader.load(context.Background(), db, slice.Interface(), fieldName)
}

// LoadAll 加载多条记录的关联（懒加载）。
// fieldName 是 T 结构体中用于存放关联数据的字段名。
func LoadAll[T, R any](db *DB, models *[]T, relation *Relation[T, R], fieldName string) error {
	preloader := MakePreloader(relation)
	if preloader == nil {
		return fmt.Errorf("tdb: unknown relation type")
	}
	return preloader.load(context.Background(), db, models, fieldName)
}

// collectNestedItems 从已加载的根级关联中收集所有子实例（指针形式），
// 用于层叠预加载（如 "Profile.Photos"）。
//
// 返回值是 *[]*ElemType（指向指针切片的指针），
// 可直接传给下一层 preloader.load()。
func collectNestedItems(slicePtr any, fieldName string) (any, error) {
	v := reflect.ValueOf(slicePtr)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return nil, fmt.Errorf("tdb: expected slice, got %s", v.Kind())
	}

	length := v.Len()
	if length == 0 {
		return nil, nil
	}

	// 从类型信息推断元素类型（避免运行时反射开销）
	var elemType reflect.Type
	var collected []reflect.Value

	for i := 0; i < length; i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Pointer {
			if elem.IsNil() {
				continue
			}
			elem = elem.Elem()
		}
		fv := findField(elem, fieldName)
		if !fv.IsValid() {
			continue
		}

		if elemType == nil {
			switch fv.Kind() {
			case reflect.Pointer:
				if fv.IsNil() {
					continue
				}
				elemType = fv.Type().Elem()
			case reflect.Struct:
				elemType = fv.Type()
			default:
				return nil, fmt.Errorf("tdb: field %q is not a struct or pointer to struct", fieldName)
			}
		}

		switch fv.Kind() {
		case reflect.Pointer:
			if fv.IsNil() {
				continue
			}
			collected = append(collected, fv) // *Profile，直接指向原始值
		case reflect.Struct:
			// fv 从可寻址的 elem 中来，Addr() 安全
			collected = append(collected, fv.Addr()) // &Profile
		}
	}

	if elemType == nil || len(collected) == 0 {
		return nil, nil
	}

	// 构建 []*elemType 切片
	sliceType := reflect.SliceOf(reflect.PointerTo(elemType))
	slice := reflect.MakeSlice(sliceType, len(collected), len(collected))
	for i, item := range collected {
		slice.Index(i).Set(item)
	}

	// 返回 *[]*elemType
	slicePtrVal := reflect.New(sliceType)
	slicePtrVal.Elem().Set(slice)
	return slicePtrVal.Interface(), nil
}
