// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"database/sql"
	"log"

	"github.com/jackc/pgx/stdlib"
	jinzhu "github.com/jinzhu/gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func gormClient() {
	sql.Register("pgx", &stdlib.Driver{})
	sqlDB, err := sql.Open("pgx", "postgres://localhost:5432")
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
	sql.Register("pgx", &stdlib.Driver{})
	db, err := jinzhu.Open("pgx", "postgres://localhost:5432")
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
