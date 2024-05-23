package main

import (
	"database/sql"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

func init() {
	sql.Register("postgres", &pq.Driver{})
}

func sqlxConnectWithExplicitDriver() {
	db, err := sqlx.Connect("postgres", "postgres://pgotest:password@localhost?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlxUseDb(db)
}

func sqlxConnectWithNoDriver() {
	db := sqlx.MustConnect("postgres", "postgres://pgotest:password@localhost?sslmode=disable")
	defer db.Close()
	sqlxUseDb(db)
}

func sqlxOpenWithExplicitDriver() {
	db, err := sqlx.Open("postgres", "postgres://pgotest:password@localhost?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	sqlxUseDb(db)
}

func sqlxOpenWithNoDriver() {
	db := sqlx.MustOpen("postgres", "postgres://pgotest:password@localhost?sslmode=disable")
	defer db.Close()
	sqlxUseDb(db)
}

func sqlxUseDb(db *sqlx.DB) {
	query, args, err := sqlx.In("SELECT * FROM users WHERE level IN (?)", []int{4, 6, 7})
	if err != nil {
		log.Fatal(err)
	}

	query = db.Rebind(query)
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
}
