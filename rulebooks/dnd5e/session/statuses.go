// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"reflect"
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
// A status that does not compile is refused here as it is at launch
// ([ErrUnknownContent]); launch refuses it first, so this is a site changed
// under a running session.
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
	for _, g := range granted {
		compiled, err := status.Compile(&status.CompileInput{ID: g.Grant.ID, Status: g.Grant.Status})
		if err != nil {
			return fmt.Errorf("%w: status %q: %w", ErrUnknownContent, g.Grant.ID, err)
		}
		// THE CAST NOW, per status: a status given a moment ago is on the
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
			// below with the rest of the give.
			WhenGiven: compiled.WhenGiven,
		})
		if err != nil {
			return fmt.Errorf("status %q for %q: %w", g.Grant.ID, g.Member, translateResolution(err))
		}
		if out.AlreadyHeld {
			continue
		}
		// Landed on the verb's encounter, mid-verb: the sheets are saved, and
		// the verb commits. Nothing is told and no concentration rides it.
		if _, err := m.land(ctx, scope, out.Resolved, &landing{Live: scope.enc}); err != nil {
			return fmt.Errorf("status %q for %q: %w", g.Grant.ID, g.Member, err)
		}
	}
	return nil
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
		if _, err := status.Compile(&status.CompileInput{ID: id, Status: grants[id].Status}); err != nil {
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
