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
// MONOTONIC, WITH A HEAVY-TAILED STEP. A fresh millisecond draws fresh
// entropy. An id issued in the same millisecond as the last one (or after a
// clock that stepped back) keeps the last id's time and adds a step of
// 1 + (x mod 2^r) to its entropy, with x a random 64-bit value and r uniform
// in 1..64. The sum of such steps is dominated by its largest one, so the
// distance between two ids of one millisecond carries only log-scale
// information about how many ids lie between them, not a count (R30). That is
// the honest claim: not "nothing", and not a count. Two ids in different
// milliseconds carry the clock, not a count.
//
// A millisecond's entropy starts with 2^78 of headroom; a burst that would
// run past 80 bits is refused (an error), never wrapped.
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
	// time and step its entropy forward by the heavy-tailed step.
	var draw [9]byte
	if err := src.Entropy(draw[:]); err != nil {
		return ulid{}, fmt.Errorf("mint id: entropy: %w", err)
	}
	carry, low := heavyStep(draw)
	return stepULID(last, carry, low)
}

// heavyStep turns nine random bytes into a step of 1 + (x mod 2^r): r uniform
// in 1..64 from the first byte, x the next eight. The result is 1..2^64,
// returned as (carry, low): carry is 1 only for the single value 2^64.
func heavyStep(draw [9]byte) (carry, low uint64) {
	r := uint(draw[0]%64) + 1
	x := binary.BigEndian.Uint64(draw[1:])
	if r < 64 {
		x &= (uint64(1) << r) - 1
	}
	low = x + 1
	if low == 0 { // x was 2^64-1
		return 1, 0
	}
	return 0, low
}

// stepULID adds carry*2^64 + low to the 80-bit entropy of u, keeping its
// millisecond. A sum past 80 bits exhausts the millisecond: refused.
func stepULID(u ulid, carry, low uint64) (ulid, error) {
	lo := binary.BigEndian.Uint64(u[8:])
	sum := lo + low
	if sum < lo {
		carry++
	}
	hi := uint64(binary.BigEndian.Uint16(u[6:8])) + carry
	if hi > 0xffff {
		return ulid{}, fmt.Errorf("mint id: millisecond %d exhausted", u.millis())
	}
	out := u
	binary.BigEndian.PutUint64(out[8:], sum)
	binary.BigEndian.PutUint16(out[6:8], uint16(hi))
	return out, nil
}

// MintID issues an id from the log's id space without appending an entry —
// for a caller that names something by the same ordering key its entries
// carry (an act). It sorts after every id issued before it and before every
// id issued after it, across a save and load too: [Log.ToData] writes the
// last id issued whenever it is above the last retained entry's, so no entry
// is ever given it.
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
