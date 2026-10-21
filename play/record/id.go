// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// id.go is THE WIRE ID (rpg-project ideas/event-stream/design.md R27, R30):
// every entry carries an id that is an ORDERING KEY, NOT A COUNTER.
//
// THE FORM IS A ULID: 26 Crockford base32 characters holding a 48-bit Unix
// millisecond reading and 80 bits of entropy. Two ids compare as strings in
// the order they were issued, inside one millisecond and across many.
//
// MONOTONIC AND SPARSE. A fresh millisecond draws fresh entropy. An id issued
// in the same millisecond as the last one (or after a clock that stepped
// back) keeps the last id's time and adds a RANDOM step of 1 to 2^32 to its
// entropy, never one. So ids strictly increase, and the distance between two
// of them says nothing about how many ids lie between — a viewer sent two ids
// cannot count the beats it was not sent (R30).
//
// THE RANDOMNESS IS THE SOURCE'S, NOT THE LOG'S (R7). Time and entropy come
// from an [IDSource]: the wall clock and crypto/rand unless the caller
// supplies one with [WithIDSource]. Everything the log decides is
// deterministic given its source, which is how a composition with its own
// no-randomness law keeps its goldens exact.
//
// Seq is unchanged: the internal, gapless sequence. The id is the one a wire
// carries.

// IDSource supplies what a minted id is made of.
type IDSource interface {
	// NowMillis is the current Unix time in milliseconds.
	NowMillis() uint64
	// Entropy fills p with random bytes, or fails and fills nothing the log
	// keeps.
	Entropy(p []byte) error
}

// Option configures a Log at NewLog or LoadLog.
type Option func(*Log)

// WithIDSource makes src the source of every id the log mints. A nil src
// keeps the default, the wall clock and crypto/rand.
func WithIDSource(src IDSource) Option {
	return func(l *Log) {
		if src != nil {
			l.ids = src
		}
	}
}

// wallClock is the default IDSource.
type wallClock struct{}

func (wallClock) NowMillis() uint64 { return uint64(time.Now().UnixMilli()) } //nolint:gosec // post-1970 clock

func (wallClock) Entropy(p []byte) error {
	_, err := rand.Read(p)
	return err
}

// ulid is an id in its 16-byte big-endian form: bytes 0–5 the millisecond,
// 6–15 the entropy. The zero value is "no id issued yet".
type ulid [16]byte

const (
	ulidLen      = 26
	crockford    = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	maxULIDMilli = 1<<48 - 1
)

func (u ulid) isZero() bool { return u == ulid{} }

func (u ulid) millis() uint64 {
	return uint64(u[0])<<40 | uint64(u[1])<<32 | uint64(u[2])<<24 |
		uint64(u[3])<<16 | uint64(u[4])<<8 | uint64(u[5])
}

func (u *ulid) setMillis(ms uint64) {
	for i := 5; i >= 0; i-- {
		u[i] = byte(ms)
		ms >>= 8
	}
}

// String is the 26-character Crockford form: the 128 bits read as a 130-bit
// number with two leading zero bits, five bits per character.
func (u ulid) String() string {
	var out [ulidLen]byte
	for i := 0; i < ulidLen; i++ {
		// Character i holds bits [130-5(i+1), 130-5i) of the padded value;
		// in the unpadded 128-bit value that is bit offset start..start+4
		// counting from the most significant bit, start = 5i - 2.
		var v byte
		for b := 0; b < 5; b++ {
			pos := 5*i - 2 + b
			v <<= 1
			if pos >= 0 && u[pos/8]&(0x80>>(pos%8)) != 0 {
				v |= 1
			}
		}
		out[i] = crockford[v]
	}
	return string(out[:])
}

// parseULID reads the 26-character form, refusing any other length, any
// character outside upper-case Crockford base32, and a first character above
// '7' (a value wider than 128 bits).
func parseULID(s string) (ulid, error) {
	var u ulid
	if len(s) != ulidLen {
		return u, fmt.Errorf("id %q: length %d, want %d", s, len(s), ulidLen)
	}
	for i := 0; i < ulidLen; i++ {
		v := -1
		for j := 0; j < len(crockford); j++ {
			if crockford[j] == s[i] {
				v = j
				break
			}
		}
		if v < 0 {
			return ulid{}, fmt.Errorf("id %q: character %q is not base32", s, s[i])
		}
		if i == 0 && v > 7 {
			return ulid{}, fmt.Errorf("id %q: wider than 128 bits", s)
		}
		for b := 0; b < 5; b++ {
			pos := 5*i - 2 + b
			if pos >= 0 && v&(0x10>>b) != 0 {
				u[pos/8] |= 0x80 >> (pos % 8)
			}
		}
	}
	return u, nil
}

// legacyULID is the deterministic id an entry persisted before ids existed
// takes at load: millisecond zero, the seq as its entropy. It sorts before
// every id minted from a real clock.
func legacyULID(seq uint64) ulid {
	var u ulid
	binary.BigEndian.PutUint64(u[8:], seq)
	return u
}

// next computes the id after last from src without changing anything — the
// caller commits it (R5).
func nextULID(src IDSource, last ulid) (ulid, error) {
	ms := src.NowMillis()
	if ms > maxULIDMilli {
		return ulid{}, fmt.Errorf("mint id: clock %d beyond the id's range", ms)
	}
	if last.isZero() || ms > last.millis() {
		var u ulid
		u.setMillis(ms)
		if err := src.Entropy(u[6:]); err != nil {
			return ulid{}, fmt.Errorf("mint id: entropy: %w", err)
		}
		// Two bits of headroom, so a burst inside this millisecond has
		// 2^78 of room to step through before it could overflow.
		u[6] &= 0x3f
		return u, nil
	}
	// The same millisecond as the last id, or a clock behind it: keep its
	// time and step its entropy forward by a random 1..2^32.
	var step [4]byte
	if err := src.Entropy(step[:]); err != nil {
		return ulid{}, fmt.Errorf("mint id: entropy: %w", err)
	}
	add := uint64(binary.BigEndian.Uint32(step[:])) + 1
	u := last
	lo := binary.BigEndian.Uint64(u[8:])
	sum := lo + add
	binary.BigEndian.PutUint64(u[8:], sum)
	if sum < lo {
		// Carry into the entropy's top two bytes.
		hi := binary.BigEndian.Uint16(u[6:8])
		if hi == 0xffff {
			return ulid{}, fmt.Errorf("mint id: millisecond %d exhausted", last.millis())
		}
		binary.BigEndian.PutUint16(u[6:8], hi+1)
	}
	return u, nil
}

// MintID issues an id from the log's id space without appending an entry —
// for a caller that names something by the same ordering key its entries
// carry (an act). It sorts after every id issued before it and before every
// id issued after it, and no entry is ever given it. The minted id is not
// persisted; [Log.ToData] keeps only what entries carry (and, for a log
// trimmed to nothing, the last id issued).
//
// Errors: the id source's entropy failure, or an exhausted millisecond; on
// error nothing changed.
func (l *Log) MintID() (string, error) {
	u, err := nextULID(l.ids, l.last)
	if err != nil {
		return "", err
	}
	l.last = u
	return u.String(), nil
}
