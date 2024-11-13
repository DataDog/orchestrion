// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package fingerprint_test

import (
	"fmt"
	"testing"

	"github.com/DataDog/orchestrion/internal/fingerprint"
	"github.com/stretchr/testify/require"
)

func TestCast(t *testing.T) {
	type mySlice []int
	require.Equal(
		t,
		fingerprint.List[fingerprint.Int]{0, -1, -2},
		fingerprint.Cast(mySlice{0, 1, 2}, func(i int) fingerprint.Int { return fingerprint.Int(-i) }),
	)
}

func TestFingerprint(t *testing.T) {
	cases := map[string]struct {
		hashable fingerprint.Hashable
		hash     string
	}{
		"nil": {
			hashable: nil,
			hash:     "z4PhNX7vuL3xVChQ1m2AB9Yg5AULVxXcg_SpIdNs6c5H0NE8XYXysP-DGNKHfuwvY7kxvUdBeoGlODJ6-SfaPg==",
		},
		"bool.true": {
			hashable: fingerprint.Bool(true),
			hash:     "kSDNX67wegjpcf8CSj_L6h46a0QUKm2CyijGxC5PhSWVvPU9gdd28QVBBFq9t8N5UGKUFdDcZsjYbGSlYG0y3g==",
		},
		"bool.false": {
			hashable: fingerprint.Bool(false),
			hash:     "cZ-mfu9JxLKiuD8MYr3diMEGqq234hrgV8iAK3AONvgf4_FEgS2LBdZtxmPZCLJWReFTJiz21FeqNOaEr54yjQ==",
		},
		"int": {
			hashable: fingerprint.Int(0),
			hash:     "MbygIJTreBJqUXsgaojHPPqexvcExwMNGCEsrOgg8CXwC_DqaNvz86VDbKY7U797-ArY1d59g1nQt_7Z28OrmQ==",
		},
		"string.empty": {
			hashable: fingerprint.String(""),
			hash:     "z4PhNX7vuL3xVChQ1m2AB9Yg5AULVxXcg_SpIdNs6c5H0NE8XYXysP-DGNKHfuwvY7kxvUdBeoGlODJ6-SfaPg==",
		},
		"string.test": {
			hashable: fingerprint.String("test"),
			hash:     "7iaw3Ur350mqGo7jwQrpkj9hiYB3Lkc_iBml1JQODbJ6wYX4oOHV-E-IvIh_1nsUNzLDBMxfqa2Ob1f1ACio_w==",
		},
		"list.empty": {
			hashable: fingerprint.List[fingerprint.Hashable]{},
			hash:     "t73cX9aZGc2y4Ofhd5w7m6czkOCqC7PYzMqxQIO9ZlVZCkbLEfnOFkJhitFdYdt7kmuggBZO2hECT92dJPtxcQ==",
		},
		"list.items": {
			hashable: fingerprint.List[fingerprint.Hashable]{fingerprint.Bool(true), fingerprint.Int(0), fingerprint.String("test")},
			hash:     "xhHdbmBljPK815Zc1NwNxLY2XMeZW_Qw60mpuAN61wVkZGKm7BruVWq30tQrVdO1oFaf3LpDx0Ge2lsuDZog_w==",
		},
		"map": {
			hashable: fingerprint.Map(
				map[int]bool{1: true, 2: false},
				func(k int, v bool) (string, fingerprint.Bool) {
					return fmt.Sprintf("key-%d", k), fingerprint.Bool(v)
				},
			),
			hash: "pBhVPXumhjSRbrnfSKW56cAesx676vMwcqOLcSjl_jpPUXSdBbMagdVQFfIc13k3rkbk-4WPlcsAh_f-8_87Hw==",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			hash, err := fingerprint.Fingerprint(tc.hashable)
			require.NoError(t, err)
			require.Equal(t, tc.hash, hash)
		})
	}
}
