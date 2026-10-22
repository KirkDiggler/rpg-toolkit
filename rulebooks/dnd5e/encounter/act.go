// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// act.go is THE ACT ID (rpg-project#627, ideas/event-stream/design.md R3,
// R20, R24, R27, Law 2): one act is one verb, and every beat appended inside
// it carries the act's id as its record correlation and as the [ActTag] tag.
//
// THE ID IS `<verb>:<id>`, minted here from the record's own id space
// ([record.Log.MintID]) — the act's own id, not a lookup of its first beat.
// The record owns its id space, so no two acts in one story can share an id
// and no caller has to invent one. E1's `<verb>:<first seq>` form left with
// the v1alpha1 stream (R27); a beat recorded before then keeps the
// correlation it was written with.
//
// ACTS NEST. The open acts are a stack, and a beat carries the top of it. A
// driven creature's walk opened inside a player's end_turn names the walk's
// beats only; closing it restores the end_turn act for the turn-ended beat
// that follows. A reaction REOPENS its trigger's id rather than minting one
// (Law 2), which is what [OpenActInput.Act] is for.
//
// THE STACK IS NEVER PERSISTED. An act lives inside one call. The one act
// that outlives a call is a walk held by a reaction window, and it travels on
// the pause ([PauseData.ActID]) with the acts it was opened inside
// ([PauseData.OuterIDs]); the resume pushes both back, outermost first.

// ActTag is the record tag every beat appended inside an act carries: the
// act's id, `<verb>:<id>`, the same value as the beat's correlation. Absent
// on a beat outside any act.
const ActTag = "act"

// OpenActInput names the act to open.
type OpenActInput struct {
	// Verb is what the act is, in snake_case ("move", "attack", "speak").
	// The minted id is Verb + ":" + a fresh record id. Ignored when Act is
	// set.
	Verb string

	// Act reopens an existing act by its id (`<verb>:<id>`) instead of
	// minting one: a reaction answering its trigger, or a walk continued in
	// a later call.
	Act string
}

// OpenActOutput reports the act now open.
type OpenActOutput struct {
	// Act is the open act's id, which every beat appended until the
	// matching [Encounter.CloseAct] carries as its correlation, unless it
	// names its own, and as its [ActTag] tag.
	Act string
}

// OpenAct pushes an act onto the stack. An empty Act mints `<Verb>:<id>`; a
// non-empty Act reopens that act. Every OpenAct is paired with one
// [Encounter.CloseAct].
//
// Errors: ErrNilInput; ErrInvalidData when both Verb and Act are empty; an id
// mint failure.
func (e *Encounter) OpenAct(in *OpenActInput) (*OpenActOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("open act: %w", ErrNilInput)
	}
	act, err := e.mintAct(in.Verb, in.Act)
	if err != nil {
		return nil, err
	}
	e.acts = append(e.acts, act)
	return &OpenActOutput{Act: act}, nil
}

// mintAct is the act OpenAct pushes: reopen as it is when it names an act,
// otherwise a fresh act for verb.
func (e *Encounter) mintAct(verb, reopen string) (string, error) {
	if reopen != "" {
		return reopen, nil
	}
	if verb == "" {
		return "", fmt.Errorf("open act: names no verb and no act: %w", ErrInvalidData)
	}
	id, err := e.story.MintID()
	if err != nil {
		return "", fmt.Errorf("open act %q: %w", verb, err)
	}
	return verb + ":" + id, nil
}

// CloseAct pops the act [Encounter.OpenAct] opened last, restoring the act
// it was opened inside, if any.
//
// Errors: ErrInvalidData when no act is open.
func (e *Encounter) CloseAct() error {
	if len(e.acts) == 0 {
		return fmt.Errorf("close act: no act is open: %w", ErrInvalidData)
	}
	e.acts = e.acts[:len(e.acts)-1]
	return nil
}

// PausedAct returns the id of the act the held pause belongs to — the walk
// or strike a reaction window interrupted — or "" when nothing is paused or
// the pause was saved without one.
func (e *Encounter) PausedAct() string {
	if e.pause == nil {
		return ""
	}
	return e.pause.act
}

// CurrentAct returns the open act's id — the one a beat appended now carries
// — or "" when no act is open.
//
// A HOST READS IT FROM INSIDE A CAPABILITY. A [Mover] or [Striker] that
// poses a reaction window is called while the driven intent's own act is
// open and before the encounter stores its pause, so this, not
// [Encounter.PausedAct], is what names the act the window answers.
func (e *Encounter) CurrentAct() string {
	if len(e.acts) == 0 {
		return ""
	}
	return e.acts[len(e.acts)-1]
}

// inAct runs fn inside an act: a freshly minted one when act is empty, or
// act reopened. The act is closed however fn returns.
func (e *Encounter) inAct(verb, act string, fn func() error) error {
	depth := len(e.acts)
	if _, err := e.OpenAct(&OpenActInput{Verb: verb, Act: act}); err != nil {
		return err
	}
	// TO THE DEPTH RECORDED AT OPEN, not one entry: a capability that opened
	// an act and left it open, or closed one too many, cannot shift the id
	// every later beat of the call carries.
	defer func() { e.acts = e.acts[:depth] }()
	return fn()
}

// outerActs is a copy of every act open beneath the current one, outermost
// first — what a pause keeps beside its own act so the beats after the
// resumed walk land in the act the walk was opened inside (a driven walk's
// end_turn), not in whatever the resuming caller has open.
func (e *Encounter) outerActs() []string {
	if len(e.acts) < 2 {
		return nil
	}
	return append([]string(nil), e.acts[:len(e.acts)-1]...)
}

// pushActs pushes acts, outermost first, and returns the restore that
// truncates the stack back to the depth it had before the push.
func (e *Encounter) pushActs(acts ...string) func() {
	depth := len(e.acts)
	for _, a := range acts {
		if a != "" {
			e.acts = append(e.acts, a)
		}
	}
	return func() { e.acts = e.acts[:depth] }
}

// underAct runs fn inside act when there is one, and as it is when act is
// empty — a pause saved without an act id resumes with none, which is
// degraded rather than refused.
func (e *Encounter) underAct(act string, fn func() error) error {
	if act == "" {
		return fn()
	}
	return e.inAct("", act, fn)
}

// intentVerb names the act a driven [TurnIntent] is: a walk of either kind
// is a move, a swing an attack, a pass a pass.
func intentVerb(intent TurnIntent) string {
	switch intent.(type) {
	case Move, Routed:
		return "move"
	case Attack:
		return "attack"
	case Pass:
		return "pass"
	default:
		return "intent"
	}
}
