// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// AttackInput swings one member at another.
//
// MEMBER-NEUTRAL ON PURPOSE. Both sides are IDs and nothing here is
// character-shaped, even though v1 only compiles character attackers (see
// [Manager.Attack]). Scope by the case, shape for the future: the day a
// monster attacker is compiled, this input does not change, and no host that
// wrote against it has to.
type AttackInput struct {
	// Session is the session to act in.
	Session string

	// Attacker is who swings. Required.
	Attacker string

	// Target is who they swing at. Required.
	Target string
}

// AttackOutput is what the swing produced.
type AttackOutput struct {
	// Roll is the d20 as rolled, after any advantage or disadvantage.
	Roll int `json:"roll"`

	// Total is the roll plus everything the attack chain added.
	Total int `json:"total"`

	// Against is the number the total had to reach.
	Against int `json:"against"`

	// Hit is whether it landed.
	Hit bool `json:"hit"`

	// Critical is whether it was a critical hit.
	Critical bool `json:"critical"`

	// Damage is what was dealt. Zero on a miss.
	Damage int `json:"damage,omitempty"`

	// Seq is the story sequence of the recorded beat.
	Seq uint64 `json:"seq"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// Attack swings one member's weapon at another and records what happened.
//
// # v1 compiles CHARACTER attackers only
//
// A monster attacker is refused with [ErrNotACharacter]. The refusal is scope
// rather than a limitation nobody thought about: a monster's action can declare
// a save gate, which makes a strike's rider — a whole second interaction with
// its own DC, ability and imposed effects — reachable, and this seam has no
// vocabulary for recording one. That case belongs to the monster behavior work,
// which is its caller, and it earns its shape then. The same discipline
// [DissolveCause] uses for defeat.
//
// Main hand, one-handed, melee. Two-handed grips and the off-hand swing are
// the compiler's own named gaps and arrive with the economy that decides when
// a second swing is allowed at all.
//
// # Nothing spends
//
// A character can attack as many times in a turn as the caller asks. That is a
// KNOWN GAP, not a ruling that attacks are free: the thing that spends is an
// economy machine that sits above this one and does not exist. It is named at
// the one place it will go — see the comment at the resolution call below —
// and pinned by TestNothingSpendsYet so it cannot be mistaken for a decision.
//
// # How it runs
//
// The world goes into resolution as data and a different world comes back, so
// the scope adopts the returned one before anything else touches it
// ([Manager.adopt] carries that invariant). The outcome is then recorded on the
// world the interaction produced — a consequence lands after its cause — and
// every dirty sheet is written back.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember, ErrNotACharacter, ErrNoSheet, ErrNoCharacter,
// ErrBadCharacter, ErrBadRepository, ErrBadAttack, ErrClosed, or ErrSaveFailed
// with a populated report.
//
// ErrNoCharacter and ErrBadCharacter mean here exactly what they mean
// everywhere else in this package: the repository does not hold that sheet,
// versus it holds bytes that will not reconstitute. A host branches on the
// difference — re-check the ID, versus go and inspect storage — so the two are
// worth reading off carefully.
func (m *Manager) Attack(ctx context.Context, in *AttackInput) (*AttackOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("attack: %w", ErrNilInput)
	}
	if in.Attacker == "" || in.Target == "" {
		return nil, fmt.Errorf("attack: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}
	kinds := map[string]encounter.MemberKind{}
	for _, member := range roster {
		kinds[string(member.ID)] = member.Kind
	}
	if _, ok := kinds[in.Attacker]; !ok {
		return nil, fmt.Errorf("attack: attacker %q: %w", in.Attacker, ErrNoMember)
	}
	if _, ok := kinds[in.Target]; !ok {
		return nil, fmt.Errorf("attack: target %q: %w", in.Target, ErrNoMember)
	}
	if kinds[in.Attacker] != encounter.MemberKind(KindPlayer) {
		return nil, fmt.Errorf("attack: attacker %q: %w", in.Attacker, ErrNotACharacter)
	}

	// A member with no stored sheet cannot be swung at: there is nothing to
	// read an armour class off and nothing for damage to land on. Authored
	// content placed straight into a world has no sheet until something spawns
	// it, so this is reachable and worth naming here — the alternative is the
	// strike failing later, further from the cause.
	if kinds[in.Target] == encounter.MemberKind(KindMonster) {
		if _, ok := m.storedNPC(scope, in.Target); !ok {
			return nil, fmt.Errorf("attack: target %q: %w", in.Target, ErrNoSheet)
		}
	}

	profile, err := m.compileAttack(ctx, in.Attacker)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	cast, err := m.castFor(ctx, scope, roster)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	// THE ONE PLACE A SPEND WILL GO. An economy machine belongs above this
	// call: it chooses what the actor is doing, spends the action or bonus
	// action that buys it, and requests the strike below. Today nothing does,
	// so nothing is spent — the strike runs directly. When the economy lands
	// (its home is the combat package, whose ActionEconomy vocabulary already
	// exists), the machine handed to Resolve becomes that one, and this line
	// is where it changes. Nothing persistent or wire-shaped assumes attacks
	// are free, so that change costs no migration.
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        scope.enc.ToData(),
		Participants: cast,
		Initiative:   m.initiative,
		Machine: resolution.NewStrike(&resolution.StrikeInput{
			AttackerID: in.Attacker,
			TargetID:   in.Target,
			Attack:     profile,
			// The machine rolls the attack and its damage; Input.Roller only
			// reconstitutes effects that need one. Two rollers because they
			// are two jobs — and BOTH must be the host's, or the swing is
			// resolved with randomness the host never supplied. StrikeInput's
			// still defaults silently when nil, which is the class resolution
			// just closed on its own Input (#1033); leaving it unset here is
			// how this verb first resolved with unreproducible dice.
			Roller: &diceSeam{roller: m.dice},
		}),
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("attack: %w", translateResolution(err))
	}

	struck, ok := out.Outcome.(resolution.StrikeOutcome)
	if !ok {
		return nil, fmt.Errorf("attack: %w: strike produced %T", ErrInvalidWorld, out.Outcome)
	}

	// The world that came back is the only true one now.
	if err := m.adopt(scope, out.World); err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	recorded, err := scope.enc.Record(recordFor(in, struck))
	if err != nil {
		return nil, fmt.Errorf("attack: %w", translate(err))
	}

	if err := m.saveDirty(ctx, scope, out); err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	return &AttackOutput{
		Roll:     struck.Roll,
		Total:    struck.Total,
		Against:  struck.TargetAC,
		Hit:      struck.Hit,
		Critical: struck.Critical,
		Damage:   struck.Damage,
		Seq:      recorded.Seq,
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// translateResolution maps the resolution module's sentinels onto this
// package's own.
//
// The same reason translate exists for the composition's: a sentinel is not a
// type in a signature, so the boundary test cannot see it, and a host that
// matched on resolution.ErrBadParticipant would be coupled to a module we
// intend to keep replaceable. Unrecognised errors pass through wrapped rather
// than flattened — an error nobody anticipated is more useful with its own
// message intact.
func translateResolution(err error) error {
	switch {
	case errors.Is(err, resolution.ErrBadParticipant):
		return fmt.Errorf("%w: %w", ErrBadCharacter, err)
	case errors.Is(err, resolution.ErrNoCombatant):
		// Reachable when a member has no stored sheet — an authored monster
		// standing in a world nobody spawned. Refused earlier by name, so this
		// arm is the backstop rather than the path.
		return fmt.Errorf("%w: %w", ErrNoSheet, err)
	case errors.Is(err, resolution.ErrNilInput), errors.Is(err, resolution.ErrNoMachine):
		return fmt.Errorf("%w: %w", ErrNilInput, err)
	default:
		return err
	}
}

// recordFor turns a strike into the outcome the composition will stamp.
//
// CRITICAL IS NOT RECORDED, and the reason has an expiry date worth writing
// down. ValueRoll is the RAW d20, so today a beat carrying roll:20 already says
// the blow was critical — a reader derives it, and adding a kind for it would
// be vocabulary earning nothing.
//
// That holds ONLY while every reachable attack crits on a natural 20 alone.
// The machine already supports a crit threshold, so an expanded range — a
// Champion's 19–20 — produces a critical hit at roll:19 that this beat cannot
// tell from an ordinary one. THAT is the caller that earns recorded crit
// vocabulary, and when it arrives this comment is where to start.
func recordFor(in *AttackInput, struck resolution.StrikeOutcome) *encounter.RecordInput {
	values := map[encounter.OutcomeValue]int{
		encounter.ValueRoll:    struck.Roll,
		encounter.ValueTotal:   struck.Total,
		encounter.ValueAgainst: struck.TargetAC,
	}
	kind := encounter.OutcomeMissed
	if struck.Hit {
		kind = encounter.OutcomeStruck
		values[encounter.ValueAmount] = struck.Damage
	}
	return &encounter.RecordInput{
		Kind:    kind,
		Actor:   encounter.MemberID(in.Attacker),
		Targets: []encounter.MemberID{encounter.MemberID(in.Target)},
		Values:  values,
	}
}

// compileAttack turns the attacker's stored sheet into the neutral profile the
// strike machine takes.
//
// The sheet is loaded here as well as inside Resolve, and that is not a
// duplicate to be optimised away: character.Load is bus-free, so this
// reconstitution attaches nothing and subscribes nothing. The compiler needs a
// live character to read static facts off; resolution needs its own cast to
// attach effects to. Two purposes, one stored sheet, and no shared bus between
// them.
func (m *Manager) compileAttack(ctx context.Context, attacker string) (resolution.AttackProfile, error) {
	data, err := m.fetchCharacterData(ctx, "attacker", attacker)
	if err != nil {
		return resolution.AttackProfile{}, err
	}

	loaded, err := character.Load(ctx, data)
	if err != nil {
		return resolution.AttackProfile{}, fmt.Errorf("attacker %q: %w: %w", attacker, ErrBadCharacter, err)
	}

	profile, err := resolution.AttackFromCharacter(loaded, &resolution.CharacterAttackInput{
		Slot: character.SlotMainHand,
	})
	if err != nil {
		return resolution.AttackProfile{}, fmt.Errorf("attacker %q: %w: %w", attacker, ErrBadAttack, err)
	}
	return profile, nil
}

// castFor gathers every member of the encounter as a participant.
//
// EVERY member, not the two swinging: scope is the caller's and applicability
// is the effect's own predicate (ADR-0038). A bard three cells away whose
// Bless is running has to be in the room for their subscription to fire, and
// deciding they are irrelevant here would be this package deciding a rule.
// storedNPC finds an NPC's sheet in the session record.
func (m *Manager) storedNPC(scope *writeScope, id string) (*monster.Data, bool) {
	for i := range scope.data.NPCs {
		if scope.data.NPCs[i].ID == id {
			return &scope.data.NPCs[i], true
		}
	}
	return nil, false
}

func (m *Manager) castFor(
	ctx context.Context, scope *writeScope, roster []encounter.Member,
) ([]resolution.Participant, error) {
	npcs := map[string]*monster.Data{}
	for i := range scope.data.NPCs {
		npcs[scope.data.NPCs[i].ID] = &scope.data.NPCs[i]
	}

	cast := make([]resolution.Participant, 0, len(roster))
	for _, member := range roster {
		id := string(member.ID)
		if member.Kind == encounter.MemberKind(KindMonster) {
			sheet, ok := npcs[id]
			if !ok {
				continue // content with no stored sheet contributes nothing
			}
			cast = append(cast, resolution.Participant{Monster: sheet})
			continue
		}

		data, err := m.fetchCharacterData(ctx, "participant", id)
		if err != nil {
			return nil, err
		}
		cast = append(cast, resolution.Participant{Character: data})
	}
	return cast, nil
}

// saveDirty writes back every sheet the interaction changed.
//
// Characters go to the host's repository; NPCs live in the session record, so
// they are folded into it and the record is marked touched. This is the first
// verb in the package that writes a character at all — damage has to persist —
// and the no-clobber pin gained a row for it rather than losing its guard.
func (m *Manager) saveDirty(ctx context.Context, scope *writeScope, out *resolution.Output) error {
	// The report names what LANDED as well as what did not (S6). A sheet
	// written before the failure is durable, and a caller told only about the
	// failure would retry a write that already succeeded — which is the
	// difference between repair and retry that the report exists to carry.
	var written []string
	for _, data := range out.DirtyCharacters {
		if data == nil {
			continue
		}
		if err := m.characters.SaveCharacter(ctx, data); err != nil {
			report := SaveReport{Written: written, Failed: []string{"character:" + data.ID}}
			return &SaveError{Report: report, Err: fmt.Errorf("saving character: %w", err)}
		}
		written = append(written, "character:"+data.ID)
	}

	for _, dirty := range out.DirtyMonsters {
		if dirty == nil {
			continue
		}
		for i := range scope.data.NPCs {
			if scope.data.NPCs[i].ID == dirty.ID {
				scope.data.NPCs[i] = *dirty
				scope.touched = true
				break
			}
		}
	}
	return nil
}
