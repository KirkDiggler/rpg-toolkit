// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// PauseVersion is written into every frozen header and checked on every
// resume. It sits above every version an earlier build wrote (the strike wrote
// 3), so no old blob can pass for a new one.
const PauseVersion = 4

// PauseKind names the question a pause asks. It is the innermost question;
// the containers that hold it (sequence, movement, cast, contest) are machine
// state, never a kind.
type PauseKind string

const (
	// PauseBeforeRoll is an attack-roll reaction offered before the d20.
	PauseBeforeRoll PauseKind = "before_roll"
	// PausePostRoll is an offer on an attack's d20.
	PausePostRoll PauseKind = "post_roll"
	// PausePostHit is a reaction to a settled hit.
	PausePostHit PauseKind = "post_hit"
	// PauseCheckRoll is an offer on an ability check's d20.
	PauseCheckRoll PauseKind = "check_roll"
	// PauseSaveRoll is an offer on a saving throw's d20.
	PauseSaveRoll PauseKind = "save_roll"
	// PauseOpportunity asks a player whether to swing at a mover. It stops
	// the step (E8): the step settles on the resume, after the swing, and
	// reach is measured in the world before the step.
	PauseOpportunity PauseKind = "opportunity"
)

// Pause is a machine stopped for an answer from outside the process.
//
// It is the only suspension. Everything that settled before the machine
// stopped leaves on [Output.Outcome]; the pause carries only the question, its
// price, and the machine state needed to finish.
//
// # Frozen is opaque on purpose
//
// The bytes are authored by the machine and read back only by [Resume] (or
// [ResumeCheck]). They begin with one header — {"v", "machine", "kind",
// "state"} — whose version is [PauseVersion]. What a caller does with them is
// store them and hand them back.
//
// # One pause per run
//
// A machine that pauses twice in one call is not designed here and is not
// refused here: the driver returns the FIRST pause and stops.
type Pause struct {
	// Kind is the innermost question this pause asks.
	Kind PauseKind `json:"kind"`

	// Ask is the whole question.
	Ask Ask `json:"ask"`

	// Cost is the price of [Take], from the one price table. Nil is free. It
	// is charged at the resume, by the one door, at exactly this price; a host
	// never hands a price back.
	Cost *Cost `json:"cost,omitempty"`

	// Frozen is the machine's own state behind the one header. Opaque to
	// every caller.
	Frozen []byte `json:"frozen"`

	// settled is what the machine settled before it paused, carried from the
	// machine to the driver. Never marshalled.
	settled Outcome

	// followUps are the concentration checks rolled before the pause that no
	// settled unit carries — a cast's earlier targets, or a hit inside a cast,
	// which is one told unit and tells nothing at its pause. The driver
	// attributes them to the paused output with everything else that settled,
	// so a check's roll and the break it caused are told once, at the pause.
	// Never marshalled.
	followUps []FollowUpOutcome
}

func (Pause) isStep() {}

// Ask is the whole question: who is asked, what is offered, and the numbers
// they need to decide with.
//
// It is DATA. A caller renders it, stores it, restarts the process, and answers
// it later; nothing about the machine that paused survives except
// [Pause.Frozen].
type Ask struct {
	// Audience is the member being asked.
	Audience string `json:"audience"`

	// Offer is what is offered, in the one shape every chain's offer is
	// converted to at the pause.
	Offer Offer `json:"offer"`

	// Roll is the d20 as rolled and Total the number the offer would join.
	// TARGET AC IS DELIBERATELY ABSENT: a player who could see it would be
	// deciding "does this close the gap" rather than "is this worth spending",
	// which is a different question and a different game.
	Roll  int `json:"roll,omitempty"`
	Total int `json:"total,omitempty"`

	// Calculation is the settled pre-offer arithmetic the offered die would
	// join — the same numbers Roll and Total summarise, with the faces and the
	// keep record behind them (rpg-project#462 R5). Whatever the answer adds
	// lands on the resumed outcome's own calculation, not here.
	Calculation *dnd5eEvents.RollCalculation `json:"calculation,omitempty"`
}

// Offer is the one shape of what is offered, whichever chain declared it.
//
// The bus payloads that declare offers (dnd5eEvents.Offer, AttackRollOffer,
// PostHitOffer) stay the chains' own shapes; the machine converts at the
// pause.
type Offer struct {
	// Ref names what is offering — the condition or feature that made the
	// offer. It is the offer's source, carried so a host can name it.
	Ref core.Ref `json:"ref"`

	// Name is the offerer's display name.
	Name string `json:"name"`

	// Description is the offerer's authored prose: what taking it does.
	Description string `json:"description,omitempty"`

	// Die is set when taking rolls a die, in dice notation.
	Die string `json:"die,omitempty"`

	// SourceID is whose die it is.
	SourceID string `json:"source_id,omitempty"`

	// Choices are the options Take may carry. Empty: Take carries no option.
	Choices []Choice `json:"choices,omitempty"`
}

// Choice is one option an offer lists, authored by the offerer.
type Choice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Description is the producer's authored prose for this option. It is
	// copied from the event that declared it and never read when resuming.
	Description string `json:"description,omitempty"`
}

// Answer is [Take] or [Decline]. The zero Answer is refused: a caller that has
// not asked anybody yet has no answer to give.
type Answer struct {
	// given separates [Decline] from the zero Answer; neither takes.
	given  bool
	take   bool
	option string
}

// Take accepts the offer. option is one of the offer's [Offer.Choices] when it
// lists any, and empty when it lists none.
func Take(option string) Answer { return Answer{given: true, take: true, option: option} }

// Decline refuses the offer. Declining costs nothing.
func Decline() Answer { return Answer{given: true} }

// Taken reports whether this answer takes the offer.
func (a Answer) Taken() bool { return a.take }

// Option is the chosen option, or empty.
func (a Answer) Option() string { return a.option }

// accepts reports whether this answer is one the offer accepts: Take with an
// option exactly when the offer lists choices, the option one of them, or
// Decline with nothing. Everything else — the zero Answer first — is
// [ErrNotOffered].
func (a Answer) accepts(o Offer) error {
	switch {
	case !a.given:
		return fmt.Errorf("%w: no answer was given", ErrNotOffered)
	case !a.take:
		if a.option != "" {
			return fmt.Errorf("%w: declining carries no option", ErrNotOffered)
		}
		return nil
	case len(o.Choices) == 0:
		if a.option != "" {
			return fmt.Errorf("%w: %q offers no choices, and %q is not one", ErrNotOffered, o.Name, a.option)
		}
		return nil
	case a.option == "":
		return fmt.Errorf("%w: %q lists choices and none was taken", ErrNotOffered, o.Name)
	default:
		for _, choice := range o.Choices {
			if choice.ID == a.option {
				return nil
			}
		}
		return fmt.Errorf("%w: %q is not a choice %q offered", ErrNotOffered, a.option, o.Name)
	}
}

// frozenHeader is the one header every frozen machine writes.
type frozenHeader struct {
	V       int             `json:"v"`
	Machine string          `json:"machine"`
	Kind    PauseKind       `json:"kind"`
	State   json.RawMessage `json:"state"`
}

// The machine names a header may carry. Each is written by exactly one freeze
// site and read back by exactly one resumer.
const (
	machineBeforeRoll  = "strike.before_roll"
	machinePostRoll    = "strike.post_roll"
	machinePostHit     = "strike.post_hit"
	machineSequence    = "sequence"
	machineMovement    = "movement"
	machineOpportunity = "opportunity"
	machineCast        = "cast"
	machineContest     = "contest"
	machineSave        = "save"
	machineCheck       = "check"
)

// writeFrozen writes state behind the one header.
func writeFrozen(machine string, kind PauseKind, state any) ([]byte, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("%w: freeze %s: %v", ErrBadFrozen, machine, err)
	}
	out, err := json.Marshal(frozenHeader{V: PauseVersion, Machine: machine, Kind: kind, State: raw})
	if err != nil {
		return nil, fmt.Errorf("%w: freeze %s: %v", ErrBadFrozen, machine, err)
	}
	return out, nil
}

// readFrozen reads the one header. A version that is not [PauseVersion] is
// [ErrStalePause] — before anything else is read, loaded or charged. Anything
// else unreadable is [ErrBadFrozen].
func readFrozen(raw []byte) (frozenHeader, error) {
	if len(raw) == 0 {
		return frozenHeader{}, fmt.Errorf("%w: no frozen state", ErrBadFrozen)
	}
	var h frozenHeader
	if err := json.Unmarshal(raw, &h); err != nil {
		return frozenHeader{}, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if h.V != PauseVersion {
		return frozenHeader{}, fmt.Errorf("%w: version %d, this build writes %d", ErrStalePause, h.V, PauseVersion)
	}
	if h.Machine == "" || h.Kind == "" || len(h.State) == 0 {
		return frozenHeader{}, fmt.Errorf("%w: header names no machine, kind or state", ErrBadFrozen)
	}
	return h, nil
}

// decodeState reads a header's state into the machine's own frozen type.
func decodeState(h frozenHeader, into any) error {
	if err := json.Unmarshal(h.State, into); err != nil {
		return fmt.Errorf("%w: %s state: %v", ErrBadFrozen, h.Machine, err)
	}
	return nil
}

// ResumeInput continues any pause [Resolve] posed.
type ResumeInput struct {
	// Pause is the pause as the host stored it, whole.
	Pause Pause

	// Answer is [Take] or [Decline].
	Answer Answer

	// Roller rolls whatever the resumed machine rolls. REQUIRED whichever the
	// answer is: refusing a nil one at the door means a mis-wired caller finds
	// out on the first decline instead of the first take.
	Roller dice.Roller
}

// resumer finishes one machine's pause.
type resumer func(h frozenHeader, in *ResumeInput) (Machine, error)

// resumers is the one resume table: a header's machine name to its resumer.
// Populated in init to break the reference cycle between the table and the
// containers that resume their inner pause through it.
var resumers map[string]resumer

func init() {
	resumers = map[string]resumer{
		machineBeforeRoll:  resumeBeforeRoll,
		machinePostRoll:    resumePostRoll,
		machinePostHit:     resumePostHit,
		machineSequence:    resumeSequence,
		machineMovement:    resumeMovement,
		machineOpportunity: resumeOpportunity,
		machineCast:        resumeCast,
	}
}

// Resume returns the machine that finishes a pause. The machine is handed to
// [Resolve] exactly like a fresh one.
//
// It refuses, in this order and before anything loads or is charged: a nil
// input ([ErrNilInput]); a nil roller ([ErrNoRoller]); a frozen header whose
// version is not [PauseVersion] ([ErrStalePause]); a header whose kind is not
// the pause's ([ErrBadFrozen]); an answer the offer does not accept
// ([ErrNotOffered]); a machine this table does not hold ([ErrBadFrozen]); and,
// in the resumer, a frozen price that is not the pause's [Pause.Cost]
// ([ErrBadFrozen]).
func Resume(in *ResumeInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Roller == nil {
		return nil, fmt.Errorf("%w: a resumed machine rolls with no roller", ErrNoRoller)
	}
	h, err := readFrozen(in.Pause.Frozen)
	if err != nil {
		return nil, err
	}
	if h.Kind != in.Pause.Kind {
		return nil, fmt.Errorf("%w: frozen %q, paused %q", ErrBadFrozen, h.Kind, in.Pause.Kind)
	}
	if err := in.Answer.accepts(in.Pause.Ask.Offer); err != nil {
		return nil, err
	}
	resume, ok := resumers[h.Machine]
	if !ok {
		return nil, fmt.Errorf("%w: no resumer for machine %q", ErrBadFrozen, h.Machine)
	}
	return resume(h, in)
}

// resumeInner resumes a container's inner pause through the same table. The
// inner frozen bytes are a whole header of their own; kind and price are the
// container's, because a container's are its inner's. allowed names the
// machines this container can hold; any other is [ErrBadFrozen].
func resumeInner(inner []byte, in *ResumeInput, allowed ...string) (Machine, error) {
	h, err := readFrozen(inner)
	if err != nil {
		return nil, err
	}
	if h.Kind != in.Pause.Kind {
		return nil, fmt.Errorf("%w: inner frozen %q, paused %q", ErrBadFrozen, h.Kind, in.Pause.Kind)
	}
	held := false
	for _, name := range allowed {
		held = held || name == h.Machine
	}
	if !held {
		return nil, fmt.Errorf("%w: machine %q cannot be held here", ErrBadFrozen, h.Machine)
	}
	nested := *in
	nested.Pause.Frozen = inner
	if h.Machine == machineContest {
		return resumeContest(h, &nested)
	}
	resume, ok := resumers[h.Machine]
	if !ok {
		return nil, fmt.Errorf("%w: no resumer for inner machine %q", ErrBadFrozen, h.Machine)
	}
	return resume(h, &nested)
}

// strikeMachines are the three kinds a sequence or a walk can hold.
var strikeMachines = []string{machineBeforeRoll, machinePostRoll, machinePostHit}

// samePrice refuses a frozen price that is not the price the pause states.
// The host never hands a price back; an edited one is a blob nobody should act
// on.
func samePrice(frozen, stated *Cost) error {
	a, err := json.Marshal(frozen)
	if err != nil {
		return fmt.Errorf("%w: frozen price: %v", ErrBadFrozen, err)
	}
	b, err := json.Marshal(stated)
	if err != nil {
		return fmt.Errorf("%w: stated price: %v", ErrBadFrozen, err)
	}
	if !bytes.Equal(a, b) {
		return fmt.Errorf("%w: the pause states a price the machine did not freeze", ErrBadFrozen)
	}
	return nil
}

// offerFromRoll converts a die offer on a d20 into the one Offer.
func offerFromRoll(o dnd5eEvents.Offer) Offer {
	out := Offer{Name: o.Name, Description: o.Description, Die: o.Die, SourceID: o.SourceID}
	if o.Ref != nil {
		out.Ref = *o.Ref
	}
	return out
}

// useChoice is the one choice a before-roll reaction offers.
const useChoice = "use"

// offerFromAttackRoll converts a before-roll reaction into the one Offer.
func offerFromAttackRoll(o dnd5eEvents.AttackRollOffer) Offer {
	return Offer{
		Ref: o.Ref, Name: o.Name, Description: o.Description,
		Choices: []Choice{{ID: useChoice, Label: "Use " + o.Name}},
	}
}

// offerFromPostHit converts a post-hit reaction into the one Offer.
func offerFromPostHit(o dnd5eEvents.PostHitOffer) Offer {
	choices := make([]Choice, 0, len(o.Options))
	for _, option := range o.Options {
		choices = append(choices, Choice{ID: option.ID, Label: option.Label, Description: option.Description})
	}
	return Offer{Ref: o.Ref, Name: o.Name, Description: o.Description, Choices: choices}
}
