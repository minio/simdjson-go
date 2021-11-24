// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Modified for 'f' format with inlined variables.

package simdjson

import "math"

func appendFloatF(dst []byte, val float64) []byte {
	var prec int
	var bits uint64
	bits = math.Float64bits(val)
	//var float64info = floatInfo{mantbits: 52, expbits: 11, bias: -1023}
	const mantbits = 52
	const expbits = 11
	const bias = -1023
	//flt = &float64info

	neg := bits>>(expbits+mantbits) != 0
	exp := int(bits>>mantbits) & (1<<expbits - 1)
	mant := bits & (uint64(1)<<mantbits - 1)

	switch exp {
	case 0:
		// denormalized
		exp++

	default:
		// add implicit top bit
		mant |= uint64(1) << mantbits
	}
	exp += bias

	var digs decimalSlice
	// Use Ryu algorithm.
	var buf [32]byte
	digs.d = buf[:]
	ryuFtoaShortest(&digs, mant, exp-mantbits)
	// Precision for shortest representation mode.

	prec = max(digs.nd-digs.dp, 0)
	return fmtF(dst, neg, digs, prec)
}

type decimalSlice struct {
	d      []byte
	nd, dp int
	neg    bool
}

// %f: -ddddddd.ddddd
func fmtF(dst []byte, neg bool, d decimalSlice, prec int) []byte {
	// sign
	if neg {
		dst = append(dst, '-')
	}

	// integer, padded with zeros as needed.
	if d.dp > 0 {
		m := min(d.nd, d.dp)
		dst = append(dst, d.d[:m]...)
		for ; m < d.dp; m++ {
			dst = append(dst, '0')
		}
	} else {
		dst = append(dst, '0')
	}

	// fraction
	if prec > 0 {
		dst = append(dst, '.')
		for i := 0; i < prec; i++ {
			ch := byte('0')
			if j := d.dp + i; 0 <= j && j < d.nd {
				ch = d.d[j]
			}
			dst = append(dst, ch)
		}
	}

	return dst
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
