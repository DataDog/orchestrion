// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package taint

import (
	"bytes"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type scalarTransferByte byte

type scalarTransferRune rune

// forceScalarStackGrowth prevents inlining while growing the current goroutine stack.
//
//go:noinline
func forceScalarStackGrowth(depth int) byte {
	var frame [2048]byte
	frame[0] = byte(depth)
	if depth == 0 {
		return frame[0]
	}
	return frame[0] + forceScalarStackGrowth(depth-1)
}

func Test_ByteScalarTaintSurvivesStackGrowth(t *testing.T) {
	// Given
	value := byte('s')
	MarkByteFromString(&value, SourceString("s"), 0)
	forceScalarStackGrowth(128)

	// When
	var byteBuffer bytes.Buffer
	byteError := BufferWriteByte(&byteBuffer, &value)
	var runeBuffer bytes.Buffer
	_, runeError := BufferWriteRuneFromByte(&runeBuffer, &value)
	ReleaseByte(&value)

	// Then
	if byteError != nil {
		t.Fatalf("BufferWriteByte() error = %v", byteError)
	}
	if runeError != nil {
		t.Fatalf("BufferWriteRuneFromByte() error = %v", runeError)
	}
	requireRanges(t, RangesBytes(byteBuffer.Bytes()), Range{Start: 0, End: 1})
	requireRanges(t, RangesBytes(runeBuffer.Bytes()), Range{Start: 0, End: 1})
}

func Test_CallByteTransfersAndClearsScalarTaint(t *testing.T) {
	// Given
	source := byte('s')
	MarkByteFromString(&source, SourceString("s"), 0)
	t.Cleanup(func() { ReleaseByte(&source) })
	var destination byte
	t.Cleanup(func() { ReleaseByte(&destination) })

	// When
	CallByte(&destination, func(value byte) byte { return value }, &source)

	// Then
	require.True(t, active.byteScalarTainted(scalarAddress(&destination)))

	// When
	clean := byte('c')
	CallByte(&destination, func(value byte) byte { return value }, &clean)

	// Then
	require.False(t, active.byteScalarTainted(scalarAddress(&destination)))
}

func Test_CallRuneTransfersAndClearsScalarTaint(t *testing.T) {
	// Given
	source := rune('s')
	MarkRuneFromRunes(&source, StringToRunesAs[[]rune](SourceString("s")), 0)
	t.Cleanup(func() { ReleaseRune(&source) })
	var destination rune
	t.Cleanup(func() { ReleaseRune(&destination) })

	// When
	CallRune(&destination, func(value rune) rune { return value }, &source)

	// Then
	require.True(t, active.runeScalarTainted(scalarAddress(&destination)))

	// When
	clean := rune('c')
	CallRune(&destination, func(value rune) rune { return value }, &clean)

	// Then
	require.False(t, active.runeScalarTainted(scalarAddress(&destination)))
}

func Test_CallTransfersNamedScalarTaint(t *testing.T) {
	// Given
	byteSource := scalarTransferByte('s')
	MarkByteFromString(&byteSource, SourceString("s"), 0)
	t.Cleanup(func() { ReleaseByte(&byteSource) })
	var byteDestination scalarTransferByte
	t.Cleanup(func() { ReleaseByte(&byteDestination) })
	runeSource := scalarTransferRune('s')
	MarkRuneFromRunes(&runeSource, StringToRunesAs[[]rune](SourceString("s")), 0)
	t.Cleanup(func() { ReleaseRune(&runeSource) })
	var runeDestination scalarTransferRune
	t.Cleanup(func() { ReleaseRune(&runeDestination) })

	// When
	CallByte(&byteDestination, func(value scalarTransferByte) scalarTransferByte { return value }, &byteSource)
	CallRune(&runeDestination, func(value scalarTransferRune) scalarTransferRune { return value }, &runeSource)

	// Then
	require.True(t, active.byteScalarTainted(scalarAddress(&byteDestination)))
	require.True(t, active.runeScalarTainted(scalarAddress(&runeDestination)))
}

func Test_ScalarByteMapPacketsClearOnLiteralOverwriteDeleteAndClear(t *testing.T) {
	// Given
	values := NewScalarByteMap[string]()
	source := byte('s')
	MarkByteFromString(&source, SourceString("s"), 0)
	t.Cleanup(func() { ReleaseByte(&source) })
	var destination byte
	t.Cleanup(func() { ReleaseByte(&destination) })

	// When
	MapSetByte(values, "value", PackByte(&source))
	MapGetByte(&destination, values, "value")

	// Then
	require.True(t, active.byteScalarTainted(scalarAddress(&destination)))

	// When
	MapSetByte(values, "value", CleanByte('c'))
	MapGetByte(&destination, values, "value")

	// Then
	require.False(t, active.byteScalarTainted(scalarAddress(&destination)))

	// When
	MapSetByte(values, "value", PackByte(&source))
	delete(values, "value")
	MapGetByte(&destination, values, "value")

	// Then
	require.False(t, active.byteScalarTainted(scalarAddress(&destination)))

	// When
	MapSetByte(values, "value", PackByte(&source))
	clear(values)
	MapGetByte(&destination, values, "value")

	// Then
	require.False(t, active.byteScalarTainted(scalarAddress(&destination)))
}

func Test_ScalarRuneMapPacketsClearOnLiteralOverwriteDeleteAndClear(t *testing.T) {
	// Given
	values := NewScalarRuneMap[string]()
	source := rune('s')
	MarkRuneFromRunes(&source, StringToRunesAs[[]rune](SourceString("s")), 0)
	t.Cleanup(func() { ReleaseRune(&source) })
	var destination rune
	t.Cleanup(func() { ReleaseRune(&destination) })

	// When
	MapSetRune(values, "value", PackRune(&source))
	MapGetRune(&destination, values, "value")

	// Then
	require.True(t, active.runeScalarTainted(scalarAddress(&destination)))

	// When
	MapSetRune(values, "value", CleanRune('c'))
	MapGetRune(&destination, values, "value")

	// Then
	require.False(t, active.runeScalarTainted(scalarAddress(&destination)))

	// When
	MapSetRune(values, "value", PackRune(&source))
	delete(values, "value")
	MapGetRune(&destination, values, "value")

	// Then
	require.False(t, active.runeScalarTainted(scalarAddress(&destination)))

	// When
	MapSetRune(values, "value", PackRune(&source))
	clear(values)
	MapGetRune(&destination, values, "value")

	// Then
	require.False(t, active.runeScalarTainted(scalarAddress(&destination)))
}

func Test_ScalarByteChannelPacketsStayBoundToConcurrentSenders(t *testing.T) {
	// Given
	channel := NewScalarByteChannel(2)
	tainted := byte('s')
	MarkByteFromString(&tainted, SourceString("s"), 0)
	t.Cleanup(func() { ReleaseByte(&tainted) })
	var senders sync.WaitGroup
	senders.Add(2)
	go func() {
		defer senders.Done()
		SendByte(channel, PackByte(&tainted))
	}()
	go func() {
		defer senders.Done()
		SendByte(channel, CleanByte('c'))
	}()
	senders.Wait()

	// When
	var first, second byte
	ReceiveByte(&first, channel)
	firstTainted := active.byteScalarTainted(scalarAddress(&first))
	ReceiveByte(&second, channel)
	secondTainted := active.byteScalarTainted(scalarAddress(&second))

	// Then
	require.NotEqual(t, first, second)
	require.Equal(t, first == 's', firstTainted)
	require.Equal(t, second == 's', secondTainted)
}

func Test_ScalarRuneChannelPacketsStayBoundToConcurrentSenders(t *testing.T) {
	// Given
	channel := NewScalarRuneChannel(2)
	tainted := rune('s')
	MarkRuneFromRunes(&tainted, StringToRunesAs[[]rune](SourceString("s")), 0)
	t.Cleanup(func() { ReleaseRune(&tainted) })
	var senders sync.WaitGroup
	senders.Add(2)
	go func() {
		defer senders.Done()
		SendRune(channel, PackRune(&tainted))
	}()
	go func() {
		defer senders.Done()
		SendRune(channel, CleanRune('c'))
	}()
	senders.Wait()

	// When
	var first, second rune
	ReceiveRune(&first, channel)
	firstTainted := active.runeScalarTainted(scalarAddress(&first))
	ReceiveRune(&second, channel)
	secondTainted := active.runeScalarTainted(scalarAddress(&second))

	// Then
	require.NotEqual(t, first, second)
	require.Equal(t, first == 's', firstTainted)
	require.Equal(t, second == 's', secondTainted)
}
