// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package perception

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// Channel identifies a source of testimony (sight, hearing, etc). Vocabulary
// is open beyond the one predeclared value; a caller's Reach supplies the
// physics for whatever channel it names.
type Channel string

// Sight is the predeclared visual channel.
const Sight Channel = "sight"

// Presence is one thing that can be perceived this pass, and what it says.
// The payload is opaque: this package never decodes it, only carries it —
// encoded once by the caller before the pass, and identical for every
// observer who perceives it (rule 1).
type Presence struct {
	ID      core.EntityID
	Payload []byte
}

// Reach answers whether an observer's channel reaches a subject this pass.
// It is the whole of the physics, and this package supplies none of it —
// range, blocking, lighting, and cover all collapse into the one yes or no
// that the caller owns.
type Reach interface {
	Reaches(channel Channel, observer, subject core.EntityID) bool
}

// Pass is one complete perception cycle on one channel: every presence this
// pass, every observer this pass, and the Reach that connects them.
type Pass struct {
	At        uint64
	Channel   Channel
	Presences []Presence
	Observers []core.EntityID
	Reach     Reach
}

// Holding is what one observer holds about one subject. Observed is when
// this content was first perceived; Confirmed is when it was last perceived
// to still say the same thing. Current false is a ghost: still held, no
// longer delivered.
type Holding struct {
	Subject   core.EntityID
	Payload   []byte
	Channel   Channel
	Observed  uint64
	Confirmed uint64
	Current   bool
}

// Delta is what one pass did to one observer's knowledge. The lists answer
// different questions and are not a partition of the percept: Refreshed
// fires on every pass for everything perceived, and Changed and Reacquired
// each refine it rather than carve it up (matching intel.SurveilOutput,
// which this wraps).
type Delta struct {
	// FirstContact is every subject this observer had no holding for at
	// all, now created by this pass. The payload rides along because
	// nothing the caller holds could supply it.
	FirstContact []Presence
	// Refreshed is every subject whose holding already existed and was
	// re-perceived this pass — on every pass, for everything perceived.
	Refreshed []core.EntityID
	// Changed refines Refreshed to the subjects whose content is actually
	// new this pass (rule 7).
	Changed []core.EntityID
	// Faded is every subject that stopped being delivered this pass. The
	// holding survives — the ghost goblin.
	Faded []core.EntityID
	// Reacquired is every subject that was a ghost at the top of this pass
	// and is delivered again. It refines Refreshed; a reacquired subject
	// appears in both lists.
	Reacquired []core.EntityID
}

// Perception holds, per observer, channel-sourced testimony about subjects:
// what was perceived, when, and whether it is still current. It is backed by
// play/intel, which callers never see. Zero value not usable; construct via
// New or Load.
type Perception struct {
	intel *intel.Intel
}

// New creates an empty Perception.
func New() (*Perception, error) {
	i, err := intel.NewIntel()
	if err != nil {
		return nil, fmt.Errorf("new: %w", err)
	}
	return &Perception{intel: i}, nil
}

// Observe runs one Pass and returns the Delta it caused for every observer
// named in it. An observer absent from Observers is not touched at all —
// nothing fades for them (rule 5). An observer present but reaching nothing
// still lands a complete empty percept, so everything it held fades (rule
// 4); skipping that call is how ghosts stay falsely current forever.
//
// Validates before any mutation, in order: nil Reach (ErrNoReach), empty
// Channel (ErrNoChannel), an empty ID on any Presence (ErrNoSubject), an
// empty ID on any Observer (ErrNoObserver). On a non-nil error, nothing is
// written.
func (p *Perception) Observe(pass Pass) (map[core.EntityID]*Delta, error) {
	if err := validatePass(pass); err != nil {
		return nil, fmt.Errorf("observe: %w", err)
	}

	// Rule 6: presences are processed in sorted ID order, which is what
	// makes every Delta below sorted too — intel.Surveil lands FirstContact,
	// Refreshed, and Reacquired in percept order, and derives Faded sorted
	// on its own.
	sorted := make([]Presence, len(pass.Presences))
	copy(sorted, pass.Presences)
	slices.SortFunc(sorted, func(a, b Presence) int {
		return cmp.Compare(a.ID, b.ID)
	})

	deltas := make(map[core.EntityID]*Delta, len(pass.Observers))
	for _, observer := range pass.Observers {
		out, err := p.intel.Surveil(&intel.SurveilInput{
			Observer: observer,
			Channel:  intel.Channel(pass.Channel),
			Percept:  percept(sorted, observer, pass.Channel, pass.Reach),
			At:       pass.At,
		})
		if err != nil {
			return nil, fmt.Errorf("observe: %w", err)
		}

		delta, err := p.deltaFrom(observer, out, pass.At)
		if err != nil {
			return nil, fmt.Errorf("observe: %w", err)
		}
		deltas[observer] = delta
	}

	return deltas, nil
}

// Held returns everything an observer holds, sorted by subject.
func (p *Perception) Held(observer core.EntityID) ([]Holding, error) {
	if observer == "" {
		return nil, fmt.Errorf("held: %w", ErrNoObserver)
	}

	holdings, err := p.intel.HeldBy(&intel.HeldByInput{Observer: observer})
	if err != nil {
		return nil, fmt.Errorf("held: %w", err)
	}

	result := make([]Holding, 0, len(holdings))
	for _, h := range holdings {
		result = append(result, fromIntelHolding(h))
	}
	return result, nil
}

// On returns what one observer holds about one subject. Returns ErrNotHeld
// if the observer holds nothing on it — intel's own not-held error is
// translated to this package's sentinel, never passed through, so a caller
// never has to import play/intel to ask the single most common question
// this package answers.
func (p *Perception) On(observer, subject core.EntityID) (Holding, error) {
	if observer == "" {
		return Holding{}, fmt.Errorf("on: %w", ErrNoObserver)
	}
	if subject == "" {
		return Holding{}, fmt.Errorf("on: %w", ErrNoSubject)
	}

	h, err := p.intel.On(&intel.OnInput{Observer: observer, Subject: intel.Subject(subject)})
	if err != nil {
		if errors.Is(err, intel.ErrNotHeld) {
			return Holding{}, fmt.Errorf("on: %w", ErrNotHeld)
		}
		return Holding{}, fmt.Errorf("on: %w", err)
	}
	return fromIntelHolding(h), nil
}

// validatePass checks a Pass before any mutation, in the order rule 8
// requires: nil Reach, empty Channel, an empty Presence ID, an empty
// Observer ID.
func validatePass(pass Pass) error {
	if pass.Reach == nil {
		return ErrNoReach
	}
	if pass.Channel == "" {
		return ErrNoChannel
	}
	for _, presence := range pass.Presences {
		if presence.ID == "" {
			return ErrNoSubject
		}
	}
	for _, observer := range pass.Observers {
		if observer == "" {
			return ErrNoObserver
		}
	}
	return nil
}

// percept builds one observer's percept from a sorted presence list: every
// presence that observer isn't (rule 3) and that Reach says this channel
// reaches (rule 2).
func percept(sorted []Presence, observer core.EntityID, channel Channel, reach Reach) []intel.Report {
	reports := make([]intel.Report, 0, len(sorted))
	for _, presence := range sorted {
		if presence.ID == observer {
			continue
		}
		if !reach.Reaches(channel, observer, presence.ID) {
			continue
		}
		reports = append(reports, intel.Report{
			Subject: intel.Subject(presence.ID),
			Payload: presence.Payload,
		})
	}
	return reports
}

// deltaFrom converts one intel.SurveilOutput into this package's Delta and
// derives Changed (rule 7): a subject in Refreshed whose holding now reads
// Observed == at is one whose content just moved; that is exactly what
// intel's two stamps buy.
func (p *Perception) deltaFrom(observer core.EntityID, out *intel.SurveilOutput, at uint64) (*Delta, error) {
	delta := &Delta{}

	for _, report := range out.FirstContact {
		delta.FirstContact = append(delta.FirstContact, Presence{
			ID:      core.EntityID(report.Subject),
			Payload: report.Payload,
		})
	}
	for _, subject := range out.Faded {
		delta.Faded = append(delta.Faded, core.EntityID(subject))
	}
	for _, subject := range out.Reacquired {
		delta.Reacquired = append(delta.Reacquired, core.EntityID(subject))
	}

	for _, subject := range out.Refreshed {
		id := core.EntityID(subject)
		delta.Refreshed = append(delta.Refreshed, id)

		holding, err := p.intel.On(&intel.OnInput{Observer: observer, Subject: subject})
		if err != nil {
			return nil, err
		}
		if holding.Observed == at {
			delta.Changed = append(delta.Changed, id)
		}
	}

	return delta, nil
}

// fromIntelHolding converts an intel.Holding into this package's Holding.
// Observed and Confirmed pass through unchanged; Current maps from intel's
// Status == Current (rule 9).
func fromIntelHolding(h intel.Holding) Holding {
	return Holding{
		Subject:   core.EntityID(h.Subject),
		Payload:   h.Payload,
		Channel:   Channel(h.Channel),
		Observed:  h.Observed,
		Confirmed: h.Confirmed,
		Current:   h.Status == intel.Current,
	}
}
