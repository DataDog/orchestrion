// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build integration

package gorm

import (
	"context"
	"orchestrion/integration/validator/trace"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3" // Auto-register sqlite3 driver
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type TestCase struct {
	*gorm.DB
}

func (tc *TestCase) Setup(t *testing.T) {
	var err error
	tc.DB, err = gorm.Open("sqlite3", "file::memory:")
	require.NoError(t, err)

	require.NoError(t, tc.DB.AutoMigrate(&Note{}).Error)
	for _, note := range []*Note{
		{UserID: 1, Content: `Hello, John. This is John. You are leaving a note for yourself. You are welcome and thank you.`},
		{UserID: 1, Content: `Hey, remember to mow the lawn.`},
		{UserID: 2, Content: `Reminder to submit that report by Thursday.`},
		{UserID: 2, Content: `Opportunities don't happen, you create them.`},
		{UserID: 3, Content: `Pick up cabbage from the store on the way home.`},
		{UserID: 3, Content: `Review PR #1138`},
	} {
		require.NoError(t, tc.DB.Create(note).Error)
	}
}

func (tc *TestCase) Run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	var note Note
	require.NoError(t, gormtrace.WithContext(ctx, tc.DB).Where("user_id = ?", 2).First(&note).Error)
}

func (tc *TestCase) Teardown(t *testing.T) {
	assert.NoError(t, tc.DB.Close())
}

func (*TestCase) ExpectedTraces() trace.Traces {
	return trace.Traces{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Traces{
				{
					Tags: map[string]any{
						"resource": "SELECT * FROM \"notes\"  WHERE \"notes\".\"deleted_at\" IS NULL AND ((user_id = ?)) ORDER BY \"notes\".\"id\" ASC LIMIT 1",
						"type":     "sql",
						"name":     "gorm.query",
						"service":  "gorm.db",
					},
					Meta: map[string]string{
						"component": "jinzhu/gorm",
					},
				},
			},
		},
	}
}

type Note struct {
	gorm.Model
	UserID  int
	Content string
}
