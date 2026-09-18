// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"database/sql"
	"fmt"
)

type stringDestinationScanner struct {
	target *string
}

func (scanner stringDestinationScanner) Scan(source any) error {
	if source == nil {
		return fmt.Errorf("converting NULL to string is unsupported")
	}
	if value, ok := source.([]byte); ok {
		*scanner.target = BytesToString(value)
		return nil
	}

	var value sql.NullString
	if err := value.Scan(source); err != nil {
		return err
	}
	*scanner.target = value.String
	return nil
}

// RowsScan scans rows while preserving taint from driver bytes into string destinations.
func RowsScan(rows *sql.Rows, destinations ...any) error {
	adapted := make([]any, len(destinations))
	for index, destination := range destinations {
		target, ok := destination.(*string)
		if ok && target != nil {
			adapted[index] = stringDestinationScanner{target: target}
			continue
		}
		adapted[index] = destination
	}
	return rows.Scan(adapted...)
}
