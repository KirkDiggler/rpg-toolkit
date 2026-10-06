// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
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

// TestAStaleSightingOfAnUntouchedMemberDrawsNoRecheck: R19 is an EVENT — a
// change to a member's conditions refreshes sightings of that member — not a
// standing diff between sightings and sheets. Goblin one's sheet gains Faerie
// Fire with no verb declaring it, so the rogue's current sighting of it is
// stale; later verbs that never touch the goblin (alice dodges; the host
// re-looks at alice) must not re-look at it or send a beat naming it. Whatever
// ordinary sight refresh those verbs run may still renew the goblin's
// testimony — that is sight doing its job, not this trigger.
func (s *EffectRowsSuite) TestAStaleSightingOfAnUntouchedMemberDrawsNoRecheck() {
	s.cave(s.rogue())
	data := s.sessions.byID[erSession]
	for i := range data.NPCs {
		if data.NPCs[i].ID == erGoblin1 {
			data.NPCs[i].Conditions = append(data.NPCs[i].Conditions, s.raw(s.faerieFireOn(erGoblin1)))
		}
	}
	s.stream.published = nil

	// Two verbs that touch alice and never the goblin: the dodge, then the
	// host's re-look at alice. A standing diff would re-look at the goblin on
	// each of them, forever.
	_, err := s.mgr.Activate(s.ctx, &session.ActivateInput{
		Session: erSession, Member: "alice", DeclarationID: activationSelector(s.T(), s.mgr, "alice", "dnd5e:combat_abilities:dodge"),
	})
	s.Require().NoError(err)
	_, err = s.mgr.Recheck(s.ctx, &session.RecheckInput{Session: erSession, Members: []string{"alice"}})
	s.Require().NoError(err)

	for _, event := range s.stream.published {
		if body, ok := event.Body.(session.SightedBody); ok {
			s.NotContains(body.Changed, erGoblin1, "no beat names the untouched goblin: %+v", body)
		}
	}
}

// TestConditionChangeRechecksWatchers: the cleric's Guiding Bolt lights goblin
// one, nobody moves, and the cleric's next spell attack against it carries the
// Guiding Bolt row — the commit re-looked at the goblin whose conditions the
// cast changed (R19).
func (s *EffectRowsSuite) TestConditionChangeRechecksWatchers() {
	s.cave(s.blessedCleric())
	data := s.sessions.byID[erSession]
	for i := range data.NPCs {
		if data.NPCs[i].ID == erGoblin1 {
			data.NPCs[i].HitPoints, data.NPCs[i].MaxHitPoints = 100, 100 // survives the bolt
		}
	}
	bolt := s.castOf(s.afford("cleric"), spells.GuidingBolt)
	s.Require().Empty(s.candidate(bolt, erGoblin1).HeldEffects, "precondition: nothing lit yet")

	_, err := s.mgr.Cast(s.ctx, &session.CastInput{
		Session: erSession, Member: "cleric", DeclarationID: bolt.ID, Targets: []string{erGoblin1},
	})
	s.Require().NoError(err)

	held := s.candidate(s.castOf(s.afford("cleric"), spells.GuidingBolt), erGoblin1).HeldEffects
	s.Require().Len(held, 1, "the goblin's sighting carries the light with nobody moving")
	s.Equal("target:"+refs.Conditions.GuidingBolt().String()+"@cleric", held[0].ID)
	s.Equal(session.EffectApplies, held[0].State)
}

// TestDeliveredPayloadsCarryNoConditions: what a sighting snapshots of a
// member's conditions is testimony for rules, never something delivered.
// Goblin one holds Faerie Fire and has been re-looked at: neither alice's
// View sightings nor any sighted beat on the stream carries a "conditions" key.
func (s *EffectRowsSuite) TestDeliveredPayloadsCarryNoConditions() {
	s.cave(s.rogue())
	s.seatOnGoblin(erGoblin1, s.faerieFireOn(erGoblin1))
	s.Require().NotEmpty(s.candidate(s.mainAttack(s.afford("alice")), erGoblin1).HeldEffects,
		"precondition: the testimony does hold the condition")

	seen, err := s.mgr.View(s.ctx, &session.ViewInput{Session: erSession, Member: "alice"})
	s.Require().NoError(err)
	checked := 0
	for _, sighting := range seen.Sightings {
		var payload map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(sighting.Payload, &payload))
		s.NotContains(payload, "conditions", "sighting of %s", sighting.Subject)
		checked++
	}
	s.Positive(checked, "precondition: alice holds sightings")

	beats := 0
	for _, event := range s.stream.published {
		if event.Kind != session.EventSighted {
			continue
		}
		var payload map[string]json.RawMessage
		s.Require().NoError(json.Unmarshal(event.Payload, &payload))
		s.NotContains(payload, "conditions", "sighted beat to %s", event.Recipient)
		beats++
	}
	s.Positive(beats, "precondition: the re-look sent sighted beats")
}

// TestACorruptOwnConditionBlocksTheTurnVisibly pins what a player sees when
// their own sheet holds a condition that cannot be read: the offer compiler
// loads the actor strictly, so every action is unavailable for that reason and
// carries no rows — there is no available attack whose swing would then fail.
// Only ending the turn remains.
func (s *EffectRowsSuite) TestACorruptOwnConditionBlocksTheTurnVisibly() {
	s.cave(s.rogue())
	stored := s.characters.byID["alice"]
	stored.Conditions = append(stored.Conditions, json.RawMessage(`{"ref":"nonsense","x":`))

	out, err := s.mgr.Afford(s.ctx, &session.AffordInput{Session: erSession, Member: "alice"})
	s.Require().NoError(err, "the panel still answers")

	for _, declaration := range out.Declarations {
		s.Empty(declaration.Effects, "%s carries no row", declaration.Verb)
		if declaration.Verb == session.VerbEndTurn {
			s.True(declaration.Available, "ending the turn is still possible")
			continue
		}
		s.False(declaration.Available, "%s is not offered", declaration.Verb)
		s.Require().NotNil(declaration.Why, declaration.Verb)
		s.Equal(session.ShortfallUnreadable, declaration.Why.Reason, declaration.Verb)
		s.Nil(declaration.Attack, "no compiled attack to swing: %s", declaration.Verb)
	}
}

// TestACorruptTargetConditionFailsTheRead: a condition that cannot be read on
// a candidate target's sheet fails Afford itself, loudly — resolution attaches
// the target strictly to judge participation — rather than offering an attack
// the swing would then refuse.
func (s *EffectRowsSuite) TestACorruptTargetConditionFailsTheRead() {
	s.cave(s.rogue())
	data := s.sessions.byID[erSession]
	for i := range data.NPCs {
		if data.NPCs[i].ID == erGoblin1 {
			data.NPCs[i].Conditions = append(data.NPCs[i].Conditions,
				json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"faerie_fire"},"member_id":""}`))
		}
	}

	out, err := s.mgr.Afford(s.ctx, &session.AffordInput{Session: erSession, Member: "alice"})

	s.Require().Error(err)
	s.Nil(out)
	s.Contains(err.Error(), erGoblin1)
}

// TestAHitOnAFreshMonsterSendsNoSightedBeat: alice wounds a goblin that has
// just arrived, and it stays on its feet. Nobody's conditions change — every
// combatant holds its opportunity attack from the moment it is a participant,
// whether or not its stored sheet has caught up, and the wound is what writes
// the goblin's sheet back with it — so no watcher is told to look again.
func (s *EffectRowsSuite) TestAHitOnAFreshMonsterSendsNoSightedBeat() {
	s.cave(s.rogue())
	data := s.sessions.byID[erSession]
	for i := range data.NPCs {
		if data.NPCs[i].ID == erGoblin1 {
			data.NPCs[i].HitPoints, data.NPCs[i].MaxHitPoints = 100, 100 // wounded, still standing
		}
	}
	s.stream.published = nil

	out, err := s.mgr.Attack(s.ctx, &session.AttackInput{
		Session: erSession, Attacker: "alice", Target: erGoblin1,
		DeclarationID: s.mainAttack(s.afford("alice")).ID,
	})
	s.Require().NoError(err)
	s.Require().True(out.Hit, "precondition: the swing lands")

	for _, event := range s.stream.published {
		if body, ok := event.Body.(session.SightedBody); ok {
			s.Empty(body.Changed, "no condition changed, so nobody is told to look again: %+v to %s", body, event.Recipient)
		}
	}
}
