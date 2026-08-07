package mariadb

import (
	"github.com/xmszy/tingo/contrib/drivers/mysql"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{ mysql.Driver }

func (Driver) Name() string { return "mariadb" }

func init() {
	driver := Driver{}
	tdb.RegisterSchemaDriver(driver.Name(), driver)
	tdb.MustRegisterDriver(tdb.NewDriverFromWithConnector("mariadb", "mysql", tdb.SQLConnector("mysql"), driver, tdb.Capabilities{
		Upsert: true, Savepoint: true, LastInsertID: true,
	}))
}