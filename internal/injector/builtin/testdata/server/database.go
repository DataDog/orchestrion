// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log"
//line <generated>:1
	sql1 "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
)

//line samples/server/database.go:16
func init() {
	sql.Register("test", &testDriver{})
}

type testDriver struct{}

func (t *testDriver) Open(name string) (driver.Conn, error) {
	return &testConn{}, nil
}

type testConn struct{}

func (t *testConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

func (t *testConn) Close() error {
	return nil
}

func (t *testConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("NOT IMPLEMENTED")
}

type testConnector struct{}

func (t *testConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return &testConn{}, nil
}

func (t *testConnector) Driver() driver.Driver {
	return &testDriver{}
}

func openDatabase() (*sql.DB, error) {
	_, err :=
//line <generated>:1
		sql1.Open(
//line samples/server/database.go:51
			"test", "mypath")
//line samples/server/database.go:52
	if err != nil {
		log.Printf("Some error: %v", err)
	}
	return sql1. //line <generated>:1
			Open(
//line samples/server/database.go:55
			"test", "mypath")
}

//line samples/server/database.go:58
func openDatabase2() *sql.DB {
	_ =
//line <generated>:1
		sql1.OpenDB(
//line samples/server/database.go:59
			&testConnector{})
//line samples/server/database.go:60
	return sql1. //line <generated>:1
			OpenDB(
//line samples/server/database.go:60
			&testConnector{})
}
