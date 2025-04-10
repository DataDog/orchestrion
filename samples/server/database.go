// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"log"
)

func init() {
	sql.Register("test", &testDriver{})
}

type testDriver struct{}

func (*testDriver) Open(string) (driver.Conn, error) {
	return &testConn{}, nil
}

type testConn struct{}

func (*testConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.ErrUnsupported
}

func (*testConn) Close() error {
	return nil
}

func (*testConn) Begin() (driver.Tx, error) {
	return nil, errors.ErrUnsupported
}

type testConnector struct{}

func (*testConnector) Connect(context.Context) (driver.Conn, error) {
	return &testConn{}, nil
}

func (*testConnector) Driver() driver.Driver {
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
