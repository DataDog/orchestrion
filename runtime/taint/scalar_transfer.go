// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

type scalarByte struct {
	value   byte
	tainted bool
}

type scalarRune struct {
	value   rune
	tainted bool
}

// NewScalarByteMap creates a byte scalar map with embedded provenance.
func NewScalarByteMap[K comparable]() map[K]scalarByte {
	return make(map[K]scalarByte)
}

// NewScalarRuneMap creates a rune scalar map with embedded provenance.
func NewScalarRuneMap[K comparable]() map[K]scalarRune {
	return make(map[K]scalarRune)
}

// NewScalarByteChannel creates a byte scalar channel with embedded provenance.
func NewScalarByteChannel(capacity int) chan scalarByte {
	return make(chan scalarByte, capacity)
}

// NewScalarRuneChannel creates a rune scalar channel with embedded provenance.
func NewScalarRuneChannel(capacity int) chan scalarRune {
	return make(chan scalarRune, capacity)
}

// PackByte captures the taint attached to value.
func PackByte(value *byte) scalarByte {
	return scalarByte{value: *value, tainted: active.byteScalarTainted(scalarAddress(value))}
}

// PackRune captures the taint attached to value.
func PackRune(value *rune) scalarRune {
	return scalarRune{value: *value, tainted: active.runeScalarTainted(scalarAddress(value))}
}

// CleanByte creates an explicitly clean byte packet.
func CleanByte(value byte) scalarByte {
	return scalarByte{value: value}
}

// CleanRune creates an explicitly clean rune packet.
func CleanRune(value rune) scalarRune {
	return scalarRune{value: value}
}

// CallByte evaluates function and transfers value's taint to destination.
func CallByte[T ~byte](destination *T, function func(T) T, value *T) {
	*destination = function(*value)
	active.setByteScalarTaint(scalarAddress(destination), active.byteScalarTainted(scalarAddress(value)))
}

// CallRune evaluates function and transfers value's taint to destination.
func CallRune[T ~rune](destination *T, function func(T) T, value *T) {
	*destination = function(*value)
	active.setRuneScalarTaint(scalarAddress(destination), active.runeScalarTainted(scalarAddress(value)))
}

// MapSetByte stores a byte packet.
func MapSetByte[K comparable](values map[K]scalarByte, key K, value scalarByte) {
	values[key] = value
}

// MapSetRune stores a rune packet.
func MapSetRune[K comparable](values map[K]scalarRune, key K, value scalarRune) {
	values[key] = value
}

// MapGetByte restores byte provenance from a packet map lookup.
func MapGetByte[K comparable](destination *byte, values map[K]scalarByte, key K) {
	packet := values[key]
	*destination = packet.value
	active.setByteScalarTaint(scalarAddress(destination), packet.tainted)
}

// MapGetRune restores rune provenance from a packet map lookup.
func MapGetRune[K comparable](destination *rune, values map[K]scalarRune, key K) {
	packet := values[key]
	*destination = packet.value
	active.setRuneScalarTaint(scalarAddress(destination), packet.tainted)
}

// SendByte sends a byte packet.
func SendByte(channel chan<- scalarByte, value scalarByte) {
	channel <- value
}

// SendRune sends a rune packet.
func SendRune(channel chan<- scalarRune, value scalarRune) {
	channel <- value
}

// ReceiveByte restores byte provenance from the received packet.
func ReceiveByte(destination *byte, channel <-chan scalarByte) {
	packet := <-channel
	*destination = packet.value
	active.setByteScalarTaint(scalarAddress(destination), packet.tainted)
}

// ReceiveRune restores rune provenance from the received packet.
func ReceiveRune(destination *rune, channel <-chan scalarRune) {
	packet := <-channel
	*destination = packet.value
	active.setRuneScalarTaint(scalarAddress(destination), packet.tainted)
}
