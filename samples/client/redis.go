// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"context"
	"fmt"

	redisv7 "github.com/go-redis/redis/v7"
	redisv8 "github.com/go-redis/redis/v8"
	redigo "github.com/gomodule/redigo/redis"
	redisv9 "github.com/redis/go-redis/v9"
)

func redisV7Client() {
	client := redisv7.NewClient(&redisv7.Options{Addr: "127.0.0.1", Password: "", DB: 0})
	defer client.Close()
	if res := client.Set("test_key", "test_value", 0); res.Err() != nil {
		panic(res.Err())
	}
}

func redisV8Client(ctx context.Context) {
	client := redisv8.NewClient(&redisv8.Options{Addr: "127.0.0.1", Password: "", DB: 0})
	defer client.Close()
	if res := client.Set(ctx, "test_key", "test_value", 0); res.Err() != nil {
		panic(res.Err())
	}
}

func redisV9Client(ctx context.Context) {
	client := redisv9.NewClient(&redisv9.Options{Addr: "127.0.0.1", Password: "", DB: 0})
	defer client.Close()
	if res := client.Set(ctx, "test_key", "test_value", 0); res.Err() != nil {
		panic(res.Err())
	}
}

func redigoClient(ctx context.Context, net, addr string) {
	use := func(conn redigo.Conn) {
		conn.Do("SET", "test_key", "test_value", ctx)
	}

	if conn, err := redigo.Dial(net, addr); err != nil {
		panic(err)
	} else {
		use(conn)
	}

	if conn, err := redigo.DialContext(ctx, net, addr); err != nil {
		panic(err)
	} else {
		use(conn)
	}

	if conn, err := redigo.DialURL(fmt.Sprintf("%s://%s", net, addr)); err != nil {
		panic(err)
	} else {
		use(conn)
	}
}
