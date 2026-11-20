// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// status_receivers_internal_test.go is WHO RECEIVES A STATUS EFFECT
// (rpg-project#990) on the hold hall: a line's row gives a status to the
// members its `to` word resolves to when the row lands, by the resolver
// `give` uses (whonow.go). The guard sees two cells: alice beside him,
// bob and carol of the party far down the yard until moved.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// heldStatus is the status the hall's lines give.
const heldStatus = "held"

// statusGrantTo is the hall's status, given to the word.
func statusGrantTo(to string) Consequence {
	return Consequence{ApplyStatus: &StatusGrant{ID: heldStatus, Status: json.RawMessage(`{"name":"Held"}`), To: to}}
}

// holdLine is a plain line, offered once the gate has admitted, whose
// answer gives the hall's status to each word given, one grant each.
func holdLine(id string, to ...string) Option {
	o := Option{
		ID: id, Say: "Let us by.",
		Requires: []Trigger{atTheAdmittedGate},
		Success:  []Answer{{Weight: 1, Say: "Stay right there. All of you."}},
	}
	for _, w := range to {
		o.Grants = append(o.Grants, RowGrant{Row: 0, Consequence: statusGrantTo(w)})
	}
	return o
}

// pushLine is a checked line whose FAILURE row gives the hall's status to
// the speaker's party in the guard's sight: the push-past of the castle.
func pushLine() Option {
	return Option{
		ID: "push", Say: "Out of my way.",
		Check:    []CheckApproach{{Ability: "intimidation", DC: 13}},
		Requires: []Trigger{atTheAdmittedGate},
		Success:  []Answer{{Weight: 1, Say: "Go on, then."}},
		Failure:  []Answer{{Weight: 1, Say: "Not so fast."}},
		Grants:   []RowGrant{{Row: 0, Failure: true, Consequence: statusGrantTo(string(RecognitionPartyInSight))}},
	}
}

// receiverHall is the hold hall with the guard carrying the claim and the
// given lines, seeing two cells, bob and carol out of his sight and the
// claim already admitted, so the lines are offered.
func receiverHall(t *testing.T, sightCells int, lines []Option, members ...MemberInput) *Encounter {
	t.Helper()
	guard := recognisingGuard(t, postedGuard, holdNorthWest, true)
	guard.Options = append(guard.Options, lines...)
	in := recognisingSetup(t, false, append([]MemberInput{
		guard,
		{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 8, Y: 4}},
		{ID: "carol", Kind: KindPlayer, Position: spatial.Position{X: 9, Y: 4}},
	}, members...)...)
	in.Sight = shortSighted{member: postedGuard, cells: sightCells}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	return enc
}

// receivers is who the output's statuses name, each a granted [heldStatus].
func receivers(t *testing.T, given []GrantedStatus) []MemberID {
	t.Helper()
	var out []MemberID
	for _, g := range given {
		require.Equal(t, heldStatus, g.Grant.ID)
		require.JSONEq(t, `{"name":"Held"}`, string(g.Grant.Status), "the authored block rides unread")
		out = append(out, g.Member)
	}
	return out
}

// sayTo has alice say the line to the guard.
func sayTo(t *testing.T, enc *Encounter, option string) *SpeakOutput {
	t.Helper()
	out, err := enc.Speak(context.Background(), &SpeakInput{
		Actor: "alice", Target: postedGuard, OptionID: option, Roller: wakeRollsOne{}, Range: 3,
		Revision: enc.interactions[hailGate].revision,
	})
	require.NoError(t, err)
	return out
}

func TestAStatusGoesToWhomItsWordNamesWhenTheRowLands(t *testing.T) {
	lines := []Option{
		holdLine("hold-speaker", string(RecognitionSpeaker)),
		holdLine("hold-party", string(RecognitionPartyInSight)),
		holdLine("hold-spoken-to", string(RecognitionSpokenTo)),
		holdLine("hold-short", ""),
		holdLine("hold-twice", string(RecognitionSpeaker), string(RecognitionPartyInSight)),
	}
	// Bob comes up the yard into the guard's sight; carol stays out of it.
	inSight := func(t *testing.T, cells int) *Encounter {
		enc := receiverHall(t, cells, lines)
		placeAt(t, enc, "bob", holdSouthWest)
		return enc
	}

	t.Run("party_in_sight is the speaker's party the one spoken to sees", func(t *testing.T) {
		enc := inSight(t, 2)
		got := receivers(t, sayTo(t, enc, "hold-party").Statuses)
		require.ElementsMatch(t, []MemberID{"alice", "bob"}, got, "carol, out of his sight, receives nothing")
	})
	t.Run("speaker is the speaker alone", func(t *testing.T) {
		enc := inSight(t, 2)
		require.Equal(t, []MemberID{"alice"}, receivers(t, sayTo(t, enc, "hold-speaker").Statuses))
	})
	t.Run("spoken_to is the creature the line was said to", func(t *testing.T) {
		enc := inSight(t, 2)
		require.Equal(t, []MemberID{postedGuard}, receivers(t, sayTo(t, enc, "hold-spoken-to").Statuses))
	})
	t.Run("no word is the speaker, until the grammar requires one", func(t *testing.T) {
		enc := inSight(t, 2)
		require.Equal(t, []MemberID{"alice"}, receivers(t, sayTo(t, enc, "hold-short").Statuses))
	})
	t.Run("nobody is named twice", func(t *testing.T) {
		enc := inSight(t, 2)
		got := receivers(t, sayTo(t, enc, "hold-twice").Statuses)
		require.ElementsMatch(t, []MemberID{"alice", "bob"}, got, "alice is the speaker and in his sight: once")
	})
}

// THE RECEIVER `speaker` CARRIES NO SIGHT: asked of the resolver for a line
// whose creature sees nobody (a line Speak would refuse unwitnessed, but
// that a settle judged as spoken or a later caller may land), the status's
// `speaker` is still the speaker, where give's `speaker` and
// `party_in_sight` are nobody.
func TestTheReceiverSpeakerIsTheSpeakerUnseen(t *testing.T) {
	enc := receiverHall(t, 2, nil)
	enc.sight = shortSighted{member: postedGuard, cells: 0}
	l := enc.landingOf(recognitionLine{speaker: "alice", spokenTo: postedGuard})
	who := func(place, word string) []MemberID {
		got, err := l.whoNow(place, word)
		require.NoError(t, err)
		return got
	}
	require.Equal(t, []MemberID{"alice"}, who(WhoInApplyStatusTo, string(RecognitionSpeaker)), "seen or not")
	require.Empty(t, who(WhoInApplyStatusTo, string(RecognitionPartyInSight)), "he sees none of the party")
	require.Empty(t, who(WhoInGiveAbout, string(RecognitionSpeaker)), "give's speaker needs him to see them")
	require.Equal(t, []MemberID{postedGuard}, who(WhoInApplyStatusTo, string(RecognitionSpokenTo)))
}

func TestAStatusOnAMissedRowLandsOnlyWhenMissed(t *testing.T) {
	enc := receiverHall(t, 2, []Option{pushLine()})
	placeAt(t, enc, "bob", holdSouthWest)
	push := func(beaten bool) *SpeakOutput {
		out, err := enc.Speak(context.Background(), &SpeakInput{
			Actor: "alice", Target: postedGuard, OptionID: "push", Roller: wakeRollsOne{}, Range: 3,
			Revision: enc.interactions[hailGate].revision,
			Skill:    "intimidation", Beaten: beaten, DC: 13, Total: map[bool]int{true: 18, false: 4}[beaten],
		})
		require.NoError(t, err)
		return out
	}
	require.Empty(t, push(true).Statuses, "a row that does not land gives nothing")
	require.ElementsMatch(t, []MemberID{"alice", "bob"}, receivers(t, push(false).Statuses))
}

// A STAGED CHECK IS JUDGED WHEN IT LANDS: reserved while bob is out of the
// guard's sight, settled after he came into it, the row gives to whom the
// guard sees at the settle — live sight when the verdict lands, the same
// moment a direct line is judged at — because the verdict, not the reserve,
// is the line landing. (An ABANDONED settle is the one exception, and the
// resolver's: it is judged in the scene it was spoken in, as `give` is.)
func TestAStagedStatusGoesToWhomTheGuardSeesWhenItLands(t *testing.T) {
	enc := receiverHall(t, 2, []Option{pushLine()})
	at := enc.interactions[hailGate].revision
	_, err := enc.Reserve(&ReserveInput{Actor: "alice", Target: postedGuard, OptionID: "push", Revision: at, Range: 3})
	require.NoError(t, err)
	placeAt(t, enc, "bob", holdSouthWest)

	out, err := enc.Speak(context.Background(), &SpeakInput{
		Actor: "alice", Target: postedGuard, OptionID: "push", Roller: wakeRollsOne{}, Range: 3, Revision: at,
		Skill: "intimidation", Beaten: false, DC: 13, Total: 4,
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []MemberID{"alice", "bob"}, receivers(t, out.Statuses),
		"bob came into sight before it landed; carol never did")
}

// A WORLD NPC IS IN NO FACTION (field.go), so the speaker's party never
// reaches one, however close it stands: the rule that a world NPC has no
// sheet to hold a status on (rpg-project#990) is met at `party_in_sight` by
// this, and at `spoken_to` by the grammar.
func TestPartyInSightNeverReachesAWorldNPC(t *testing.T) {
	cook := MemberInput{ID: "cook", Kind: KindWorld, Name: "Cook", Position: holdBeside}
	enc := receiverHall(t, 2, []Option{holdLine("hold-party", string(RecognitionPartyInSight))}, cook)
	got := receivers(t, sayTo(t, enc, "hold-party").Statuses)
	require.NotContains(t, got, MemberID("cook"), "a world NPC beside the guard is nobody's party")
	require.Contains(t, got, MemberID("alice"))
}

// A THING'S ROW HAS NOBODY SPOKEN TO: its `party_in_sight` and `spoken_to`
// resolve to nobody, and its `speaker` to the speaker.
func TestOnAThingsRowOnlyTheSpeakerIsAnybody(t *testing.T) {
	enc := receiverHall(t, 2, nil)
	l := enc.landingOf(recognitionLine{speaker: "alice"})
	for word, want := range map[string][]MemberID{
		string(RecognitionSpeaker):      {"alice"},
		string(RecognitionPartyInSight): nil,
		string(RecognitionSpokenTo):     nil,
	} {
		got, err := l.whoNow(WhoInApplyStatusTo, word)
		require.NoError(t, err)
		require.Equal(t, want, got, word)
	}
}

func TestAStatusGrantNamesAReceiverTheTableHas(t *testing.T) {
	g := statusGrantTo("the_taken")
	require.ErrorContains(t, validateConsequence("row", g, false), "who receives it is one of")
	for _, w := range WhoWordsIn(WhoInApplyStatusTo) {
		require.NoError(t, validateConsequence("row", statusGrantTo(w), false), w)
	}
	_, err := enc0(t).landingOf(recognitionLine{speaker: "alice"}).whoNow(WhoInApplyStatusTo, "the_taken")
	require.Error(t, err, "a word its place does not allow is never resolved")
}

// enc0 is the bare hall, for asking the resolver alone.
func enc0(t *testing.T) *Encounter {
	t.Helper()
	return receiverHall(t, 2, nil)
}
