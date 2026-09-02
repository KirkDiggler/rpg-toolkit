// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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

	// Target is who they swing at. Required, and must be an available member
	// candidate on the selected current declaration.
	Target string

	// DeclarationID is the opaque current Attack selector returned by Afford.
	// Required. The client echoes it and never parses it.
	DeclarationID string
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

	// Attack is what was swung — ref, name and damage type. The beat line's
	// "6 slashing" and "with a longsword" come from here; the numbers above
	// crossed the seam from the first swing and the weapon that produced
	// them did not, until now (rpg-toolkit#866).
	Attack AttackRef `json:"attack"`
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
// Afford normally compiles the main-hand swing and, after a qualifying Attack
// action, also compiles its granted bonus attack: Martial Arts' Unarmed Strike
// or the other weapon from two-weapon fighting. The selector chooses the
// complete authored definition and price; no hand flag or weapon choice crosses
// this seam. Two-handed and ranged semantics likewise come from the selected
// definition.
//
// # A swing costs something, in a fight
//
// A character in a fight pays for the swing before it is resolved: the first in
// a turn takes the Attack action, and what that action banks is what the swings
// after it spend. A level-1 fighter therefore gets one swing per turn and a
// level-5 fighter gets two. Afford reports the exhausted offer with its exact
// NoBudget shortfall; attempting to execute that now-unavailable selector is
// [ErrStaleDeclaration] before resolution. [ErrCannotAfford] remains the
// defensive payment-door translation if state changes beneath that final gate.
//
// The price is compiled into the selected definition before its selector ID is
// generated, then the same definition and a cloned matching resolution cost are
// reused here. Resolution charges after pure machine preflight and before its
// first executable step — so a refused swing rolls nothing, damages nobody,
// and writes nothing at all. See [Manager.priceSwing] and
// [character.CostOfSwing].
//
// ATTACK HAS NO WORLD-CLOCK OFFER. Afford returns no declarations in free roam,
// so there is no valid selector to echo there; Move alone retains its explicit
// empty-selector world-clock form.
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
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoDeclarationID,
// ErrNoSession, ErrNoEncounter, ErrNoMember, ErrNotACharacter, ErrNoSheet,
// ErrNoCharacter, ErrBadCharacter, ErrBadRepository, ErrBadAttack,
// ErrStaleDeclaration, ErrCannotAfford, ErrBadCost, ErrClosed, or ErrSaveFailed
// with a populated report.
//
// Participant dependency failures normally surface before this verb through
// Afford: unreadable targets keep candidate rows with ShortfallUnreadable, an
// unreadable non-target cast member disables the declaration globally, and an
// unreadable actor/Attack emits an early blocker with no selector. Echoing an
// unavailable compiled selector is ErrStaleDeclaration; resolution receives
// the exact raw cast compilation already preflighted and performs no repository
// refetch after selection.
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
	if kinds[in.Attacker] != encounter.MemberKind(KindPlayer) {
		return nil, fmt.Errorf("attack: attacker %q: %w", in.Attacker, ErrNotACharacter)
	}

	// NOT YOUR TURN, checked FIRST among the fact-about-this-member
	// refusals and before anything touches character storage — the same
	// precedence Move's own gate keeps (Copilot's finding on #1171,
	// repeated by Copilot here on #1174: this compiled and loaded a sheet
	// via compileAttack before checking whose turn it was). Free roam asks
	// nothing here — there is no active member to compare against — which
	// is the same clock read priceSwing makes below for a different
	// question.
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Attacker)})
	if err != nil {
		return nil, fmt.Errorf("attack: %w", translate(err))
	}
	if ClockKind(clock.Kind) == ClockTurn && string(clock.Active) != in.Attacker {
		return nil, fmt.Errorf("attack: attacker %q: %w", in.Attacker, ErrNotYourTurn)
	}

	// Attack has no world-clock declaration. Keep its independent standing
	// gate for historical refusal precedence, then reject every selector: there
	// is no world-clock offer to select.
	if ClockKind(clock.Kind) != ClockTurn {
		if err := refuseIfDown(scope, "attacker", in.Attacker); err != nil {
			return nil, fmt.Errorf("attack: %w", err)
		}
		if in.DeclarationID == "" {
			return nil, fmt.Errorf("attack: %w", ErrNoDeclarationID)
		}
		return nil, fmt.Errorf("attack: %w", ErrStaleDeclaration)
	}

	// The turn path loads the actor strictly ONCE. The downed verdict and every
	// piece of the regenerated offer are derived from this same snapshot; a
	// repository cannot answer standing to one gate and downed to compilation.
	actor := m.loadActorSheet(ctx, in.Attacker)
	if actor.downed {
		return nil, fmt.Errorf("attack: attacker %q: %w", in.Attacker, ErrDowned)
	}
	if in.DeclarationID == "" {
		return nil, fmt.Errorf("attack: %w", ErrNoDeclarationID)
	}

	// Regenerate under this verb's already-loaded write scope and select the
	// exact current offer. compileOffersFor owns assembly, pricing, selector
	// identity, and target preflight; execution reuses those compiled values
	// instead of independently compiling a second attack after selection.
	offers, err := m.compileOffersFor(
		ctx, scope.enc, scope.data, scope.session, in.Attacker, clock, actor, VerbAttack,
	)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}
	selected, err := selectCompiledOffer(offers, VerbAttack, in.DeclarationID)
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}
	candidate, ok := selected.targets[in.Target]
	if !ok || !candidate.available {
		return nil, fmt.Errorf("attack: target %q: %w", in.Target, ErrStaleDeclaration)
	}
	if selected.attack == nil || selected.price == nil || selected.sheet == nil || len(selected.cast) == 0 {
		return nil, fmt.Errorf("attack: %w", ErrStaleDeclaration)
	}

	definition := *selected.attack
	price := selected.price
	cost := price.cost

	// The selected offer owns the one exact raw cast snapshot gathered and
	// strictly preflighted during compilation. Resolution reconstitutes those
	// bytes; execution performs no participant repository read after selection.
	cast := selected.cast
	machine, err := resolution.NewAction(&resolution.ActionInput{
		Definition: definition,
		AttackerID: in.Attacker,
		TargetID:   in.Target,
		Roller:     &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("attack: %w: %v", ErrBadAttack, err)
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	world := scope.enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        world,
		Participants: cast,
		Initiative:   m.initiative,
		Standing:     scope.standing,
		Sight:        &sightSeam{members: append([]encounter.MemberData(nil), world.Members...)},
		TurnDriver:   m.turnDriver,
		// The concealment pair (rpg-toolkit#1378), bound to the same live
		// scope openForWrite and adopt bind — the one-seam consistency law:
		// a concealed world refuses to reconstruct without them, and
		// resolution carries them without consulting either, since no verb
		// runs inside an interaction.
		CheckResolver: checkSeam{m: m, scope: scope},
		Witness:       witnessSeam{scope: scope},
		Cost:          cost,
		Machine:       machine,
		// The machine rolls the attack and its damage; Input.Roller only
		// reconstitutes effects that need one. Two rollers because they
		// are two jobs — and BOTH must be the host's, or the swing is
		// resolved with randomness the host never supplied. ActionInput's
		// still defaults silently when nil, which is the class resolution
		// just closed on its own Input (#1033); leaving it unset here is
		// how this verb first resolved with unreproducible dice.
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		translated := translateResolution(err)
		if errors.Is(translated, ErrOutOfReach) {
			return nil, fmt.Errorf("attack: target %q: %w", in.Target, translated)
		}
		return nil, fmt.Errorf("attack: %w", translated)
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
	recorded, err := scope.enc.Record(recordFor(in, struck, definition))
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
		Seq:      scope.deliveredSeq(in.Attacker, recorded.Seq),
		Saved:    report,
		Delivery: delivery,
		Attack:   attackRefFor(definition),
	}, nil
}

// attackRefFor projects a compiled attack profile's identity onto the wire
// shape — ref, name, damage type — carried on AttackOutput, on the
// Struck/Missed beat every witness reads (rpg-toolkit#866), and on the
// compiled Attack declaration's AttackRef (rpg-toolkit#272/273).
//
// The ref is the FULL definition.Ref.String — "dnd5e:weapons:longsword",
// "dnd5e:monster-actions:unarmed-strike" — so the same identity a client
// maps to a model and icon on a beat is the one a compiled offer carries,
// and the one execution regenerates from. The same
// helper serves all three call sites so they cannot drift.
//
// The damage type reported is the FIRST declared pool's, which is every
// weapon this assembler produces today. A weapon
// that ever declares two would need this to say which one the beat line
// means, and that decision belongs beside the day such a weapon compiles,
// not guessed at here.
func attackRefFor(definition combatActions.Definition) AttackRef {
	ref := AttackRef{Ref: definition.Ref.String(), Name: definition.Name}
	if definition.Attack != nil && len(definition.Attack.Damage) > 0 {
		ref.DamageType = DamageType(definition.Attack.Damage[0].Type)
	}
	return ref
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
	case errors.Is(err, resolution.ErrActivationRefused):
		// The activation half of the same split, and the same argument: an
		// ability that said no is a fact about the game, and its own words
		// ("no rage uses remaining") ride along so the refusal is something a
		// client can say out loud.
		return fmt.Errorf("%w: %v", ErrCannotActivate, err)
	case errors.Is(err, resolution.ErrBadActivation):
		return fmt.Errorf("%w: %v", ErrBadActivation, err)
	case errors.Is(err, resolution.ErrOutOfRange):
		return fmt.Errorf("%w: %v", ErrOutOfReach, err)
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
// It copies resolution-owned facts once; replay decodes this record rather
// than reconstructing damage or modifier attribution later.
func recordFor(
	in *AttackInput, struck resolution.StrikeOutcome, definition combatActions.Definition,
) *encounter.RecordInput {
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

	ref := attackRefFor(definition)
	recorded := &encounter.RecordInput{
		Kind:     kind,
		Actor:    encounter.MemberID(in.Attacker),
		Targets:  []encounter.MemberID{encounter.MemberID(in.Target)},
		Values:   values,
		Critical: struck.Critical,
		Attack:   &encounter.AttackIdentity{Ref: ref.Ref, Name: ref.Name, DamageType: string(ref.DamageType)},
	}
	if struck.Hit {
		recorded.DamageComponents = recordDamageComponents(struck.DamageComponents)
		recorded.AdvantageSources = recordAttackModifierSources(struck.Folded.AdvantageSources)
		recorded.DisadvantageSources = recordAttackModifierSources(struck.Folded.DisadvantageSources)
	}
	return recorded
}

func recordDamageComponents(in []dnd5eEvents.DamageComponent) []encounter.DamageComponent {
	if len(in) == 0 {
		return nil
	}
	out := make([]encounter.DamageComponent, 0, len(in))
	for _, component := range in {
		var sourceRef string
		if component.SourceRef != nil {
			sourceRef = component.SourceRef.String()
		}
		var multiplier *float64
		if component.Multiplier != nil {
			value := *component.Multiplier
			multiplier = &value
		}
		out = append(out, encounter.DamageComponent{
			Source: string(component.Source), SourceRef: sourceRef, Dice: component.Dice,
			FinalRolls: append([]int(nil), component.FinalDiceRolls...),
			FlatBonus:  component.FlatBonus, DamageType: string(component.DamageType),
			Multiplier: multiplier,
		})
	}
	return out
}

func recordAttackModifierSources(in []dnd5eEvents.AttackModifierSource) []encounter.AttackModifierSource {
	if len(in) == 0 {
		return nil
	}
	out := make([]encounter.AttackModifierSource, 0, len(in))
	for _, source := range in {
		var sourceRef string
		if source.SourceRef != nil {
			sourceRef = source.SourceRef.String()
		}
		out = append(out, encounter.AttackModifierSource{
			SourceRef: sourceRef,
			SourceID:  source.SourceID,
		})
	}
	return out
}

// loadAttackSheet reconstitutes the attacker's stored sheet for assembly and pricing.
//
// The sheet is loaded here as well as inside Resolve, and that is not a
// duplicate to be optimised away: character.Load is bus-free, so this
// reconstitution attaches nothing and subscribes nothing. The assembler needs a
// live character to read static facts off; resolution needs its own cast to
// attach effects to. Two purposes, one stored sheet, and no shared bus between
// them.
//
// THE LOADED SHEET COMES BACK OUT because the economy needs the same one. Its
// turn is readied on this instance and its price compiled from what that leaves
// ([Manager.priceSwing]), and loading a third copy to do it would ready a turn
// on a sheet nobody hands over.
//
// Load errors keep their inner reason as text so the host sees only this seam's
// sentinel vocabulary.
func (m *Manager) loadAttackSheet(ctx context.Context, attacker string) (*character.Character, error) {
	data, err := m.fetchCharacterData(ctx, "attacker", attacker)
	if err != nil {
		return nil, err
	}

	loaded, err := character.Load(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("attacker %q: %w: %v", attacker, ErrBadCharacter, err)
	}
	return loaded, nil
}

type resolutionDependencyFailure struct {
	member string
	err    error
}

// compileResolutionCast gathers one raw data snapshot for every roster member
// and strictly preflights each available sheet through the same public pure
// loaders and attach APIs resolution uses. It returns all dependency failures
// so offer compilation can preserve every candidate row while applying
// candidate-specific and global Unreadable gates.
func (m *Manager) compileResolutionCast(
	ctx context.Context,
	data *SessionData,
	roster []encounter.Member,
	readied *character.Data,
) ([]resolution.Participant, []resolutionDependencyFailure) {
	npcs := make(map[string]*monster.Data, len(data.NPCs))
	for i := range data.NPCs {
		npcs[data.NPCs[i].ID] = &data.NPCs[i]
	}

	cast := make([]resolution.Participant, 0, len(roster))
	failures := make([]resolutionDependencyFailure, 0)
	for _, member := range roster {
		id := string(member.ID)
		if member.Kind == encounter.MemberKind(KindMonster) {
			sheet, ok := npcs[id]
			if !ok {
				failures = append(failures, resolutionDependencyFailure{member: id, err: ErrNoSheet})
				continue
			}
			cast = append(cast, resolution.Participant{Monster: sheet})
			continue
		}

		if readied != nil && readied.ID == id {
			cast = append(cast, resolution.Participant{Character: readied})
			continue
		}

		sheet, err := m.fetchCharacterData(ctx, "participant", id)
		if err != nil {
			failures = append(failures, resolutionDependencyFailure{member: id, err: err})
			continue
		}
		if sheet.ID != id {
			failures = append(failures, resolutionDependencyFailure{
				member: id,
				err:    fmt.Errorf("GetCharacter(%q) returned character %q: %w", id, sheet.ID, ErrBadRepository),
			})
			continue
		}
		cast = append(cast, resolution.Participant{Character: sheet})
	}

	// THE ATTACH HALF IS RESOLUTION'S. This function used to reconstitute every
	// participant here, on an ephemeral bus, to find out which of them an
	// interaction would refuse — which meant this package held a bus and did
	// the one thing a bus is for.
	//
	// What stays is the FETCH: gathering the stored record behind each member,
	// which is what this seam is for and where its own vocabulary lives
	// (ErrNoSheet, ErrBadRepository). What goes is the reconstituting. The
	// answer comes back as a row per refused member, which is what the offer
	// menu above needs — see [resolution.Preflight] for why it collects rather
	// than stopping at the first.
	preflight, err := resolution.Preflight(ctx, &resolution.PreflightInput{
		Participants: cast,
		Roller:       &diceSeam{roller: m.dice},
	})
	if err != nil {
		// The entry refused the question itself rather than answering it about
		// a participant — a malformed cast, which is this package's own bug
		// and not a member's. Reported against no member, because naming one
		// would be a guess.
		failures = append(failures, resolutionDependencyFailure{member: "", err: err})

		return cast, failures
	}

	for _, refusal := range preflight.Unreadable {
		failures = append(failures, resolutionDependencyFailure{member: refusal.Member, err: refusal.Reason})
	}

	return cast, failures
}

// castFor gathers every member for a monster-driven strike. Player Attack
// offers use compileResolutionCast instead so Afford can validate and retain
// the exact raw snapshot selected execution consumes.
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

		// Keep the newer save — only normalize the report identity. A
		// first-admission Join may already have saved this same character's
		// rest before placement drove an attack. saveWalker enforces the same
		// one-entry-per-aggregate rule for repeated movement-side writes.
		aggregate := "character:" + data.ID
		alreadyWritten := false
		for _, written := range scope.written {
			if written == aggregate {
				alreadyWritten = true
				break
			}
		}
		if !alreadyWritten {
			scope.written = append(scope.written, aggregate)
		}
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
