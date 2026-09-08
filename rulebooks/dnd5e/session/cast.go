// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// CastInput casts a spell the member knows.
//
// IT MIRRORS [ActivateInput] FIELD FOR FIELD, minus the one field an
// activation needs and a cast does not: nothing here is rolled against
// observers. The shape is deliberate rather than coincidental — a cast and an
// activation are the same question from a host's side ("run the thing this
// selector names, at this target"), and the two verbs differ underneath rather
// than at the door.
type CastInput struct {
	// Session is the session to act inside.
	Session string

	// Member is who is casting. Required.
	//
	// THE HOST MUST BIND THIS TO THE AUTHENTICATED CALLER, exactly as
	// [Manager.Afford], [Manager.Activate] and [Manager.Where] require and for
	// the same reason: this package cannot tell who is asking, and a
	// client-supplied ID wired through unchecked turns a caller-scoped verb
	// into one that acts for anybody.
	Member string

	// DeclarationID is the opaque selector echoed from [Manager.Afford], and
	// it is also WHICH SPELL this casts: one verb compiles one offer per
	// castable cantrip, so the selector names the row rather than the verb.
	// Required.
	//
	// There is deliberately no spell ref on this input. A caller that named
	// the spell itself would be deciding something Afford already decided, and
	// the two could disagree — the same argument [ActivateInput.DeclarationID]
	// makes for an ability.
	DeclarationID string

	// Target is who it lands on, for a cast whose profile names one creature.
	// Empty for a self-targeted cast, and a populated ID on one is refused
	// rather than ignored.
	Target string
}

// CastOutput is what a cast produced.
//
// AN ACKNOWLEDGEMENT, by the same ruling [ActivateOutput] carries: it says
// nothing about what the member can do next, because a second declaration
// surface beside [Manager.Afford] would be two reads answering "what can I do"
// and free to disagree. The caller re-reads Afford.
//
// What it does carry is S6's law, which every mutating verb here keeps: a cast
// writes sheets and publishes conditions, so both halves can be half-done, and
// a caller told only "fine" could not tell a durable condition from one that
// never reached disk.
type CastOutput struct {
	// Spell is the spell that was cast, echoed back — so a caller that
	// dispatched by selector learns what the selector meant without parsing
	// it.
	Spell SpellRef `json:"spell"`

	// Saved reports the gate's saving throw, or nil for a cast with no gate.
	//
	// NIL IS THE HONEST ZERO. True Strike delivers a condition and rolls
	// nothing, and a report reading 0 against DC 0 would say a roll happened
	// that never did — the same presence law [encounter.RecordCastInput.Save]
	// keeps one layer down.
	Saved *CastSaveReport `json:"saved,omitempty"`

	// Seqs are the story sequences of the recorded beats, in the order they
	// were appended: the cast, then the save if there was one, then one per
	// delivered effect.
	Seqs []uint64 `json:"seqs"`

	// Persisted names what was written.
	Persisted SaveReport `json:"persisted"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`
}

// CastSaveReport is one saving throw a cast's gate produced, as the caster's
// own response carries it: who rolled, what with, the d20 and the number it
// reached, the DC, and whether it beat it.
//
// SUCCEEDED IS A BOOL AND NOT AN OUTCOME WORD, because a save has exactly two
// answers — the same reading [SavedBody] and the composition's own CastSave
// keep, so the response and the beat do not describe one roll two ways.
type CastSaveReport struct {
	Saver     string `json:"saver"`
	Ability   string `json:"ability"`
	Roll      int    `json:"roll"`
	Total     int    `json:"total"`
	DC        int    `json:"dc"`
	Succeeded bool   `json:"succeeded"`
}

// Cast casts a cantrip the member knows at a target the offer named.
//
// # It is Activate's twin everywhere but the door
//
// Same gates in the same order, same regenerate-and-select, same
// adopt/save/record/commit tail. The difference is one field, and it is the
// whole reason this verb exists separately (design R10).
//
// # THIS ONE IS PAID AT THE DOOR
//
// [resolution.Input.Cost] is NON-NIL here, and [Manager.Activate]'s is nil.
// That is not an inconsistency between two verbs that look alike: an
// activation's ability spends its own slot underneath, so a Cost passed
// alongside would charge the same ledger twice and the second charge would look
// exactly like the first. A cantrip has nothing underneath it — no
// ActivateAbility, no feature-owned spend, no second currency — so the door
// charges the one action and the machine is never told, which is the ignorance
// [resolution.Input.Cost]'s own doc asks for.
//
// The charge lands after pure machine preflight and before the first yielded
// step, and it is all-or-none by the gate's construction. A bard with no action
// left is refused BEFORE anything moves: no publish, no roll, no dirty sheet.
// The price is exactly what the Afford row already showed, because the offer
// this verb regenerates is the offer that was priced.
//
// # A second cast in one turn is refused by the ledger, not by a rule here
//
// Nothing in this verb counts casts. The first one spends the action, the
// regenerated offer for the second one reads the same ledger and compiles
// unavailable, and the selector is refused as stale — which is how a client
// finds out before the click, and how the door finds out after it.
//
// # Everyone is in the cast
//
// R3: pass everyone in, exactly as [Manager.Activate] does. A condition a cast
// publishes is applied by the OWNER'S keeper, which is only attached for
// participants, and a participant list trimmed to "whoever this obviously
// affects" would be a rule decided in the one place that cannot see it.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoDeclarationID,
// ErrNoSession, ErrNoEncounter, ErrNoMember, ErrNotACharacter, ErrNotYourTurn,
// ErrDowned, ErrStaleDeclaration, ErrNoSheet, ErrNoCharacter, ErrBadCharacter,
// ErrBadRepository, ErrBadCost, ErrCannotAfford, ErrBadCast, ErrInvalidWorld,
// ErrClosed, or ErrSaveFailed with a populated report. ErrWindowOpen refuses
// the whole verb while an interrupt window is open anywhere in the fight, the
// freeze every other declaring verb is refused by.
//
// ErrBadCharacter covers a cast member whose sheet will not load — everyone is
// a participant here, so ANY unreadable member refuses the whole cast rather
// than just the caster's own — and ErrInvalidWorld covers a machine that
// returned an outcome this verb has no case for, which is a provider defect
// this fails closed on rather than reporting as a successful cast that did
// nothing.
func (m *Manager) Cast(ctx context.Context, in *CastInput) (*CastOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("cast: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("cast: %w", ErrNoMemberID)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("cast: %w", translate(err))
	}
	kind, inRoster := encounter.MemberKind(""), false
	for _, member := range roster {
		if string(member.ID) == in.Member {
			kind, inRoster = member.Kind, true
			break
		}
	}
	if !inRoster {
		return nil, fmt.Errorf("cast: member %q: %w", in.Member, ErrNoMember)
	}
	if kind != encounter.MemberKind(KindPlayer) {
		// A monster is in the roster and still cannot declare: an innate cast
		// is driven by behaviour rather than chosen, and its economy belongs
		// to whoever runs its turn. The same line Activate and Attack draw.
		return nil, fmt.Errorf("cast: member %q: %w", in.Member, ErrNotACharacter)
	}

	// NOT YOUR TURN, checked before anything touches character storage — the
	// precedence every declaring verb keeps.
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("cast: %w", translate(err))
	}
	if ClockKind(clock.Kind) == ClockTurn && string(clock.Active) != in.Member {
		return nil, fmt.Errorf("cast: member %q: %w", in.Member, ErrNotYourTurn)
	}
	if ClockKind(clock.Kind) != ClockTurn {
		// There are no world-clock casts. Afford returns no declarations
		// there at all, so every selector is stale by construction — and an
		// empty one is still the caller's omission rather than a stale offer.
		if in.DeclarationID == "" {
			return nil, fmt.Errorf("cast: %w", ErrNoDeclarationID)
		}
		return nil, fmt.Errorf("cast: %w", ErrStaleDeclaration)
	}

	// Loaded strictly ONCE, so standing and every piece of the regenerated
	// offer come from the same repository snapshot.
	actor := m.loadActorSheet(ctx, in.Member)
	if actor.downed {
		return nil, fmt.Errorf("cast: member %q: %w", in.Member, ErrDowned)
	}
	if in.DeclarationID == "" {
		return nil, fmt.Errorf("cast: %w", ErrNoDeclarationID)
	}

	// Regenerate and select. compileOffersFor owns identity and availability;
	// selectCompiledOffer refuses a selector that names nothing, names two
	// things, or names an offer that is no longer available — all as
	// ErrStaleDeclaration, because from a client's side they are the same
	// event: you saw an offer and the world moved. A second cast in one turn
	// arrives here, as an offer whose budget gate no longer passes.
	offers, err := m.compileOffersFor(
		ctx, scope.enc, scope.data, scope.session, in.Member, clock, actor, VerbCast,
	)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}
	selected, err := selectCompiledOffer(offers, VerbCast, in.DeclarationID)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}
	if selected.declaration.Spell == nil || selected.spell == nil || selected.sheet == nil {
		// A compiled Cast offer always carries all three. Failing closed here
		// keeps a provider defect from reaching resolution as a nil definition.
		return nil, fmt.Errorf("cast: %w", ErrStaleDeclaration)
	}
	definition := selected.spell

	target, err := castTarget(definition, selected, in.Target)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	machine, err := resolution.NewAction(&resolution.ActionInput{
		Definition: definition.Clone(),
		AttackerID: in.Member,
		TargetID:   target,
		// A machine that rolls carries its own roller (resolution's rule): a
		// gated cast rolls the target's save, and it rolls with the HOST'S
		// dice through the same seam every other roll takes. There is no
		// process-global fallback for this verb to inherit
		// (rpg-toolkit#1427).
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("cast: %w", translateResolution(err))
	}

	// The readied sheet goes into the participants rather than being fetched
	// again: compileOffersFor readied this turn's economy on it, and a second
	// read would hand resolution a ledger that had not been filled.
	readied := selected.sheet.ToData()
	participants, failures := m.compileResolutionCast(ctx, scope.data, roster, readied)
	if len(failures) > 0 {
		return nil, fmt.Errorf("cast: participant %q: %w: %v",
			failures[0].member, ErrBadCharacter, failures[0].err)
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	world := scope.enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        world,
		Participants: participants,
		Initiative:   m.initiative,
		Standing:     scope.standing,
		Sight:        &sightSeam{members: worldMembers(world)},
		TurnDriver:   m.turnDriver,
		// The concealment pair (rpg-toolkit#1378), bound to the same live
		// scope openForChange and adopt bind — the one-seam consistency law.
		CheckResolver: checkSeam{m: m, scope: scope},
		Witness:       witnessSeam{scope: scope},
		Machine:       machine,
		// NON-NIL ON PURPOSE, and the one thing that makes this verb different
		// from Activate — see this verb's own doc. The price is the one
		// compiled into the definition the offer was hashed from, cloned so
		// the selector material and the charge cannot alias.
		Cost: &resolution.Cost{
			PayerID: in.Member,
			Profile: combatActions.CloneSpendProfile(definition.Cost),
			Turn:    &resolution.Turn{Number: clock.Round},
		},
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("cast: %w", translateResolution(err))
	}

	save, results, err := castOutcome(out.Outcome, in.Member, *selected.declaration.Spell)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	if err := m.adopt(scope, out.World); err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}
	if err := m.saveDirty(ctx, scope, out); err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	// Record only after the adopted sheets are durable. RecordCast's
	// post-append noticeDown consult must see the same hit points and
	// conditions the cast produced, matching Attack's and Activate's
	// save -> record -> commit path: a skeleton dropped by 1d4 psychic is
	// noticed from the sheet this call already wrote. If that consult fails,
	// the mechanical sheet writes remain durable and are named by
	// reportUnrecorded while this unsaved encounter scope is dropped.
	recorded, err := scope.enc.RecordCast(&encounter.RecordCastInput{
		Actor:  encounter.MemberID(in.Member),
		Target: encounter.MemberID(target),
		Spell: encounter.SpellIdentity{
			Ref:  selected.declaration.Spell.Ref,
			Name: selected.declaration.Spell.Name,
		},
		Save:    save,
		Results: results,
	})
	if err != nil {
		return nil, fmt.Errorf("cast: %w", reportUnrecorded(scope, translate(err)))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	return &CastOutput{
		Spell:     *selected.declaration.Spell,
		Saved:     castSaveReport(save),
		Seqs:      recorded.Seqs,
		Persisted: report,
		Delivery:  delivery,
	}, nil
}

// castTarget enforces the profile's own target rule against what the caller
// asked for, and re-enforces the offer's per-candidate gate.
//
// # The rule comes from the content, and the reach from the offer
//
// [combatActions.CastProfile.Target] says whether this cast names anybody;
// the compiled offer says which of the members in sight are actually in range
// and eligible. Both are checked here rather than only at the door, for the
// reason Attack re-enforces reach on execution: a client may echo a selector
// that was compiled a moment ago against a target that has since walked out of
// range, and a verb that trusted the click would resolve a 60-foot cantrip
// across the room.
//
// A populated target on a self-targeted cast is REFUSED rather than ignored.
// Ignoring it would let a client believe True Strike had been pointed at the
// skeleton when the profile never offered that choice.
func castTarget(
	definition *combatActions.Definition, selected compiledOffer, requested string,
) (string, error) {
	if definition.Cast.Target == combatActions.CastTargetSelf {
		if requested != "" {
			return "", fmt.Errorf("%w: spell %q names no target",
				ErrBadCast, definition.Ref.String())
		}
		return "", nil
	}

	if requested == "" {
		return "", fmt.Errorf("%w: spell %q requires a target",
			ErrBadCast, definition.Ref.String())
	}
	candidate, offered := selected.targets[requested]
	if !offered {
		// Not in the candidate universe at all: out of sight, out of the
		// roster, or never a candidate for this spell's range. From a client's
		// side that is the same event as any other stale offer.
		return "", fmt.Errorf("%w", ErrStaleDeclaration)
	}
	if !candidate.available {
		reason := "target is unavailable"
		if candidate.why != nil {
			reason = candidate.why.Text
		}
		return "", fmt.Errorf("%w: target %q: %s", ErrStaleDeclaration, requested, reason)
	}
	return requested, nil
}

// castSaveReport projects the composition's save onto the caller's own.
// Nil in, nil out: a cast with no gate rolled nothing, and the absence is the
// answer.
func castSaveReport(save *encounter.CastSave) *CastSaveReport {
	if save == nil {
		return nil
	}
	return &CastSaveReport{
		Saver:     string(save.Saver),
		Ability:   save.Ability,
		Roll:      save.Roll,
		Total:     save.Total,
		DC:        save.DC,
		Succeeded: save.Succeeded,
	}
}
