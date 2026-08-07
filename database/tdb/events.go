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
func (db *DB) fireEvent(ctx context.Context, ev tevent.Event[ModelEventData], data ModelEventData) {
	if db == nil || db.eventBus == nil {
		return
	}
	_ = tevent.Dispatch(db.eventBus, ctx, ev, data)
}

// ---- 在 CRUD 操作中集成事件 ----

// fireModelEvent 触发模型前后事件。
func (m *Model[T]) fireModelEvent(ctx context.Context, ev tevent.Event[ModelEventData], model any, result any) {
	m.db.fireEvent(ctx, ev, ModelEventData{
		Table:  m.table,
		Event:  ev.Name(),
		Model:  model,
		Result: result,
	})
}

// ensureEventsUsed prevents unused method warnings until events are wired into CRUD.
var _ = (*Model[struct{}]).fireModelEvent
