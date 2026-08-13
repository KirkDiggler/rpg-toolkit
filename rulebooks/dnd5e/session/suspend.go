// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// OptionKind classifies an offered choice in a way a client can branch on.
//
// It is an enumerated kind rather than a free-form string because this is the
// field clients switch on, and a field clients switch on must never be prose.
// New kinds may be added; existing ones never change meaning.
type OptionKind string

const (
	// OptionContinue resumes the suspended resolution from where it stopped.
	OptionContinue OptionKind = "continue"

	// OptionStop abandons whatever remained of the suspended resolution. What
	// already happened stays happened — stopping is not undoing.
	OptionStop OptionKind = "stop"
)

// Option is one choice offered at an open window.
type Option struct {
	// Kind is what this choice means, for a client deciding how to render it.
	Kind OptionKind `json:"kind"`

	// Token is what to pass back as AnswerInput.Option to select this choice.
	//
	// It is separate from Kind so the token can become opaque later — a signed
	// or scoped value, say — without changing how clients classify choices.
	Token string `json:"token"`
}

// Prompt is what the member who owes an answer is looking at.
//
// It carries the moment, never the mechanism. There is deliberately no field
// naming which kind of checkpoint fired: a client renders what is in front of
// the player and branches on OptionKind, so new checkpoint kinds — a trap
// underfoot, an offered reaction, something a later wave has not thought of —
// arrive without any client learning a new reason code.
type Prompt struct {
	// Member is who is being asked.
	Member string `json:"member"`

	// Room is where they stand.
	Room string `json:"room"`

	// At is the cell they stand in — where the resolution stopped, not where
	// it was headed.
	At spatial.Position `json:"at"`

	// Sighted names what just came into view, sorted, if anything did.
	Sighted []string `json:"sighted,omitempty"`
}

// Pending is an open window: a resolution stopped and is waiting on a decision.
//
// While one is open the world is frozen — every verb that would change it is
// rejected until the window is answered. Read verbs stay available, because a
// client cannot render "waiting on Alice" without asking what the world looks
// like.
type Pending struct {
	// Window identifies this window for Answer. It is opaque and stable across
	// a process restart; do not parse it.
	Window string `json:"window"`

	// Audience is the member who owes the answer. Nobody else may give it.
	Audience string `json:"audience"`

	// Options are the choices on offer. Answering with anything else is a
	// rejection.
	Options []Option `json:"options"`

	// Prompt is enough to render the moment.
	Prompt Prompt `json:"prompt"`
}

// frozenResolution is the in-between state of a suspended resolution, stored as
// opaque bytes in the window that suspended it.
//
// It is a value, never a goroutine and never a stack (S7). That is what lets a
// window outlive the process that opened it: the answer may arrive days later,
// on a different machine, and resuming is just reading this back.
//
// Kind discriminates which resolution froze. Today only a walk can suspend, but
// the field is written from the first version so that a later resolution kind
// is additive rather than an ambiguous payload nobody can safely parse.
type frozenResolution struct {
	// Kind names the resolution that froze.
	Kind string `json:"kind"`

	// Member is whose resolution it is.
	Member string `json:"member"`

	// Path is the whole walk as originally requested.
	Path []spatial.Position `json:"path"`

	// Index is how many cells of Path have actually been entered, which is the
	// explicit phase index: resuming means continuing at Path[Index].
	Index int `json:"index"`

	// Prompt is what the audience was shown, kept here so Pending can answer
	// "what is open?" from session state alone, without reloading the world and
	// trying to recompute a perception delta that has already been folded in.
	Prompt Prompt `json:"prompt"`
}

// kindWalk marks a frozen path walk.
const kindWalk = "walk"

// walkResult is what one run of the walk phase machine produced.
type walkResult struct {
	steps      []Step
	discovered map[string]Discovery
	outcome    *Outcome
	pending    *Pending
}

// runWalk advances a walk from its phase index toward the end of its path,
// stopping at the first checkpoint, the first ending, or the last cell.
//
// This is the re-enterable phase machine. The loop holds nothing across a
// suspension: everything it would need to continue lives in the frozenResolution
// it is handed, which is a value that can be written to storage and read back by
// a different process. A straight-line loop with the state on the Go stack would
// have made suspension impossible without parking a goroutine, and a parked
// goroutine is a resolution that dies when the process does.
func (m *Manager) runWalk(scope *writeScope, w *frozenResolution) (*walkResult, error) {
	res := &walkResult{discovered: map[string]Discovery{}}

	for w.Index < len(w.Path) {
		cell := w.Path[w.Index]

		moved, err := scope.enc.Move(&encounter.MoveInput{
			Member: encounter.MemberID(w.Member),
			To:     cell,
		})
		if err != nil {
			// Nothing is saved on a mid-walk rejection. The member has really
			// moved in memory for the steps already taken, but that encounter is
			// discarded unsaved, so the persisted world is untouched.
			return nil, fmt.Errorf("step %d of %d: %w",
				w.Index+1, len(w.Path), translate(err))
		}

		w.Index++
		res.steps = append(res.steps, Step{Position: moved.Moved.To, Seq: moved.Seq})

		delta := projectDiscoveries(moved.IntelDeltas)
		mergeDiscoveries(res.discovered, delta)

		if moved.Outcome != nil {
			// The encounter ended underfoot. Every remaining step is abandoned:
			// a closed encounter refuses movement anyway, and attempting them
			// would turn a clean stop into a rejection the caller must interpret.
			res.outcome = projectOutcome(moved.Outcome)
			return res, nil
		}

		// A checkpoint on the final cell would be a window whose every option is
		// a no-op: there is nothing left to continue and nothing left to stop.
		// Freezing the world to ask a question that cannot change anything is
		// worse than not asking.
		if w.Index == len(w.Path) {
			break
		}

		sighted := firstContactFor(delta, w.Member)
		if len(sighted) == 0 {
			continue
		}

		pending, err := m.poseWalk(scope, w, sighted, moved.Seq)
		if err != nil {
			return nil, err
		}
		res.pending = pending
		return res, nil
	}

	return res, nil
}

// firstContactFor reports what a member saw for the first time in one step,
// sorted.
//
// Sorting is not cosmetic. C8, the encounter composition's determinism law,
// requires that what a checkpoint enumerates be a function of persisted data
// rather than of iteration or subscription order — otherwise a resolution
// reloaded and re-run could pose its windows in a different order and the
// "identical inputs, identical outputs" promise would hold everywhere except
// the one place a human is watching.
func firstContactFor(delta map[string]Discovery, member string) []string {
	seen := delta[member].FirstContact
	if len(seen) == 0 {
		return nil
	}
	subjects := make([]string, 0, len(seen))
	for _, r := range seen {
		subjects = append(subjects, r.Subject)
	}
	sort.Strings(subjects)
	return subjects
}

// poseWalk freezes the walk into an open window and returns what the audience
// is being asked.
func (m *Manager) poseWalk(
	scope *writeScope, w *frozenResolution, sighted []string, at uint64,
) (*Pending, error) {
	room, cell, err := whereIs(scope.enc, encounter.MemberID(w.Member))
	if err != nil {
		return nil, err
	}

	w.Prompt = Prompt{
		Member:  w.Member,
		Room:    room,
		At:      cell,
		Sighted: sighted,
	}

	payload, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("freezing walk: %w", err)
	}

	posed, err := scope.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(w.Member),
		Options:  []interrupt.Option{interrupt.Option(OptionContinue), interrupt.Option(OptionStop)},
		Payload:  payload,
		At:       at,
	})
	if err != nil {
		return nil, fmt.Errorf("opening window: %w", err)
	}

	scope.touched = true
	return projectPending(posed.Window, w.Prompt), nil
}

// projectPending turns a custody-module window into the SDK's own shape.
//
// The inner Window never crosses the boundary: the whole point of the custody
// module being replaceable is that a host names Pending and Option, not
// interrupt.Window and interrupt.Option.
func projectPending(w interrupt.Window, prompt Prompt) *Pending {
	options := make([]Option, 0, len(w.Options))
	for _, opt := range w.Options {
		options = append(options, Option{Kind: OptionKind(opt), Token: string(opt)})
	}
	return &Pending{
		Window:   strconv.FormatUint(uint64(w.ID), 10),
		Audience: string(w.Audience),
		Options:  options,
		Prompt:   prompt,
	}
}

// thaw reads a frozen resolution back out of a window's payload.
//
// A payload this module did not write is a rejection rather than a best-effort
// parse: resuming a resolution we cannot fully understand would move a member
// through cells on the strength of a guess.
func thaw(payload []byte) (*frozenResolution, error) {
	var w frozenResolution
	if err := json.Unmarshal(payload, &w); err != nil {
		return nil, fmt.Errorf("%w: unreadable frozen resolution: %w", ErrInvalidSession, err)
	}
	if w.Kind != kindWalk {
		return nil, fmt.Errorf("%w: frozen resolution of kind %q", ErrInvalidSession, w.Kind)
	}
	if w.Index < 0 || w.Index > len(w.Path) {
		return nil, fmt.Errorf("%w: frozen walk at step %d of %d",
			ErrInvalidSession, w.Index, len(w.Path))
	}
	return &w, nil
}
