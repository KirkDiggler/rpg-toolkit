// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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
	// castable known entry, so the selector names the row rather than the verb.
	// Required.
	//
	// There is deliberately no spell ref on this input. A caller that named
	// the spell itself would be deciding something Afford already decided, and
	// the two could disagree — the same argument [ActivateInput.DeclarationID]
	// makes for an ability.
	DeclarationID string

	// Target is the legacy single-target spelling.
	// Deprecated: use Targets; put a single target in a one-element slice.
	Target string

	// Targets is the canonical ordered target list. Its bounds come from the
	// selected declaration.
	Targets []string

	// Cell is the cell a [TargetCell] cast is aimed at, and nothing else ever
	// carries one. Required for that shape and refused on every other.
	//
	// A REFERENCE, NOT A CALCULATION. The caster points; the shape's geometry
	// is derived from this cell and the caster's own, by the composition that
	// owns placement. Nothing here reads it as a distance, a facing, or a
	// target, and a creature standing on it is not thereby aimed at.
	//
	// NOT THE CASTER'S OWN CELL. A cube anchored on the caster's edge takes
	// its direction from the line between the two, and a cell with no line
	// out of it is not a direction. Refused at the door rather than defaulted
	// to whatever the caster is facing, which is a fact this seam does not
	// hold.
	//
	// A POINTER because absence is the question. spatial.Position's zero value
	// is a real cell somewhere on the canvas, so a missing cell and a cell at
	// the origin must not be the same value.
	Cell *spatial.Position
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
	// that never did — the same presence law each target result keeps one layer
	// down.
	// Saved is the legacy single-target save projection. It is populated only
	// when exactly one target produced a save.
	Saved *CastSaveReport `json:"saved,omitempty"`

	// Caught are members an area cast's footprint reached that this build could
	// not resolve against — a placed world member with no sheet behind it.
	//
	// REPORTED RATHER THAN DROPPED. Silently omitting them would make "nobody
	// was standing there" and "somebody was standing there and we have nothing
	// to do about it" the same answer, and the second is a missing capability
	// that should stay visible until it is built. Nil when nothing was caught
	// this way, which is every cast that is not an area and most that are.
	Caught []CaughtMember `json:"caught,omitempty"`

	// Seqs are the story sequences of the recorded beats, in the order they
	// were appended: the cast, then the save if there was one, then one per
	// delivered effect.
	Seqs []uint64 `json:"seqs"`

	// Persisted names what was written.
	Persisted SaveReport `json:"persisted"`

	// Delivery names what reached the event stream.
	Delivery DeliveryReport `json:"delivery"`

	// Paused reports that a creature this cast sent running STOPPED MID-WALK
	// to ask somebody whether they swing at it, and the table is frozen until
	// they answer.
	//
	// THE CAST ITSELF IS WHOLE. Everything above is final: the save was rolled,
	// the damage landed, the beat is on the story, and the price is paid. What
	// is unfinished is the walk — the creature is standing where the held step
	// was announced from, and the cast beat's own distance is what the route
	// priced rather than what has been taken so far. Answering the open window
	// with [Manager.React] finishes the run and writes the movement beats this
	// call did not.
	//
	// A cast whose push does not provoke never pauses, because nothing is
	// asked; see [Manager.Cast] on why a flee is the one directive that does.
	Paused bool `json:"paused,omitempty"`
}

// CastSaveReport is one saving throw a cast's gate produced, as the caster's
// own response carries it: who rolled, what with, the d20 and the number it
// reached, the DC, and whether it beat it.
//
// SUCCEEDED IS A BOOL AND NOT AN OUTCOME WORD, because a save has exactly two
// answers — the same reading [SavedBody] and the composition's own CastSave
// keep, so the response and the beat do not describe one roll two ways.
type CastSaveReport struct {
	Saver       string           `json:"saver"`
	Ability     string           `json:"ability"`
	Roll        int              `json:"roll"`
	Total       int              `json:"total"`
	DC          int              `json:"dc"`
	Succeeded   bool             `json:"succeeded"`
	Calculation *RollCalculation `json:"calculation,omitempty"`
}

// Cast casts a known spell at the ordered targets the offer permits.
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
// Cast passes the complete provider-authored Definition.Cost — an action for a
// cantrip, and an action plus the level-one pool for Bane — without selecting a
// resource key or pricing a spell itself. The charge lands after pure machine
// preflight and before the first yielded step, and it is all-or-none by the
// gate's construction. A bard with no action or slot left is refused BEFORE
// anything moves: no publish, no roll, no dirty sheet. The price is exactly
// what the Afford row already showed, because the offer this verb regenerates
// is the offer that was priced.
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
// ErrBadRepository, ErrBadCost, ErrCannotAfford, ErrBadCast, ErrBadActivation,
// ErrInvalidWorld,
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
	targets, err := normalizeCastTargets(in.Target, in.Targets)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
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
	var casterAt spatial.Position
	for _, member := range roster {
		if string(member.ID) == in.Member {
			kind, casterAt, inRoster = member.Kind, member.Position, true
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

	targets, err = castTargets(definition, selected, targets, castAim{cell: in.Cell, casterAt: casterAt})
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	// An area cast's recipients are worked out here, from the shape the content
	// declared and the roster the composition placed — never from what the
	// caller sent, which castTargets has just confirmed was nobody.
	var caught *areaCaught
	if definition.Cast != nil && definition.Cast.Target == combatActions.CastTargetArea {
		caught, err = deriveAreaMembers(scope.enc, definition.Cast, in.Member, roster, in.Cell)
		if err != nil {
			return nil, fmt.Errorf("cast: %w", err)
		}
	}

	machine, err := resolution.NewAction(&resolution.ActionInput{
		Definition: definition.Clone(),
		AttackerID: in.Member,
		TargetIDs:  targets,
		// Empty for every other arm. Derived recipients travel separately from
		// named ones all the way down, so no gate has to guess which it holds.
		AreaMembers: areaMemberIDs(caught),
		// A machine that rolls carries its own roller (resolution's rule): a
		// gated cast rolls the target's save, and it rolls with the HOST'S
		// dice through the same seam every other roll takes. There is no
		// process-global fallback for this verb to inherit
		// (rpg-toolkit#1427).
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		// ErrBadAction NAMED HERE rather than left to translateResolution,
		// which Attack also uses and which would have to answer for two verbs
		// with one sentinel. A definition resolution refuses to build a machine
		// from is this verb's own ErrBadCast — and it must be A sentinel rather
		// than resolution's own error travelling out, because no inner type
		// crosses this boundary (S2).
		//
		// It is a BACKSTOP and not the path: castTarget above already refuses a
		// target on a self-cast and a missing one on a cast that needs it, so
		// this arm catches content or wiring the door could not have known was
		// wrong. Reached before anything is charged either way.
		if errors.Is(err, resolution.ErrBadAction) {
			return nil, fmt.Errorf("cast: spell %q: %w: %v",
				definition.Ref.String(), ErrBadCast, err)
		}
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
		Equipment:    equipmentBeside(scope.standing),
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

	targetResults, pushes, err := castOutcome(out.Outcome, in.Member, *selected.declaration.Spell)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	// THE ROUTE IS TAKEN BEFORE THE RECORD AND THE WALK AFTER IT, which is the
	// whole of how a push is ordered here. Route is a pure computation and
	// writes nothing, so the cast's own beat can say the blast moved somebody
	// one cell instead of two; the walk that follows puts the movement beats
	// after the cast beat, which is the order a client animates them in.
	// Thunder, then the slide. See [castPush].
	if err := routeCastPushes(scope.enc, pushes, targetResults); err != nil {
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
		Actor: encounter.MemberID(in.Member),
		Spell: encounter.SpellIdentity{
			Ref:  selected.declaration.Spell.Ref,
			Name: selected.declaration.Spell.Name,
		},
		Targets: targetResults,
		// PASSED THROUGH, exactly as the strike passes them: resolution
		// assembled both lists and this seam copies two slice headers. A cast
		// ends a concentration two ways — displacing one by casting again, and
		// breaking somebody else's with its damage — and resolution has
		// already put them in the order they happened.
		ConcentrationChecks: out.ConcentrationChecks,
		ConcentrationBreaks: out.ConcentrationBreaks,
	})
	if err != nil {
		return nil, fmt.Errorf("cast: %w", reportUnrecorded(scope, translate(err)))
	}

	// The pushes, now that the cast beat naming them is on the story. A
	// failure here leaves the cast recorded and the shove untaken, which is
	// the same shape RecordCast's own failure has and is reported the same
	// way: the durable writes are named and this unsaved scope is dropped.
	//
	// A PAUSE IS NOT ONE OF THOSE FAILURES, and telling them apart is the whole
	// of what this line does. When a flee provokes and the reactor is a player,
	// the walk stops to ask — and by the time it does, the seam that asked has
	// already written the windows into this scope's session record. Dropping
	// the scope would throw the question away and leave the table waiting on a
	// window nobody can see; so the verb commits, and says on its own output
	// that the walk is unfinished.
	paused, err := walkCastPushes(ctx, scope.enc, pushes, definition.Ref)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", reportUnrecorded(scope, err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("cast: %w", err)
	}

	var singleSave *encounter.CastSave
	if len(targetResults) == 1 {
		singleSave = targetResults[0].Save
	}
	return &CastOutput{
		Spell:     *selected.declaration.Spell,
		Saved:     castSaveReport(singleSave),
		Caught:    areaUnresolved(caught),
		Seqs:      recorded.Seqs,
		Paused:    paused,
		Persisted: report,
		Delivery:  delivery,
	}, nil
}

// normalizeCastTargets resolves the deprecated scalar at the public boundary.
// It always returns independently owned storage and never edits caller slices.
func normalizeCastTargets(target string, targets []string) ([]string, error) {
	if target != "" && len(targets) != 0 {
		return nil, fmt.Errorf("%w: received both Target and Targets", ErrBadCast)
	}
	if target != "" {
		return []string{target}, nil
	}
	return append([]string(nil), targets...), nil
}

// castAim is where the caller pointed this cast, and where they were standing
// when they did.
//
// Both halves travel together because the one refusal that needs them needs
// BOTH: a caster-edge shape takes its direction from the line between the two
// cells, and a line whose ends are the same cell is not a direction. Carried as
// a value rather than read back off the encounter here, so the cell this gate
// judges is the same cell the shape is later derived from.
type castAim struct {
	// cell is the cell a TargetCell cast was aimed at, nil for every other
	// shape.
	cell *spatial.Position

	// casterAt is the caster's own cell, as the composition placed them.
	casterAt spatial.Position
}

// castTargets enforces the profile's own target rule against what the caller
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
func castTargets(
	definition *combatActions.Definition, selected compiledOffer, requested []string, aim castAim,
) ([]string, error) {
	profile := definition.Cast

	// ONE GUARD RATHER THAN AN ARM APIECE. A cell belongs to exactly one
	// selector shape, so the refusal is stated once, before the shapes are
	// told apart, and a kind added later cannot quietly start accepting one by
	// forgetting to say no.
	if selected.declaration.TargetKind != TargetCell && aim.cell != nil {
		return nil, fmt.Errorf("%w: spell %q names no cell",
			ErrBadCast, definition.Ref.String())
	}

	if profile.Target == combatActions.CastTargetSelf || profile.Target == combatActions.CastTargetArea {
		// Neither lets the caller name anybody, and a populated list is
		// REFUSED rather than ignored for the same reason in both cases: a
		// client that believed it had pointed the spell somewhere must be told
		// it had not. An area cast's recipients are derived after this, from
		// the shape, and never from what arrived here.
		if len(requested) != 0 {
			return nil, fmt.Errorf("%w: spell %q names no targets",
				ErrBadCast, definition.Ref.String())
		}
		if selected.declaration.TargetKind == TargetCell {
			// THE OFFER SAID A CELL WAS WANTED, so its absence is the caller's
			// omission rather than a shape this door has to guess at. Nothing
			// below can proceed without it: the cube has no direction, and a
			// direction chosen here would be a rule invented at the seam.
			if aim.cell == nil {
				return nil, fmt.Errorf("%w: spell %q needs a cell to aim at",
					ErrBadCast, definition.Ref.String())
			}
			if *aim.cell == aim.casterAt {
				return nil, fmt.Errorf(
					"%w: spell %q cannot be aimed at the caster's own cell, which is no direction",
					ErrBadCast, definition.Ref.String())
			}
		}
		return []string{}, nil
	}

	if len(requested) < profile.MinTargets || len(requested) > profile.MaxTargets {
		return nil, fmt.Errorf("%w: spell %q requires %d..%d targets; got %d",
			ErrBadCast, definition.Ref.String(), profile.MinTargets, profile.MaxTargets, len(requested))
	}
	for _, target := range requested {
		if target == "" {
			return nil, fmt.Errorf("%w: spell %q has an empty target",
				ErrBadCast, definition.Ref.String())
		}
		candidate, offered := selected.targets[target]
		if !offered {
			return nil, fmt.Errorf("%w", ErrStaleDeclaration)
		}
		if !candidate.available {
			reason := "target is unavailable"
			if candidate.why != nil {
				reason = candidate.why.Text
			}
			return nil, fmt.Errorf("%w: target %q: %s", ErrStaleDeclaration, target, reason)
		}
	}
	return append([]string(nil), requested...), nil
}

// castSaveReport projects the composition's save onto the caller's own.
// Nil in, nil out: a cast with no gate rolled nothing, and the absence is the
// answer.
func castSaveReport(save *encounter.CastSave) *CastSaveReport {
	if save == nil {
		return nil
	}
	return &CastSaveReport{
		Saver:       string(save.Saver),
		Ability:     save.Ability,
		Roll:        save.Roll,
		Total:       save.Total,
		DC:          save.DC,
		Succeeded:   save.Succeeded,
		Calculation: sessionRollCalculationFor(save.Calculation),
	}
}
