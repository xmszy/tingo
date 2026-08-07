package tdb

// Hook 接口族 —— Model 实现这些接口即可在 CRUD 生命周期中获得回调。
//
// 所有 Hook 方法在调用时均传入目标实体指针，以便在回调内修改实体字段。
// 比如在 BeforeInsert() 内填充 CreatedAt/UpdatedAt 等自动时间戳。

// BeforeInserter 在 INSERT 执行前调用。
type BeforeInserter interface {
	BeforeInsert() error
}

// AfterInserter 在 INSERT 执行成功后调用。
type AfterInserter interface {
	AfterInsert() error
}

// BeforeUpdater 在 UPDATE 执行前调用。
type BeforeUpdater interface {
	BeforeUpdate() error
}

// AfterUpdater 在 UPDATE 执行成功后调用。
type AfterUpdater interface {
	AfterUpdate() error
}

// BeforeDeleter 在 DELETE 执行前调用。
type BeforeDeleter interface {
	BeforeDelete() error
}

// AfterDeleter 在 DELETE 执行成功后调用。
type AfterDeleter interface {
	AfterDelete() error
}

// BeforeQuerier 在 SELECT 查询执行前调用。
type BeforeQuerier interface {
	BeforeQuery() error
}

// AfterQuerier 在 SELECT 查询结果扫描后调用（每行）。
type AfterQuerier interface {
	AfterQuery() error
}

// BeforeSaver 在 Save（INSERT or UPDATE）执行前调用，用于自动时间戳等。
type BeforeSaver interface {
	BeforeSave() error
}
