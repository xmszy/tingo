package pgsql

import (
	"github.com/xmszy/tingo/contrib/drivers/postgres"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{ postgres.Driver }

func (Driver) Name() string { return "pgsql" }

func init() {
	driver := Driver{}
	tdb.RegisterSchemaDriver(driver.Name(), driver)
	tdb.MustRegisterDriver(tdb.NewDriverFromWithConnector("pgsql", "postgres", tdb.SQLConnector("pgx"), driver, tdb.Capabilities{
		Returning: true, Upsert: true, Savepoint: true,
	}))
}