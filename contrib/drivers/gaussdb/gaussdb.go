package gaussdb

import (
	"github.com/xmszy/tingo/contrib/drivers/postgres"
	"github.com/xmszy/tingo/database/tdb"
)

type Driver struct{ postgres.Driver }

func (Driver) Name() string { return "gaussdb" }

func init() {
	driver := Driver{}
	tdb.RegisterSchemaDriver(driver.Name(), driver)
	tdb.MustRegisterDriver(tdb.NewDriverFromWithConnector("gaussdb", "postgres", tdb.SQLConnector("pgx"), driver, tdb.Capabilities{
		Returning: true, Upsert: true, Savepoint: true,
	}))
}