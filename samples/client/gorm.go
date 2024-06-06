// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"database/sql"
	"log"

	jinzhu "github.com/jinzhu/gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
)

func gormClient() {
	sqlDB, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	var user struct {
		gorm.Model
		Name string
	}
	db.Where("name = ?", "gorm.io").First(&user)
}

func jinzhuGormClient() {
	db, err := jinzhu.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var user struct {
		jinzhu.Model
		Name string
	}
	db.Where("name = ?", "jinzhu").First(&user)
}
