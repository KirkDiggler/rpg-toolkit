// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// act.go is THE ACT ID (rpg-project#627, ideas/event-stream/design.md R3,
// R20, R24, Law 2): one act is one verb, and every beat appended inside it
// carries the act's id as its record correlation.
//
// THE ID IS `<verb>:<first seq>`, minted here — the seq the act's first beat
// takes, which is the story's next seq when the act opens. The story owns its
// own sequence, so no two acts in one story can share an id and no caller has
// to invent one. [Encounter.RecordEquip]'s `equip:<first>` was the template.
//
// ACTS NEST. The open acts are a stack, and a beat carries the top of it. A
// driven creature's walk opened inside a player's end_turn names the walk's
// beats only; closing it restores the end_turn act for the turn-ended beat
// that follows. A reaction REOPENS its trigger's id rather than minting one
// (Law 2), which is what [OpenActInput.Act] is for.
//
// THE STACK IS NEVER PERSISTED. An act lives inside one call. The one act
// that outlives a call is a walk held by a reaction window, and it travels on
// the pause ([PauseData.Act]) with the acts it was opened inside
// ([PauseData.Outer]); the resume pushes both back, outermost first.
//
// EVERY ACT HAS TWO NAMES (rpg-project#628, event-stream R27). The v1alpha1
// form `<verb>:<first seq>` is the record correlation, unchanged from E1. The
// v1alpha2 form `<verb>:<id>` is minted at the same moment from the record's
// own id space ([record.Log.MintID]) — the act's own id, not a lookup of its
// first beat — and stamped on every beat of the act as the record tag
// [ActTag]. Both travel together on the stack, on a pause, and through a
// reopen; v1's form leaves when v1alpha1's stream is deleted.

// ActTag is the record tag every beat appended inside an act carries: the
// act's v1alpha2 id, `<verb>:<id>`. Absent on a beat outside any act, and on
// a beat of an act reopened without its v2 id (a pause or window saved
// before v2 ids existed).
const ActTag = "act"

// actRef is one open act under both of its names.
type actRef struct {
	// corr is the v1alpha1 form, `<verb>:<first seq>`: the record
	// correlation.
	corr string
	// id is the v1alpha2 form, `<verb>:<id>`: the [ActTag] value. Empty on an
	// act reopened without one.
	id string
}

// OpenActInput names the act to open.
type OpenActInput struct {
	// Verb is what the act is, in snake_case ("move", "attack", "speak").
	// The minted id is Verb + ":" + the story's next seq. Ignored when Act
	// is set.
	Verb string

	// Act reopens an existing act by its v1alpha1 id (`<verb>:<first seq>`)
	// instead of minting one: a reaction answering its trigger, or a walk
	// continued in a later call.
	Act string

	// ID is the reopened act's v1alpha2 id (`<verb>:<id>`), carried beside
	// Act. Optional on a reopen: an act reopened without it — saved before
	// v2 ids existed — stamps no [ActTag]. Refused without Act.
	ID string
}

// OpenActOutput reports the act now open.
type OpenActOutput struct {
	// Act is the open act's v1alpha1 id, which every beat appended until the
	// matching [Encounter.CloseAct] carries as its correlation unless it
	// names its own.
	Act string

	// ID is the open act's v1alpha2 id, which those beats carry as the
	// [ActTag] record tag.
	ID string
}

// OpenAct pushes an act onto the stack. An empty Act mints both names,
// `<Verb>:<first seq>` and `<Verb>:<id>`; a non-empty Act reopens that act
// with the ID given beside it. Every OpenAct is paired with one
// [Encounter.CloseAct].
//
// Errors: ErrNilInput; ErrInvalidData when both Verb and Act are empty, or
// when ID is given without Act; a story read or id mint failure.
func (e *Encounter) OpenAct(in *OpenActInput) (*OpenActOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("open act: %w", ErrNilInput)
	}
	act, err := e.mintAct(in.Verb, actRef{corr: in.Act, id: in.ID})
	if err != nil {
		return nil, err
	}
	e.acts = append(e.acts, act)
	return &OpenActOutput{Act: act.corr, ID: act.id}, nil
}

// mintAct is the act OpenAct pushes: reopen as it is when it names an act,
// otherwise a fresh act for verb under both names.
func (e *Encounter) mintAct(verb string, reopen actRef) (actRef, error) {
	if reopen.corr != "" {
		return reopen, nil
	}
	if reopen.id != "" {
		return actRef{}, fmt.Errorf("open act: v2 id %q names no act: %w", reopen.id, ErrInvalidData)
	}
	if verb == "" {
		return actRef{}, fmt.Errorf("open act: names no verb and no act: %w", ErrInvalidData)
	}
	next, err := e.NextStorySeq()
	if err != nil {
		return actRef{}, fmt.Errorf("open act %q: %w", verb, err)
	}
	id, err := e.story.MintID()
	if err != nil {
		return actRef{}, fmt.Errorf("open act %q: %w", verb, err)
	}
	return actRef{corr: fmt.Sprintf("%s:%d", verb, next), id: verb + ":" + id}, nil
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

// PausedAct returns the act the held pause belongs to — the walk or strike a
// reaction window interrupted — or "" when nothing is paused or the pause was
// saved before acts existed.
func (e *Encounter) PausedAct() string {
	if e.pause == nil {
		return ""
	}
	return e.pause.act.corr
}

// PausedActID is [Encounter.PausedAct]'s act under its v1alpha2 id, or ""
// when nothing is paused or the pause was saved without one.
func (e *Encounter) PausedActID() string {
	if e.pause == nil {
		return ""
	}
	return e.pause.act.id
}

// CurrentAct returns the open act's id — the one a beat appended now carries
// — or "" when no act is open.
//
// A HOST READS IT FROM INSIDE A CAPABILITY. A [Mover] or [Striker] that
// poses a reaction window is called while the driven intent's own act is
// open and before the encounter stores its pause, so this, not
// [Encounter.PausedAct], is what names the act the window answers.
func (e *Encounter) CurrentAct() string {
	return e.currentAct().corr
}

// CurrentActID is [Encounter.CurrentAct]'s act under its v1alpha2 id — the
// [ActTag] a beat appended now carries — or "" when no act is open or the
// open act was reopened without one.
func (e *Encounter) CurrentActID() string {
	return e.currentAct().id
}

// currentAct is the top of the stack, or the zero act when none is open.
func (e *Encounter) currentAct() actRef {
	if len(e.acts) == 0 {
		return actRef{}
	}
	return e.acts[len(e.acts)-1]
}

// inAct runs fn inside an act: a freshly minted one when act is empty, or
// act reopened. The act is closed however fn returns.
func (e *Encounter) inAct(verb string, act actRef, fn func() error) error {
	depth := len(e.acts)
	if _, err := e.OpenAct(&OpenActInput{Verb: verb, Act: act.corr, ID: act.id}); err != nil {
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
func (e *Encounter) outerActs() []actRef {
	if len(e.acts) < 2 {
		return nil
	}
	return append([]actRef(nil), e.acts[:len(e.acts)-1]...)
}

// pushActs pushes acts, outermost first, and returns the restore that
// truncates the stack back to the depth it had before the push.
func (e *Encounter) pushActs(acts ...actRef) func() {
	depth := len(e.acts)
	for _, a := range acts {
		if a.corr != "" {
			e.acts = append(e.acts, a)
		}
	}
	return func() { e.acts = e.acts[:depth] }
}

// underAct runs fn inside act when there is one, and as it is when act is
// empty — a pause saved before acts existed resumes with no act id, which is
// degraded rather than refused.
func (e *Encounter) underAct(act actRef, fn func() error) error {
	if act.corr == "" {
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

// actRefsToData splits acts into their persisted v1 and v2 lists, index for
// index. The v2 list is nil when no act carries a v2 id, so a pause that
// never had one writes exactly what it wrote before.
func actRefsToData(acts []actRef) (corrs, ids []string) {
	if len(acts) == 0 {
		return nil, nil
	}
	corrs = make([]string, len(acts))
	any := false
	for i, a := range acts {
		corrs[i] = a.corr
		any = any || a.id != ""
	}
	if !any {
		return corrs, nil
	}
	ids = make([]string, len(acts))
	for i, a := range acts {
		ids[i] = a.id
	}
	return corrs, ids
}

// actRefsFromData zips the persisted v1 and v2 lists back into acts. ids is
// either empty (written before v2 ids existed) or as long as corrs — which
// validatePause has already checked.
func actRefsFromData(corrs, ids []string) []actRef {
	if len(corrs) == 0 {
		return nil
	}
	acts := make([]actRef, len(corrs))
	for i, c := range corrs {
		acts[i].corr = c
		if i < len(ids) {
			acts[i].id = ids[i]
		}
	}
	return acts
}
