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

	_, err = conn.Query(ctx, "SELECT * FROM users")
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

	_, err = pool.Query(ctx, "SELECT 1")
	if err != nil {
		panic(err)
	}
}
