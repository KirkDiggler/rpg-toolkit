// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/status"
)

// statuses.go is A LINE'S ANSWER GIVES ITS STATUS EFFECT (rpg-project#1024).
//
// The encounter carries a status in the author's words and never applies it:
// a landed row returns it ([encounter.SpeakOutput.Statuses]). This file is the
// courier the encounter cannot be (C1): it compiles the author's block with
// the rules' [status.Compile], gives it with [resolution.GiveStatus] on the
// verb's own scope, and lands what that changed — exactly as the compelled
// driver hands a word to [resolution.Obey]. It owns no rule: what a status
// does, and whether a second giving changes anything, are the rules'.
//
// THE SAME ID IS THE SAME STATUS, FOR THE RUN: the application id is the
// session, the status and the member, so a second giving of one status to one
// member answers "already held" and the line still says its words.
//
// NO RECORD ENTRY OF ITS OWN. The composition's record has no beat for "a
// member was given a status by a line": the line's own answer is already in
// the record, and the sheet shows the status. Inventing a beat here would be
// a second author of the record; the day the composition has one, this is
// where it is written.

// statusSourceRef is the ref a given status names as its source: the site's
// status, in the `site:` namespace site content already uses for its refs.
func statusSourceRef(id string) core.Ref {
	return core.Ref{Module: "site", Type: "statuses", ID: id}
}

// statusApplicationID is one status's identity on one member for one run.
func statusApplicationID(session, statusID string, member encounter.MemberID) string {
	return fmt.Sprintf("status:%s:%s:%s", session, statusID, member)
}

// giveStatuses gives each status a landed row returned to its member, on the
// verb's scope, mid-verb: the sheets it changed are saved, and the calling
// verb commits. source is who or what the line was said to or done to.
//
// SEVERAL RECEIVERS, ALL OR NOTHING (rpg-project#990): a row may give one
// status to each member of the speaker's party in sight. Every give is asked
// of the rules FIRST, and nothing is landed until all of them answered: a
// receiver the rules refuse (one with no sheet in the cast, say) refuses the
// whole line before any sheet is written, on a plain repository as on a
// fenced one. A receiver who already holds it answers "already held", and
// the others are still given it. Only then is each give landed. A failure
// while landing is a save failing: on a fenced repository nothing lands,
// because the verb never commits; on a plain one the sheets already saved
// stay saved and the [SaveError] names them, as for every verb that saves
// more than one sheet there.
//
// A member given two statuses by one row is the one exception to asking
// every give first: the second is asked against the sheet the first changed,
// so what was asked before it is landed first. Authored content never does
// this (a row gives one status), but the encounter allows it.
//
// A status that does not compile is refused here as it is at launch
// ([ErrUnknownContent]); launch refuses it first, so this is a site changed
// under a running session.
// untilOf is what the author wrote on a giving, in the rules' words; nil for
// a giving with no end of its own.
func untilOf(u *encounter.StatusUntil) *status.Until {
	if u == nil {
		return nil
	}
	return &status.Until{Rounds: u.Rounds}
}

func (m *Manager) giveStatuses(
	ctx context.Context, scope *writeScope, source string, granted []encounter.GrantedStatus,
) error {
	if len(granted) == 0 {
		return nil
	}
	roster, err := scope.enc.Members()
	if err != nil {
		return translate(err)
	}
	type asked struct {
		grant encounter.GrantedStatus
		out   *resolution.GiveStatusOutput
	}
	var answered []asked
	landAll := func() error {
		for _, a := range answered {
			if a.out.AlreadyHeld {
				continue
			}
			// Landed on the verb's encounter, mid-verb: the sheets are saved,
			// and the verb commits. Nothing is told and no concentration rides it.
			if _, err := m.land(ctx, scope, a.out.Resolved, &landing{Live: scope.enc}); err != nil {
				return fmt.Errorf("status %q for %q: %w", a.grant.Grant.ID, a.grant.Member, err)
			}
			// A FRESH GIVING ARMS OR CLEARS ITS TIMER, and only a fresh one.
			// The session does this, not the encounter, because only the
			// rules know whether a giving was fresh. Clearing matters: a
			// status ended early by another ending (a fight starting) leaves
			// its timer behind, and the application id is reused when it is
			// given again, so a stale timer would end the new giving early.
			application := statusApplicationID(scope.session, a.grant.Grant.ID, a.grant.Member)
			if a.grant.Grant.Until != nil {
				if err := scope.enc.ArmStatusLapse(&encounter.ArmStatusLapseInput{
					Member: a.grant.Member, Status: a.grant.Grant.ID,
					Application: application, Rounds: a.grant.Grant.Until.Rounds,
				}); err != nil {
					return fmt.Errorf("status %q for %q: %w", a.grant.Grant.ID, a.grant.Member, translate(err))
				}
			} else if err := scope.enc.ClearStatusLapse(&encounter.ClearStatusLapseInput{Application: application}); err != nil {
				return fmt.Errorf("status %q for %q: %w", a.grant.Grant.ID, a.grant.Member, translate(err))
			}
		}
		answered = nil
		return nil
	}
	for _, g := range granted {
		if slices.ContainsFunc(answered, func(a asked) bool { return a.grant.Member == g.Member }) {
			if err := landAll(); err != nil {
				return err
			}
		}
		compiled, err := status.Compile(&status.CompileInput{ID: g.Grant.ID, Status: g.Grant.Status, Until: untilOf(g.Grant.Until)})
		if err != nil {
			return fmt.Errorf("%w: status %q: %w", ErrUnknownContent, g.Grant.ID, err)
		}
		// THE CAST NOW, per status: what was landed a moment ago is on the
		// sheet the next one is admitted against.
		cast, err := announcerSeam{m: m, scope: scope}.boundaryCast(ctx, roster)
		if err != nil {
			return err
		}
		out, err := resolution.GiveStatus(ctx, &resolution.GiveStatusInput{
			Interaction: m.resolutionInput(ctx, scope, resolutionAsk{
				// Machine is absent: giving a status is its own machine.
				Live:         scope.enc,
				Participants: cast,
			}),
			Member:        string(g.Member),
			Library:       compiled.Library,
			StatusID:      compiled.DefinitionID,
			SourceID:      source,
			Source:        statusSourceRef(g.Grant.ID),
			ApplicationID: statusApplicationID(scope.session, g.Grant.ID, g.Member),
			// What the status does once, on a first giving (a heal): resolution
			// runs it only on a fresh give, and the sheet it changed lands
			// with the rest of the give.
			WhenGiven: compiled.WhenGiven,
		})
		if err != nil {
			return fmt.Errorf("status %q for %q: %w", g.Grant.ID, g.Member, translateResolution(err))
		}
		answered = append(answered, asked{grant: g, out: out})
	}
	return landAll()
}

// StatusRefusal is one status a site's line can give that the rules refuse.
type StatusRefusal struct {
	// StatusID is the status's id under the site's `statuses:`.
	StatusID string
	// Path is where the status is set in the site file: `statuses.<id>`.
	Path string
	// Message is the rules' refusal, which names the place inside the status
	// (for example `changes[0].when.skill`) and the words allowed there.
	Message string
}

// CheckContentInput is a compiled site to check, as dungeonspec returns it.
type CheckContentInput struct {
	Dungeon *dungeonspec.Compiled
}

// CheckContentOutput is every refusal the content check found; empty when
// the site would launch.
type CheckContentOutput struct {
	Refusals []StatusRefusal

	// Items is every custom item the rules refuse, in item id order.
	Items []ItemRefusal
}

// ItemRefusal is one custom item a site can give that the rules refuse.
type ItemRefusal struct {
	// ItemID is the item's id under the site's `customItems:`.
	ItemID string
	// Path is where the item is set in the site file: `customItems.<id>`.
	Path string
	// Message is the rules' refusal, which names the item and the word.
	Message string
}

// CheckContent is the content check: what [Manager.Launch] refuses in a
// site's content, asked without launching anything, so a studio's save can
// refuse what a launch would. Today it is every status a line of the site
// can give, compiled with the rules ([status.Compile]); a status the rules
// refuse is one [StatusRefusal], in status id order.
//
// Errors: a nil input or dungeon.
func CheckContent(in *CheckContentInput) (*CheckContentOutput, error) {
	if in == nil || in.Dungeon == nil {
		return nil, fmt.Errorf("check content: %w", ErrNilInput)
	}
	out := &CheckContentOutput{}
	grants := siteStatusGrants(in.Dungeon)
	for _, id := range sortedGrantIDs(grants) {
		if _, err := status.Compile(&status.CompileInput{ID: id, Status: grants[id].Status, Until: untilOf(grants[id].Until)}); err != nil {
			out.Refusals = append(out.Refusals, StatusRefusal{StatusID: id, Path: "statuses." + id, Message: err.Error()})
		}
	}
	return out, nil
}

// refuseLaunchStatuses is the content check as launch asks it: the first
// refusal, as [ErrUnknownContent] naming the status and the rules' words.
func refuseLaunchStatuses(dungeon *dungeonspec.Compiled) error {
	checked, err := CheckContent(&CheckContentInput{Dungeon: dungeon})
	if err != nil {
		return err
	}
	if len(checked.Refusals) == 0 {
		return nil
	}
	r := checked.Refusals[0]
	return fmt.Errorf("%w: status %q at %s: %s", ErrUnknownContent, r.StatusID, r.Path, r.Message)
}

// siteStatusGrants is every status any line of the compiled site can give,
// by id: found by walking the compiled form for [encounter.StatusGrant], so a
// line on a creature, a faction or a thing is never missed.
func siteStatusGrants(dungeon *dungeonspec.Compiled) map[string]encounter.StatusGrant {
	found := map[string]encounter.StatusGrant{}
	grantType := reflect.TypeOf(encounter.StatusGrant{})
	seen := map[uintptr]bool{}
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() || seen[v.Pointer()] {
				return
			}
			seen[v.Pointer()] = true
			walk(v.Elem())
		case reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		case reflect.Struct:
			if v.Type() == grantType {
				g := v.Interface().(encounter.StatusGrant)
				found[g.ID] = g
				return
			}
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				walk(iter.Value())
			}
		}
	}
	walk(reflect.ValueOf(dungeon))
	return found
}

func sortedGrantIDs(grants map[string]encounter.StatusGrant) []string {
	ids := make([]string, 0, len(grants))
	for id := range grants {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
