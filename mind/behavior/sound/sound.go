// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package sound is where a noise meets the people who heard it.
//
// A sound is testimony the way a deed is: it lands on its own channel through
// perception's Report door, it is in the past the moment it exists, and a
// table's `when: { heard: { kind, within } }` reads it. What a sound is
// (`clang`, `crash`, `voice`) and who is near enough to hear it are the
// caller's: the rulebook names the kinds and runs the physics, and this
// package lands the report on whoever it was told heard.
//
// # A sound never names its maker
//
// The law of this channel (rpg-project#569, R4): a heard report is a kind, a
// place and a time, and NEVER an identity. A deed names its actor to each
// witness who could see them; a sound has no figures in it at all, so every
// hearer holds the same report, and there is nothing to blank. A guard who
// hears a clang knows something metal happened over there, not who swung.
// Who did it is what the guard finds out by walking over and LOOKING — which
// is the sight channel's job, and the reason this one carries no actor field
// a caller could fill by mistake.
package sound

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Channel is the channel sounds land on.
const Channel perception.Channel = "sounds"

// wireType is what a sound payload says it is, so a reader for another
// channel's payloads reads it as an unknown kind and [Held] refuses anything
// else.
const wireType = "sound"

// ErrNoKind reports a sound that does not say what it was. The kind is what a
// `heard` condition matches and the subject the report is filed under, so a
// kindless sound is one nothing could ever hear as anything.
var ErrNoKind = errors.New("sound: a sound has no kind")

// ErrNotASound reports a payload on the sounds channel this package did not
// write. Only this package can tell; the store holds bytes.
var ErrNotASound = errors.New("sound: payload is not a sound")

// Sound is one noise: what kind it was, where it was made, and when.
//
// NO ACTOR AND NO TARGET, BY CONSTRUCTION (R4). See the package doc.
type Sound struct {
	// Kind is what was heard, in the caller's sealed vocabulary ("clang",
	// "crash", "boom", "voice"). This package never interprets it.
	Kind string

	// Where is the DUNGEON-ABSOLUTE cell the sound was made at — the same
	// position a table's Selector carries, so a caller can walk a
	// hearer toward it without a conversion.
	Where spatial.Position

	// At is the clock reading the sound was made at, in the caller's own
	// unit — the same unit the table's Facts.Now counts, which is what a
	// `heard` condition subtracts it from. The report is stamped with it.
	At uint64
}

// Subject is the subject a sound of one kind is held under, qualified by
// channel so it can never collide with a sight holding.
//
// ONE SUBJECT PER KIND, the deeds channel's own narrowing: a hearer holds the
// freshest sound of each kind and nothing before it. A `heard` condition asks
// how recently a kind was heard, and the latest one answers that.
func Subject(kind string) core.EntityID {
	return perception.Qualify(Channel, core.EntityID(kind))
}

// Store is what Land needs of a perception: its Report door, which
// [perception.Perception] has, so the caller hands its own store and nothing
// wraps it.
type Store interface {
	Report(in perception.ReportInput) (*perception.ReportOutput, error)
}

// LandInput is a sound, the store whose observers heard it, and who they
// were. Who heard is the caller's: loudness, distance and doors are the
// physics this package leaves to it, the way perception leaves sight's.
type LandInput struct {
	Store   Store
	Sound   Sound
	Hearers []core.EntityID
}

// Land tells every hearer what they heard. ErrNoKind if the sound does not
// say what it was. A Store's errors pass through unwrapped.
//
// It mirrors the deeds channel's landing and blanks nothing: a sound carries
// no figures, so every hearer holds the same report, stamped at the sound's
// own At. Held and never current — nothing sustains a noise. No hearers is
// not an error: a sound nobody heard happened in an empty room.
func Land(in *LandInput) error {
	if in.Sound.Kind == "" {
		return fmt.Errorf("land: %w", ErrNoKind)
	}

	payload := encode(in.Sound)
	for _, hearer := range in.Hearers {
		_, err := in.Store.Report(perception.ReportInput{
			Observer: hearer,
			Channel:  Channel,
			Reports:  []perception.Presence{{ID: Subject(in.Sound.Kind), Payload: payload}},
			At:       in.Sound.At,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Held is every sound in an observer's holdings, read back for a table's
// Facts: the holdings on [Channel], each decoded, with At the holding's
// confirmed stamp. Holdings on any other channel are skipped. ErrNotASound
// for a payload on this channel this package did not write.
//
// THE ONE READER, so a caller building Facts never parses the payload itself
// (behaviour's R2: a payload is the caller's to read, except a channel this
// module wrote).
func Held(held []perception.Holding) ([]Sound, error) {
	out := make([]Sound, 0, len(held))
	for _, h := range held {
		if h.Channel != Channel {
			continue
		}
		var w wire
		if err := json.Unmarshal(h.Payload, &w); err != nil {
			return nil, fmt.Errorf("sound %s: %w", h.Subject, err)
		}
		if w.Type != wireType {
			return nil, fmt.Errorf("sound %s: %w", h.Subject, ErrNotASound)
		}
		out = append(out, Sound{Kind: w.Kind, Where: w.Where, At: h.Confirmed})
	}

	return out, nil
}

// wire is the payload: what it is, the kind, and where. The time is the
// report's own stamp, so it is not repeated here.
type wire struct {
	Type  string           `json:"type"`
	Kind  string           `json:"kind"`
	Where spatial.Position `json:"where"`
}

func encode(s Sound) []byte {
	out, err := json.Marshal(wire{Type: wireType, Kind: s.Kind, Where: s.Where})
	if err != nil {
		// The struct is closed and every field marshals; a failure here
		// would be a defect in this package, not a runtime condition.
		panic("sound: encode: " + err.Error())
	}

	return out
}
