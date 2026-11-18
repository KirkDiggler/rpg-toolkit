// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// escape_internal_test.go is THE ESCAPE ROW on a small site (rpg-project#925,
// jail.go's sweepEscapes): a two-room hall split by a locked leaf — the cell
// to the west, the yard to the east — one arresting guard in the yard, a
// captain who is the garrison's mind, a jailed prisoner, a free friend. Each
// row of the rule is proved here, cheaply; the castle keeps only the wiring.

const (
	escCell    RegionID  = "cell"
	escYard    RegionID  = "yard"
	escGuard   MemberID  = "guard"
	escCaptain MemberID  = "captain"
	escPris    MemberID  = "pris"
	escFriend  MemberID  = "friend"
	escFact    FactID    = "escaped"
	escGate    FactionID = "garrison"
)

// escapeSite builds the hall with the prisoner where given, custody as given,
// and the guard holding `arrested` about the prisoner.
func escapeSite(t *testing.T, custody CustodyPhase, prisAt spatial.Position) *Encounter {
	t.Helper()
	return escapeSiteWith(t, custody, prisAt, nil, nil)
}

// escapeSiteWith is [escapeSite] with props on the floor, and the secrets
// hiding some of them (rpg-project#959).
func escapeSiteWith(
	t *testing.T, custody CustodyPhase, prisAt spatial.Position, props []PropInput, concealments []ConcealmentInput,
) *Encounter {
	t.Helper()
	leaf := spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: 40, D: 1}},
		Origin:    spatial.Point{X: 12.5, Y: 8.66},
	}
	lock := DoorIsLocked(Lock{Approaches: []CheckApproach{{Ability: "dex", DC: 13}}})
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Canvas:       openAir(),
			Regions:      []RegionInput{rectRegion(string(escCell), 0, 0, 2, 5), rectRegion(string(escYard), 2, 0, 3, 5)},
			Doors:        []DoorInput{{ID: "leaf", Placement: &leaf, State: lock}},
			Props:        props,
			Concealments: concealments,
			Factions:     []FactionInput{{ID: escGate, Mind: escCaptain}},
			Dispositions: []DispositionInput{{
				Between: [2]FactionID{escGate, FactionParty}, Stance: StanceNeutral,
				Until: []UntilStep{{When: TriggerFact{Fact: escFact}, Then: StanceHostile}},
			}},
		},
		Members: []MemberInput{
			{ID: escPris, Kind: KindPlayer, Position: prisAt},
			{ID: escFriend, Kind: KindPlayer, Position: spatial.Position{X: 3, Y: 3}},
			{ID: escGuard, Kind: KindMonster, Faction: escGate, Position: spatial.Position{X: 4, Y: 1}},
			{ID: escCaptain, Kind: KindMonster, Faction: escGate, Position: spatial.Position{X: 4, Y: 4}},
		},
		Endings:      []EndingInput{{Key: "withdrawn", Trigger: TriggerExternal{}}},
		Capabilities: escapeCapabilitiesFor(concealments),
	})
	require.NoError(t, err)
	// The guard carries the gate's conversation, and the gate arrested and
	// jailed the prisoner (the record a jail leaves, set down directly).
	enc.members[escGuard].Options = []Option{{ID: "halt", Say: "Halt.", Advance: &Advance{Interaction: "gate", Success: "x"}}}
	enc.interactions = map[InteractionID]*interactionRecord{"gate": {arrest: &ArrestRecord{
		State: "arrested", Cell: escCell, Custody: custody, Escape: escFact,
		Taken: []ArrestedMember{{ID: escPris, InArea: true}}, Jailed: jailedIf(custody),
	}}}
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, escGuard, []MemberID{escPris}, TopicArrested, ChannelJudgement,
		recognitionLine{interaction: "gate", holdersOnly: true}))
	require.Equal(t, StanceNeutral, enc.stanceOf(escGate, FactionParty), "precondition: the garrison is civil")
	return enc
}

// escapeCapabilitiesFor is [escapeCapabilities], with the two a field with
// secrets needs when it has any: nothing is ever found by a search, and no
// open door is ever witnessed.
func escapeCapabilitiesFor(concealments []ConcealmentInput) Capabilities {
	caps := escapeCapabilities()
	if len(concealments) > 0 {
		caps.CheckResolver, caps.Witness = searchFindsNothing{}, nobodyPerceives{}
	}
	return caps
}

// searchFindsNothing fails every check it is asked, a search included.
type searchFindsNothing struct{}

// ResolveCheck fails the check.
func (searchFindsNothing) ResolveCheck(in *ResolveCheckInput) (*ResolveCheckOutput, error) {
	return &ResolveCheckOutput{Applied: in.Approaches[0]}, nil
}

// ResolveDiscoveryCheck fails the automatic check the same way.
func (searchFindsNothing) ResolveDiscoveryCheck(in *ResolveCheckInput) (*ResolveCheckOutput, error) {
	return &ResolveCheckOutput{Applied: in.Approaches[0]}, nil
}

// escapeCapabilities is the small site's host: everyone sees the whole map,
// nobody falls, and nothing acts on a table.
func escapeCapabilities() Capabilities {
	return Capabilities{
		Sight: everyoneSeesTheWholeMap{}, Equipment: UnobservedEquipment{}, Conditions: UnobservedEquipment{},
		Sheets: zeroSheets{}, Participation: everyoneStanding{}, Initiative: orderAsGiven{}, Driver: passDriver{},
		Actors: Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

func jailedIf(c CustodyPhase) []MemberID {
	if c == CustodyJailed {
		return []MemberID{escPris}
	}
	return nil
}

func (e *Encounter) stanceOf(a, b FactionID) Stance { return e.stanceTable()[pairOf(a, b)] }

// escapedKnown is whether the captain knows of the escape.
func escapedKnown(e *Encounter) bool { return e.world.knowsFact(escCaptain, escFact) }

// TestAnEscapeIsSeenAndTurnsTheGarrison: the jailed prisoner picks the leaf
// from inside — the guard now sees into the cell, and nothing turns — then
// steps out into the yard; the guard who arrested them sees them out of the
// cell, the captain learns of it, the garrison turns, and the fight forms.
// The free friend in the yard all along turned nothing. A second refresh
// teaches nothing again.
func TestAnEscapeIsSeenAndTurnsTheGarrison(t *testing.T) {
	enc := escapeSite(t, CustodyJailed, spatial.Position{X: 1, Y: 1})
	require.False(t, escapedKnown(enc), "the free friend in the yard turns nothing")

	_, err := enc.Unlock(&UnlockInput{Door: "leaf", Actor: escPris, Beaten: true, Total: 15,
		Applied: CheckApproach{Ability: "dex", DC: 13}})
	require.NoError(t, err, "the lock is picked from inside")
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.True(t, reach.Reaches(perception.Sight, escGuard, escPris), "precondition: the guard sees into the cell")
	require.False(t, escapedKnown(enc), "seen inside the cell through the opening: nothing")

	_, err = enc.Step(&StepInput{Member: escPris, To: enc.field.cellAt(spatial.Position{X: 2, Y: 1}), EndWalk: true})
	require.NoError(t, err, "the jailed walk out")
	region, _ := enc.field.regionOf(enc.field.cellAt(spatial.Position{X: 2, Y: 1}))
	require.Equal(t, escYard, region, "precondition: out of the cell")
	require.True(t, escapedKnown(enc), "the captain learns of the escape")
	require.Equal(t, StanceHostile, enc.stanceOf(escGate, FactionParty), "the garrison turns")
	bubble, err := enc.bubbleFor(escPris)
	require.NoError(t, err)
	require.NotNil(t, bubble, "and the fight forms")

	facts := len(enc.world.log.All())
	_, _, err = enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
	require.Len(t, enc.world.log.All(), facts, "already turned: nothing is taught again")
}

// TestNoEscapeWhereNothingIsJailed: a prisoner only held — not jailed —
// standing in the guard's sight outside the cell turns nothing.
func TestNoEscapeWhereNothingIsJailed(t *testing.T) {
	enc := escapeSite(t, CustodyHeld, spatial.Position{X: 3, Y: 1})
	_, _, err := enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
	require.False(t, escapedKnown(enc))
	require.Equal(t, StanceNeutral, enc.stanceOf(escGate, FactionParty))
}

// TestNoEscapeWithoutAMind: the captain, the garrison's mind, has left the
// run; the guard sees the jailed prisoner out of the cell and nothing turns.
func TestNoEscapeWithoutAMind(t *testing.T) {
	enc := escapeSite(t, CustodyJailed, spatial.Position{X: 1, Y: 1})
	_, err := enc.Exit(&ExitInput{Member: escCaptain})
	require.NoError(t, err)
	_, err = enc.Unlock(&UnlockInput{Door: "leaf", Actor: escPris, Beaten: true, Total: 15,
		Applied: CheckApproach{Ability: "dex", DC: 13}})
	require.NoError(t, err)
	_, err = enc.Step(&StepInput{Member: escPris, To: enc.field.cellAt(spatial.Position{X: 2, Y: 1}), EndWalk: true})
	require.NoError(t, err)
	require.Equal(t, StanceNeutral, enc.stanceOf(escGate, FactionParty), "a faction with no mind learns nothing")
}

// TestTheEscapeAsksTheJailedPhase pins the escape's custody gate
// (rpg-toolkit#2423, item 1): the prisoner is on the record's Jailed list but
// their phase is held — a record that never jailed them — and standing out
// of the cell in the guard's sight raises no escape.
func TestTheEscapeAsksTheJailedPhase(t *testing.T) {
	enc := escapeSite(t, CustodyHeld, spatial.Position{X: 3, Y: 1})
	enc.interactions["gate"].arrest.Jailed = []MemberID{escPris}
	refresh(t, enc)
	require.False(t, escapedKnown(enc), "held, not jailed: no escape")
}

// TestTheEscapeAsksTheJailedList pins the escape's restriction to the jailed
// at the trigger itself (rpg-toolkit#2423, item 2): the record's phase is
// jailed and somebody was jailed, but not this prisoner (not on its Jailed
// list), and standing outside in the guard's sight raises no escape.
func TestTheEscapeAsksTheJailedList(t *testing.T) {
	enc := escapeSite(t, CustodyJailed, spatial.Position{X: 3, Y: 1})
	enc.interactions["gate"].arrest.Jailed = []MemberID{"someone-else"}
	refresh(t, enc)
	require.False(t, escapedKnown(enc), "never jailed: no escape")
}

// TestARejoinedWantedMemberRaisesNoEscape (rpg-toolkit#2423, item 3): a taken
// member who left before the jail and came back is wanted and was never
// jailed; seen outside the cell by the guard who arrested them, they raise no
// escape — they are seized instead (wanted.go).
func TestARejoinedWantedMemberRaisesNoEscape(t *testing.T) {
	enc := escapeSite(t, CustodyJailed, spatial.Position{X: 3, Y: 1})
	made := enc.interactions["gate"].arrest
	made.Jailed = []MemberID{"someone-else"}
	made.Wanted = []WantedMember{{ID: escPris, RememberedBy: []MemberID{escGuard}}}
	reach, err := enc.placedSightReach()
	require.NoError(t, err)
	require.NoError(t, enc.holdTopic(reach, escGuard, []MemberID{escPris}, TopicWanted, ChannelJudgement,
		recognitionLine{interaction: "gate", holdersOnly: true}))
	refresh(t, enc)
	require.False(t, escapedKnown(enc), "a rejoined wanted member is no escapee")
	require.True(t, enc.inCustody(escPris), "they are seized, held in their own right")
}
