// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func pgxExample() {
	ctx := context.Background()
	dbURL := "postgres://username:password@localhost:5432/database_name"
	var (
		conn *pgx.Conn
		err  error
	)

	conn, err = pgx.Connect(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	cfg, err := pgx.ParseConfig(dbURL)
	if err != nil {
		panic(err)
	}
	conn, err = pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		panic(err)
	}
	conn, err = pgx.ConnectWithOptions(ctx, dbURL, pgx.ParseConfigOptions{})
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)

	var name string
	var weight int64
	err = conn.QueryRow(context.Background(), "select name, weight from widgets where id=$1", 4).Scan(&name, &weight)
	if err != nil {
		panic(err)
	}
}

func pgxPoolExample() {
	ctx := context.Background()
	dbURL := "postgres://username:password@localhost:5432/database_name"

	var (
		pool *pgxpool.Pool
		err  error
	)

	pool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		panic(err)
	}
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	var name string
	var weight int64
	err = pool.QueryRow(context.Background(), "select name, weight from widgets where id=$1", 4).Scan(&name, &weight)
	if err != nil {
		panic(err)
	}
}
