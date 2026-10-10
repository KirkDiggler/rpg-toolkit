// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
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

	// Warded reports that a Sanctuary-style ward on the target stopped this
	// attack before any roll — Roll, Total, Against, Hit, Critical and
	// Damage all stay zero/false, the same "these are not answers" shape
	// Paused documents below. WardedBy names the caster whose ward it was.
	Warded   bool   `json:"warded,omitempty"`
	WardedBy string `json:"warded_by,omitempty"`

	// Paused reports that the swing STOPPED to ask the attacker something and
	// has not landed yet. When it is true, Hit, Critical, Damage and Against
	// are not answers — Against is zero because the AC has deliberately not
	// been shown, and Hit is false because nothing has hit yet.
	//
	// Roll and Total are the two numbers that ARE true: the d20 as rolled and
	// the total before whatever is being offered joins it. Answering the
	// window with [Manager.React] finishes the attack and writes the beat
	// this call did not.
	Paused bool `json:"paused,omitempty"`

	// Seq is the story sequence of the recorded beat.
	Seq uint64 `json:"seq"`

	// FollowUpSeqs are the story sequences of the beats this swing's
	// consequences produced, in the order they were appended: for each
	// concentration it broke, the check that was failed, the break itself, and
	// one condition-removed per address the ending spell was holding. Empty on
	// a swing that broke nobody's concentration.
	//
	// SEPARATE FROM Seq rather than folded into it, mirroring the
	// composition's own split: Seq answers the question the caller asked —
	// where the blow I reported landed — and an attack whose own sequence
	// moved depending on how many spells it happened to end would answer a
	// different one.
	//
	// Recipient-local like Seq, and renumbered through the same seam: these
	// are positions in THIS member's stream, so a caller may compare them with
	// Seq and with the sequences on its own events, and with nobody else's.
	FollowUpSeqs []uint64 `json:"follow_up_seqs,omitempty"`

	// Saved names what was persisted.
	Saved SaveReport `json:"saved"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`

	// Attack is what was swung — ref, name and damage type. The beat line's
	// "6 slashing" and "with a longsword" come from here; the numbers above
	// crossed the seam from the first swing and the weapon that produced
	// them did not, until now (rpg-toolkit#866).
	Attack AttackRef `json:"attack"`

	// Calculation is the authoritative sourced attack-roll arithmetic.
	Calculation *RollCalculation `json:"calculation,omitempty"`

	// PresentationID is the opaque token the attacker and every witness share
	// for THIS one roll. The client echoes it and never parses it.
	//
	// It is what lets a table watch one die. The player who rolls simulates
	// the d20 falling through the room and publishes that throw; everybody
	// else replays it locally, and the only way a witness can tell which roll
	// a throw belongs to is a token minted once and copied onto every copy of
	// the beat. The story sequence cannot answer that question — a recipient's
	// sequence is recipient-local (rpg-toolkit#1377), so the attacker's number
	// and a witness's number for one swing are different numbers.
	//
	// Always present on an accepted swing: the same token reaches
	// [StruckBody.PresentationID] and [MissedBody.PresentationID], and a
	// generator that cannot produce a usable one refuses the command rather
	// than recording a roll nobody can correlate.
	PresentationID string `json:"presentation_id"`
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
// ErrNotATarget, ErrStaleDeclaration, ErrCannotAfford, ErrBadCost, ErrClosed,
// or ErrSaveFailed with a populated report.
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
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Attacker == "" || in.Target == "" {
		return nil, fmt.Errorf("attack: %w", ErrNoMemberID)
	}

	scope, err := m.openForChange(ctx, in.Session)
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

	// AN NPC IS NOT A TARGET (rpg-project#493, R4), and the seam says so in
	// its own words rather than letting the candidate gate answer.
	//
	// The composition refuses this too, and translate carries its sentinel —
	// but nothing reaches it: buildTargetPreflight's universe already drops
	// every KindWorld member, so a host forwarding a swing at a merchant used
	// to be told ErrStaleDeclaration. That is a LIE about a permanent fact.
	// Stale means re-read the offers; re-reading produces the same empty
	// candidate list forever, and the only thing that would change the answer
	// is authoring the creature as a monster with a disposition.
	//
	// BEFORE THE TURN GATE BELOW, because nothing about the clock is part of
	// this answer. Whose turn it is cannot make a merchant attackable, and a
	// host told "not your turn" would wait for a turn that changes nothing.
	// It is also before any sheet is loaded and any world is written, which
	// is what "refused at the verb, fail closed" means here.
	if kinds[in.Target] == encounter.KindWorld {
		return nil, fmt.Errorf(
			"attack: target %q is an npc and cannot be attacked; author it as a monster to make it a target: %w",
			in.Target, ErrNotATarget)
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
	if errors.Is(actor.err, ErrNoCharacter) {
		// THE STORE'S ONE ANSWER (rpg-project#542): an attacker the store does
		// not hold is refused by name, not as a stale offer — re-reading
		// Afford would answer the same empty sheet forever.
		return nil, fmt.Errorf("attack: %w", actor.err)
	}
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

	// The token this roll will be known by, minted BEFORE the dice for the
	// reason the payment door is charged before them: a host whose generator
	// cannot produce a usable one has a defect, and a refusal that arrives
	// after the swing resolved would have already damaged somebody. Same
	// discipline, same place in the order, as [Manager.DeathSave]'s own.
	presentationID := m.presentationIDs.Generate()
	if err := validatePresentationID(presentationID); err != nil {
		return nil, fmt.Errorf("attack: generated presentation id: %w", err)
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
	// storage boundary (encounter v0.43.0, #1385). The machine rolls the attack
	// and its damage; the input's Roller only reconstitutes effects that need
	// one. Two rollers because they are two jobs — and BOTH are the host's
	// (resolutionInput installs this session's dice), or the swing is resolved
	// with randomness the host never supplied.
	world := scope.enc.WorldView()
	out, err := resolution.Resolve(ctx, m.resolutionInput(ctx, scope, resolutionAsk{
		World:        world,
		Participants: cast,
		Machine:      machine,
		Cost:         cost,
	}))
	if err != nil {
		translated := translateAttack(err)
		if errors.Is(translated, ErrOutOfReach) {
			return nil, fmt.Errorf("attack: target %q: %w", in.Target, translated)
		}
		return nil, fmt.Errorf("attack: %w", translated)
	}

	// ONE LANDING, PAUSED OR FINISHED. What settled is told — a swing that
	// hit and then stopped to ask is told now, with its concentration and
	// its areas — and the pause, when there is one, is posed. A swing that
	// stopped before its d20 settled has told nothing: there is no outcome
	// yet, and a beat saying otherwise would be the story getting ahead of
	// the fight. The cost was charged at the door either way.
	story := windowStory{
		Kind: storyAttack, Attacker: in.Attacker, Target: in.Target,
		Definition: definition, PresentationID: presentationID,
	}
	if out.Posed != nil && out.Posed.Kind == resolution.PausePostRoll && out.Posed.Ask.Audience != in.Attacker {
		// R5, checked on this side of the seam too: an offer on a d20 is
		// posed to its roller and to nobody else.
		return nil, fmt.Errorf("attack: %w: the machine asked %q on %q's roll",
			ErrInvalidWorld, out.Posed.Ask.Audience, in.Attacker)
	}
	var got attackLanded
	result, err := m.land(ctx, scope, out, m.attackLanding(scope, nil, story, out, &got))
	if err != nil {
		return nil, fmt.Errorf("attack: %w", err)
	}

	if out.Posed != nil {
		// The numbers so far, and Paused to say they are not final. The
		// attacker is the one member who learns this synchronously;
		// everybody else reads the beat.
		paused := &AttackOutput{
			Paused: true, Saved: result.Saved, Delivery: result.Delivery,
			Attack: attackRefFor(definition), PresentationID: presentationID,
		}
		switch {
		case got.hit != nil:
			struck, _ := out.Outcome.(resolution.StrikeOutcome)
			paused.Roll, paused.Total = struck.Roll, struck.Total
			paused.Seq = scope.deliveredSeq(in.Attacker, got.hit.Seq)
		case got.window != nil:
			paused.Roll, paused.Total = out.Posed.Ask.Roll, out.Posed.Ask.Total
			paused.Seq = scope.deliveredSeq(in.Attacker, got.window.Seq)
		}
		return paused, nil
	}

	struck, ok := out.Outcome.(resolution.StrikeOutcome)
	if !ok || got.hit == nil {
		return nil, fmt.Errorf("attack: %w: strike produced %T", ErrInvalidWorld, out.Outcome)
	}
	recorded := got.hit

	var wardedBy string
	if struck.Warded != nil {
		wardedBy = struck.Warded.SourceID
	}
	return &AttackOutput{
		Roll:         struck.Roll,
		Total:        struck.Total,
		Against:      struck.TargetAC,
		Hit:          struck.Hit,
		Critical:     struck.Critical,
		Damage:       struck.Damage,
		Warded:       struck.Warded != nil,
		WardedBy:     wardedBy,
		Seq:          scope.deliveredSeq(in.Attacker, recorded.Seq),
		FollowUpSeqs: deliveredSeqs(scope, in.Attacker, recorded.FollowUpSeqs),
		Saved:        result.Saved,
		Delivery:     result.Delivery,
		Attack:       attackRefFor(definition),
		Calculation:  sessionRollCalculationFor(rollCalculationFor(struck.Calculation)),

		PresentationID: presentationID,
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

// sheetRefusal reports which of this package's own sheet sentinels err
// carries, or nil: the refusals the sheet seam (sheets.go) makes about a member
// it holds no readable sheet for — absent or corrupt character, absent stat
// block, a store that answered wrongly, a stat block that will not load or a
// member with no roster kind (ErrInvalidSession), and a main hand the
// projection cannot compile (ErrBadAttack).
func sheetRefusal(err error) error {
	for _, own := range []error{
		ErrNoCharacter, ErrBadCharacter, ErrNoSheet, ErrBadRepository, ErrInvalidSession, ErrBadAttack,
	} {
		if errors.Is(err, own) {
			return own
		}
	}
	return nil
}

// badCostUnlessSheet is how Afford reports a world read that failed while it
// was pricing: [ErrBadCost], unless the failure was a sheet the sheet seam
// could not read — then that refusal's own sentinel, for [sheetRefusal]'s
// reason (the repair is the sheet's, not the cost's).
func badCostUnlessSheet(err error) error {
	if own := sheetRefusal(err); own != nil {
		return fmt.Errorf("%w: %v", own, err)
	}
	return fmt.Errorf("%w: %v", ErrBadCost, err)
}

// translateAttack is [translateResolution] for a swing: resolution's one
// out-of-range refusal there is the delivery's, a target the weapon or the
// spell attack cannot reach, and that stays the attack's own refusal
// (ErrOutOfReach) rather than the out-of-range every other verb reports.
// Resolution names one sentinel for every such refusal and leaves the word to
// the host; this is the host choosing it by verb.
func translateAttack(err error) error {
	if sheetRefusal(err) == nil && errors.Is(err, resolution.ErrOutOfRange) {
		return fmt.Errorf("%w: %v", ErrOutOfReach, err)
	}
	return translateResolution(err)
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
	// A SHEET THIS PACKAGE COULD NOT READ IS THE CAUSE, whatever resolution
	// was doing when it asked. The sheet seam (sheets.go) is consulted from
	// inside a resolution — a cost's witnesses ask every member's sight — and
	// resolution wraps the seam's refusal in its own sentinel for what it was
	// doing at the time. The host's repair is the sheet's, so the seam's own
	// word wins, carried alone with the account as text (S2).
	if own := sheetRefusal(err); own != nil {
		return fmt.Errorf("%w: %v", own, err)
	}
	switch {
	case errors.Is(err, character.ErrArmorInFight):
		// Before ErrBadEquip: the door wraps the armour refusal in its own
		// account, and the host's answer is "not in a fight", not "a bad
		// request".
		return fmt.Errorf("%w: %v", ErrArmorInFight, err)
	case errors.Is(err, resolution.ErrBadEquip):
		return fmt.Errorf("%w: %v", ErrBadEquip, err)
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
		// A target beyond what the action reaches is out of range on every
		// verb (rpg-project#539): a cast's or a heal's range, a known creature
		// with no believed point in range on a clear line. An attack's
		// delivery is the one exception and keeps its own word; see
		// [translateAttack].
		return fmt.Errorf("%w: %v", ErrOutOfRange, err)
	case errors.Is(err, resolution.ErrBadParticipant):
		return fmt.Errorf("%w: %v", ErrBadCharacter, err)
	case errors.Is(err, resolution.ErrWardUnreadable):
		// The ward carries no DC. A Sanctuary records its caster's spell save
		// DC when it is cast and is read from the ward alone, so the caster
		// leaving changes nothing (rpg-toolkit#1965); a ward with no DC is one
		// written before it kept one. It used to read as DC 0 and let every
		// attempt through, and now refuses. That is bad stored data on the
		// holder's sheet, so it is this package's word for a sheet it cannot
		// use, and the inner reason rides along as text.
		return fmt.Errorf("%w: %v", ErrBadCharacter, err)
	case errors.Is(err, resolution.ErrNoCombatant):
		// Reachable when a member has no stored sheet — an authored monster
		// standing in a world nobody spawned. Refused earlier by name, so this
		// arm is the backstop rather than the path.
		return fmt.Errorf("%w: %v", ErrNoSheet, err)
	case errors.Is(err, contributions.ErrRuleCannotAnswer):
		// A class-scaled rule that cannot answer from its frame: the actor's
		// sheet holds no levels in the class its effect scales with, or its
		// levels are unknown (rpg-project#538). The effect fails rather than
		// being read as level one, and the remedy is the sheet's, so it is
		// this package's word for a sheet it cannot use.
		return fmt.Errorf("%w: %v", ErrBadCharacter, err)
	case errors.Is(err, encounter.ErrNoSheets):
		// The sheet capability was not supplied, or its answer skipped a
		// member: either way some member's speed and reach would have to be
		// invented, and the remedy is a sheet this seam failed to read.
		return fmt.Errorf("%w: %v", ErrNoSheet, err)
	case errors.Is(err, encounter.ErrRefusingSheets):
		// A compile-only world was asked to pace, budget or reach: it has no
		// sheets behind it and was never meant to be played as loaded.
		return fmt.Errorf("%w: %v", ErrInvalidWorld, err)
	case errors.Is(err, resolution.ErrStalePause):
		// A frozen machine an earlier build wrote (ruling E5): the window is
		// lost rather than answered.
		return fmt.Errorf("%w: %v", ErrStalePause, err)
	case errors.Is(err, resolution.ErrNotOffered):
		// An answer the offer does not accept — an option it never listed,
		// or none where it lists some.
		return fmt.Errorf("%w: %v", ErrNotOffered, err)
	case errors.Is(err, resolution.ErrBadFrozen):
		// Frozen state this build could not have written: the stored window
		// is suspect, not the world.
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	case errors.Is(err, resolution.ErrNilInput), errors.Is(err, resolution.ErrNoMachine):
		return fmt.Errorf("%w: %v", ErrNilInput, err)
	default:
		return err
	}
}

// recordFor turns a strike into the outcome the composition will stamp.
// It copies resolution-owned facts once; replay decodes this record rather
// than reconstructing damage or modifier attribution later.
//
// presentationID is the opaque token the declaring client will correlate its
// own simulated throw against, and it is EMPTY for a swing nobody declared —
// a monster's strike, an opportunity attack the server took on a reactor's
// behalf. That is not a gap: no client pre-simulated those dice, so there is
// no throw to correlate with, and an empty token says exactly that. Only
// [Manager.Attack] mints one, because only [Manager.Attack] is a roll a
// player asked for.
// deliveredSeqs renumbers a run of story sequences into the recipient's own
// stream, one at a time through the seam a single sequence already takes.
//
// EVERY BEAT IS RENUMBERED, not just the first. The delivered numbering is
// per-recipient and gapless, so a caller handed one translated sequence and a
// run of raw ones would be comparing two numbering systems that agree only by
// accident. Nil in, nil out: a swing that ended nothing has no follow-ups, and
// an empty slice would be a second way of saying so.
func deliveredSeqs(scope *writeScope, member string, seqs []uint64) []uint64 {
	if len(seqs) == 0 {
		return nil
	}
	delivered := make([]uint64, len(seqs))
	for i, seq := range seqs {
		delivered[i] = scope.deliveredSeq(member, seq)
	}
	return delivered
}

func recordFor(
	in *AttackInput, struck resolution.StrikeOutcome, definition combatActions.Definition,
	presentationID string, told concentration,
) *encounter.RecordInput {
	return recordStrike(in.Attacker, in.Target, struck, attackRefFor(definition), presentationID,
		told.Checks, told.Breaks)
}

// rollSourceFor projects the rulebook's sourced roll identity onto the
// composition's neutral carrier — the *core.Ref reduced to its canonical
// module:type:id string, the display name, and the role label, each cloned
// into fresh values. Pure: no arithmetic, no ref parsing, no name lookup.
func rollSourceFor(source dnd5eEvents.RollSource) encounter.RollSource {
	var ref string
	if source.Ref != nil {
		ref = source.Ref.String()
	}
	return encounter.RollSource{
		Ref: ref, Name: source.Name, Label: source.Label, SourceID: source.SourceID,
	}
}

// diceRerollFor clones one ordered die replacement, source included.
func diceRerollFor(reroll dnd5eEvents.DiceReroll) encounter.DiceReroll {
	return encounter.DiceReroll{
		DieIndex: reroll.DieIndex,
		Before:   reroll.Before,
		After:    reroll.After,
		Source:   rollSourceFor(reroll.Source),
	}
}

// diceTraceFor deep-clones one dice pool's trace, or nil when there is none.
// Every face list, ordered reroll, and kept index is copied so neither side
// can alias the other's graph; nil slices stay nil so absent and present
// cannot blur.
func diceTraceFor(trace *dnd5eEvents.DiceTrace) *encounter.DiceTrace {
	if trace == nil {
		return nil
	}
	clone := &encounter.DiceTrace{
		Notation:      trace.Notation,
		DieSize:       trace.DieSize,
		OriginalRolls: append([]int(nil), trace.OriginalRolls...),
		FinalRolls:    append([]int(nil), trace.FinalRolls...),
		KeptIndices:   append([]int(nil), trace.KeptIndices...),
		Subtotal:      trace.Subtotal,
		Keep:          encounterDiceKeepFor(trace.Keep),
	}
	if trace.Rerolls != nil {
		clone.Rerolls = make([]encounter.DiceReroll, len(trace.Rerolls))
		for i, reroll := range trace.Rerolls {
			clone.Rerolls[i] = diceRerollFor(reroll)
		}
	}
	return clone
}

// encounterDiceKeepFor carries the keep record onto the persisted trace. A
// field nobody copies is a field the story never sees: without this the pair
// of faces reaches the record with nothing to say which one counted or why
// (rpg-project#462 R1).
func encounterDiceKeepFor(keep *dnd5eEvents.DiceKeep) *encounter.DiceKeep {
	if keep == nil {
		return nil
	}

	return &encounter.DiceKeep{
		Rule:    encounter.KeepRule(keep.Rule),
		Granted: encounterRollSourcesFor(keep.Granted),
		Imposed: encounterRollSourcesFor(keep.Imposed),
	}
}

func encounterRollSourcesFor(sources []dnd5eEvents.RollSource) []encounter.RollSource {
	if len(sources) == 0 {
		return nil
	}

	mapped := make([]encounter.RollSource, len(sources))
	for i, source := range sources {
		mapped[i] = encounter.RollSource{
			Name: source.Name, Label: source.Label, SourceID: source.SourceID,
		}
		if source.Ref != nil {
			mapped[i].Ref = source.Ref.String()
		}
	}
	return mapped
}

// sessionDiceKeepFor carries the keep record from the persisted trace onto the
// host-facing one, interpreting nothing.
func sessionDiceKeepFor(keep *encounter.DiceKeep) *DiceKeep {
	if keep == nil {
		return nil
	}

	return &DiceKeep{
		Rule:    KeepRule(keep.Rule),
		Granted: sessionRollSourcesFor(keep.Granted),
		Imposed: sessionRollSourcesFor(keep.Imposed),
	}
}

func sessionRollSourcesFor(sources []encounter.RollSource) []RollSource {
	if len(sources) == 0 {
		return nil
	}

	mapped := make([]RollSource, len(sources))
	for i, source := range sources {
		mapped[i] = RollSource{
			Ref: source.Ref, Name: source.Name, Label: source.Label, SourceID: source.SourceID,
		}
	}
	return mapped
}

// rollComponentFor deep-clones one component's roll facts: the sourced
// identity, the dice trace when the component rolled dice, and the modifier
// pointer when it contributed one — present even when the value is zero.
func rollComponentFor(component dnd5eEvents.RollComponent) encounter.RollComponent {
	var modifier *int
	if component.Modifier != nil {
		value := *component.Modifier
		modifier = &value
	}
	return encounter.RollComponent{
		Source:       rollSourceFor(component.Source),
		Dice:         diceTraceFor(component.Dice),
		Modifier:     modifier,
		SubtractDice: component.SubtractDice,
	}
}

// rollCalculationFor deep-clones one sourced roll calculation, or nil when
// there is none. Components keep their production order; every component's
// roll facts are cloned through rollComponentFor.
func rollCalculationFor(calculation *dnd5eEvents.RollCalculation) *encounter.RollCalculation {
	if calculation == nil {
		return nil
	}
	clone := &encounter.RollCalculation{Total: calculation.Total}
	if calculation.Components != nil {
		clone.Components = make([]encounter.RollComponent, len(calculation.Components))
		for i, component := range calculation.Components {
			clone.Components[i] = rollComponentFor(component)
		}
	}
	return clone
}

// sessionRollCalculationFor deep-clones the neutral persisted calculation onto
// the host-facing shape without interpreting any source or arithmetic.
func sessionRollCalculationFor(calculation *encounter.RollCalculation) *RollCalculation {
	if calculation == nil {
		return nil
	}
	clone := &RollCalculation{Total: calculation.Total}
	if calculation.Components != nil {
		clone.Components = make([]RollComponent, len(calculation.Components))
		for i, component := range calculation.Components {
			clone.Components[i] = sessionRollComponentFor(component)
		}
	}
	return clone
}

func sessionRollComponentFor(component encounter.RollComponent) RollComponent {
	var modifier *int
	if component.Modifier != nil {
		value := *component.Modifier
		modifier = &value
	}
	return RollComponent{
		Source: RollSource{
			Ref: component.Source.Ref, Name: component.Source.Name,
			Label: component.Source.Label, SourceID: component.Source.SourceID,
		},
		Dice: diceTraceFromEncounter(component.Dice), Modifier: modifier,
		SubtractDice: component.SubtractDice,
	}
}

func diceTraceFromEncounter(trace *encounter.DiceTrace) *DiceTrace {
	if trace == nil {
		return nil
	}
	clone := &DiceTrace{
		Notation: trace.Notation, DieSize: trace.DieSize,
		OriginalRolls: append([]int(nil), trace.OriginalRolls...),
		FinalRolls:    append([]int(nil), trace.FinalRolls...),
		KeptIndices:   append([]int(nil), trace.KeptIndices...), Subtotal: trace.Subtotal,
		Keep: sessionDiceKeepFor(trace.Keep),
	}
	if trace.Rerolls != nil {
		clone.Rerolls = make([]DiceReroll, len(trace.Rerolls))
		for i, reroll := range trace.Rerolls {
			clone.Rerolls[i] = DiceReroll{
				DieIndex: reroll.DieIndex, Before: reroll.Before, After: reroll.After,
				Source: RollSource{
					Ref: reroll.Source.Ref, Name: reroll.Source.Name,
					Label: reroll.Source.Label, SourceID: reroll.Source.SourceID,
				},
			}
		}
	}
	return clone
}

// recordDamageComponents copies the received damage's one trace onto the
// struck beat: the dealt lines, the save's halving, and the target's answers
// (immune, resisted, vulnerable, reduced, cannot fall below zero) as labelled
// modifier lines whose changes total the damage taken (rpg-project#539).
// Resolution never sets a raw multiplier any more, so none is read here; the
// label and the modifier on each line are the whole account.
func recordDamageComponents(in []dnd5eEvents.DamageComponent) []encounter.DamageComponent {
	if len(in) == 0 {
		return nil
	}
	out := make([]encounter.DamageComponent, 0, len(in))
	for _, component := range in {
		out = append(out, encounter.DamageComponent{
			Source: string(component.Source), Roll: rollComponentFor(component.Roll),
			DamageType: string(component.DamageType),
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
	data, err := m.sheetsFor(nil).load(ctx, "attacker", attacker)
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

		if member.Kind == encounter.MemberKind(KindWorld) {
			continue // placed world NPC — no sheet, contributes nothing to the cast
		}

		if readied != nil && readied.ID == id {
			cast = append(cast, resolution.Participant{Character: readied})
			continue
		}

		sheet, err := m.sheetsFor(nil).load(ctx, "participant", id)
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

	// A row here is resolution's own finding that this member's stored sheet
	// will not reconstitute and attach: a corrupt sheet, named as one at the
	// point it is known (ErrBadCharacter), so nothing downstream has to guess.
	for _, refusal := range preflight.Unreadable {
		failures = append(failures, resolutionDependencyFailure{
			member: refusal.Member,
			err:    fmt.Errorf("%w: %v", ErrBadCharacter, refusal.Reason),
		})
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

		if member.Kind == encounter.MemberKind(KindWorld) {
			continue // placed world NPC — no sheet, contributes nothing to the cast
		}

		if readied != nil && readied.ID == id {
			cast = append(cast, resolution.Participant{Character: readied})
			continue
		}

		data, err := m.sheetsFor(nil).load(ctx, "participant", id)
		if err != nil {
			return nil, err
		}
		cast = append(cast, resolution.Participant{Character: data})
	}
	return cast, nil
}
