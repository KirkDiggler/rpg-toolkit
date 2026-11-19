// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ContentHash is the hash of everything this compiled content is: the
// lower-case hex SHA-256 of its canonical encoding (rpg-project#1001). A
// host carries it into [encounter.SetupInput.Content], so a run's record
// names the content it was a run of.
//
// IT IS OF THE COMPILED FORM, NOT THE SOURCE BYTES. Anything the compiler
// keeps changes it: a line's words, a check's DC, a machine's step, a post,
// a creature, a region, the canvas's void and hex orientation, an ending's
// trigger, the key and the name. Anything it drops does not: comments, key
// order, quoting and layout in the YAML, and declarations the compiler does
// not keep (a `coordinateFrame`, which changes nothing a run does). Two
// compiles of one file hash alike, and so does a reload. A compiler that
// compiles the same file differently (another engine version) gives it
// another hash, so compare two hashes only under one engine.
//
// THE ENCODING IS THE HASH'S OWN, NOT THE TYPES' JSON. JSON drops what an
// interface's concrete type is (a void, an orientation and a trigger
// encode their type by it) and every unexported field, so a part of the
// content could hash as nothing. [canonicalEncode] writes every field,
// exported or not, and every interface value's concrete type, and refuses
// what it cannot spell (a function, a channel). The types' JSON is left as
// it is: nothing persisted changes.
//
// Errors: the compiled form holds a value the encoding cannot spell, which
// a compile never produces.
func (c *Compiled) ContentHash() (encounter.ContentHash, error) {
	var b bytes.Buffer
	if err := canonicalEncode(&b, reflect.ValueOf(c).Elem(), "Compiled", 0); err != nil {
		return "", fmt.Errorf("content hash: %w", err)
	}
	sum := sha256.Sum256(b.Bytes())
	return encounter.ContentHash(hex.EncodeToString(sum[:])), nil
}

// canonicalDepth bounds the walk: the compiled form is a tree, and a value
// nested deeper than this is a cycle the encoding refuses.
const canonicalDepth = 64

// canonicalEncode writes v deterministically into b: every struct field by
// name (exported or not), every interface value as its concrete type then
// its value, maps by sorted key, and a nil as nil. path names where v sits,
// for a refusal.
func canonicalEncode(b *bytes.Buffer, v reflect.Value, path string, depth int) error {
	if depth > canonicalDepth {
		return fmt.Errorf("%s: nested past %d: a cycle", path, canonicalDepth)
	}
	switch v.Kind() {
	case reflect.Invalid:
		b.WriteString("nil")
	case reflect.Interface:
		if v.IsNil() {
			b.WriteString("nil")
			return nil
		}
		elem := v.Elem()
		b.WriteString("<" + typeName(elem.Type()) + ">")
		return canonicalEncode(b, elem, path, depth+1)
	case reflect.Pointer:
		if v.IsNil() {
			b.WriteString("nil")
			return nil
		}
		b.WriteString("&")
		return canonicalEncode(b, v.Elem(), path, depth+1)
	case reflect.Struct:
		t := v.Type()
		b.WriteString(typeName(t) + "{")
		for i := 0; i < t.NumField(); i++ {
			b.WriteString(t.Field(i).Name + ":")
			if err := canonicalEncode(b, v.Field(i), path+"."+t.Field(i).Name, depth+1); err != nil {
				return err
			}
			b.WriteString(";")
		}
		b.WriteString("}")
	case reflect.Map:
		if v.IsNil() {
			b.WriteString("nil")
			return nil
		}
		type entry struct{ key, value string }
		entries := make([]entry, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			var k, val bytes.Buffer
			if err := canonicalEncode(&k, iter.Key(), path+"[key]", depth+1); err != nil {
				return err
			}
			if err := canonicalEncode(&val, iter.Value(), path+"["+k.String()+"]", depth+1); err != nil {
				return err
			}
			entries = append(entries, entry{k.String(), val.String()})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
		b.WriteString("map[")
		for _, e := range entries {
			b.WriteString(e.key + "=" + e.value + ";")
		}
		b.WriteString("]")
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			b.WriteString("nil")
			return nil
		}
		b.WriteString("[" + strconv.Itoa(v.Len()) + ":")
		for i := 0; i < v.Len(); i++ {
			if err := canonicalEncode(b, v.Index(i), path+"["+strconv.Itoa(i)+"]", depth+1); err != nil {
				return err
			}
			b.WriteString(",")
		}
		b.WriteString("]")
	case reflect.String:
		b.WriteString(strconv.Quote(v.String()))
	case reflect.Bool:
		b.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		b.WriteString(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		b.WriteString(strconv.FormatFloat(v.Float(), 'g', -1, 64))
	default:
		// A function, a channel, an unsafe pointer or a complex number holds
		// state no encoding can spell: refused, never hashed as nothing.
		if v.Kind() == reflect.Func && v.IsNil() {
			b.WriteString("nil")
			return nil
		}
		return fmt.Errorf("%s: a %s cannot be hashed", path, v.Kind())
	}
	return nil
}

// typeName is a type's full name, its package path included, so two types
// of one name in two packages are told apart; an unnamed type its spelling.
func typeName(t reflect.Type) string {
	if t.Name() == "" {
		return t.String()
	}
	return t.PkgPath() + "." + t.Name()
}
