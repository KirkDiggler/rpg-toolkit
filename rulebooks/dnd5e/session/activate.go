// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// ActivateInput uses a combat ability or feature the member already carries.
type ActivateInput struct {
	// Session is the session to act inside.
	Session string

	// Member is who is activating. Required.
	//
	// THE HOST MUST BIND THIS TO THE AUTHENTICATED CALLER, exactly as
	// [Manager.Afford] and [Manager.Where] require and for the same reason:
	// this package cannot tell who is asking, and a client-supplied ID wired
	// through unchecked turns a caller-scoped verb into one that acts for
	// anybody.
	Member string

	// DeclarationID is the opaque selector echoed from [Manager.Afford], and
	// it is also WHICH ABILITY this activates: one verb compiles one offer per
	// thing the member carries, so the selector names the row rather than the
	// verb. Required.
	//
	// There is deliberately no ability ref on this input. A caller that named
	// the ability itself would be deciding something Afford already decided,
	// and the two could disagree.
	DeclarationID string

	// Target is who it lands on, for the one level-1 ability that takes
	// somebody: Help. Empty for the rest, and a populated ID on an ability
	// that takes none is refused rather than ignored.
	Target string

	// ObserverPassivePerceptions is the passive Perception of everyone who
	// could notice, which Hide's Stealth check is rolled against. Empty for
	// every other ability — and empty is a legitimate answer for Hide too
	// (nobody is watching), so it cannot be validated by presence.
	ObserverPassivePerceptions []int
}

// ActivateOutput is what an activation produced.
//
// AN ACKNOWLEDGEMENT, by Kirk's ruling on rpg-project#300: it says nothing
// about what the member can do next. Returning a refreshed ability list would
// put a SECOND declaration surface beside [Manager.Afford] — two reads
// answering "what can I do", free to disagree, with the caller left to learn
// which one wins. The caller re-reads Afford.
//
// What it does carry is S6's law, which every mutating verb here keeps: an
// activation writes a sheet and publishes a condition, so both halves can be
// half-done, and a caller told only "fine" could not tell a durable rage from
// one that never reached disk.
type ActivateOutput struct {
	// Ability is the ref that ran, echoed back — so a caller that dispatched
	// by selector learns what the selector meant without parsing it.
	Ability string

	// GrantedCapacity is the ability's own description of what it banked
	// ("30ft movement" for Dash), or empty. Display text authored by the
	// ability and never parsed: Dash's effect on the ledger is in the ledger.
	GrantedCapacity string

	Saved    SaveReport
	Delivery DeliveryReport
}

// Activate uses a combat ability or feature the member already carries —
// Dodge, Dash, Disengage, Help, Hide, Rage, Second Wind.
//
// # A refusal is an error, never a field
//
// The rulebook underneath answers every refusal as a SUCCESSFUL call carrying
// a false: "not in combat", "unknown ability", "no rage uses remaining". This
// verb does not propagate that shape. An ability that said no comes back
// [ErrCannotActivate] with its own words attached; a malformed activation
// comes back [ErrBadActivation]. The split is [ErrCannotAfford] versus
// [ErrBadCost]'s, for the same reason — one wants a different verb, the other
// wants a developer.
//
// [ErrCannotActivate] is NOT reachable through this verb today, and saying so
// is better than implying a path. [Manager.Afford] consults the same gates the
// sheet does, so an unavailable ability is refused as a stale selector before
// the sheet is ever asked, and the one combat ability with a precondition
// beyond the economy is Help — whose missing target the activation machine
// catches first, as [ErrBadActivation].
//
// The arm exists anyway, and not as a placeholder. The sheet's contract
// genuinely answers refusals as a successful call carrying a false, so a verb
// that ASSUMED the two gates always agree would report a refusal as a
// successful activation that did nothing. This is that assumption declined.
// Pinned in translate_internal_test.go, where the reachability is written down
// beside it.
//
// # Nothing is charged here
//
// [resolution.Input.Cost] stays nil, deliberately. The ability spends its own
// slot, so a Cost passed alongside would charge the same ledger twice and the
// second charge would look exactly like the first. This is the one executing
// verb at this seam that does not pay at the door, and it is not free — see
// [resolution.NewActivation].
//
// # Everyone is in the cast
//
// R3: pass everyone in. Rage lands on its owner alone, but Help aids an ally
// and Hide is judged against whoever is watching — and more to the point, the
// condition an activation publishes is applied by the OWNER'S keeper, which is
// only attached for participants. A cast trimmed to "whoever this obviously
// affects" would be a rule decided here, in the one place that cannot see it.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoDeclarationID,
// ErrNoSession, ErrNoEncounter, ErrNoMember, ErrNotACharacter, ErrNotYourTurn,
// ErrDowned, ErrStaleDeclaration, ErrNoSheet, ErrNoCharacter, ErrBadCharacter,
// ErrBadRepository, ErrBadCost, ErrCannotActivate, ErrBadActivation,
// ErrInvalidWorld, ErrClosed, or ErrSaveFailed with a populated report.
//
// The two the list gains over an obvious reading, since both come from paths
// this verb walks rather than from the ones it names: ErrBadCharacter covers a
// cast member whose sheet will not load — everyone is a participant here (see
// above), so ANY unreadable member refuses the whole activation rather than
// just the actor's own — and ErrInvalidWorld covers a machine that returned an
// outcome this verb has no case for, which is a provider defect this fails
// closed on rather than reporting as a successful activation that did nothing.
func (m *Manager) Activate(ctx context.Context, in *ActivateInput) (*ActivateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("activate: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("activate: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return nil, fmt.Errorf("activate: %w", translate(err))
	}
	kind, inRoster := encounter.MemberKind(""), false
	for _, member := range roster {
		if string(member.ID) == in.Member {
			kind, inRoster = member.Kind, true
			break
		}
	}
	if !inRoster {
		return nil, fmt.Errorf("activate: member %q: %w", in.Member, ErrNoMember)
	}
	if kind != encounter.MemberKind(KindPlayer) {
		// A monster is in the roster and still cannot declare: its abilities
		// are driven by behaviour rather than chosen, and its economy belongs
		// to whoever runs its turn.
		return nil, fmt.Errorf("activate: member %q: %w", in.Member, ErrNotACharacter)
	}

	// NOT YOUR TURN, checked before anything touches character storage — the
	// precedence Attack and Move both keep.
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, fmt.Errorf("activate: %w", translate(err))
	}
	if ClockKind(clock.Kind) == ClockTurn && string(clock.Active) != in.Member {
		return nil, fmt.Errorf("activate: member %q: %w", in.Member, ErrNotYourTurn)
	}
	if ClockKind(clock.Kind) != ClockTurn {
		// There are no world-clock activations. Afford returns no declarations
		// there at all, so every selector is stale by construction — and an
		// empty one is still the caller's omission rather than a stale offer.
		if in.DeclarationID == "" {
			return nil, fmt.Errorf("activate: %w", ErrNoDeclarationID)
		}
		return nil, fmt.Errorf("activate: %w", ErrStaleDeclaration)
	}

	// Loaded strictly ONCE. Standing and every piece of the regenerated offer
	// come from this same snapshot, so a repository cannot answer standing to
	// one gate and downed to compilation.
	actor := m.loadActorSheet(ctx, in.Member)
	if actor.downed {
		return nil, fmt.Errorf("activate: member %q: %w", in.Member, ErrDowned)
	}
	if in.DeclarationID == "" {
		return nil, fmt.Errorf("activate: %w", ErrNoDeclarationID)
	}

	// Regenerate and select. compileOffersFor owns identity and availability;
	// selectCompiledOffer refuses a selector that names nothing, names two
	// things, or names an offer that is no longer available — all as
	// ErrStaleDeclaration, because from a client's side they are the same
	// event: you saw an offer and the world moved.
	offers, err := m.compileOffersFor(
		ctx, scope.enc, scope.data, scope.session, in.Member, clock, actor, VerbActivate,
	)
	if err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}
	selected, err := selectCompiledOffer(offers, VerbActivate, in.DeclarationID)
	if err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}
	if selected.declaration.Ability == nil || selected.sheet == nil {
		// A compiled Activate offer always carries both. Failing closed here
		// keeps a provider defect from reaching resolution as a nil ref.
		return nil, fmt.Errorf("activate: %w", ErrStaleDeclaration)
	}

	// Back through core.ParseString, because the declaration carries the ref
	// as the STRING a client echoes. It round-trips by construction — the
	// declaration was built from a *core.Ref moments ago — so a failure here
	// is a provider defect rather than bad input, and it says so.
	ability, err := core.ParseString(selected.declaration.Ability.Ref)
	if err != nil {
		return nil, fmt.Errorf("activate: %w: unreadable ability ref %q: %v",
			ErrBadActivation, selected.declaration.Ability.Ref, err)
	}

	machine, err := resolution.NewActivation(&resolution.ActivationInput{
		MemberID:                   in.Member,
		Ability:                    ability,
		TargetID:                   in.Target,
		ObserverPassivePerceptions: in.ObserverPassivePerceptions,
	})
	if err != nil {
		return nil, fmt.Errorf("activate: %w", translateResolution(err))
	}

	// The readied sheet goes into the cast rather than being fetched again:
	// compileOffersFor readied this turn's economy on it, and a second read
	// would hand resolution a ledger that had not been filled.
	readied := selected.sheet.ToData()
	cast, failures := m.compileResolutionCast(ctx, scope.data, roster, readied)
	if len(failures) > 0 {
		return nil, fmt.Errorf("activate: participant %q: %w: %v",
			failures[0].member, ErrBadCharacter, failures[0].err)
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
		Machine:       machine,
		// Cost is nil ON PURPOSE — see this verb's own doc.
		Roller: &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("activate: %w", translateResolution(err))
	}

	activated, ok := out.Outcome.(resolution.ActivationOutcome)
	if !ok {
		return nil, fmt.Errorf("activate: %w: activation produced %T", ErrInvalidWorld, out.Outcome)
	}

	if err := m.adopt(scope, out.World); err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}
	if err := m.saveDirty(ctx, scope, out); err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("activate: %w", err)
	}

	return &ActivateOutput{
		Ability:         activated.Ability,
		GrantedCapacity: activated.GrantedCapacity,
		Saved:           report,
		Delivery:        delivery,
	}, nil
}
