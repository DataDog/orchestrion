// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"database/sql"
	"database/sql/driver"

	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
)

func Open(driverName, dataSourceName string, opts ...sqltrace.Option) (*sql.DB, error) {
	return sqltrace.Open(driverName, dataSourceName, opts...)
}

func OpenDB(c driver.Connector, opts ...sqltrace.Option) *sql.DB {
	return sqltrace.OpenDB(c, opts...)
}

func SqlWithServiceName(name string) sqltrace.Option {
	return sqltrace.WithServiceName(name)
}

func SqlWithCustomTag(key string, value interface{}) sqltrace.Option {
	return sqltrace.WithCustomTag(key, value)
}
