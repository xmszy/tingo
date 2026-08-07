package mssql

import (
	"github.com/xmszy/tingo/contrib/drivers/sqlserver"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{ sqlserver.Driver }

func (Driver) Name() string { return "mssql" }

func init() {
	driver := Driver{}
	tdb.RegisterSchemaDriver(driver.Name(), driver)
	tdb.MustRegisterDriver(tdb.NewDriverFromWithConnector("mssql", "sqlserver", tdb.SQLConnector("sqlserver"), driver, tdb.Capabilities{
		Returning: true, Savepoint: true, NamedParameters: true,
	}))
}