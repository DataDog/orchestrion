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
)

// stubDriver is a minimal in-process database/sql/driver implementation.
// It exists only to hand the tainted value back as a driver-produced
// []byte, so that sql.Rows.Scan exercises the real stdlib convertAssign
// path (which builds a fresh string from the []byte) instead of a
// hand-rolled conversion.
type stubDriver struct{}

type stubConn struct {
	value string
}

type stubStmt struct {
	conn *stubConn
}

type stubRows struct {
	value string
	sent  bool
}

func (stubDriver) Open(name string) (driver.Conn, error) {
	return &stubConn{value: name}, nil
}

func (c *stubConn) Prepare(query string) (driver.Stmt, error) {
	return &stubStmt{conn: c}, nil
}

func (c *stubConn) Close() error { return nil }

func (c *stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("stubdriver: transactions not supported")
}

func (s *stubStmt) Close() error { return nil }

func (s *stubStmt) NumInput() int { return -1 }

func (s *stubStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("stubdriver: exec not supported")
}

func (s *stubStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &stubRows{value: s.conn.value}, nil
}

func (r *stubRows) Columns() []string { return []string{"path"} }

func (r *stubRows) Close() error { return nil }

func (r *stubRows) Next(dest []driver.Value) error {
	if r.sent {
		return io.EOF
	}
	r.sent = true
	dest[0] = []byte(r.value)
	return nil
}

func init() {
	sql.Register("taintstub", stubDriver{})
}

func main() {
	path := os.Getenv("TAINT_PATH")

	db, err := sql.Open("taintstub", path)
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT path")
	if err != nil {
		return
	}
	defer rows.Close()

	var scanned string
	if rows.Next() {
		_ = rows.Scan(&scanned)
	}

	_, _ = os.Open(scanned)
}
