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

func openLog(name string) (*os.File, error) {
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}

	// Unlike [os.OpenFile], also allow the file to be deleted while it is open; otherwise, Windows
	// refuses to remove it (and hence the directory containing it) until it is closed. The access
	// rights are those [os.OpenFile] uses for O_WRONLY|O_APPEND.
	handle, err := windows.CreateFile(
		path,
		windows.FILE_APPEND_DATA|windows.FILE_WRITE_ATTRIBUTES|windows.FILE_WRITE_EA|windows.STANDARD_RIGHTS_WRITE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return os.NewFile(uintptr(handle), name), nil
}
