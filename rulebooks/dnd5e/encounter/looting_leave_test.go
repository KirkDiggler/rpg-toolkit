// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// looting_leave_test.go is A LOOTER LEAVES WHAT THEY ARE TOLD TO LEAVE
// (rpg-project#850, rpg-project#1176): the caller of Loot names equipped items
// the looter does not take, and they stay on the body, untold, for someone else.
// The encounter does not know what a looter owns; the caller does. The law it
// must not break is loot.go's: a loot looks the same whether or not the body
// had anything.

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

const cloakID = "lieutenants-cloak"

// lootLeaving downs the captain, brings the raider beside them and loots,
// leaving the named items.
func (s *HoldingsSuite) lootLeaving(enc *encounter.Encounter, leave ...string) *encounter.LootOutput {
	s.drop(enc)
	s.walkTo(enc, raider, captainCell)
	out, err := enc.Loot(&encounter.LootInput{Member: raider, Target: captain, Leave: leave})
	s.Require().NoError(err)
	return out
}

// reloaded saves the encounter through JSON and loads it back.
func (s *HoldingsSuite) reloaded(enc *encounter.Encounter) *encounter.Encounter {
	raw, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(raw, &data))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: s.capabilities(s.standing)})
	s.Require().NoError(err)
	return loaded
}

// Tests 1 and 2: what is left stays, untold, through save and load, and the
// next player takes it.
func (s *HoldingsSuite) TestWhatTheLooterLeavesStaysOnTheBodyForTheNextPlayer() {
	enc := s.openGeared(false, cloakGrant)
	out := s.lootLeaving(enc, cloakID)

	s.Require().Empty(out.Items)
	s.Require().Empty(s.beatsOfKind(enc, raider, encounter.BeatTaken), "no taken beat")
	s.Require().Equal(jsonOf(s, []encounter.ItemGrant{cloakGrant}), jsonOf(s, s.bodyGear(enc)), "byte for byte")

	loaded := s.reloaded(enc)
	s.Require().Equal(jsonOf(s, []encounter.ItemGrant{cloakGrant}), jsonOf(s, s.bodyGear(loaded)), "through save and load")

	for name, host := range map[string]*encounter.Encounter{"the live encounter": enc, "the loaded one": loaded} {
		s.Run("a second player takes it from "+name, func() {
			s.walkTo(host, partner, captainCell)
			got, err := host.Loot(&encounter.LootInput{Member: partner, Target: captain})
			s.Require().NoError(err)
			s.Require().Len(got.Items, 1)
			s.Require().Equal(partner, got.Items[0].Member)
			s.Require().Equal(cloakID, got.Items[0].Grant.ID)
			s.Require().Equal(cloakJSON, string(got.Items[0].Grant.Item))
			s.Require().Len(s.beatsOfKind(host, partner, encounter.BeatTaken), 1)
			s.Require().Empty(s.bodyGear(host), "the body has nothing")
		})
	}
}

// Test 3: two items, one left: the looter gets the other.
func (s *HoldingsSuite) TestLeavingOneOfTwoTakesTheOther() {
	enc := s.openGeared(false, cloakGrant, ringGrant)
	out := s.lootLeaving(enc, cloakID)

	s.Require().Len(out.Items, 1)
	s.Require().Equal(ringGrant.ID, out.Items[0].Grant.ID)
	taken := s.beatsOfKind(enc, raider, encounter.BeatTaken)
	s.Require().Len(taken, 1)
	s.Require().Equal(ringGrant.ID, taken[0]["item"])
	s.Require().Equal(jsonOf(s, []encounter.ItemGrant{cloakGrant}), jsonOf(s, s.bodyGear(enc)))
}

// Test 4: the caller does not know the body.
func (s *HoldingsSuite) TestLeaveIdsThatDoNotMatchAreIgnored() {
	cases := map[string][]string{
		"an id the body does not have": {"no-such-item", cloakID},
		"an empty string":              {"", cloakID},
		"the same id twice":            {cloakID, cloakID},
	}
	for name, leave := range cases {
		s.Run(name, func() {
			s.SetupTest()
			enc := s.openGeared(false, cloakGrant, ringGrant)
			out := s.lootLeaving(enc, leave...)
			s.Require().Len(out.Items, 1)
			s.Require().Equal(ringGrant.ID, out.Items[0].Grant.ID)
			s.Require().Equal(jsonOf(s, []encounter.ItemGrant{cloakGrant}), jsonOf(s, s.bodyGear(enc)))
		})
	}

	s.Run("only ids the body lacks takes everything", func() {
		s.SetupTest()
		enc := s.openGeared(false, cloakGrant)
		out := s.lootLeaving(enc, "no-such-item", "")
		s.Require().Len(out.Items, 1)
		s.Require().Empty(s.bodyGear(enc))
	})
}

// Test 5: loot looks the same. Everything but Items and the taken beat is
// equal across a loot with no Leave, a loot leaving the cloak, and a body with
// no equipment at all.
func (s *HoldingsSuite) TestLootLooksTheSameWhateverTheLooterLeaves() {
	look := func(leave []string, gear ...encounter.ItemGrant) string {
		s.SetupTest()
		enc := s.openGeared(true, gear...)
		out := s.lootLeaving(enc, leave...)
		story := map[string][]map[string]any{}
		for _, member := range []encounter.MemberID{raider, partner, captain, sentry} {
			for _, b := range s.beats(enc, member) {
				if b["beat"] != encounter.BeatTaken {
					story[string(member)] = append(story[string(member)], b)
				}
			}
		}
		rest := *out
		rest.Items = nil
		return jsonOf(s, map[string]any{"output": rest, "story": story})
	}

	everything := look(nil, cloakGrant)
	left := look([]string{cloakID}, cloakGrant)
	bare := look(nil)
	s.Require().Equal(everything, left, "leaving the cloak looks like taking it")
	s.Require().Equal(everything, bare, "and like a body with nothing")
}

// Test 6: a looter that is not a player takes nothing, Leave or not.
func (s *HoldingsSuite) TestAMonsterLooterWithLeaveChangesNothing() {
	enc := s.openGeared(false, cloakGrant)
	s.drop(enc)
	out, err := enc.Loot(&encounter.LootInput{Member: sentry, Target: captain, Range: 50, Leave: []string{cloakID}})
	s.Require().NoError(err)
	s.Require().Nil(out.Items)
	for _, member := range []encounter.MemberID{raider, partner, sentry} {
		s.Require().Empty(s.beatsOfKind(enc, member, encounter.BeatTaken))
	}
	s.Require().Equal([]encounter.ItemGrant{cloakGrant}, s.bodyGear(enc))
}
