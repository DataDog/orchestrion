// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log"
)

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
	_, err := sql.Open("test", "mypath")
	if err != nil {
		log.Printf("Some error: %v", err)
	}
	return sql.Open("test", "mypath")
}

func openDatabase2() *sql.DB {
	_ = sql.OpenDB(&testConnector{})
	return sql.OpenDB(&testConnector{})
}
