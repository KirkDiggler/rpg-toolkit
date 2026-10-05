// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// Held rows (rpg-project#520, R18): an effect a TARGET holds is a full row on
// that target's candidate, never on the declaration, and an effect no
// candidate holds appears nowhere.

// seatOnGoblin writes a condition onto a goblin's sheet in the session record
// and tells the session the goblin changed, as a host does after its own
// write: watchers re-look and their sightings carry it.
func (s *EffectRowsSuite) seatOnGoblin(goblin string, condition interface {
	ToJSON() (json.RawMessage, error)
}) {
	s.T().Helper()
	data := s.sessions.byID[erSession]
	seated := false
	for i := range data.NPCs {
		if data.NPCs[i].ID == goblin {
			data.NPCs[i].Conditions = append(data.NPCs[i].Conditions, s.raw(condition))
			seated = true
		}
	}
	s.Require().True(seated, "no goblin %q on the session record", goblin)
	_, err := s.mgr.Recheck(s.ctx, &session.RecheckInput{Session: erSession, Members: []string{goblin}})
	s.Require().NoError(err)
}

func (s *EffectRowsSuite) faerieFireOn(goblin string) *conditions.FaerieFireCondition {
	s.T().Helper()
	ff, err := conditions.NewFaerieFireCondition(conditions.NewFaerieFireConditionInput{
		MemberID: goblin, SourceID: erAlly, SourceRef: refs.Spells.FaerieFire(),
	})
	s.Require().NoError(err)
	return ff
}

func (s *EffectRowsSuite) TestCandidateCarriesFaerieFireHeldRow() {
	s.cave(s.rogue())
	s.seatOnGoblin(erGoblin1, s.faerieFireOn(erGoblin1))

	attack := s.mainAttack(s.afford("alice"))

	display, found := conditions.DisplayFor(*refs.Conditions.FaerieFire())
	s.Require().True(found)
	s.Equal([]session.EffectRow{{
		ID:            "target:" + refs.Conditions.FaerieFire().String() + "@" + erAlly,
		Ref:           refs.Conditions.FaerieFire().String(),
		Name:          conditions.FaerieFireName,
		Description:   display.Detail,
		State:         session.EffectApplies,
		Reason:        "You can see the outlined target",
		Participation: session.ContributesNow,
		Benefit:       "Advantage on the attack roll",
	}}, s.candidate(attack, erGoblin1).HeldEffects)
	s.Empty(s.candidate(attack, erGoblin2).HeldEffects, "goblin two holds nothing that bears")
	for _, row := range attack.Effects {
		s.NotEqual(refs.Conditions.FaerieFire().String(), row.Ref, "a target's effect is never a declaration row")
	}
}

// TestHeldRowIDsDistinctFromDeclarationRows: a prone rogue swings at a prone
// goblin. Both rows exist — the actor's own Prone on the declaration, the
// goblin's on its candidate — and their IDs differ.
func (s *EffectRowsSuite) TestHeldRowIDsDistinctFromDeclarationRows() {
	rogue := s.rogue()
	rogue.Conditions = append(rogue.Conditions, s.raw(conditions.NewProneCondition("alice")))
	s.cave(rogue)
	s.seatOnGoblin(erGoblin1, conditions.NewProneCondition(erGoblin1))

	attack := s.mainAttack(s.afford("alice"))

	declared := s.row(attack, refs.Conditions.Prone().String())
	held := s.candidate(attack, erGoblin1).HeldEffects
	s.Require().Len(held, 1)
	s.Equal("target:"+refs.Conditions.Prone().String(), held[0].ID)
	s.NotEqual(declared.ID, held[0].ID)
	s.Equal(session.EffectApplies, held[0].State)
}

// TestUnseenHoldingsYieldNoHeldRows: the goblin holds Faerie Fire, but the
// rogue's sighting of it predates the condition and no re-look has happened,
// so its holdings as the rogue knows them hold nothing — no row, and the
// declaration still builds.
func (s *EffectRowsSuite) TestUnseenHoldingsYieldNoHeldRows() {
	s.cave(s.rogue())
	data := s.sessions.byID[erSession]
	for i := range data.NPCs {
		if data.NPCs[i].ID == erGoblin1 {
			data.NPCs[i].Conditions = append(data.NPCs[i].Conditions, s.raw(s.faerieFireOn(erGoblin1)))
		}
	}

	attack := s.mainAttack(s.afford("alice"))

	s.Empty(s.candidate(attack, erGoblin1).HeldEffects, "rows are testimony, never a live read of the sheet")
}

func (s *EffectRowsSuite) TestHeldEffectsWireKey() {
	s.cave(s.rogue())
	s.seatOnGoblin(erGoblin1, s.faerieFireOn(erGoblin1))

	raw, err := json.Marshal(s.mainAttack(s.afford("alice")))
	s.Require().NoError(err)
	var wire struct {
		Candidates []struct {
			Member      string                       `json:"member"`
			HeldEffects []map[string]json.RawMessage `json:"held_effects"`
		} `json:"candidates"`
	}
	s.Require().NoError(json.Unmarshal(raw, &wire))
	carried := false
	for _, candidate := range wire.Candidates {
		if candidate.Member != erGoblin1 {
			continue
		}
		s.Require().Len(candidate.HeldEffects, 1, "%s", raw)
		for _, key := range []string{"id", "ref", "name", "description", "state", "reason", "participation", "benefit"} {
			s.Contains(candidate.HeldEffects[0], key, "a held row is a full row: %s", raw)
		}
		carried = true
	}
	s.True(carried, "goblin one carries its held row: %s", raw)
}
