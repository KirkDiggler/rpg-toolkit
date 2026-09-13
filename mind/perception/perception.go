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

// Presence is one subject and what it says about itself. The payload is
// opaque: this package never decodes it, only carries it — encoded once by
// the caller and identical for every observer who receives it (rule 1).
//
// Both verbs take it, and each reads the name differently. In a Pass it is
// something present to be perceived, gated by Reach. In a Report it is
// discrete testimony handed to the one observer named, gated by nothing — a
// witness told what somebody did is not a thing standing in a room.
//
// ID is the unit of identity this package will not look past (rule 11): a
// caller perceiving one figure on two channels qualifies the ID by channel,
// and the store then cannot merge them. Merging is the observer's judgment,
// which is the whole reason a mind has one.
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
// to still say the same thing.
//
// Channel is the provenance of the latest accepted testimony, which is not
// the same question as what is sustaining the holding now. Report moves
// Channel without sustaining anything, so the two answers genuinely differ:
// a rumour is not a sighting.
//
// CurrentVia is every channel delivering this subject right now, sorted, and
// empty is a ghost — still held, no longer delivered. Ask it through
// CurrentOn rather than testing its length: "is anything at all delivering
// this" is almost never the question a caller means (rule 9).
type Holding struct {
	Subject    core.EntityID
	Payload    []byte
	Channel    Channel
	Observed   uint64
	Confirmed  uint64
	CurrentVia []Channel
}

// CurrentOn reports whether the named channel is delivering this subject
// right now. A ghost is current on nothing, and so is a subject held only
// from a Report: discrete testimony sustains nothing, by construction.
//
// This is the whole reason v0.2.0 replaced a Current bool. With one channel
// in existence the bool read as "currently delivered", and every consumer
// meant sight; correctness rested on subject ids never colliding across
// channels rather than on any caller saying which channel it meant. Naming
// the channel is now the only way to ask.
func (h Holding) CurrentOn(channel Channel) bool {
	return slices.Contains(h.CurrentVia, channel)
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
	// new this pass — intel's own comparison at landing, reported rather
	// than re-derived (rule 7).
	Changed []core.EntityID
	// Faded is every subject that stopped being current via any channel
	// this pass — not merely absent from this one. A subject still
	// sustained by another channel is not Faded. The holding survives —
	// the ghost goblin.
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
// Validates before any mutation, in order (rule 8): nil Reach (ErrNoReach),
// empty Channel (ErrNoChannel), an empty ID on any Presence (ErrNoSubject),
// an empty ID on any Observer (ErrNoObserver), a duplicate Presence ID
// (ErrDuplicateSubject), a duplicate Observer (ErrDuplicateObserver). On a
// validation error, nothing is written — true today because Surveil itself
// has no error path once past its own validation, not because this package
// guarantees atomicity beyond that boundary.
func (p *Perception) Observe(pass Pass) (map[core.EntityID]*Delta, error) {
	if err := validatePass(pass); err != nil {
		return nil, fmt.Errorf("observe: %w", err)
	}

	// Rule 6: presences are processed in sorted ID order, which is what
	// makes every Delta below sorted too — intel.Surveil lands FirstContact,
	// Refreshed, Changed, and Reacquired in percept order, and derives Faded
	// sorted on its own. validatePass has already rejected duplicate IDs, so
	// this sort has nothing ambiguous left to order.
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

		deltas[observer] = deltaFrom(out)
	}

	return deltas, nil
}

// ReportInput is discrete testimony landed on exactly one observer: what a
// witness was told, or what it noticed in a way no pass models. Reports name
// subjects the way a Pass does; Channel is their provenance, and At stamps
// them like any other landing.
//
// Unlike a Pass this addresses one observer, so there is no Reach and no
// reachability question: being told is the delivery. Nothing else is
// touched — an observer not named here holds exactly what it held.
type ReportInput struct {
	Observer core.EntityID
	Channel  Channel
	Reports  []Presence
	At       uint64
}

// ReportOutput is what one Report did to that observer's knowledge. Like
// Delta, the lists refine rather than partition: Changed is the subset of
// Updated whose content is actually new.
type ReportOutput struct {
	// FirstContact is every subject this observer had no holding for at
	// all, now created by this report. The payload rides along because
	// nothing the caller holds could supply it.
	FirstContact []Presence
	// Updated is every subject whose holding already existed and was
	// landed on again by this report.
	Updated []core.EntityID
	// Changed refines Updated to the subjects whose content is actually
	// new — intel's own comparison at landing, reported rather than
	// re-derived (rule 7). First contact is never in Changed.
	Changed []core.EntityID
}

// Report lands discrete testimony on one observer: held, and sustaining
// nothing. A reported subject is never current on the channel that reported
// it, which is the whole difference from Observe — a deed is in the past the
// moment it exists, and a rumour was never a delivery. Nothing fades here
// either: Report makes no claim about what the observer is not being told,
// so it cannot retire a holding the way a complete percept does (rule 10).
//
// A subject this observer already holds on a DIFFERENT channel is overwritten
// rather than merged — one payload per (observer, subject) is the store's
// shape, and the last landing wins. That is not a hazard to remember so much
// as the reason rule 11 exists: qualify a subject id by channel and the case
// cannot arise. It is deliberately not refused. The shape of multi-channel
// holdings is still open, and a refusal would wall off a design before the
// use cases have finished arguing for one.
//
// Validates before any mutation, in Observe's order (rule 8): empty Channel
// (ErrNoChannel), an empty ID on any report (ErrNoSubject), an empty
// Observer (ErrNoObserver). Repeated subjects are NOT rejected the way a
// Pass rejects them: that rejection exists because sorting a Pass makes
// last-wins dedupe depend on an unstable sort, and Report does not sort, so
// intel's dedupe is already deterministic — last wins, at the last
// occurrence's position.
func (p *Perception) Report(in ReportInput) (*ReportOutput, error) {
	if in.Channel == "" {
		return nil, fmt.Errorf("report: %w", ErrNoChannel)
	}
	for _, report := range in.Reports {
		if report.ID == "" {
			return nil, fmt.Errorf("report: %w", ErrNoSubject)
		}
	}
	if in.Observer == "" {
		return nil, fmt.Errorf("report: %w", ErrNoObserver)
	}

	reports := make([]intel.Report, 0, len(in.Reports))
	for _, report := range in.Reports {
		reports = append(reports, intel.Report{
			Subject: intel.Subject(report.ID),
			Payload: report.Payload,
		})
	}

	out, err := p.intel.Report(&intel.ReportInput{
		Observer: in.Observer,
		Channel:  intel.Channel(in.Channel),
		Reports:  reports,
		At:       in.At,
	})
	if err != nil {
		return nil, fmt.Errorf("report: %w", err)
	}

	result := &ReportOutput{}
	for _, report := range out.FirstContact {
		result.FirstContact = append(result.FirstContact, Presence{
			ID:      core.EntityID(report.Subject),
			Payload: report.Payload,
		})
	}
	for _, subject := range out.Updated {
		result.Updated = append(result.Updated, core.EntityID(subject))
	}
	for _, subject := range out.Changed {
		result.Changed = append(result.Changed, core.EntityID(subject))
	}

	return result, nil
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
// Observer ID, a duplicate Presence ID, a duplicate Observer. Duplicates are
// checked only once every empty-ID check has cleared the whole Pass, so
// which violation is reported never depends on where in either slice it
// sits — an empty ID anywhere always outranks a duplicate anywhere.
//
// Duplicates matter because rule 6's determinism promise depends on
// Presences forming a set once sorted by ID: slices.SortFunc is not stable,
// so which of two same-ID presences survives intel's last-wins dedupe would
// otherwise depend on input order. Rather than let that ride, both a
// repeated Presence ID and a repeated Observer are rejected as caller bugs.
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

	seen := make(map[core.EntityID]struct{}, len(pass.Presences))
	for _, presence := range pass.Presences {
		if _, dup := seen[presence.ID]; dup {
			return ErrDuplicateSubject
		}
		seen[presence.ID] = struct{}{}
	}

	seen = make(map[core.EntityID]struct{}, len(pass.Observers))
	for _, observer := range pass.Observers {
		if _, dup := seen[observer]; dup {
			return ErrDuplicateObserver
		}
		seen[observer] = struct{}{}
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

// deltaFrom converts one intel.SurveilOutput into this package's Delta.
// Changed (rule 7) is intel's own field since play/intel v0.4.0 — the
// bytes.Equal comparison the store already makes at landing, reported
// directly rather than reconstructed from stamps. The old reconstruction
// compared a holding's Observed to the pass's At, which was only correct
// while At strictly increased; consuming intel's answer makes that trap
// impossible by construction instead of documenting it.
func deltaFrom(out *intel.SurveilOutput) *Delta {
	delta := &Delta{}

	for _, report := range out.FirstContact {
		delta.FirstContact = append(delta.FirstContact, Presence{
			ID:      core.EntityID(report.Subject),
			Payload: report.Payload,
		})
	}
	for _, subject := range out.Refreshed {
		delta.Refreshed = append(delta.Refreshed, core.EntityID(subject))
	}
	for _, subject := range out.Changed {
		delta.Changed = append(delta.Changed, core.EntityID(subject))
	}
	for _, subject := range out.Faded {
		delta.Faded = append(delta.Faded, core.EntityID(subject))
	}
	for _, subject := range out.Reacquired {
		delta.Reacquired = append(delta.Reacquired, core.EntityID(subject))
	}

	return delta
}

// fromIntelHolding converts an intel.Holding into this package's Holding.
// Observed and Confirmed pass through unchanged; CurrentVia is intel's own
// per-channel list, which intel sorts, retyped one channel at a time (rule
// 9). intel.Status is not carried over: it is derived from CurrentVia being
// non-empty, and re-exporting a derived answer beside the thing it derives
// from is how the Current bool happened in the first place.
//
// A holding sustained by nothing keeps a nil CurrentVia rather than an empty
// slice, matching what intel returns, so a ghost compares equal to a ghost.
func fromIntelHolding(h intel.Holding) Holding {
	var via []Channel
	if len(h.CurrentVia) > 0 {
		via = make([]Channel, 0, len(h.CurrentVia))
		for _, channel := range h.CurrentVia {
			via = append(via, Channel(channel))
		}
	}

	return Holding{
		Subject:    core.EntityID(h.Subject),
		Payload:    h.Payload,
		Channel:    Channel(h.Channel),
		Observed:   h.Observed,
		Confirmed:  h.Confirmed,
		CurrentVia: via,
	}
}
