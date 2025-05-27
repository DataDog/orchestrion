// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package typed

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// typeParser is a simple recursive descent parser for Go type expressions.
type typeParser struct {
	input string
	pos   int
}

// parseType is the main entry point for parsing a type string.
func parseType(s string) (Type, error) {
	p := &typeParser{input: strings.TrimSpace(s), pos: 0}
	if p.input == "" {
		return nil, errors.New("empty type string")
	}

	t, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}

	// Ensure we've consumed the entire input
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected characters after type: %q", p.input[p.pos:])
	}

	return t, nil
}

// parseTypeExpr parses a complete type expression.
//
// Currently supported:
//   - Named types: "string", "net/http.Request", "time.Duration"
//   - Pointer types: "*string", "*net/http.Request"
//   - Slice types: "[]string", "[]*User"
//   - Array types: "[10]string", "[0xFF]byte"
//   - Map types: "map[string]int", "map[string]*User"
//
// Not yet supported (will return an error):
//   - Channel types: "chan int", "<-chan int", "chan<- int"
//   - Function types: "func()", "func(int) string"
//   - Interface types: "interface{}", "interface{ Method() }"
//   - Struct types: "struct{}", "struct{ Name string }"
//   - Generic types: "List[T]", "Map[K, V]"
//
// TODO: Add support for these types if needed in the future.
func (p *typeParser) parseTypeExpr() (Type, error) {
	p.skipWhitespace()

	if p.pos >= len(p.input) {
		return nil, errors.New("unexpected end of input")
	}

	// Check for pointer
	if p.input[p.pos] == '*' {
		p.pos++
		p.skipWhitespace()

		// Check for invalid double pointer with space (e.g., "* *string")
		if p.pos < len(p.input) && p.input[p.pos] == '*' {
			return nil, errors.New("invalid pointer syntax: space between * operators")
		}

		elem, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
		return &PointerType{Elem: elem}, nil
	}

	// Check for slice or array
	if p.input[p.pos] == '[' {
		return p.parseSliceOrArray()
	}

	// Check for map
	if p.consumeKeyword("map") {
		return p.parseMap()
	}

	// TODO: Add support for channels (check for "chan" keyword)
	// TODO: Add support for functions (check for "func" keyword)
	// TODO: Add support for interfaces (check for "interface" keyword)
	// TODO: Add support for structs (check for "struct" keyword)

	// Otherwise, it must be a named type
	return p.parseNamedType()
}

// parseSliceOrArray parses slice or array types starting after the '['.
func (p *typeParser) parseSliceOrArray() (Type, error) {
	p.pos++ // consume '['
	p.skipWhitespace()

	// Check if it's a slice (empty brackets)
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++ // consume ']'
		elem, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
		return &SliceType{Elem: elem}, nil
	}

	// It's an array, parse the size
	size, err := p.parseArraySize()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	if p.pos >= len(p.input) || p.input[p.pos] != ']' {
		return nil, errors.New("expected ']' after array size")
	}
	p.pos++ // consume ']'

	elem, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}

	return &ArrayType{Size: size, Elem: elem}, nil
}

// parseArraySize parses an integer literal for array size.
func (p *typeParser) parseArraySize() (int, error) {
	start := p.pos

	// Handle hex (0x...) or octal (0...) prefixes
	if p.pos < len(p.input) && p.input[p.pos] == '0' {
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == 'x' || p.input[p.pos] == 'X') {
			p.pos++
			// Parse hex digits
			if !p.consumeHexDigits() {
				return 0, errors.New("invalid hex number")
			}
		} else {
			// Parse octal digits
			p.consumeOctalDigits()
		}
	} else {
		// Parse decimal digits
		if !p.consumeDigits() {
			return 0, errors.New("expected array size")
		}
	}

	sizeStr := p.input[start:p.pos]
	size, err := strconv.ParseInt(sizeStr, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid array size %q: %w", sizeStr, err)
	}

	if size < 0 {
		return 0, errors.New("array size cannot be negative")
	}

	return int(size), nil
}

// parseMap parses a map type after the "map" keyword.
func (p *typeParser) parseMap() (Type, error) {
	p.skipWhitespace()

	if p.pos >= len(p.input) || p.input[p.pos] != '[' {
		return nil, errors.New("expected '[' after 'map'")
	}
	p.pos++ // consume '['

	// Parse key type
	key, err := p.parseTypeExpr()
	if err != nil {
		return nil, fmt.Errorf("invalid map key type: %w", err)
	}

	p.skipWhitespace()
	if p.pos >= len(p.input) || p.input[p.pos] != ']' {
		return nil, errors.New("expected ']' after map key type")
	}
	p.pos++ // consume ']'

	// Parse value type
	value, err := p.parseTypeExpr()
	if err != nil {
		return nil, fmt.Errorf("invalid map value type: %w", err)
	}

	return &MapType{Key: key, Value: value}, nil
}

// parseNamedType parses a named type (possibly qualified with a package).
func (p *typeParser) parseNamedType() (Type, error) {
	start := p.pos

	// Save the full string from start
	fullType := p.input[start:]

	// Look for the last dot that comes after a slash (indicating package.Type)
	lastDotAfterSlash := -1
	hasSlash := false

	for i := 0; i < len(fullType); i++ {
		ch := fullType[i]
		// Stop at characters that can't be part of a type name
		if ch == '[' || ch == ']' || ch == '*' || unicode.IsSpace(rune(ch)) {
			fullType = fullType[:i]
			break
		}
		if ch == '/' {
			hasSlash = true
		} else if ch == '.' && hasSlash {
			lastDotAfterSlash = i
		}
	}

	// Advance position by the length of what we consumed
	p.pos += len(fullType)

	// If we found a package separator, split there
	if lastDotAfterSlash > 0 {
		typeName := fullType[lastDotAfterSlash+1:]
		// Validate the type name
		if !isValidIdentifier(typeName) {
			return nil, fmt.Errorf("invalid type name after package path: %q", typeName)
		}
		return &NamedType{
			ImportPath: fullType[:lastDotAfterSlash],
			Name:       typeName,
		}, nil
	}

	// Check if there's a dot without a slash (e.g., "time.Duration", "context.Context")
	lastDot := strings.LastIndex(fullType, ".")
	if lastDot > 0 {
		pkg := fullType[:lastDot]
		typeName := fullType[lastDot+1:]

		// Validate both parts
		if isValidIdentifier(pkg) && isValidIdentifier(typeName) {
			return &NamedType{
				ImportPath: pkg,
				Name:       typeName,
			}, nil
		}
	}

	// No package path, just a type name
	if len(fullType) > 0 {
		// Validate that it's a valid identifier
		if !isValidIdentifier(fullType) {
			return nil, fmt.Errorf("invalid type syntax: %q", fullType)
		}
		return &NamedType{
			Name: fullType,
		}, nil
	}

	return nil, fmt.Errorf("expected type name at position %d", start)
}

// Helper methods

func (p *typeParser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *typeParser) consumeKeyword(keyword string) bool {
	if p.pos+len(keyword) > len(p.input) {
		return false
	}

	if p.input[p.pos:p.pos+len(keyword)] != keyword {
		return false
	}

	// Ensure the keyword is not part of a larger identifier
	if p.pos+len(keyword) < len(p.input) {
		next := p.input[p.pos+len(keyword)]
		if unicode.IsLetter(rune(next)) || unicode.IsDigit(rune(next)) || next == '_' {
			return false
		}
	}

	p.pos += len(keyword)
	return true
}

func (p *typeParser) consumeDigits() bool {
	start := p.pos
	for p.pos < len(p.input) && unicode.IsDigit(rune(p.input[p.pos])) {
		p.pos++
	}
	return p.pos > start
}

func (p *typeParser) consumeHexDigits() bool {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			break
		}
		p.pos++
	}
	return p.pos > start
}

func (p *typeParser) consumeOctalDigits() bool {
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '7' {
		p.pos++
	}
	return p.pos > start
}

func (p *typeParser) consumeIdentifier() bool {
	start := p.pos

	// First character must be a letter or underscore
	if p.pos >= len(p.input) {
		return false
	}
	ch := rune(p.input[p.pos])
	if !unicode.IsLetter(ch) && ch != '_' {
		return false
	}
	p.pos++

	// Subsequent characters can be letters, digits, or underscores
	for p.pos < len(p.input) {
		ch := rune(p.input[p.pos])
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		p.pos++
	}

	return p.pos > start
}

// consumePackageComponent consumes a package path component which may contain dashes
func (p *typeParser) consumePackageComponent() bool {
	start := p.pos

	// First character must be a letter, digit or underscore (not dash)
	if p.pos >= len(p.input) {
		return false
	}
	ch := rune(p.input[p.pos])
	if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
		return false
	}
	p.pos++

	// Subsequent characters can include dashes
	for p.pos < len(p.input) {
		ch := rune(p.input[p.pos])
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '-' {
			break
		}
		p.pos++
	}

	return p.pos > start
}

// isValidIdentifier checks if a string is a valid Go identifier
func isValidIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}

	// First character must be a letter or underscore
	ch := rune(s[0])
	if !unicode.IsLetter(ch) && ch != '_' {
		return false
	}

	// Subsequent characters must be letters, digits, or underscores
	for _, ch := range s[1:] {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			return false
		}
	}

	return true
}
