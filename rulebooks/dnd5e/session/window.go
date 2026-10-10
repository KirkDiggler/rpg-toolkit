// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// windowIDString renders an interrupt window's id as the string this package
// puts on its exported surface.
//
// A STRING BECAUSE interrupt.WindowID MAY NOT CROSS (law S2, boundary_test.go).
// The ledger's own id type is an inner type; a REACT declaration encodes this
// rendering of it instead, and [Manager.React] matches a declaration id back to
// a window by regenerating every open window's selector rather than by parsing
// anything out of it.
func windowIDString(id interrupt.WindowID) string {
	return strconv.FormatUint(uint64(id), 10)
}

// windowVersion is written on every stored window and checked on every read.
// A payload carrying any other version — every payload an earlier build
// wrote carries none — is [ErrStalePause] (one pause envelope, ruling E5).
const windowVersion = 1

// The two answers this package poses on the ledger. They are the ledger's
// bookkeeping and nothing else: what a window accepts is the offer's to say,
// and [Manager.React] checks an [Answer] against the offer, never against
// these. The ledger refuses a window with no options, so it is handed the two
// shapes every answer takes.
const (
	ledgerTake    interrupt.Option = "take"
	ledgerDecline interrupt.Option = "decline"
)

// pendingWindow is the one stored payload of an open window (one pause
// envelope, ruling E2).
//
// THE PAUSE IS RESOLUTION'S, STORED WHOLE. Its question, its price and its
// frozen machine are handed back to [resolution.Resume] exactly as they came
// out; this package reads the question to render a row and never reads the
// frozen bytes at all. The story beside it is what this package needs to TELL
// the resume, and nothing else: no bookkeeping about what was already told
// exists, because what settled before the pause was told at the pause.
type pendingWindow struct {
	// Version is [windowVersion].
	Version int `json:"version"`

	// Kind is the pause's kind, repeated so a mis-paired payload is refused.
	Kind resolution.PauseKind `json:"kind"`

	// Audience is who is asked — always the window's own audience and the
	// pause's own.
	Audience string `json:"audience"`

	// Pause is resolution's pause, whole.
	Pause resolution.Pause `json:"pause"`

	// Story is what the resume needs to be told.
	Story windowStory `json:"story"`
}

// storyKind names which verb a window's resume tells, and so which record
// function lands it.
type storyKind string

const (
	// storyAttack is a declared or driven attack, lone or sequence.
	storyAttack storyKind = "attack"
	// storyStep is an announced step and the reactions to it.
	storyStep storyKind = "step"
	// storyCast is a cast stopped on one of its saves.
	storyCast storyKind = "cast"
	// storyCheck is a check stopped on its d20: Unlock, Persuade, Intimidate.
	storyCheck storyKind = "check"
)

// windowStory is what the session needs to TELL a resume, and nothing
// resolution settled or asks.
type windowStory struct {
	// Kind picks the record function.
	Kind storyKind `json:"kind"`

	// Attacker, Target and Definition name an attack story's swing. Target is
	// a step story's mover.
	Attacker   string                   `json:"attacker,omitempty"`
	Target     string                   `json:"target,omitempty"`
	Definition combatActions.Definition `json:"definition"`

	// Components is the attacker's action list a sequence's steps name their
	// components from.
	Components []combatActions.Definition `json:"components,omitempty"`

	// PresentationID is the token a declaring client correlates its throw
	// against; empty for a swing nobody declared.
	PresentationID string `json:"presentation_id,omitempty"`

	// WalkPath is the rest of a player's walk a reaction stopped, carried so
	// the last answer walks it.
	WalkPath []spatial.Position `json:"walk_path,omitempty"`

	// Door is the lock an Unlock check faced; Verb and Target the social verb
	// a check was for. Exactly one of Door and Target is set on a check.
	Door string `json:"door,omitempty"`
	Verb Verb   `json:"verb,omitempty"`

	// Caster, Spell and Caught are a cast story's: who cast what, and the
	// footprint fixed before any save was rolled.
	Caster string         `json:"caster,omitempty"`
	Spell  SpellRef       `json:"spell"`
	Caught []CaughtMember `json:"caught,omitempty"`
}

// validate refuses a story missing what its kind needs to tell a resume.
func (s windowStory) validate() error {
	switch s.Kind {
	case storyAttack:
		if s.Attacker == "" || s.Target == "" || s.Definition.Ref.ID == "" {
			return fmt.Errorf("%w: an attack window names no attacker, target or attack", ErrInvalidSession)
		}
	case storyStep:
		if s.Target == "" {
			return fmt.Errorf("%w: a step window names no mover", ErrInvalidSession)
		}
	case storyCast:
		if s.Caster == "" || s.Spell.Ref == "" {
			return fmt.Errorf("%w: a cast window names no caster or spell", ErrInvalidSession)
		}
	case storyCheck:
		// EXACTLY ONE of the two "what to finish" fields: a check names the
		// door it faced or the target it addressed, and answering one that
		// named both or neither would pick a verb by accident.
		if (s.Door == "") == (s.Target == "") {
			return fmt.Errorf("%w: a check window names %s to finish", ErrInvalidSession,
				map[bool]string{true: "neither a door nor a target", false: "both a door and a target"}[s.Door == ""])
		}
		if s.Target != "" && s.Verb != VerbIntimidate && s.Verb != VerbPersuade {
			return fmt.Errorf("%w: a check window names target %q under verb %q, which is not a social verb",
				ErrInvalidSession, s.Target, s.Verb)
		}
		if s.Door != "" && s.Verb != "" {
			return fmt.Errorf("%w: a check window names a door and the verb %q", ErrInvalidSession, s.Verb)
		}
	default:
		return fmt.Errorf("%w: window story kind %q is not one this build tells", ErrInvalidSession, s.Kind)
	}
	return nil
}

// poseWindow is the only writer of a window payload. It stores the pause
// whole beside the story, reads its own output back through [thawWindow] so
// it can never write what it would refuse, and poses it on the ledger.
func poseWindow(scope *writeScope, pause resolution.Pause, story windowStory) error {
	raw, err := json.Marshal(pendingWindow{
		Version:  windowVersion,
		Kind:     pause.Kind,
		Audience: pause.Ask.Audience,
		Pause:    pause,
		Story:    story,
	})
	if err != nil {
		return fmt.Errorf("%w: marshal window: %v", ErrInvalidSession, err)
	}
	if pause.Ask.Audience == "" {
		return fmt.Errorf("%w: the machine asked nobody", ErrInvalidWorld)
	}
	if _, err := thawWindow(raw, pause.Ask.Audience); err != nil {
		return err
	}
	if _, err := scope.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID(pause.Ask.Audience),
		Options:  []interrupt.Option{ledgerTake, ledgerDecline},
		Payload:  raw,
		// The sequence this verb started from, the only story coordinate a
		// pose holds: the beats this verb appends land after it.
		At: scope.baseline,
	}); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	scope.data.Windows = scope.ledger.ToData()
	scope.touched = true
	return nil
}

// thawWindow reads a stored window back, refusing anything this build could
// not have written.
//
// REJECT, NEVER GUESS. It refuses, in this order: an undecodable payload
// ([ErrInvalidSession]); a version that is not [windowVersion]
// ([ErrStalePause]) — every payload an earlier build wrote, so a table caught
// mid-reaction across a deploy loses that window rather than answering a
// question this build cannot read; an audience that is empty or not the
// window's; a kind that is not the pause's; an offer with no ref or name;
// empty or duplicate choice ids; no frozen machine; and a story missing what
// its kind needs. Every refusal but the version is [ErrInvalidSession].
func thawWindow(raw []byte, audience string) (pendingWindow, error) {
	var peek struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return pendingWindow{}, fmt.Errorf("%w: window payload: %v", ErrInvalidSession, err)
	}
	if peek.Version != windowVersion {
		return pendingWindow{}, fmt.Errorf("%w: window version %d, this build writes %d",
			ErrStalePause, peek.Version, windowVersion)
	}
	var w pendingWindow
	if err := json.Unmarshal(raw, &w); err != nil {
		return pendingWindow{}, fmt.Errorf("%w: window payload: %v", ErrInvalidSession, err)
	}
	if w.Audience == "" || w.Audience != audience || w.Pause.Ask.Audience != w.Audience {
		return pendingWindow{}, fmt.Errorf("%w: window asks %q but is posed to %q",
			ErrInvalidSession, w.Audience, audience)
	}
	if w.Kind == "" || w.Kind != w.Pause.Kind {
		return pendingWindow{}, fmt.Errorf("%w: window kind %q, pause kind %q",
			ErrInvalidSession, w.Kind, w.Pause.Kind)
	}
	offer := w.Pause.Ask.Offer
	if offer.Ref.ID == "" || offer.Name == "" {
		return pendingWindow{}, fmt.Errorf("%w: window offers nothing it can name", ErrInvalidSession)
	}
	seen := map[string]bool{}
	for _, choice := range offer.Choices {
		if choice.ID == "" || seen[choice.ID] {
			return pendingWindow{}, fmt.Errorf("%w: window lists an empty or repeated choice", ErrInvalidSession)
		}
		seen[choice.ID] = true
	}
	if len(w.Pause.Frozen) == 0 {
		return pendingWindow{}, fmt.Errorf("%w: window froze no machine to resume", ErrInvalidSession)
	}
	if err := w.Story.validate(); err != nil {
		return pendingWindow{}, err
	}
	return w, nil
}

// reactDeclaration is the only reader that offers a window: it compiles one
// open window into the row its audience sees.
//
// AVAILABLE IS ALWAYS TRUE. Every gate an offer has was passed before the
// window was posed, which is why the question was worth asking.
//
// THE ROW IS THE PAUSE'S QUESTION, read and never recomputed: the offer's
// ref, name, prose and choices, and the price resolution stated. The slot is
// [SlotReaction] exactly when that price spends a reaction. An opportunity
// row also names the mover as its one candidate, because the question is a
// swing at the member walking away.
func reactDeclaration(session, member string, window interrupt.Window) (Declaration, error) {
	w, err := thawWindow(window.Payload, string(window.Audience))
	if err != nil {
		return Declaration{}, err
	}
	id, err := reactDeclarationID(session, member, window.ID)
	if err != nil {
		return Declaration{}, err
	}
	offer := w.Pause.Ask.Offer
	ref := offer.Ref.String()

	slot := SlotNone
	cost := []CostComponent{}
	if w.Pause.Cost != nil {
		if w.Pause.Cost.Profile != nil && w.Pause.Cost.Profile.Slots[coreCombat.ActionReaction] > 0 {
			slot = SlotReaction
		}
		cost, err = castCostComponents(w.Pause.Cost.Profile)
		if err != nil {
			return Declaration{}, err
		}
	}

	var options []CastOption
	for _, choice := range offer.Choices {
		options = append(options, CastOption{ID: choice.ID, Label: choice.Label, Description: choice.Description})
	}

	// The offerer's own prose; the opportunity attack is the one reaction
	// this package names rather than receives, so its prose is this
	// package's (information.go).
	description := offer.Description
	if description == "" {
		description = reactionDescription[ref]
	}

	row := Declaration{
		Verb:        VerbReact,
		Slot:        slot,
		Available:   true,
		ID:          id,
		Reaction:    &ReactionRef{Ref: ref, Name: offer.Name},
		TargetKind:  TargetNone,
		Candidates:  []TargetCandidate{},
		Options:     options,
		Cost:        cost,
		Information: proseInformation(description),
	}
	if w.Kind == resolution.PauseOpportunity {
		row.TargetKind = TargetMember
		row.Candidates = []TargetCandidate{{Member: w.Story.Target, Available: true}}
	}
	return row, nil
}
