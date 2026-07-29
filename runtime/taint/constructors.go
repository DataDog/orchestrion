// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

// FormatString formats values and conservatively taints the full result.
func FormatString(format string, arguments ...any) string {
	result := fmt.Sprintf(format, arguments...)
	inputRanges := append(RangesString(format), argumentRanges(arguments)...)
	if ranges := conservativeRanges(len(result), inputRanges); len(ranges) > 0 {
		registerString(result, ranges)
	}
	return result
}

// JSONMarshal marshals value and conservatively taints the full byte result.
func JSONMarshal(value any) ([]byte, error) {
	result, err := json.Marshal(value)
	if err == nil {
		if ranges := conservativeRanges(len(result), argumentRanges([]any{value})); len(ranges) > 0 {
			registerBytes(result, ranges)
		}
	}
	return result, err
}

// XMLMarshal marshals value and conservatively taints the full byte result.
func XMLMarshal(value any) ([]byte, error) {
	result, err := xml.Marshal(value)
	if err == nil {
		if ranges := conservativeRanges(len(result), argumentRanges([]any{value})); len(ranges) > 0 {
			registerBytes(result, ranges)
		}
	}
	return result, err
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
	inputRanges := make([]Range, 0)
	for _, element := range elements {
		inputRanges = append(inputRanges, RangesString(element)...)
	}
	if ranges := conservativeRanges(len(result), inputRanges); len(ranges) > 0 {
		registerString(result, ranges)
	}
	return result
}

func argumentRanges(arguments []any) []Range {
	ranges := make([]Range, 0)
	for _, argument := range arguments {
		ranges = append(ranges, reflectedValueRanges(reflect.ValueOf(argument), make(map[visit]struct{}))...)
	}
	return ranges
}

type visit struct {
	typeOf  reflect.Type
	pointer uintptr
}

func reflectedValueRanges(value reflect.Value, visited map[visit]struct{}) []Range {
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return reflectedValueRanges(value.Elem(), visited)
	case reflect.Pointer:
		if value.IsNil() || seen(value, visited) {
			return nil
		}
		return reflectedValueRanges(value.Elem(), visited)
	case reflect.String:
		return RangesString(value.String())
	case reflect.Slice:
		if value.IsNil() {
			return nil
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return RangesBytes(value.Bytes())
		}
		if seen(value, visited) {
			return nil
		}
		return indexedValueRanges(value, visited)
	case reflect.Array:
		return indexedValueRanges(value, visited)
	case reflect.Map:
		if value.IsNil() || seen(value, visited) {
			return nil
		}
		result := make([]Range, 0)
		iterator := value.MapRange()
		for iterator.Next() {
			result = append(result, reflectedValueRanges(iterator.Key(), visited)...)
			result = append(result, reflectedValueRanges(iterator.Value(), visited)...)
		}
		return result
	case reflect.Struct:
		result := make([]Range, 0)
		for index := range value.NumField() {
			result = append(result, reflectedValueRanges(value.Field(index), visited)...)
		}
		return result
	}
	return nil
}

func indexedValueRanges(value reflect.Value, visited map[visit]struct{}) []Range {
	result := make([]Range, 0)
	for index := range value.Len() {
		result = append(result, reflectedValueRanges(value.Index(index), visited)...)
	}
	return result
}

func seen(value reflect.Value, visited map[visit]struct{}) bool {
	current := visit{typeOf: value.Type(), pointer: value.Pointer()}
	if _, exists := visited[current]; exists {
		return true
	}
	visited[current] = struct{}{}
	return false
}
