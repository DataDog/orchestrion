package sql

import (
	"database/sql"
	"database/sql/driver"

	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
)

func Open(driverName, dataSourceName string) (*sql.DB, error) {
	return sqltrace.Open(driverName, dataSourceName)
}

func OpenDB(c driver.Connector) *sql.DB {
	return sqltrace.OpenDB(c)
}
