// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package sql

import (
	"context"
	"database/sql"
	"testing"

	"datadoghq.dev/orchestrion/_integration-tests/validator/trace"
	_ "github.com/mattn/go-sqlite3" // Auto-register sqlite3 driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	*sql.DB
}

func (tc *TestCase) Setup(t *testing.T) {
	var err error
	tc.DB, err = sql.Open("sqlite3", "file::memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, tc.DB.Close())
	})

	_, err = tc.DB.ExecContext(context.Background(),
		`CREATE TABLE IF NOT EXISTS notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	userid INTEGER,
	content STRING,
	created STRING
)`)
	require.NoError(t, err)

	_, err = tc.DB.ExecContext(context.Background(),
		`INSERT OR REPLACE INTO notes(userid, content, created) VALUES
		(1, 'Hello, John. This is John. You are leaving a note for yourself. You are welcome and thank you.', datetime('now')),
		(1, 'Hey, remember to mow the lawn.', datetime('now')),
		(2, 'Reminder to submit that report by Thursday.', datetime('now')),
		(2, 'Opportunities don''t happen, you create them.', datetime('now')),
		(3, 'Pick up cabbage from the store on the way home.', datetime('now')),
		(3, 'Review PR #1138', datetime('now')
	);`)
	require.NoError(t, err)
}

func (tc *TestCase) Run(t *testing.T) {
	_, err := tc.DB.ExecContext(context.Background(),
		`INSERT INTO notes (userid, content, created) VALUES (?, ?, datetime('now'));`,
		1337, "This is Elite!")
	require.NoError(t, err)
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"resource": "INSERT INTO notes (userid, content, created) VALUES (?, ?, datetime('now'));",
				"type":     "sql",
				"name":     "sqlite3.query",
				"service":  "sqlite3.db",
			},
			Meta: map[string]string{
				"component":      "database/sql",
				"span.kind":      "client",
				"sql.query_type": "Exec",
			},
		},
	}
}
