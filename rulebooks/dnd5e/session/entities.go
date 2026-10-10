// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Loading an entity, and the cleanup that must not happen.
//
// There is no session process. A verb loads what it needs, attaches it to a bus
// created for that call, acts, writes back, and returns; the whole object graph
// is garbage the moment the response is written. The next verb is not the
// resumption of anything living — it is this same load-and-attach performed
// again from persisted data.
//
// That shape is forced rather than chosen. No Go stack survives the gap between
// two verbs, and a loaded character with live subscriptions is exactly that
// kind of state. So every entity is dropped at the end of a call, and anything
// a condition holds that does not survive ToData() is lost with it.
//
// THERE IS NO BUS HERE ANY MORE, and its absence is the point of the slice
// that removed it. This package used to create one per verb and share it across
// every entity in the call, because a condition on one member must be able to
// observe what happens to another. That is still true — it is just not true
// HERE. Reconstituting sheets and putting them on a bus is resolution's, along
// with the folds that need them; this seam hands over records and takes back
// answers. TestNoBusLivesInThisModule holds it.
//
// CHARACTER.CLEANUP MUST NOT BE CALLED. Its first statement is
// `c.conditions = nil`, and ToData() serializes c.conditions — so cleaning up
// before the save persists a character with ZERO conditions. Raging,
// unconscious, a death save in progress: gone, with no error and no failed
// call. Its other half, unsubscribing, buys nothing when the bus dies with the
// response; Cleanup is built for a long-lived character in a long-lived
// process, which is the architecture we do not have.
//
// Skipping it is safe rather than merely tolerable: conditions intercept on the
// bus rather than mutating character fields, so there is no modification left
// un-reversed when the character is dropped.

// instantiate builds catalog content into a new member's sheet.
//
// This is the other half of how an entity enters a session, and the split is
// between where the data comes from rather than between players and monsters.
// A character is LOADED: it already exists, the host owns it, and only the
// host's repository can produce it — which is exactly why a character has no
// ref. A monster is INSTANTIATED: it exists in code, and a ref names the code
// that builds it.
//
// That distinction is the one that survives. "Player or monster" does not: a
// durable NPC would be a monster you load, and homebrew content can be either.
//
// The ref routes on (Module, Type), which is what a ref is for — it says which
// package can produce this data. Today one route exists. A build that wants
// homebrew:monsters registers a loader for it; until then, saying "no loader"
// is honest, where guessing would not be.
//
// The ID is separate from the ref because a template cannot carry identity:
// one skeleton entry makes five skeletons, and each needs its own name in the
// encounter.
//
// # A ref has two places it can resolve, and must resolve in exactly one
//
// A dungeon may author its own stat blocks (rpg-project#555): `templates:`
// declares `guard`, and a placement names it as `dnd5e:monsters:guard`, the
// same door a skeleton comes through (R2). So the ref is answered by the
// rulebook's constructors OR by the template the caller found under the ref's
// id — never by both:
//
//   - a constructor and no template is the rulebook monster, as it always was;
//   - a template and no constructor is assembled by [monster.FromTemplate]
//     over the rulebook base the template names, which is the only function
//     that turns a template into a monster (R6);
//   - both is [ErrShadowedRef], refused rather than picked, because either
//     answer silently discards something somebody wrote;
//   - neither is [ErrUnknownContent], as it always was.
//
// The placement's own `actions:` replace the result's weapons wholesale on
// either path (R9). What comes out is an ordinary monster sheet: nothing after
// this function learns that a template existed (R1).
func instantiate(id string, ref string, actions []string, template *dungeonspec.TemplateSpec) (*monster.Data, error) {
	if ref == "" {
		return nil, ErrNoRef
	}
	parsed, err := core.ParseString(ref)
	if err != nil {
		// The parse error is carried as TEXT, not swallowed and not chained.
		// core.ParseString reports WHICH segment was wrong and why, and a
		// caller staring at a bad ref wants that far more than the string it
		// already passed in — so the message survives in full.
		//
		// What does not survive is the chain. A ref crosses this seam as a
		// STRING, so parsing it with core is an implementation detail, and
		// carrying core.ErrTooFewSegments out to the host made that detail
		// matchable — the same leak the composition's and resolution's
		// sentinels had (rpg-toolkit#1066). ErrBadRef is the whole vocabulary
		// a caller needs: the ref is not a well-formed module:type:id, and the
		// text says which part offended.
		return nil, fmt.Errorf("%q: %w: %v", ref, ErrBadRef, err)
	}
	if parsed.Module != refs.Module || parsed.Type != refs.TypeMonsters {
		return nil, fmt.Errorf("%q: %w", ref, ErrNoLoader)
	}

	// Looked up through the parsed ref rather than the caller's bytes.
	//
	// Honest about what that buys today: NOTHING. core.ParseString does not
	// normalise — it splits on the separator and validates each segment's
	// characters — so any string that parses at all round-trips identically,
	// and swapping this for `ref` is a mutant that SURVIVES. It is recorded
	// here rather than dressed up, because an earlier version of this comment
	// claimed formatting differences were being absorbed and no test could
	// have caught that being false.
	//
	// It stays written this way so the lookup follows automatically if
	// normalisation is ever added upstream, which is a cheap hedge rather
	// than a guarantee.
	build, constructed := monsters.ByRef(parsed.String())

	var built *monster.Monster
	switch {
	case template != nil:
		// The shared assembly refuses a template that shadows the rulebook,
		// so launch and the authoring echo refuse it in one place.
		built, err = assembleTemplate(id, parsed, *template)
		if err != nil {
			return nil, err
		}
	case constructed:
		built = build(id)
		if built == nil {
			return nil, fmt.Errorf("%q: %w", ref, ErrUnknownContent)
		}
	default:
		return nil, fmt.Errorf("%q: %w", ref, ErrUnknownContent)
	}

	if len(actions) > 0 {
		if err := arm(built, actions); err != nil {
			return nil, err
		}
	}

	return built.ToData(), nil
}

// arm replaces a freshly built monster's actions with the ones the author
// named, in the author's order (rpg-project#448, [dungeonspec.MonsterPlacement.Actions]).
//
// ASSEMBLED NOW, STORED ONCE. The sheet is what gets rehydrated (S4), so the
// numbers a monster is spawned with are the numbers it keeps: a later change
// to the weapons catalog or to a stat block's scores does not silently re-arm
// something mid-run.
//
// EVERY REF IS A WEAPON REF, and anything else is refused. The design keeps
// authored non-weapon actions — a claw, a bite, a multiattack — in this same
// list, and NONE EXISTS TODAY: no monster this build ships carries an action
// that is not a catalog weapon. Refusing what cannot appear is what keeps the
// failure here, at spawn, where a host is reading a file; admitting the first
// real claw is a change to this one function.
//
// IT FAILS AT SPAWN, NEVER AT A TURN. A ref the catalog does not know refuses
// the whole verb with [ErrUnknownContent], carrying the ref's own text, which
// is the same sentinel a bad monster ref returns and for the same reason: a
// host boots a shipped dungeon through this door, so a bad weapon refuses
// boot rather than surfacing as a monster that stands there doing nothing.
//
// A MALFORMED REF IS [ErrBadRef], not ErrUnknownContent — the same split the
// monster ref above is held to. "This is not a module:type:id" and "nothing
// here answers to that id" are different things to tell a host, and
// dungeonspec already refuses the first at author time.
func arm(built *monster.Monster, actions []string) error {
	ids, err := weaponIDsOf(actions)
	if err != nil {
		return err
	}

	if err := built.SetWeapons(ids); err != nil {
		return fmt.Errorf("arming %q: %w", built.Name(), err)
	}
	return nil
}

// weaponIDsOf turns authored weapon refs into catalogue ids, refusing as
// [arm] documents: a malformed ref is [ErrBadRef], anything that is not a
// weapon the catalogue knows is [ErrUnknownContent]. A placement's `actions:`
// and a template's are the same refs held to the same refusals, so they are
// read by this one function.
func weaponIDsOf(actions []string) ([]weapons.WeaponID, error) {
	ids := make([]weapons.WeaponID, 0, len(actions))
	for _, action := range actions {
		parsed, err := core.ParseString(action)
		if err != nil {
			return nil, fmt.Errorf("%q: %w: %v", action, ErrBadRef, err)
		}
		if parsed.Module != refs.Module || parsed.Type != refs.TypeWeapons {
			return nil, fmt.Errorf("%q: %w", action, ErrUnknownContent)
		}
		id := weapons.WeaponID(parsed.ID)
		if _, err := weapons.GetByID(id); err != nil {
			return nil, fmt.Errorf("%q: %w", action, ErrUnknownContent)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// templateFor is the dungeon's authored template a placement's ref names, or
// nil when it names none (rpg-project#555 R2).
//
// A template is referenced as `dnd5e:monsters:<id>`, and the id is everything
// after the second colon, so only a ref on that route can name one: a
// homebrew ref whose id happens to be `guard` does not reach this dungeon's
// guard. A ref that does not parse names no template; [instantiate] refuses
// it as itself.
func templateFor(templates map[string]dungeonspec.TemplateSpec, ref string) *dungeonspec.TemplateSpec {
	if len(templates) == 0 {
		return nil
	}
	parsed, err := core.ParseString(ref)
	if err != nil || parsed.Module != refs.Module || parsed.Type != refs.TypeMonsters {
		return nil
	}
	spec, ok := templates[parsed.ID]
	if !ok {
		return nil
	}
	return &spec
}

// projectCharacter asks resolution what this character is, and takes back an
// answer.
//
// A RECORD GOES DOWN AND NUMBERS COME BACK. Nothing live crosses either way:
// resolution builds its own truth from the record, folds what needs folding,
// and returns data. That is the whole of the read law at this seam — this
// package holds records of the world, never the world.
//
// # Why one call rather than a sheet and a fold
//
// Join used to load a character of its own, read three static facts off it, and
// send the record down separately for the armour class. Two reads of the same
// character, and two things that could disagree.
//
// They disagreed in a way that mattered. The armour class HAD to be folded in
// resolution — a fold needs game context, one door installs it, and the door is
// down there — while Speed had to come off a loaded sheet, because it is
// derived from race and stored on no record. So the seam held a sheet for the
// facts it could reach and delegated the one it could not. The entry answers
// both now, off the same sheet, in one call.
//
// # The errors
//
// Absent and unreadable stay apart, because they send whoever debugs it to
// different places: a bad request versus corrupt storage. The fetch above
// answers ErrNoCharacter; this answers ErrBadCharacter for a record resolution
// could not make a character out of.
//
// The inner reason rides as TEXT rather than as a chain. Resolution reports
// through its own sentinels, and a host matching on one of those would be
// matching on a package this seam exists to keep it away from (S2,
// rpg-toolkit#1066).
func projectCharacter(
	ctx context.Context, id string, record *character.Data,
) (*resolution.ProjectCharacterOutput, error) {
	if record == nil {
		return nil, fmt.Errorf("character %q: %w: no record to project", id, ErrBadCharacter)
	}

	projected, err := resolution.ProjectCharacter(ctx, &resolution.ProjectCharacterInput{
		Character: record,
	})
	if err != nil {
		return nil, fmt.Errorf("character %q: %w: %v", id, projectionSentinel(err), err)
	}

	return projected, nil
}

// projectionSentinel picks THIS package's word for a projection that refused.
//
// Two failures, two repairs, and a host branches on which: a main-hand weapon
// that will not compile is a broken loadout, while anything else the projection
// refuses is a sheet that will not reconstitute. Resolution reports the first
// under its own ErrBadAttack, and this is where that becomes ours.
//
// # It reads resolution's sentinel and does not pass it on
//
// The match happens here and the inner error rides out as TEXT (%v at the call
// site, never %w). A host matching resolution.ErrBadAttack would be matching on
// a package this seam exists to keep it away from, and S2 is not a promise this
// package delegates (rpg-toolkit#1066). Reading a sentinel to CHOOSE ours is a
// different act from forwarding one.
//
// # Why this is not translateResolution
//
// That function serves the verbs that run an interaction, and its vocabulary is
// about interactions — costs, ranges, activations. A projection can fail in two
// ways and neither is any of those. Routing this through it would mean widening
// a switch that reads as "what went wrong in a fight" with a case that has
// nothing to do with fighting.
func projectionSentinel(err error) error {
	if errors.Is(err, resolution.ErrBadAttack) {
		return ErrBadAttack
	}

	return ErrBadCharacter
}

// characterStateFrom maps the answer onto the shape this seam publishes.
//
// A mapping and nothing else — every field is already a number or a string by
// the time it arrives, which is what makes this function boring. It was not
// always: the version this replaces read six accessors off a live sheet and
// carried a comment explaining why it must not call ToData to do it. That
// argument now lives where the reading happens, one module down.
func characterStateFrom(projected *resolution.ProjectCharacterOutput) *CharacterState {
	if projected == nil {
		return nil
	}

	return &CharacterState{
		ID:               projected.Sheet.ID,
		Name:             projected.Sheet.Name,
		Level:            projected.Sheet.Level,
		Speed:            projected.Sheet.SpeedFeet,
		HitPoints:        projected.Sheet.HitPoints,
		MaxHitPoints:     projected.Sheet.MaxHitPoints,
		ArmorClass:       projected.ArmorClass.Total,
		ProficiencyBonus: projected.Sheet.ProficiencyBonus,
	}
}
