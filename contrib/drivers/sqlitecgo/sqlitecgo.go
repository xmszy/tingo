package sqlitecgo

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/xmszy/tingo/contrib/drivers/sqlite"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{ sqlite.Driver }

func (Driver) Name() string { return "sqlitecgo" }

func init() {
	driver := Driver{}
	tdb.RegisterSchemaDriver(driver.Name(), driver)
	tdb.MustRegisterDriver(tdb.NewDriverFromWithConnector("sqlitecgo", "sqlite", tdb.SQLConnector("sqlite3"), driver, tdb.Capabilities{
		Returning: true, Upsert: true, Savepoint: true, LastInsertID: true,
	}))
}