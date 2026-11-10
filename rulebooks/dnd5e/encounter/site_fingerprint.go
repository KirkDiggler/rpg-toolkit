// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"math"
	"reflect"
	"sort"
)

// fieldFingerprint is THE CONTENT OF A CONSTRUCTION INPUT, AS BYTES
// (rpg-project#879 S5): a SHA-256 over every value reachable from in — every
// field, exported or not, every element, every map entry in key order, every
// interface's dynamic type name beside its value, and a marker for nil — so
// two inputs with the same fingerprint are the same input and compile to the
// same site. [MemorySites] looks a [FieldInput] up by it before compiling, so
// the same content set up twice compiles once.
//
// IT IS A LOOKUP KEY, NEVER THE SITE'S HASH. Two spellings of one content (a
// nil list and an empty one) fingerprint differently and each compiles once;
// both then resolve to the one site their [SiteHash] names. A value the walk
// cannot spell — a func, a channel, a structure deeper than any field has —
// answers false, and the input is simply compiled.
//
// A FIELD OVER THE CELL BUDGET IS NOT WALKED: [compileField] refuses it
// before converting a cell, and a walk of every cell it lists would cost
// what the refusal exists to avoid. On the 10k garrison keep the walk is
// about 3 ms against a 65 ms compile.
func fieldFingerprint(in *FieldInput) (string, bool) {
	cells := len(in.Scenery) + len(in.Sealed)
	for _, r := range in.Regions {
		cells += len(r.Cells)
	}
	if cells > maxFieldCells {
		return "", false
	}
	h := sha256.New()
	if !fingerprintValue(h, reflect.ValueOf(in).Elem(), 0) {
		return "", false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// fingerprintDepth bounds the walk: a field input is a few levels deep, and a
// cycle would otherwise never end.
const fingerprintDepth = 32

func fingerprintValue(h hash.Hash, v reflect.Value, depth int) bool {
	if depth > fingerprintDepth {
		return false
	}
	var buf [8]byte
	word := func(u uint64) {
		binary.LittleEndian.PutUint64(buf[:], u)
		h.Write(buf[:])
	}
	text := func(s string) {
		word(uint64(len(s)))
		h.Write([]byte(s))
	}
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			word(1)
		} else {
			word(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		word(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		word(v.Uint())
	case reflect.Float32, reflect.Float64:
		word(math.Float64bits(v.Float()))
	case reflect.String:
		text(v.String())
	case reflect.Pointer:
		if v.IsNil() {
			word(0)
			return true
		}
		word(1)
		return fingerprintValue(h, v.Elem(), depth+1)
	case reflect.Interface:
		if v.IsNil() {
			text("")
			return true
		}
		text(v.Elem().Type().String())
		return fingerprintValue(h, v.Elem(), depth+1)
	case reflect.Slice:
		if v.IsNil() {
			word(math.MaxUint64)
			return true
		}
		fallthrough
	case reflect.Array:
		word(uint64(v.Len()))
		for i := 0; i < v.Len(); i++ {
			if !fingerprintValue(h, v.Index(i), depth+1) {
				return false
			}
		}
	case reflect.Map:
		if v.IsNil() {
			word(math.MaxUint64)
			return true
		}
		// Entries in the order of their keys' own fingerprints: a map's
		// iteration order is not its content.
		keys := v.MapKeys()
		spelled := make([]string, len(keys))
		for i, k := range keys {
			kh := sha256.New()
			if !fingerprintValue(kh, k, depth+1) {
				return false
			}
			spelled[i] = string(kh.Sum(nil))
		}
		order := make([]int, len(keys))
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(a, b int) bool { return spelled[order[a]] < spelled[order[b]] })
		word(uint64(len(keys)))
		for _, i := range order {
			if !fingerprintValue(h, keys[i], depth+1) || !fingerprintValue(h, v.MapIndex(keys[i]), depth+1) {
				return false
			}
		}
	case reflect.Struct:
		text(v.Type().String())
		for i := 0; i < v.NumField(); i++ {
			if !fingerprintValue(h, v.Field(i), depth+1) {
				return false
			}
		}
	default:
		return false
	}
	return true
}
