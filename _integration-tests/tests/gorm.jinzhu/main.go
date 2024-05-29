// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"orchestrion/integration"
	"os"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	gormtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm"
)

type Note struct {
	ID        int
	UserID    int `gorm:"column:userid"`
	Content   string
	CreatedAt string `gorm:"column:created"`
}

func main() {
	//dd:ignore
	defer func() func() error {
		db, err := sql.Open("sqlite3", "./notedb.sqlite")
		if err != nil {
			log.Fatalf("Failed to open the notes database: %v", err)
		}
		defer func() {
			if err := recover(); err != nil {
				os.Remove("./notedb.sqlite")
				panic(err)
			}
		}()

		if _, err := db.ExecContext(context.Background(),
			`CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			userid INTEGER NOT NULL,
			content STRING NOT NULL,
			created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`); err != nil {
			log.Fatalf("Failed to create table: %v", err)
		}

		if _, err := db.ExecContext(context.Background(),
			`INSERT OR REPLACE INTO notes(userid, content, created)
			VALUES (1, 'Hello, John. This is John. You are leaving a note for yourself. You are welcome and thank you.', datetime('now')),
			(1, 'Hey, remember to mow the lawn.', datetime('now')),
			(2, 'Reminder to submit that report by Thursday.', datetime('now')),
			(2, 'Opportunities don''t happen, you create them.', datetime('now')),
			(3, 'Pick up cabbage from the store on the way home.', datetime('now')),
			(3, 'Review PR #1138', datetime('now')
		);`); err != nil {
			log.Fatalf("Failed to insert test data: %v", err)
		}

		return func() error { return os.Remove("./notedb.sqlite") }
	}()()

	db, err := gorm.Open("sqlite3", "./notedb.sqlite")
	if err != nil {
		log.Fatalf("Failed to open GORM database: %v", err)
	}
	defer db.Close()

	mux := &http.ServeMux{}
	s := &http.Server{
		Addr:    "127.0.0.1:8088",
		Handler: mux,
	}

	mux.HandleFunc("/quit",
		//dd:ignore
		func(w http.ResponseWriter, r *http.Request) {
			log.Println("Shutdown requested...")
			defer s.Shutdown(context.Background())
			w.Write([]byte("Goodbye\n"))
		})

	mux.HandleFunc("/",
		//dd:ignore
		func(w http.ResponseWriter, r *http.Request) {
			// TODO: This should not be necessary (it's manual, and manual is yuck)
			db := gormtrace.WithContext(r.Context(), db)

			var note Note
			if err := db.Where("userid = ?", 2).First(&note).Error; err != nil {
				log.Printf("Error: %v\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "%v\n", err)
				return
			}
			w.Write([]byte(note.Content))
		})

	integration.OnSignal(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	})

	log.Print(s.ListenAndServe())
}
