// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"

	"github.com/DataDog/orchestrion/runtime/taint"
)

type case137Driver struct{}

type case137Conn struct {
	value string
}

type case137Stmt struct {
	conn *case137Conn
}

type case137Rows struct {
	value string
	sent  bool
}

func (case137Driver) Open(name string) (driver.Conn, error) {
	return &case137Conn{value: name}, nil
}

func (conn *case137Conn) Prepare(string) (driver.Stmt, error) {
	return &case137Stmt{conn: conn}, nil
}

func (*case137Conn) Close() error {
	return nil
}

func (*case137Conn) Begin() (driver.Tx, error) {
	return nil, errors.New("case137: transactions not supported")
}

func (*case137Stmt) Close() error {
	return nil
}

func (*case137Stmt) NumInput() int {
	return -1
}

func (*case137Stmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, errors.New("case137: exec not supported")
}

func (stmt *case137Stmt) Query([]driver.Value) (driver.Rows, error) {
	return &case137Rows{value: stmt.conn.value}, nil
}

func (*case137Rows) Columns() []string {
	return []string{"path"}
}

func (*case137Rows) Close() error {
	return nil
}

func (rows *case137Rows) Next(destination []driver.Value) error {
	if rows.sent {
		return io.EOF
	}
	rows.sent = true
	destination[0] = []byte(rows.value)
	return nil
}

func init() {
	sql.Register("case137", case137Driver{})
	register(Case{
		ID:   137,
		Name: "database sql scan driver bytes",
		Run: func() {
			_, _ = os.Open(case137Read("clean"))
			_ = os.Setenv("CASE_137_INPUT", "secret")
			_, _ = os.Open(case137Read(os.Getenv("CASE_137_INPUT")))
		},
		Want: []Expect{{Value: "secret", Ranges: []taint.Range{{Start: 0, End: 6}}}},
	})
}

func case137Read(value string) string {
	database, _ := sql.Open("case137", value)
	defer database.Close()
	rows, _ := database.Query("SELECT path")
	defer rows.Close()
	var scanned string
	if rows.Next() {
		_ = rows.Scan(&scanned)
	}
	return scanned
}
