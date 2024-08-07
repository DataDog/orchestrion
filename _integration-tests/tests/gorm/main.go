// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package gorm

import (
	"context"
	"orchestrion/integration/validator/trace"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type TestCase struct {
	*gorm.DB
}

func (tc *TestCase) Setup(t *testing.T) {
	var err error
	tc.DB, err = gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, tc.DB.AutoMigrate(&Note{}))

	require.NoError(t, tc.DB.CreateInBatches([]Note{
		{UserID: 1, Content: `Hello, John. This is John. You are leaving a note for yourself. You are welcome and thank you.`},
		{UserID: 1, Content: `Hey, remember to mow the lawn.`},
		{UserID: 2, Content: `Reminder to submit that report by Thursday.`},
		{UserID: 2, Content: `Opportunities don't happen, you create them.`},
		{UserID: 3, Content: `Pick up cabbage from the store on the way home.`},
		{UserID: 3, Content: `Review PR #1138`},
	}, 10).Error)
}

func (tc *TestCase) Run(t *testing.T) {
	span, ctx := tracer.StartSpanFromContext(context.Background(), "test.root")
	defer span.Finish()

	var note Note
	require.NoError(t, tc.DB.WithContext(ctx).Where("user_id = ?", 2).First(&note).Error)
}

func (tc *TestCase) Teardown(t *testing.T) {}

func (*TestCase) ExpectedTraces() trace.Spans {
	return trace.Spans{
		{
			Tags: map[string]any{
				"name": "test.root",
			},
			Children: trace.Spans{
				{
					Tags: map[string]any{
						"resource": "SELECT * FROM `notes` WHERE user_id = ? AND `notes`.`deleted_at` IS NULL ORDER BY `notes`.`id` LIMIT 1",
						"type":     "sql",
						"name":     "gorm.query",
						"service":  "gorm.db",
					},
					Meta: map[string]any{
						"component": "gorm.io/gorm.v1",
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
