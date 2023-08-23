// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package instrument

import (
	"net/http"

	chitrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/go-chi/chi.v5"
)

func ChiV5Middleware() func(http.Handler) http.Handler {
	// Passing an empty service name until we have a solid
	// and unified mechanism to guess the service name in Orchestrion.
	// chitrace defaults to DD_SERVICE or chi.router as service name.
	return chitrace.Middleware()
}
