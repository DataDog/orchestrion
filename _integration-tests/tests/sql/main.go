// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"orchestrion/integration"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./notedb.sqlite")
	if err != nil {
		log.Fatalf("Failed to open the notes database: %v", err)
	}

	// set up database
	_, err = db.ExecContext(context.Background(),
		`CREATE TABLE IF NOT EXISTS notes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	userid INTEGER,
	content STRING,
	created STRING
)`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.ExecContext(context.Background(),
		`INSERT OR REPLACE INTO notes(userid, content, created)
	VALUES (1, 'Hello, John. This is John. You are leaving a note for yourself. You are welcome and thank you.', datetime('now')),
		(1, 'Hey, remember to mow the lawn.', datetime('now')),
		(2, 'Reminder to submit that report by Thursday.', datetime('now')),
		(2, 'Opportunities don''t happen, you create them.', datetime('now')),
		(3, 'Pick up cabbage from the store on the way home.', datetime('now')),
		(3, 'Review PR #1138', datetime('now'));
`)
	if err != nil {
		log.Fatalf("Failed to insert test data: %v", err)
	}

	mux := &http.ServeMux{}

	//dd:ignore
	s := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "bad request", http.StatusNotFound)
			return
		}
		r.ParseForm()
		userid := r.FormValue("userid")
		content := r.FormValue("content")
		_, err = db.ExecContext(r.Context(),
			`INSERT INTO notes (userid, content, created)
VALUES (?, ?, datetime('now'));`, userid, content)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to insert: %v", err), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		ids, ok := r.Form["userid"]
		if !ok || len(ids) == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		id := ids[0]
		rows, err := db.QueryContext(r.Context(), "SELECT id, content, created FROM notes WHERE userid = ?;", id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Failed to query db: %v", err)
			return
		}

		enc := json.NewEncoder(w)
		for rows.Next() {
			var note struct {
				ID      int
				Content string
				Created string
			}
			rows.Scan(&note.ID, &note.Content, &note.Created)
			err = enc.Encode(note)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Failed to encode JSON: %v", err)
				return
			}
		}
	})

	integration.OnSignal(func() {
		s.Shutdown(context.Background())
	})

	log.Print(s.ListenAndServe())
}
