// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package proxy

import (
	"fmt"
	"reflect"
	"strings"
)

const structTagKey = "ddflag"

// parseFlags walks through the given arguments and sets the flagSet values
// present in the argument list. Unknown options, not present in the flagSet
// are accepted and skipped. The argument list is not modified.
func parseFlags(flagSet any, args []string) {
	flagSetValueMap := makeFlagSetValueMap(flagSet)

	i := 0
	for i < len(args)-1 {
		_, shift := parseOption(flagSetValueMap, args[i], args[i+1])
		i += shift
	}

	if i < len(args) {
		_, _ = parseOption(flagSetValueMap, args[i], "")
	}
}

func makeFlagSetValueMap(flagSet any) map[string]reflect.Value {
	v := reflect.ValueOf(flagSet).Elem()
	if v.Kind() != reflect.Struct {
		panic(fmt.Errorf("flagSet type %T is not a struct", flagSet))
	}
	typ := v.Type()
	flagSetValueMap := make(map[string]reflect.Value, v.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if tag, ok := field.Tag.Lookup(structTagKey); ok {
			flagSetValueMap[tag] = v.Field(i)
		}
	}
	return flagSetValueMap
}

// parseOption parses the given current argument and following one according to
// the go Flags syntax.
func parseOption(flagSetValueMap map[string]reflect.Value, arg, nextArg string) (nonOpt bool, shift int) {
	if arg[0] != '-' {
		// Not an option, return the value and shift by one.
		return true, 1
	}

	// Split the argument by its first `=` character if any, and check the
	// syntax being used.
	option, value, hasValue := strings.Cut(arg, "=")
	flag, exists := flagSetValueMap[option]

	if hasValue {
		// `-opt=val` syntax
		shift = 1
		if exists {
			flag.SetString(value)
		}
	} else if nextArg == "" || len(nextArg) > 1 && nextArg[0] != '-' {
		// `-opt val` syntax
		value := nextArg
		shift = 2
		if exists {
			switch flag.Kind() {
			case reflect.String:
				flag.SetString(value)
			case reflect.Bool:
				flag.SetBool(true)
				shift = 1
			default:
				panic(fmt.Errorf("unsupported value kind: %s", flag.Kind()))
			}
		}
	} else {
		// `-opt` syntax (no value)
		shift = 1
		if exists {
			if flag.Kind() != reflect.Bool {
				panic(fmt.Sprintf("missing value for %s flag", flag.Kind()))
			}
			flag.SetBool(true)
		}
	}

	return
}
