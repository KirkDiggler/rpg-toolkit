// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// doors.go carries the door verbs across the seam (rpg-project#268, design
// rpg-project#269; reworked for concealment on rpg-toolkit#1375): Doors reads
// door state AS ONE MEMBER KNOWS IT, OpenDoor pushes a shut one open, and
// Unlock carries the lock's verdict — the session stages the member's stored
// record, resolution resolves their best listed approach with the check
// chain live (conceal.go's resolveStagedCheck → resolution.MakeCheck, the
// re-ruled shape on rpg-project#351), and this seam TELLS the composition
// Beaten plus the route it took. The composition compares nothing
// (encounter.UnlockInput's law); what a total meeting the DC means lives
// behind resolution's door with the rest of the check.

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// DoorsInput asks for a session's doors, as one member knows them.
type DoorsInput struct {
	// Session is the session to read.
	Session string

	// Member is whose knowledge answers. Required — a concealed door the
	// member has not had revealed is absent from the list, exactly as it is
	// absent from their Atlas (rpg-toolkit#1375); for a world with no
	// concealment the list is every door, exactly as before.
	//
	// THE HOST MUST BIND Member TO THE AUTHENTICATED CALLER —
	// [AtlasInput.Member]'s own law.
	Member string
}

// DoorsOutput carries every door the member knows, with live state.
type DoorsOutput struct {
	// Doors, in the composition's own stable order.
	Doors []Door `json:"doors"`
}

// Doors reports each known door's identity and live state — the dynamic half
// of the Atlas's doorways. The Atlas says where a door's edges are; this says
// what each door is doing now. A host reads it once per member and keeps it
// fresh from EventDoor and EventDoorRevealed beats.
//
// Answered from [encounter.Encounter.DoorsFor] exclusively: the unscoped read
// is the host's internal whole truth and does not cross this seam for a
// member-shaped question.
//
// Returns ErrNilInput, ErrNoSessionID, ErrNoMemberID, ErrNoSession,
// ErrNoEncounter, or ErrNoMember if the member is not in this encounter.
func (m *Manager) Doors(ctx context.Context, in *DoorsInput) (*DoorsOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("doors: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("doors: %w", ErrNoMemberID)
	}
	enc, err := m.open(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("doors: %w", err)
	}

	doors, err := enc.DoorsFor(encounter.MemberID(in.Member))
	if err != nil {
		return nil, fmt.Errorf("doors: %w", translate(err))
	}
	out := &DoorsOutput{Doors: make([]Door, 0, len(doors))}
	for _, d := range doors {
		out.Doors = append(out.Doors, projectDoor(d))
	}
	return out, nil
}

// OpenDoorInput names the door and who pushes it.
type OpenDoorInput struct {
	// Session is the session to act in.
	Session string

	// Member is who pushes it open — the beat's actor.
	Member string

	// Door is the door's identifier, as the Atlas's doorways name it.
	Door string
}

// OpenDoorOutput reports the door as it now stands and what opening it
// revealed.
type OpenDoorOutput struct {
	// Door is the door, open now.
	Door Door `json:"door"`

	// Discovered is what the refresh brought into each observer's view —
	// an opened door is the whole reason the verb refreshes sight.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Formed is present when what the door revealed started a fight.
	Formed *Formed `json:"formed,omitempty"`

	// Seq is the door beat's sequence in the ACTOR's own delivered
	// numbering (stream.go) — the same number the actor's event for this
	// beat carries.
	Seq uint64 `json:"seq"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// OpenDoor pushes a shut door open as Member.
//
// A locked door refuses with ErrLocked. An already-open door refuses with
// ErrNoConnection, the composition's own refusal translated — and so does a
// door that does not exist FOR THIS MEMBER: a concealed door the member has
// not found answers exactly like no door at all (the composition's probe
// law), so nothing here may look the door up first and answer differently.
func (m *Manager) OpenDoor(ctx context.Context, in *OpenDoorInput) (*OpenDoorOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("opendoor: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("opendoor: %w", ErrNoMemberID)
	}
	if in.Door == "" {
		return nil, fmt.Errorf("opendoor: %w", ErrNoConnection)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("opendoor: %w", err)
	}

	opened, err := scope.enc.OpenDoor(&encounter.OpenDoorInput{
		Door:  in.Door,
		Actor: encounter.MemberID(in.Member),
	})
	if err != nil {
		return nil, fmt.Errorf("opendoor: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("opendoor: %w", err)
	}

	return &OpenDoorOutput{
		Door:       Door{ID: opened.Door, State: string(opened.State)},
		Discovered: projectDiscoveries(opened.IntelDeltas),
		Formed:     projectFormedFor(scope, in.Member, opened.Formed),
		Seq:        scope.deliveredSeq(in.Member, opened.Seq),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// UnlockInput names the lock and whose hands try it.
type UnlockInput struct {
	// Session is the session to act in.
	Session string

	// Member is whose hands — and whose sheet — try the lock.
	Member string

	// Door is the door's identifier.
	Door string
}

// UnlockOutput is the attempt, in the open: the roll is public down to the
// number (full data until v1.0), and the beat every member hears carries the
// same facts.
type UnlockOutput struct {
	// Beaten is whether the check beat the applied route's DC. The session
	// rolled it; the composition was told this verdict and nothing else.
	Beaten bool `json:"beaten"`

	// Total is what the check totalled. No omitempty: zero is an answer
	// (TestFalseIsAnAnswerOnTheWire's law).
	Total int `json:"total"`

	// DC is what it had to reach — the APPLIED route's own difficulty,
	// never another listed route's (each is priced separately,
	// rpg-project#350).
	DC int `json:"dc"`

	// Applied is the route the attempt actually took — chosen by this seam
	// as the member's best listed approach (conceal.go's mechanism ruling).
	Applied DoorApproach `json:"applied"`

	// Door is the door as it now stands: open when beaten — a beaten lock
	// OPENS the door, it does not merely unlock it — unchanged and
	// retryable when not.
	Door Door `json:"door"`

	// Discovered is what a beaten lock brought into view.
	Discovered map[string]Discovery `json:"discovered,omitempty"`

	// Formed is present when what the opened door revealed started a fight.
	Formed *Formed `json:"formed,omitempty"`

	// Seq is the door beat's sequence in the ACTOR's own delivered
	// numbering (stream.go).
	Seq uint64 `json:"seq"`

	// Paused is true when the attempt stopped to ask the member whether to
	// spend a held offer before the lock's verdict is settled —
	// [AttackOutput.Paused]'s shape, applied to a check instead of a swing.
	// Beaten, DC, Applied, Door.State's post-verdict value, Discovered and
	// Formed are all the zero value while this is true: there is no verdict
	// yet, only a question. Answer it with [Manager.React].
	Paused bool `json:"paused,omitempty"`

	// Roll is the d20 as rolled, present only when Paused — the same
	// presence law [Declaration.Remaining] keeps for a number a verb does
	// not normally carry. THE LOCK'S DC IS DELIBERATELY NOT SURFACED
	// alongside it, for [checkOfferWindowPayload.Roll]'s reason.
	Roll *int `json:"roll,omitempty"`

	Saved    SaveReport     `json:"saved"`
	Delivery DeliveryReport `json:"delivery"`
}

// Unlock tries a locked door as Member: a real ability check against the
// lock's authored approaches, resolved through the same path Search's find
// checks take ([Manager.resolveStagedCheck] → resolution.MakeCheck — the
// rules live once, behind resolution's door). Resolution picks the member's
// best listed route; this seam fills the composition's Applied with it and
// reports that route's DC outward.
//
// THE LOCK IS READ THROUGH THE MEMBER'S OWN KNOWLEDGE
// ([encounter.Encounter.DoorsFor]): a concealed door the member has not
// found carries no lock for them, so no check is rolled and the composition
// refuses the door as nonexistent — the probe law, upheld structurally
// rather than by remembering to. A failed attempt on a known lock is an
// outcome, not an error: the door is unchanged, the attempt is narrated, and
// the caller may try again.
func (m *Manager) Unlock(ctx context.Context, in *UnlockInput) (*UnlockOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("unlock: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("unlock: %w", ErrNoMemberID)
	}
	if in.Door == "" {
		return nil, fmt.Errorf("unlock: %w", ErrNoConnection)
	}

	scope, err := m.openForChange(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}

	// The lock is read before anything rolls: the DCs and approaches are
	// the composition's facts, scoped to what this member knows. A door
	// that is not locked — or not known — rolls nothing and is refused by
	// the composition itself below, in its own words.
	doors, err := scope.enc.DoorsFor(encounter.MemberID(in.Member))
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", translate(err))
	}
	var lock encounter.Lock
	var locked bool
	for _, d := range doors {
		if d.ID == in.Door {
			lock, locked = d.State.Lock()
			break
		}
	}

	var applied encounter.CheckApproach
	var calculation *encounter.RollCalculation
	beaten, total := false, 0
	if locked {
		if err := m.stageCheck(ctx, scope, "member", in.Member); err != nil {
			return nil, fmt.Errorf("unlock: %w", err)
		}
		// FALSE: a lock is not a skill verb. Forcing a door with Strength or
		// picking it with tools rolls the 2014 letter (rpg-project#457 R2).
		outcome, verr := m.resolveStagedCheckPoseable(scope, in.Member, lock.Approaches, false)
		if verr != nil {
			return nil, fmt.Errorf("unlock %q: %w", in.Door, verr)
		}

		// THE ATTEMPT STOPPED TO ASK. The checker holds something that could
		// join the roll — Guidance is the first — and resolution posed
		// rather than settling. Nothing about the lock has happened yet, so
		// there is no beat to write beyond the pause itself.
		if outcome.Posed != nil {
			return m.poseUnlockWindow(ctx, scope, in, outcome.Posed)
		}

		beaten, total, applied = outcome.Verdict.Beaten, outcome.Verdict.Total, outcome.Verdict.Applied
		calculation = outcome.Verdict.Calculation
	}

	unlocked, err := scope.enc.Unlock(&encounter.UnlockInput{
		Door:    in.Door,
		Beaten:  beaten,
		Actor:   encounter.MemberID(in.Member),
		Total:   total,
		Applied: applied,
		// An unlock's DoorChanged is a check beat and carries the roll behind
		// it (rpg-project#462 R4). Nil on a door with no lock to beat, which
		// rolled nothing.
		Calculation: calculation,
	})
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", translate(err))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}

	return &UnlockOutput{
		Beaten:     unlocked.Beaten,
		Total:      total,
		DC:         unlocked.Applied.DC,
		Applied:    projectApproach(unlocked.Applied),
		Door:       Door{ID: unlocked.Door, State: string(unlocked.State)},
		Discovered: projectDiscoveries(unlocked.IntelDeltas),
		Formed:     projectFormedFor(scope, in.Member, unlocked.Formed),
		Seq:        scope.deliveredSeq(in.Member, unlocked.Seq),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// poseUnlockWindow commits the half of the attempt that happened and asks
// the member the question resolution stopped on — [poseAttackWindow]'s
// shape, for a lock check instead of a swing.
//
// The encounter is not paused for the same reason a posed attack does not
// pause it: Unlock is not a driven turn, so this question lives entirely in
// the interrupt ledger already persisted here.
func (m *Manager) poseUnlockWindow(
	ctx context.Context, scope *writeScope, in *UnlockInput, posed *resolution.Pose,
) (*UnlockOutput, error) {
	ask := posed.Ask
	if ask.Audience != in.Member {
		// R5, checked on this side of the seam too: this build poses to the
		// checker and to nobody else.
		return nil, fmt.Errorf("unlock: %w: the machine asked %q on %q's roll",
			ErrInvalidWorld, ask.Audience, in.Member)
	}
	if ask.Offer.Ref == nil || ask.Offer.Name == "" {
		return nil, fmt.Errorf("unlock: %w: the machine asked about an unnamed offer", ErrInvalidWorld)
	}
	if len(ask.Options) != 2 {
		return nil, fmt.Errorf("unlock: %w: the machine posed %d answers and this seam poses two",
			ErrInvalidWorld, len(ask.Options))
	}

	offer := ReactionRef{Ref: ask.Offer.Ref.String(), Name: ask.Offer.Name}
	payload, err := marshalCheckOfferPayload(checkOfferWindowPayload{
		Audience:    ask.Audience,
		Door:        in.Door,
		Offer:       offer,
		Roll:        ask.Roll,
		Total:       ask.Total,
		Calculation: sessionRollCalculationOf(ask.Calculation),
		Frozen:      posed.Frozen,
	})
	if err != nil {
		return nil, fmt.Errorf("unlock: %w: %v", ErrInvalidSession, err)
	}

	// THE TWO ANSWERS ARE THIS SEAM'S, not the machine's: [poseAttackWindow]'s
	// own reasoning, reused rather than re-derived.
	if _, err := scope.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(ask.Audience),
		Options:  []interrupt.Option{interrupt.Option(ReactStrike), interrupt.Option(ReactHold)},
		Payload:  payload,
		At:       scope.baseline,
	}); err != nil {
		return nil, fmt.Errorf("unlock: %w: %v", ErrInvalidSession, err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true

	recorded, err := scope.enc.RecordRollWindow(&encounter.RollWindowInput{
		Audience: encounter.MemberID(ask.Audience),
		Offer:    encounter.ReactionIdentity{Ref: offer.Ref, Name: offer.Name},
		Roll:     ask.Roll,
		Total:    ask.Total,
		// The beat that asks shows the whole roll, not the one face the two
		// scalars could carry (rpg-project#462 R5).
		Calculation: rollCalculationFor(ask.Calculation),
	})
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", reportUnrecorded(scope, translate(err)))
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("unlock: %w", err)
	}

	roll := ask.Roll
	return &UnlockOutput{
		Paused:   true,
		Total:    ask.Total,
		Roll:     &roll,
		Seq:      scope.deliveredSeq(in.Member, recorded.Seq),
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// projectDoor is the seam's one projection of a door: the sealed state
// interface becomes a string kind, and the lock rides only while it is real.
func projectDoor(d encounter.Door) Door {
	out := Door{ID: d.ID, State: string(d.State.Kind())}
	if lock, locked := d.State.Lock(); locked {
		approaches := make([]DoorApproach, 0, len(lock.Approaches))
		for _, a := range lock.Approaches {
			approaches = append(approaches, projectApproach(a))
		}
		out.Lock = &DoorLock{Approaches: approaches}
	}
	return out
}

// projectApproach is the one translation of an authored check route (S2: the
// composition's type becomes this package's).
func projectApproach(a encounter.CheckApproach) DoorApproach {
	return DoorApproach{Ability: a.Ability, Tool: a.Tool, DC: a.DC}
}

// checkApproachesOf is [projectApproach] the other way, for a check arriving
// from a host rather than leaving for one ([SpawnInput.Intimidate]).
//
// NIL STAYS NIL, and it is load-bearing: an absent check means the rulebook
// derives the DC from the monster's own stat block, and an empty-but-present
// list would be refused as a check with no way through it.
func checkApproachesOf(approaches []DoorApproach) []encounter.CheckApproach {
	if approaches == nil {
		return nil
	}
	out := make([]encounter.CheckApproach, 0, len(approaches))
	for _, a := range approaches {
		out = append(out, encounter.CheckApproach{Ability: a.Ability, Tool: a.Tool, DC: a.DC})
	}

	return out
}
