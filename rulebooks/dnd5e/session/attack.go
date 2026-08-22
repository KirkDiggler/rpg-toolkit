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
// Main hand, one-handed, melee. Two-handed grips and the off-hand swing are the
// compiler's own named gaps, and they did NOT arrive with the economy the way
// this paragraph used to predict: the economy decides how many swings a turn
// buys, and what a two-handed grip does to one is a question for whoever
// compiles the attack rather than for whoever prices it.
//
// # A swing costs something, in a fight
//
// A character in a fight pays for the swing before it is resolved: the first in
// a turn takes the Attack action, and what that action banks is what the swings
// after it spend. A level-1 fighter therefore gets one swing per turn and a
// level-5 fighter gets two, and the swing after that is refused with
// [ErrCannotAfford] naming the currency that ran out.
//
// The price is compiled here and charged by resolution's door, before the
// machine starts — so a refused swing resolves nothing, damages nobody, and
// writes nothing at all. See [Manager.priceSwing] for what a turn costs and
// [costOfSwing] for how one price is made out of the rulebook's two.
//
// IN FREE ROAM IT COSTS NOTHING, which is a ruling rather than the old gap: the
// action economy is a fight's economy, and a member on the world clock has no
// turn to spend a turn's slots from. TestFreeRoamChargesNothing pins it.
//
// # How it runs, and why the order is not a style choice
//
// The world goes into resolution as data and a different world comes back, so
// the scope adopts the returned one before anything else touches it
// ([Manager.adopt] carries that invariant). Every dirty sheet is then written
// back, and only THEN is the outcome recorded on the world the interaction
// produced — a consequence landing after its cause.
//
// THE SHEETS GO FIRST, and that is the whole of this seam's half of
// rpg-toolkit#1083. The composition's Record now consults who is standing, which
// it does by asking [standingSeam] — and that seam answers out of the two stores
// this verb writes: the session record for NPCs, the host's repository for
// characters. Neither is current until [Manager.saveDirty] has run.
// [resolution] does not mutate what it is handed (its dirtyMonsters builds a
// fresh sheet), so recording first would ask the world about PRE-SWING hit
// points and the killing blow would be invisible to its own beat — which is the
// exact defect #1083 exists to close, reproduced one layer up.
//
// The cost is stated rather than hidden: a Record that fails now fails with the
// damage already durable. That is rpg-toolkit#1056's shape, so it is answered
// the way #1056 was — the refusal carries a [SaveError] naming the sheets that
// landed and the world that did not, and TestASwingThatCannotRecordStillNamesTheSheetItWrote
// is what keeps that true. A caller told only "it failed" would retry a swing
// whose damage is on disk.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, ErrNoMember, ErrNotACharacter, ErrNoSheet, ErrNoCharacter,
// ErrBadCharacter, ErrBadRepository, ErrBadAttack, ErrCannotAfford, ErrBadCost,
// ErrClosed, or ErrSaveFailed with a populated report.
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
		return nil, fmt.Errorf("attack: %w", translate(err))
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

	// A downed member does not swing. Asked AFTER the roster checks, so naming somebody
	// who is not here is still ErrNoMember — being down is a fact about a
	// member, and it means nothing about an ID that is not one. Asked about the
	// ATTACKER alone: a down target is refused nowhere, deliberately (see
	// refuseIfDown).
	if err := refuseIfDown(scope, "attacker", in.Attacker); err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	// A member with no stored sheet cannot be swung at: there is nothing to
	// read an armour class off and nothing for damage to land on. Authored
	// content placed straight into a world has no sheet until something spawns
	// it, so this is reachable and worth naming here — the alternative is the
	// strike failing later, further from the cause.
	if kinds[in.Target] == encounter.MemberKind(KindMonster) {
		if _, ok := npcSheet(scope.data, in.Target); !ok {
			return nil, fmt.Errorf("attack: target %q: %w", in.Target, ErrNoSheet)
		}
	}

	sheet, profile, err := m.compileAttack(ctx, in.Attacker)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	// THE ONE PLACE A SPEND GOES, and it is the place its own comment predicted.
	// The economy turned out not to need a machine above this call: the ruling
	// (docs/ideas/session-sdk/economy-gate.md) put the price on Input as DATA and
	// the debit at resolution's door, so what belongs here is compiling what this
	// actor's swing costs — which is a question about a sheet, and this is where
	// the sheets are. Nothing persistent or wire-shaped assumed attacks were
	// free, and nothing had to migrate.
	price, err := m.priceSwing(ctx, scope.enc, in.Attacker, sheet)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	// The readied sheet goes into the cast rather than the stored one: it is the
	// sheet whose turn was just lit or refilled, and the door is about to charge
	// exactly that bank. See swingPrice for why the two travel together.
	cast, err := m.castFor(ctx, scope, roster, price.payer)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        scope.enc.ToData(),
		Participants: cast,
		Initiative:   m.initiative,
		Standing:     scope.standing,
		Sight:        sightSeam{},
		TurnDriver:   m.turnDriver,
		Cost:         price.cost,
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

	if err := m.saveDirty(ctx, scope, out); err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	// And now the beat, on a world whose sheets say what the swing did — see the
	// godoc for why this is not the other way round.
	recorded, err := scope.enc.Record(recordFor(in, struck))
	if err != nil {
		return nil, fmt.Errorf("attack: %w", reportUnrecorded(scope, translate(err)))
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

// reportUnrecorded turns a failure AFTER the sheets landed into one a caller can
// repair from, and leaves every other failure exactly as it was.
//
// This is rpg-toolkit#1056's rule reaching one more call site. Recording is the
// last fallible step before the commit, and since the sheets are now written
// ahead of it (see [Manager.Attack]), a bare error would say "nothing happened"
// about a swing whose damage is durable. The host reads that as safe to retry,
// retries, and applies the damage twice.
//
// The world is named as FAILED because that is what a caller has to act on: the
// sheets describe a blow the persisted world has no beat for. It was never
// attempted rather than attempted and refused, and the repair is the same either
// way — which is why it is reported the same way persist reports its own
// world-save failure rather than given a vocabulary of its own.
//
// Nothing written means nothing to report, and the plain error is the better
// answer: a report of an empty ledger is noise, and wrapping would hand a host
// ErrSaveFailed for a verb that saved nothing.
func reportUnrecorded(scope *writeScope, err error) error {
	if len(scope.written) == 0 {
		return err
	}

	return &SaveError{
		Report: SaveReport{
			Written: append([]string(nil), scope.written...),
			Failed:  []string{"encounter:" + scope.encounter},
		},
		Err: err,
	}
}

// translateResolution maps the resolution module's sentinels onto this
// package's own.
//
// The same reason translate exists for the composition's: a sentinel is not a
// type in a signature, so the boundary test cannot see it, and a host that
// matched on resolution.ErrBadParticipant would be coupled to a module we
// intend to keep replaceable.
//
// A translated error carries our sentinel ALONE, and the inner reason rides
// along as TEXT. Every arm below used to wrap both — fmt.Errorf("%w: %w", ours,
// theirs) — which reads like generosity and is the leak itself: it satisfies a
// host matching on ours while leaving theirs just as matchable
// (rpg-toolkit#1066). The %v keeps the account for whoever debugs it and hands
// the host nothing to branch on but this package's vocabulary (S2).
//
// Unrecognised errors pass through UNCHANGED rather than being flattened, for
// the reason translate's default arm does: it carries errors that ORIGINATED
// WITH THE HOST — a failing Roller reaches the strike machine and comes back
// out through here — and flattening those to protect the host from us would
// break its matching on its own errors. The guarantee is instead that every
// resolution sentinel this seam can REACH has an arm, and that guarantee is
// mechanical: sentinels_test.go drives the refusals a caller can produce, and
// translate_internal_test.go covers every arm below.
func translateResolution(err error) error {
	switch {
	case errors.Is(err, resolution.ErrCannotPay):
		// The PLAYER-FACING one, and the reason it is not folded in with the two
		// below. An actor who spent what they had is a fact about the game, and
		// the gate's own refusal rides along as text so the message still names
		// the currency that ran out — "action: 1 needed, 0 left" is what turns a
		// refusal into something a client can say out loud.
		return fmt.Errorf("%w: %v", ErrCannotAfford, err)
	case errors.Is(err, resolution.ErrBadCost), errors.Is(err, resolution.ErrNoPayer):
		// And the PROGRAMMER-FACING one. E2 split these deliberately and this
		// seam keeps the split: a profile keyed to a currency no ledger holds, or
		// a cost naming somebody who cannot be charged, is wiring that is wrong.
		// Reporting it as "out of actions" would send whoever debugs it to a
		// player's sheet to look for a bug that is in the code.
		return fmt.Errorf("%w: %v", ErrBadCost, err)
	case errors.Is(err, resolution.ErrBadParticipant):
		return fmt.Errorf("%w: %v", ErrBadCharacter, err)
	case errors.Is(err, resolution.ErrNoCombatant):
		// Reachable when a member has no stored sheet — an authored monster
		// standing in a world nobody spawned. Refused earlier by name, so this
		// arm is the backstop rather than the path.
		return fmt.Errorf("%w: %v", ErrNoSheet, err)
	case errors.Is(err, resolution.ErrNilInput), errors.Is(err, resolution.ErrNoMachine):
		return fmt.Errorf("%w: %v", ErrNilInput, err)
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
// strike machine takes, and hands back the sheet it read it off.
//
// The sheet is loaded here as well as inside Resolve, and that is not a
// duplicate to be optimised away: character.Load is bus-free, so this
// reconstitution attaches nothing and subscribes nothing. The compiler needs a
// live character to read static facts off; resolution needs its own cast to
// attach effects to. Two purposes, one stored sheet, and no shared bus between
// them.
//
// THE LOADED SHEET COMES BACK OUT because the economy needs the same one. Its
// turn is readied on this instance and its price compiled from what that leaves
// ([Manager.priceSwing]), and loading a third copy to do it would ready a turn
// on a sheet nobody hands over.
//
// Both refusals below keep the inner reason as TEXT rather than as a chain, the
// way translateResolution's arms do. The second is the one that made it worth
// saying: an empty main hand is refused by resolution, so its ErrBadAttack was
// riding out under ours and a host could match on it (rpg-toolkit#1066). This
// is not routed THROUGH translateResolution, deliberately — the compiler
// answers everything the profile step refuses with ErrBadAttack, including the
// resolution.ErrNilInput cases, and routing it would silently re-map those onto
// a different sentinel than the one hosts have been given.
func (m *Manager) compileAttack(
	ctx context.Context, attacker string,
) (*character.Character, resolution.AttackProfile, error) {
	data, err := m.fetchCharacterData(ctx, "attacker", attacker)
	if err != nil {
		return nil, resolution.AttackProfile{}, err
	}

	loaded, err := character.Load(ctx, data)
	if err != nil {
		return nil, resolution.AttackProfile{},
			fmt.Errorf("attacker %q: %w: %v", attacker, ErrBadCharacter, err)
	}

	profile, err := resolution.AttackFromCharacter(loaded, &resolution.CharacterAttackInput{
		Slot: character.SlotMainHand,
	})
	if err != nil {
		return nil, resolution.AttackProfile{},
			fmt.Errorf("attacker %q: %w: %v", attacker, ErrBadAttack, err)
	}
	return loaded, profile, nil
}

// castFor gathers every member of the encounter as a participant.
//
// EVERY member, not the two swinging: scope is the caller's and applicability
// is the effect's own predicate (ADR-0038). A bard three cells away whose
// Bless is running has to be in the room for their subscription to fire, and
// deciding they are irrelevant here would be this package deciding a rule.
//
// One member may arrive already in hand. The attacker's sheet has had its turn
// readied by the time the cast is built (see [Manager.priceSwing]), and that
// readying is state the stored copy does not have — so the readied sheet is
// substituted rather than re-fetched. Passing nil means nobody was readied,
// which is what free roam looks like.
func (m *Manager) castFor(
	ctx context.Context, scope *writeScope, roster []encounter.Member, readied *character.Data,
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

		if readied != nil && readied.ID == id {
			cast = append(cast, resolution.Participant{Character: readied})
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
//
// IT RUNS BEFORE THE OUTCOME IS RECORDED, and that is a correctness ordering
// rather than a convenience: the composition's Record consults who is standing,
// [standingSeam] answers out of exactly these two stores, and a consult run
// against sheets this verb has not written back yet is a consult about a world
// that no longer exists. See [Manager.Attack].
func (m *Manager) saveDirty(ctx context.Context, scope *writeScope, out *resolution.Output) error {
	// The report names what LANDED as well as what did not (S6). A sheet
	// written before the failure is durable, and a caller told only about the
	// failure would retry a write that already succeeded — which is the
	// difference between repair and retry that the report exists to carry.
	//
	// Every entry goes on the SCOPE rather than a local, so it outlives this
	// call: the sheets are durable whether the next write succeeds or fails,
	// and persist opens its report with them either way. Kept in a local, they
	// were reported only when saveDirty itself failed — so a swing whose WORLD
	// save failed named nothing at all, and the host retried a swing whose
	// damage was already on disk (rpg-toolkit#1056).
	for _, data := range out.DirtyCharacters {
		if data == nil {
			continue
		}
		if err := m.characters.SaveCharacter(ctx, data); err != nil {
			report := SaveReport{Written: scope.written, Failed: []string{"character:" + data.ID}}
			return &SaveError{Report: report, Err: fmt.Errorf("saving character: %w", err)}
		}
		scope.written = append(scope.written, "character:"+data.ID)
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
