// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package may

import (
	"fmt"
)

// MatchType is an enumeration of the possible outcomes of a join point
type MatchType int

const (
	// Match indicates that the join point may match the given context
	Match MatchType = iota
	// CantMatch indicates that the join point DOES NOT match the given context
	CantMatch
	// Unknown indicates that the join point cannot determine whether it matches the given context
	Unknown
)

// Not returns the logical NOT of a MatchType value
// Truth table:
//
// | A       | NOT A   |
// |---------|---------|
// | Cant    | Match   |
// | Unknown | Unknown |
// | Match   | Cant    |
func (m MatchType) Not() MatchType {
	switch m {
	case Match:
		return CantMatch
	case CantMatch:
		return Match
	case Unknown:
		return Unknown
	default:
		panic(fmt.Sprintf("unknown MatchType: %d", m))
	}
}

// Or returns the logical OR of two MatchType values
// Truth table:
//
// | A       | B       | A OR B  |
// |---------|---------|---------|
// | Cant    | Cant    | Cant    |
// | Cant    | Unknown | Unknown |
// | Cant    | May     | Match   |
// | Unknown | Unknown | Unknown |
// | Unknown | Match   | Match   |
// | Match   | Match   | Match   |
func (m MatchType) Or(other MatchType) MatchType {
	if m == Match || other == Match {
		return Match
	}

	if m == CantMatch && other == CantMatch {
		return CantMatch
	}

	return Unknown
}

// And returns the logical AND of two MatchType values
// Truth table:
//
// | A       | B       | A AND B |
// |---------|---------|---------|
// | Cant    | Cant    | Cant    |
// | Cant    | Unknown | Cant    |
// | Cant    | Match   | Cant    |
// | Unknown | Unknown | Unknown |
// | Unknown | Match   | Unknown |
// | Match   | Match   | Match   |
func (m MatchType) And(other MatchType) MatchType {
	if m == CantMatch || other == CantMatch {
		return CantMatch
	}
	if m == Match && other == Match {
		return Match
	}
	return Unknown
}
