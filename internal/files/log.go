// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package files

import "os"

// AppendLog opens the named log file for appending, creating it if it does not exist. Unlike with
// [os.OpenFile], the file can be removed while it is open, including on Windows: log files may be
// kept open by long-lived processes (such as job servers), which must not prevent the removal of
// the directory they are in.
func AppendLog(name string) (*os.File, error) {
	return openLog(name)
}
