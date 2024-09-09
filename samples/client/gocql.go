// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"log"

	"github.com/gocql/gocql"
)

func SampleGoCQL1() {
	cluster := &gocql.ClusterConfig{
		Hosts:    []string{"127.0.0.1:9042"},
		Keyspace: "my-keyspace",
	}
	runGoCQLQuery(cluster)
}

func SampleGoCQL2() {
	cluster := gocql.ClusterConfig{
		Hosts:    []string{"127.0.0.1:9042"},
		Keyspace: "my-keyspace",
	}
	runGoCQLQuery(&cluster)
}

func SampleGoCQL3() {
	cluster := gocql.NewCluster("127.0.0.1:9042")
	runGoCQLQuery(cluster)
}

func runGoCQLQuery(cluster *gocql.ClusterConfig) {
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}

	query := session.Query("CREATE KEYSPACE if not exists trace WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor': 1}")
	if err := query.Exec(); err != nil {
		log.Fatal(err)
	}
}
