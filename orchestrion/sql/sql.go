// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

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
