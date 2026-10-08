// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// Rest (rpg-project#542, "Rest", R5, R13).
//
// A rest inside a run is a SHORT rest outside a fight, and it is the party's
// act: the members resting together rest in one call, each spending their
// own hit dice, and the world clock jumps by the rest's one hour once for all
// of them (the encounter owns the clock and the hour; this package counts no
// rounds). One rest beat is told per rester, to that rester's witnesses.
//
// The rules are resolution's and the rulebook's: each spent hit die rolls the
// class die plus the Constitution modifier, floored at zero per die, capped
// at maximum hit points; every resource whose own reset kind is a short rest
// refills; concentration and effects an hour outlasts end. The dice are the
// world's, and each die names the resting character as its source.
//
// A long rest inside a run is not taken (R13, deferred until a use case): the
// long rest is the first-admission rest only.

// RestKind names a kind of rest.
type RestKind string

const (
	// RestShort is the short rest: an hour, hit dice, short-rest resources.
	RestShort RestKind = "short"

	// RestLong is the long rest. Named so a host can pass the word it was
	// given; a run refuses it (R13) with ErrBadRest.
	RestLong RestKind = "long"
)

// RestInput asks the members resting together to rest.
type RestInput struct {
	// Session is the run they rest in.
	Session string

	// Kind is the rest's kind. Only RestShort is taken inside a run.
	Kind RestKind

	// Resters are the members resting together, each with the hit dice they
	// spend. At least one; no member twice.
	Resters []Rester
}

// Rester is one member resting and how many hit dice it spends.
type Rester struct {
	// Member is the resting player's member id, which is its character id.
	Member string

	// HitDice is how many hit dice to spend; zero spends none and still
	// refills what a short rest refills.
	HitDice int
}

// RestOutput reports the rest: one entry per rester, in the order asked.
type RestOutput struct {
	// Rested is what the rest did for each rester.
	Rested []RestedMember

	// Discovered is what changed in each observer's perception when the hour
	// passed. Absent observers saw nothing new.
	Discovered map[string]Discovery

	// Formed is present if a fight started when the hour passed.
	Formed *Formed

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// RestedMember is what one rest did for one rester.
type RestedMember struct {
	// Member is the rester.
	Member string

	// Seq is the rest beat's sequence in the rester's own delivered
	// numbering (stream.go).
	Seq uint64

	// Character is the record the verb saved for the rester.
	Character *character.Data

	// HitPointsRestored is how many hit points the rest restored.
	HitPointsRestored int

	// HitPoints is the rester's hit points after the rest.
	HitPoints int

	// HitDiceSpent is how many hit dice the rest spent.
	HitDiceSpent int

	// HitDiceRemaining is how many hit dice the rester has left.
	HitDiceRemaining int

	// Calculation is the hit dice's roll, every die sourced to the rester;
	// nil when no die was spent.
	Calculation *RollCalculation

	// ResourcesRefilled names every resource the rest refilled, by full ref,
	// in the rulebook's order.
	ResourcesRefilled []string
}

// restResult is one rester's resolved rest, carried from resolution to the
// record and the report.
type restResult struct {
	member    string
	out       *resolution.ShortRestOutput
	recording encounter.RestingMember
}

// Rest rests the members resting together, outside any fight, for one hour.
//
// Everything is refused before anything is written: a kind other than short,
// no resters or a rester named twice (ErrBadRest), a rester the run does not
// hold as a player (ErrNoMember, ErrNotACharacter), and any rester in a fight
// (ErrInBubble — the design's "in a fight"). Then each rester's short rest
// is resolved in the order asked, over the run's other sheets as they stand
// at that moment, and saved; the rest is recorded once with every rester,
// which jumps the clock; the sight areas a broken concentration leaves
// standing are closed after the record, so the story tells the rest before
// the area ends; one commit; one report.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoSession, ErrNoEncounter,
// ErrBadRest, ErrNoMember, ErrNotACharacter, ErrInBubble, ErrNoCharacter,
// ErrBadCharacter, ErrClosed, a WindowOpenError while an interrupt window is
// open, or ErrSaveFailed with a populated report.
func (m *Manager) Rest(ctx context.Context, in *RestInput) (*RestOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("rest: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if err := validateRestInput(in); err != nil {
		return nil, fmt.Errorf("rest: %w", err)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("rest: %w", err)
	}
	for _, rester := range in.Resters {
		if err := refuseRester(scope, rester.Member); err != nil {
			return nil, fmt.Errorf("rest: %w", err)
		}
	}

	// EVERY RESTER IS RESOLVED BEFORE ANY IS SAVED, Launch's shape: a later
	// rester the rulebook refuses (more hit dice than they hold) leaves the
	// earlier ones unwritten, with no beat and no hour passed. The records a
	// rest changes are carried to the next rester's resolution in memory
	// ([pendingSheets]) and saved together once every rester has resolved.
	sheets := m.sheetsFor(scope)
	pending := &pendingSheets{}
	results := make([]restResult, 0, len(in.Resters))
	for _, rester := range in.Resters {
		result, err := m.restOne(ctx, scope, sheets, pending, rester)
		if err != nil {
			return nil, fmt.Errorf("rest: %w", err)
		}
		results = append(results, result)
	}
	for _, record := range pending.ordered() {
		if err := sheets.save(ctx, record); err != nil {
			return nil, fmt.Errorf("rest: %w", err)
		}
	}

	members := make([]encounter.RestingMember, 0, len(results))
	for _, result := range results {
		members = append(members, result.recording)
	}
	recorded, err := scope.enc.RecordRest(&encounter.RecordRestInput{
		Members: members, Kind: encounter.RestShort,
	})
	if err != nil {
		return nil, fmt.Errorf("rest: %w", reportUnrecorded(scope, translate(err)))
	}

	if err := closeBrokenAreas(scope, results); err != nil {
		return nil, fmt.Errorf("rest: %w", reportUnrecorded(scope, err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("rest: %w", err)
	}

	seqs := make(map[string]uint64, len(recorded.Rested))
	for _, told := range recorded.Rested {
		seqs[string(told.Member)] = scope.deliveredSeq(string(told.Member), told.Seq)
	}
	out := &RestOutput{
		Discovered: projectDiscoveries(recorded.IntelDeltas),
		Saved:      report,
		Delivery:   delivery,
	}
	if recorded.Formed != nil && len(in.Resters) > 0 {
		out.Formed = projectFormedFor(scope, in.Resters[0].Member, recorded.Formed)
	}
	for _, result := range results {
		out.Rested = append(out.Rested, RestedMember{
			Member:            result.member,
			Seq:               seqs[result.member],
			Character:         result.out.Character,
			HitPointsRestored: result.recording.HitPointsRestored,
			HitPoints:         result.recording.HitPoints,
			HitDiceSpent:      result.recording.HitDiceSpent,
			HitDiceRemaining:  result.recording.HitDiceRemaining,
			Calculation:       sessionRollCalculationFor(result.recording.Calculation),
			ResourcesRefilled: result.recording.ResourcesRefilled,
		})
	}
	return out, nil
}

// validateRestInput refuses a rest request that is wrong in itself, before any
// load: a kind a run does not take, nobody resting, a member named twice, a
// negative hit dice count.
func validateRestInput(in *RestInput) error {
	switch in.Kind {
	case RestShort:
	case RestLong:
		return fmt.Errorf("a long rest is not taken inside a run: %w", ErrBadRest)
	default:
		return fmt.Errorf("rest kind %q: %w", in.Kind, ErrBadRest)
	}
	if len(in.Resters) == 0 {
		return fmt.Errorf("nobody rests: %w", ErrBadRest)
	}
	seen := make(map[string]bool, len(in.Resters))
	for i, rester := range in.Resters {
		if rester.Member == "" {
			return fmt.Errorf("resters[%d]: %w", i, ErrNoMemberID)
		}
		if seen[rester.Member] {
			return fmt.Errorf("member %q rests twice: %w", rester.Member, ErrBadRest)
		}
		seen[rester.Member] = true
		if rester.HitDice < 0 {
			return fmt.Errorf("member %q spends %d hit dice: %w", rester.Member, rester.HitDice, ErrBadRest)
		}
	}
	return nil
}

// refuseRester refuses a rester the run does not hold as a player, or one in a
// fight. Asked of every rester before any is rested.
func refuseRester(scope *writeScope, member string) error {
	kind, ok := scope.standing.kinds[member]
	if !ok {
		return fmt.Errorf("rester %q: %w", member, ErrNoMember)
	}
	if kind != encounter.KindPlayer {
		return fmt.Errorf("rester %q: %w", member, ErrNotACharacter)
	}
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(member)})
	if err != nil {
		return translate(err)
	}
	if ClockKind(clock.Kind) == ClockTurn {
		return fmt.Errorf("rester %q: %w", member, ErrInBubble)
	}
	return nil
}

// restOne resolves and saves one rester's short rest, over every other sheet
// the run holds as it stands now (an earlier rester's already rested), and
// shapes what the record tells.
func (m *Manager) restOne(
	ctx context.Context, scope *writeScope, sheets sheetStore, pending *pendingSheets, rester Rester,
) (restResult, error) {
	record, err := pending.load(ctx, sheets, "rester", rester.Member)
	if err != nil {
		return restResult{}, err
	}
	others, err := m.othersOf(ctx, scope, sheets, pending, rester.Member, false)
	if err != nil {
		return restResult{}, err
	}

	out, err := resolution.ShortRest(ctx, &resolution.ShortRestInput{
		Character: record,
		HitDice:   rester.HitDice,
		Roller:    &diceSeam{roller: m.dice},
		Others:    others,
	})
	if err != nil {
		return restResult{}, translateRest(rester.Member, err)
	}
	if out == nil || out.Character == nil {
		return restResult{}, fmt.Errorf("rester %q: short rest returned no character data: %w",
			rester.Member, ErrBadCharacter)
	}

	pending.hold(out.Character)
	for _, dirty := range out.DirtyCharacters {
		if dirty != nil {
			pending.hold(dirty)
		}
	}
	for _, dirty := range out.DirtyMonsters {
		if dirty != nil {
			scope.replaceMonsterSheet(dirty)
		}
	}

	restored := out.Character.HitPoints - record.HitPoints
	if restored < 0 {
		restored = 0
	}
	return restResult{
		member: rester.Member,
		out:    out,
		recording: encounter.RestingMember{
			Member:            encounter.MemberID(rester.Member),
			HitPointsRestored: restored,
			HitPoints:         out.Character.HitPoints,
			HitDiceSpent:      out.Result.HitDiceSpent,
			HitDiceRemaining:  out.Result.HitDiceRemaining,
			Calculation:       rollCalculationFor(out.Result.Healing),
			// What the rest refilled and what it ended, the rulebook's own
			// answers carried onto the rester's beat unconverted: refills by
			// full ref, each broken concentration with what it held, every
			// condition or effect the rest took off the rester.
			ResourcesRefilled:   refStrings(out.Result.Refilled),
			ConcentrationBreaks: out.ConcentrationBreaks,
			Ended:               out.Ended,
		},
	}, nil
}

// pendingSheets is the records one verb has changed and not yet saved — a
// Rest's rested and dirtied sheets, an Exit's or End's departures — newest
// record per character, in the order each was first changed. It is the verb's own
// unwritten work, not a copy of anything the repository holds — a record not
// in it is read from the store at the moment it is asked for — and it is
// saved, all of it, only once every rester has resolved.
type pendingSheets struct {
	order []string
	byID  map[string]*character.Data
}

// hold records a changed sheet, replacing any earlier change to it.
func (p *pendingSheets) hold(record *character.Data) {
	if p.byID == nil {
		p.byID = map[string]*character.Data{}
	}
	if _, seen := p.byID[record.ID]; !seen {
		p.order = append(p.order, record.ID)
	}
	p.byID[record.ID] = record
}

// load answers a changed sheet from this rest's unwritten work, or reads the
// store.
func (p *pendingSheets) load(ctx context.Context, sheets sheetStore, role, id string) (*character.Data, error) {
	if record, ok := p.byID[id]; ok {
		return record, nil
	}
	return sheets.load(ctx, role, id)
}

// ordered is every changed sheet, in the order each was first changed.
func (p *pendingSheets) ordered() []*character.Data {
	out := make([]*character.Data, 0, len(p.order))
	for _, id := range p.order {
		out = append(out, p.byID[id])
	}
	return out
}

// othersOf is every sheet the run holds except the rester's (skipMissing
// passes over a player whose sheet is absent, for a departure: Exit and End
// still work around a sheet nobody can read): each other
// player's record, read now through the verb's store, and each monster's
// stat block from the session record. A rest can end a concentration that
// was holding an effect on any of them.
func (m *Manager) othersOf(
	ctx context.Context, scope *writeScope, sheets sheetStore, pending *pendingSheets, rester string,
	skipMissing bool,
) ([]resolution.Participant, error) {
	roster, err := scope.enc.Members()
	if err != nil {
		return nil, translate(err)
	}
	var others []resolution.Participant
	for _, member := range roster {
		id := string(member.ID)
		if id == rester {
			continue
		}
		switch member.Kind {
		case encounter.KindPlayer:
			data, err := pending.load(ctx, sheets, "participant", id)
			if skipMissing && errors.Is(err, ErrNoCharacter) {
				continue
			}
			if err != nil {
				return nil, err
			}
			others = append(others, resolution.Participant{Character: data})
		case encounter.KindMonster:
			if sheet, found := npcSheet(scope.data, id); found {
				others = append(others, resolution.Participant{Monster: sheet})
			}
		}
	}
	return others, nil
}

// closeBrokenAreas closes the sight areas a rester's broken concentration
// leaves standing, through the encounter's own verb, and refreshes perception
// over what is left. The encounter tells who was inside an area that ended,
// so this runs after the rest is recorded.
func closeBrokenAreas(scope *writeScope, results []restResult) error {
	closed := false
	for _, result := range results {
		for _, broken := range result.out.ConcentrationBreaks {
			removed, err := scope.enc.RemoveSightArea(string(broken.Caster))
			if err != nil {
				return translate(err)
			}
			closed = closed || removed
		}
	}
	if !closed {
		return nil
	}
	if err := scope.enc.RefreshPerception(); err != nil {
		return translate(err)
	}
	scope.touched = true
	return nil
}

// translateRest names a short rest's refusal in this package's words.
//
// The rulebook's three refusals of the rest itself — more hit dice than
// remain, a sheet with none, a dead character — are ErrBadRest with the
// reason kept as text. Two refusals say the RUN is wrong, not a sheet or the
// request, and are ErrInvalidSession: a world with no die to throw the hit
// dice with, and a hold whose effect sits on a member the run does not hold
// (the rest is handed every other sheet in the run, so that is an invariant
// the run broke). A sheet resolution could not attach is ErrBadCharacter.
// Nothing else is guessed at.
func translateRest(member string, err error) error {
	if errors.Is(err, resolution.ErrBadParticipant) || errors.Is(err, resolution.ErrNoRoller) {
		return fmt.Errorf("rester %q: %w: %v", member, ErrInvalidSession, err)
	}
	if !errors.Is(err, resolution.ErrNilInput) {
		switch rpgerr.GetCode(err) {
		case rpgerr.CodeResourceExhausted, rpgerr.CodeNotFound, rpgerr.CodeInvalidState:
			return fmt.Errorf("rester %q: %w: %v", member, ErrBadRest, err)
		}
	}
	translated := translateResolution(err)
	if translated == err {
		return fmt.Errorf("rester %q: %w: %v", member, ErrBadCharacter, err)
	}
	return translated
}

// refStrings carries the rulebook's refs as the full ref strings the beat
// names; nil stays nil.
func refStrings(refs []*core.Ref) []string {
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref != nil {
			out = append(out, ref.String())
		}
	}
	return out
}
