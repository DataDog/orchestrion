// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

import (
	"net/http"

	"github.com/hashicorp/vault/api"
)

func vaultClient() {
	c, err := api.NewClient(&api.Config{
		Address: "http://vault.mydomain.com:8200",
	})
	if err != nil {
		panic(err)
	}
	c.Logical().Read("secret/key")
}

func vaultProvidedClient() {
	c, err := api.NewClient(&api.Config{
		HttpClient: &http.Client{},
		Address:    "http://vault.mydomain.com:8200",
	})
	if err != nil {
		panic(err)
	}
	c.Logical().Read("secret/key")
}
