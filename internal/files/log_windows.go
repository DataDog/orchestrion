// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

//go:build windows

package files

import (
	"os"

	"golang.org/x/sys/windows"
)

func openLog(name string, truncate bool) (*os.File, error) {
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}

	// These are the access rights and creation dispositions [os.OpenFile] uses for the equivalent
	// flags (O_WRONLY|O_CREATE, with either O_TRUNC or O_APPEND).
	access := uint32(windows.FILE_APPEND_DATA | windows.FILE_WRITE_ATTRIBUTES | windows.FILE_WRITE_EA | windows.STANDARD_RIGHTS_WRITE | windows.SYNCHRONIZE)
	disposition := uint32(windows.OPEN_ALWAYS)
	if truncate {
		access = windows.GENERIC_WRITE
		disposition = windows.CREATE_ALWAYS
	}

	// Unlike [os.OpenFile], also allow the file to be deleted while it is open; otherwise, Windows
	// refuses to remove it (and hence the directory containing it) until it is closed.
	handle, err := windows.CreateFile(
		path,
		access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		disposition,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return os.NewFile(uintptr(handle), name), nil
}
