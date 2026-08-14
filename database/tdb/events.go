package tdb

import (
	"context"

	"github.com/xmszy/tingo/os/tevent"
)

// ModelEventData 模型事件携带的数据。
type ModelEventData struct {
	Table  string `json:"table"`
	Event  string `json:"event"`
	Model  any    `json:"model,omitempty"`  // 关联的模型实例
	Result any    `json:"result,omitempty"` // 操作结果
}

// 模型事件声明。调用方通过 tevent.Subscribe/Once 注册监听器：
//
//	tevent.Subscribe(bus, tdb.EventAfterInsert, func(ctx context.Context, data tdb.ModelEventData) error {
//	    log.Printf("Inserted into %s", data.Table)
//	    return nil
//	})

var (
	EventBeforeInsert = tevent.New[ModelEventData]("model.before_insert")
	EventAfterInsert  = tevent.New[ModelEventData]("model.after_insert")
	EventBeforeUpdate = tevent.New[ModelEventData]("model.before_update")
	EventAfterUpdate  = tevent.New[ModelEventData]("model.after_update")
	EventBeforeDelete = tevent.New[ModelEventData]("model.before_delete")
	EventAfterDelete  = tevent.New[ModelEventData]("model.after_delete")
	EventBeforeQuery  = tevent.New[ModelEventData]("model.before_query")
	EventAfterQuery   = tevent.New[ModelEventData]("model.after_query")
	EventBeforeSave   = tevent.New[ModelEventData]("model.before_save")
	EventAfterSave    = tevent.New[ModelEventData]("model.after_save")
)

// EnableEvents 在 DB 上启用模型事件（集成 tevent 事件总线）。
//
// 用法：
//
//	bus := tevent.NewBus(true)
//	db.EnableEvents(bus)
//
//	tevent.Subscribe(bus, tdb.EventAfterInsert, func(ctx context.Context, data tdb.ModelEventData) error {
//	    log.Printf("Inserted into %s: %+v", data.Table, data.Model)
//	    return nil
//	})
func (db *DB) EnableEvents(bus *tevent.Bus) {
	db.eventBus = bus
}

// fireEvent 触发模型事件。
func (db *DB) fireEvent(ctx context.Context, ev tevent.Event[ModelEventData], data ModelEventData) error {
	if db == nil || db.eventBus == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return tevent.Dispatch(db.eventBus, ctx, ev, data)
}

// ---- 在 CRUD 操作中集成事件 ----

// fireModelEvent 触发模型前后事件，返回首个监听器错误（监听器可借此中断写操作）。
func (m *Model[T]) fireModelEvent(ctx context.Context, ev tevent.Event[ModelEventData], model any, result any) error {
	return m.db.fireEvent(ctx, ev, ModelEventData{
		Table:  m.table,
		Event:  ev.Name(),
		Model:  model,
		Result: result,
	})
}

// On 注册模型事件监听器（委托到底层 DB 的事件总线）。
// 仅当 DB 已通过 EnableEvents(bus) 启用事件后才生效，否则静默忽略。
//
// 用法：
//
//	model := db.Model[User]()
//	model.On(tdb.EventAfterInsert, func(ctx context.Context, data tdb.ModelEventData) error {
//	    log.Printf("inserted: %+v", data.Model)
//	    return nil
//	})
func (m *Model[T]) On(ev tevent.Event[ModelEventData], h tevent.Handler[ModelEventData]) {
	if db := m.sourceDB(); db != nil && db.eventBus != nil {
		tevent.Subscribe(db.eventBus, ev, h)
	}
}

// OnBeforeInsert / OnAfterInsert 等便捷订阅（见 On）。
func (m *Model[T]) OnBeforeInsert(h tevent.Handler[ModelEventData]) { m.On(EventBeforeInsert, h) }
func (m *Model[T]) OnAfterInsert(h tevent.Handler[ModelEventData])  { m.On(EventAfterInsert, h) }
func (m *Model[T]) OnBeforeUpdate(h tevent.Handler[ModelEventData]) { m.On(EventBeforeUpdate, h) }
func (m *Model[T]) OnAfterUpdate(h tevent.Handler[ModelEventData])  { m.On(EventAfterUpdate, h) }
func (m *Model[T]) OnBeforeDelete(h tevent.Handler[ModelEventData]) { m.On(EventBeforeDelete, h) }
func (m *Model[T]) OnAfterDelete(h tevent.Handler[ModelEventData])  { m.On(EventAfterDelete, h) }
func (m *Model[T]) OnBeforeSave(h tevent.Handler[ModelEventData])   { m.On(EventBeforeSave, h) }
func (m *Model[T]) OnAfterSave(h tevent.Handler[ModelEventData])    { m.On(EventAfterSave, h) }
func (m *Model[T]) OnBeforeQuery(h tevent.Handler[ModelEventData]) { m.On(EventBeforeQuery, h) }
func (m *Model[T]) OnAfterQuery(h tevent.Handler[ModelEventData])  { m.On(EventAfterQuery, h) }
