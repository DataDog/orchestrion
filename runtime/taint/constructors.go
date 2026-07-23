// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

// FormatString formats values and conservatively taints the full result.
func FormatString(format string, arguments ...any) string {
	result := fmt.Sprintf(format, arguments...)
	if len(RangesString(format)) > 0 || anyArgumentTainted(arguments) {
		registerString(result, []Range{{Start: 0, End: len(result)}})
	}
	return result
}

// JoinStrings joins elements and preserves exact element and separator ranges.
func JoinStrings(elements []string, separator string) string {
	result := strings.Join(elements, separator)
	ranges := make([]Range, 0)
	offset := 0
	for index, element := range elements {
		if index > 0 {
			ranges = append(ranges, shiftedRanges(RangesString(separator), offset)...)
			offset += len(separator)
		}
		ranges = append(ranges, shiftedRanges(RangesString(element), offset)...)
		offset += len(element)
	}
	registerString(result, ranges)
	return result
}

// JoinPath joins path elements and conservatively taints the full result.
func JoinPath(elements ...string) string {
	result := filepath.Join(elements...)
	for _, element := range elements {
		if len(RangesString(element)) > 0 {
			registerString(result, []Range{{Start: 0, End: len(result)}})
			break
		}
	}
	return result
}

func anyArgumentTainted(arguments []any) bool {
	for _, argument := range arguments {
		if reflectedValueTainted(reflect.ValueOf(argument), make(map[visit]struct{})) {
			return true
		}
	}
	return false
}

type visit struct {
	typeOf  reflect.Type
	pointer uintptr
}

func reflectedValueTainted(value reflect.Value, visited map[visit]struct{}) bool {
	if !value.IsValid() {
		return false
	}
	switch value.Kind() {
	case reflect.Interface:
		return !value.IsNil() && reflectedValueTainted(value.Elem(), visited)
	case reflect.Pointer:
		if value.IsNil() || seen(value, visited) {
			return false
		}
		return reflectedValueTainted(value.Elem(), visited)
	case reflect.String:
		return len(RangesString(value.String())) > 0
	case reflect.Slice:
		if value.IsNil() {
			return false
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return len(RangesBytes(value.Bytes())) > 0
		}
		if seen(value, visited) {
			return false
		}
		return indexedValuesTainted(value, visited)
	case reflect.Array:
		return indexedValuesTainted(value, visited)
	case reflect.Map:
		if value.IsNil() || seen(value, visited) {
			return false
		}
		iterator := value.MapRange()
		for iterator.Next() {
			if reflectedValueTainted(iterator.Key(), visited) || reflectedValueTainted(iterator.Value(), visited) {
				return true
			}
		}
	case reflect.Struct:
		for index := range value.NumField() {
			if reflectedValueTainted(value.Field(index), visited) {
				return true
			}
		}
	}
	return false
}

func indexedValuesTainted(value reflect.Value, visited map[visit]struct{}) bool {
	for index := range value.Len() {
		if reflectedValueTainted(value.Index(index), visited) {
			return true
		}
	}
	return false
}

func seen(value reflect.Value, visited map[visit]struct{}) bool {
	current := visit{typeOf: value.Type(), pointer: value.Pointer()}
	if _, exists := visited[current]; exists {
		return true
	}
	visited[current] = struct{}{}
	return false
}
