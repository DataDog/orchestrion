// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"orchestrion/integration"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	gormtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/jinzhu/gorm"
)

type Note struct {
	gorm.Model
	UserID  int
	Content string
}

func main() {
	db, err := gorm.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		log.Fatalf("Failed to open GORM database: %v", err)
	}
	defer db.Close()

	db.AutoMigrate(&Note{})
	for _, note := range []*Note{
		{UserID: 1, Content: `Hello, John. This is John. You are leaving a note for yourself. You are welcome and thank you.`},
		{UserID: 1, Content: `Hey, remember to mow the lawn.`},
		{UserID: 2, Content: `Reminder to submit that report by Thursday.`},
		{UserID: 2, Content: `Opportunities don't happen, you create them.`},
		{UserID: 3, Content: `Pick up cabbage from the store on the way home.`},
		{UserID: 3, Content: `Review PR #1138`},
	} {
		db.Create(note)
	}

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
			if err := db.Where("user_id = ?", 2).First(&note).Error; err != nil {
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
