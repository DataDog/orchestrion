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
	input []rune // Changed from string to []rune for proper Unicode handling
	pos   int
}

// parseType is the main entry point for parsing a type string.
func parseType(s string) (Type, error) {
	p := &typeParser{input: []rune(strings.TrimSpace(s)), pos: 0}
	if len(p.input) == 0 {
		return nil, errors.New("empty type string")
	}

	t, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}

	// Ensure we've consumed the entire input
	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected characters after type: %q", string(p.input[p.pos:]))
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
//   - Interface types: "interface{}/any", "interface{ Method() }"
//   - Struct types: "struct{}", "struct{ Name string }"
//   - Generic types: "List[T]", "Map[K, V]"
//
// TODO: Add support for these types if needed in the future.
func (p *typeParser) parseTypeExpr() (Type, error) {
	p.skipWhitespace()

	if p.pos >= len(p.input) {
		return nil, errors.New("unexpected end of input")
	}

	// Use switch for first character to determine type
	switch p.input[p.pos] {
	case '*':
		return p.parsePointer()
	case '[':
		return p.parseSliceOrArray()
	default:
		// Check for keyword-based types
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
}

// parsePointer parses a pointer type starting at '*'
func (p *typeParser) parsePointer() (Type, error) {
	p.pos++ // consume '*'
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

	// Check for special number formats starting with 0
	if p.pos < len(p.input) && p.input[p.pos] == '0' {
		// Might be 0x, 0b, 0o, or just 0
		if err := p.parseZeroPrefixedNumber(); err != nil {
			return 0, err
		}
	} else {
		// Regular decimal number
		if !p.consumeDigits() {
			return 0, errors.New("expected array size")
		}
	}

	// Convert the parsed number string to integer
	sizeStr := string(p.input[start:p.pos])
	// Parse with platform-appropriate bit size to match int type
	size, err := strconv.ParseInt(sizeStr, 0, strconv.IntSize)
	if err != nil {
		return 0, fmt.Errorf("invalid array size %q: %w", sizeStr, err)
	}

	if size < 0 {
		return 0, errors.New("array size cannot be negative")
	}

	return int(size), nil
}

// parseZeroPrefixedNumber handles numbers starting with 0 (hex, binary, octal, or just zero)
func (p *typeParser) parseZeroPrefixedNumber() error {
	p.pos++ // consume '0'

	// If at end of input or non-digit follows, it's just zero
	if p.pos >= len(p.input) || !unicode.IsDigit(p.input[p.pos]) && p.input[p.pos] != 'x' && p.input[p.pos] != 'X' && p.input[p.pos] != 'b' && p.input[p.pos] != 'B' && p.input[p.pos] != 'o' && p.input[p.pos] != 'O' {
		return nil
	}

	// Check the prefix character
	switch p.input[p.pos] {
	case 'x', 'X':
		// Hexadecimal
		p.pos++
		if !p.consumeHexDigits() {
			return errors.New("invalid hex number")
		}
	case 'b', 'B':
		// Binary
		p.pos++
		if !p.consumeBinaryDigits() {
			return errors.New("invalid binary number")
		}
	case 'o', 'O':
		// Explicit octal (Go 1.13+)
		p.pos++
		if !p.consumeOctalDigits() {
			return errors.New("invalid octal number")
		}
	default:
		// Traditional octal (leading 0) or just zero
		p.pos-- // Back up to include the leading 0
		p.consumeDigits()
	}

	return nil
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

	// Save the full type starting from current position
	end := p.pos
	for end < len(p.input) {
		ch := p.input[end]
		// Stop at characters that can't be part of a type name
		if ch == '[' || ch == ']' || ch == '*' || unicode.IsSpace(ch) {
			break
		}
		end++
	}

	fullType := p.input[start:end]
	p.pos = end

	// Look for the last dot that comes after a slash (indicating package.Type)
	lastDotAfterSlash := -1
	hasSlash := false

	for i := 0; i < len(fullType); i++ {
		ch := fullType[i]
		if ch == '/' {
			hasSlash = true
		} else if ch == '.' && hasSlash {
			lastDotAfterSlash = i
		}
	}

	// If we found a package separator, split there
	if lastDotAfterSlash > 0 {
		if !isValidIdentifier(fullType[lastDotAfterSlash+1:]) {
			return nil, fmt.Errorf("invalid type name after package path: %q", string(fullType[lastDotAfterSlash+1:]))
		}

		return &NamedType{
			Path: string(fullType[:lastDotAfterSlash]),
			Name: string(fullType[lastDotAfterSlash+1:]),
		}, nil
	}

	// Check if there's a dot without a slash (e.g., "time.Duration", "context.Context")
	lastDot := strings.LastIndex(string(fullType), ".")
	if lastDot > 0 {
		if isValidIdentifier(fullType[:lastDot]) && isValidIdentifier(fullType[lastDot+1:]) {
			return &NamedType{
				Path: string(fullType[:lastDot]),
				Name: string(fullType[lastDot+1:]),
			}, nil
		}
	}

	// No package path, just a type name
	if len(fullType) > 0 {
		// Validate that it's a valid identifier
		if !isValidIdentifier(fullType) {
			return nil, fmt.Errorf("invalid type syntax: %q", string(fullType))
		}
		return &NamedType{
			Name: string(fullType),
		}, nil
	}

	return nil, fmt.Errorf("expected type name at position %d", start)
}

// Helper methods

func (p *typeParser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(p.input[p.pos]) {
		p.pos++
	}
}

func (p *typeParser) consumeKeyword(keyword string) bool {
	keywordRunes := []rune(keyword)
	if p.pos+len(keywordRunes) > len(p.input) {
		return false
	}

	if string(p.input[p.pos:p.pos+len(keywordRunes)]) != keyword {
		return false
	}

	// Ensure the keyword is not part of a larger identifier
	if p.pos+len(keywordRunes) >= len(p.input) {
		// Keyword is at the end of input, it's valid
		p.pos += len(keywordRunes)
		return true
	}

	// Check if next character would continue the identifier
	next := p.input[p.pos+len(keywordRunes)]
	if unicode.IsLetter(next) || unicode.IsDigit(next) || next == '_' {
		return false
	}

	p.pos += len(keywordRunes)
	return true
}

// isHexDigit checks if a rune is a valid hexadecimal digit
func isHexDigit(ch rune) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// isBinaryDigit checks if a rune is a valid binary digit
func isBinaryDigit(ch rune) bool {
	return ch == '0' || ch == '1'
}

// isOctalDigit checks if a rune is a valid octal digit
func isOctalDigit(ch rune) bool {
	return ch >= '0' && ch <= '7'
}

// consumeDigitsWithPredicate consumes digits with underscore separators
func (p *typeParser) consumeDigitsWithPredicate(isValidDigit func(rune) bool) bool {
	start := p.pos
	hasDigit := false

	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if isValidDigit(ch) {
			hasDigit = true
			p.pos++
			continue
		}

		// Check for underscore separator
		if ch == '_' && p.pos > start && p.pos+1 < len(p.input) && isValidDigit(p.input[p.pos+1]) {
			p.pos++
			continue
		}

		// Not a valid digit or underscore pattern
		break
	}

	return hasDigit
}

func (p *typeParser) consumeDigits() bool {
	return p.consumeDigitsWithPredicate(unicode.IsDigit)
}

func (p *typeParser) consumeHexDigits() bool {
	return p.consumeDigitsWithPredicate(isHexDigit)
}

func (p *typeParser) consumeBinaryDigits() bool {
	return p.consumeDigitsWithPredicate(isBinaryDigit)
}

func (p *typeParser) consumeOctalDigits() bool {
	return p.consumeDigitsWithPredicate(isOctalDigit)
}

func (p *typeParser) consumeIdentifier() bool {
	// Early return if at end of input
	if p.pos >= len(p.input) {
		return false
	}

	// First character must be a letter or underscore
	ch := p.input[p.pos]
	if !unicode.IsLetter(ch) && ch != '_' {
		return false
	}
	p.pos++

	// Consume subsequent characters
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			break
		}
		p.pos++
	}

	return true
}

// consumePackageComponent consumes a package path component which may contain dashes
func (p *typeParser) consumePackageComponent() bool {
	// Early return if at end of input
	if p.pos >= len(p.input) {
		return false
	}

	// First character must be a letter, digit or underscore (not dash)
	ch := p.input[p.pos]
	if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
		return false
	}
	p.pos++

	// Consume subsequent characters (can include dashes)
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' && ch != '-' {
			break
		}
		p.pos++
	}

	return true
}

// isValidIdentifier checks if a slice of runes is a valid Go identifier
func isValidIdentifier(runes []rune) bool {
	if len(runes) == 0 {
		return false
	}

	// First character must be a letter or underscore
	if !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		return false
	}

	// Subsequent characters must be letters, digits, or underscores
	for _, ch := range runes[1:] {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' {
			return false
		}
	}

	return true
}
