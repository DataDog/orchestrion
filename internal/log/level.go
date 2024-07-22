// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package log

import "strings"

type Level int

const (
	LevelNone  Level = iota // No logging at all
	LevelError              // Log only ERROR messages
	LevelWarn               // Log ERROR and WARN messages
	LevelInfo               // Log ERROR, WARN, and INFO messages
	LevelDebug              // Log ERROR, WARN, INFO, and DEBUG messages
	LevelTrace              // Log ERROR, WARN, INFO, DEBUG, and TRACE messages
)

func LevelNamed(name string) (Level, bool) {
	switch strings.ToUpper(name) {
	case "NONE", "OFF":
		return LevelNone, true
	case "ERROR":
		return LevelError, true
	case "WARN":
		return LevelWarn, true
	case "INFO":
		return LevelInfo, true
	case "DEBUG":
		return LevelDebug, true
	case "TRACE":
		return LevelTrace, true
	default:
		return LevelNone, false
	}
}

func (l Level) Printf(format string, args ...any) {
	write(l, format, args...)
}

func (l Level) String() string {
	switch l {
	case LevelError:
		return "‼️ ERROR"
	case LevelWarn:
		return "⚠️ WARN"
	case LevelInfo:
		return "ℹ️ INFO"
	case LevelDebug:
		return "🐛 DEBUG"
	case LevelTrace:
		return "🐾 TRACE"
	default:
		return "NONE"
	}
}
