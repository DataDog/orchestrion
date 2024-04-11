// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package log

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

var (
	level            = LevelNone
	writer  *os.File = os.Stderr
	writerM sync.Mutex

	context     = make(map[string]string)
	contextKeys []string
	contextM    sync.RWMutex
)

func Close() (err error) {
	writerM.Lock()
	defer writerM.Unlock()

	if writer != os.Stderr && writer != os.Stdout {
		err = writer.Close()
	}
	writer = os.Stderr

	contextM.Lock()
	defer contextM.Unlock()
	context = make(map[string]string)

	return
}

func SetLevel(l Level) {
	level = l
}

func SetOutput(f *os.File) {
	writerM.Lock()
	defer writerM.Unlock()

	writer = f
}

func SetContext(key string, value string) {
	contextM.Lock()
	defer contextM.Unlock()

	if value != "" {
		if _, found := context[key]; !found {
			contextKeys = append(contextKeys, key)
		}
		context[key] = value
	} else {
		delete(context, key)
		for i := 0; i < len(contextKeys); {
			if contextKeys[i] == key {
				contextKeys = append(contextKeys[:i], contextKeys[i+1:]...)
			} else {
				i++
			}
		}
	}
}

func Errorf(format string, args ...any) {
	write(LevelError, format, args...)
}

func Warnf(format string, args ...any) {
	write(LevelWarn, format, args...)
}

func Infof(format string, args ...any) {
	write(LevelInfo, format, args...)
}

func Debugf(format string, args ...any) {
	write(LevelDebug, format, args...)
}

func Tracef(format string, args ...any) {
	write(LevelTrace, format, args...)
}

func write(at Level, format string, args ...any) {
	if at > level {
		return
	}

	writerM.Lock()
	defer writerM.Unlock()

	// We flock the output file to ensure lines don't get mangled by concurrent access. On Windows
	// with NTFS, if the log file is a O_APPEND file, this also has the benefit of preventing further
	// data corruption, as NTFS tries its best to emulate O_APPEND, but this is brittle.
	syscall.Flock(int(writer.Fd()), syscall.LOCK_EX)
	defer syscall.Flock(int(writer.Fd()), syscall.LOCK_UN)

	fmt.Fprintf(writer, "[%-7s", at)

	contextM.RLock()
	defer contextM.RUnlock()
	for _, key := range contextKeys {
		fmt.Fprintf(writer, "|%s=%s", key, context[key])
	}

	fmt.Fprint(writer, "] ")
	fmt.Fprintf(writer, format, args...)
}
