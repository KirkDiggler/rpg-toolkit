// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// held_unlock_resume_test.go is a paused unlock on a door held since
// (rpg-project#852, R7): alice's lock check stops on her Guidance die, and
// before she answers a skeleton behind the door stands to its post. Her
// answer is refused by the hold, and the pause is CLOSED, declined: her die
// is not spent, the lock stands, and the session is not left frozen behind
// a window nobody can answer.

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// heldVaultSkeleton is a skeleton in the vault past the gate, idle at first;
// its `posted` state holds the gate from the cell it stands on.
func (s *GuidedUnlockSuite) heldVaultSkeleton() dungeonspec.MonsterPlacement {
	ref, err := core.ParseString("dnd5e:machines:vault-post")
	s.Require().NoError(err)
	skeleton := monsterAt("skel-1", refs.Monsters.Skeleton().String(), 9, 3)
	skeleton.Behaviors.Instances[0].Instance.Recipe = machine.Recipe{Ref: *ref, Initial: "idle", States: map[string]machine.State{"idle": {}, "posted": {}}}
	skeleton.Posts = map[string]encounter.Post{"posted": {Door: "gate", Cells: []spatial.Position{hexCell(9, 3)}}}
	return skeleton
}

// standToPost puts id in state in the stored world.
func (s *GuidedUnlockSuite) standToPost(id, state string) {
	data, ok := s.encounters.byID["sess"]
	s.Require().True(ok, "the launch saved the world")
	for i := range data.Members {
		if data.Members[i].ID == encounter.MemberID(id) {
			b := data.Members[i].Behaviors
			s.Require().NotNil(b)
			for j := range b.Progress.Instances {
				if b.Progress.Instances[j].ID == b.BodyInstance {
					b.Progress.Instances[j].Blob.State = state
					return
				}
			}
			s.Require().FailNow("designated body is present")
		}
	}
	s.Require().Failf("no member", "the world holds no %s", id)
}

func (s *GuidedUnlockSuite) gateStateInTheRecord() string {
	for _, d := range s.encounters.byID["sess"].Doors {
		if d.ID == "gate" {
			return d.State
		}
	}
	s.Require().Fail("the record holds no gate")
	return ""
}

func (s *GuidedUnlockSuite) TestAHeldDoorClosesAPausedUnlock() {
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	world := gatedWorld(tombLock())
	world.Monsters = append(world.Monsters, s.heldVaultSkeleton())
	launchScene(s.T(), mgr, world)
	s.guide()

	out := s.tryLock(mgr)
	s.Require().True(out.Paused, "precondition: the attempt waits on her Guidance answer")
	row := s.reactRow(mgr, "alice")
	s.Require().NotEmpty(row.ID, "precondition: the window is open")

	s.standToPost("skel-1", "posted")

	_, err = mgr.React(context.Background(), &session.ReactInput{
		Session: "sess", Member: "alice", DeclarationID: row.ID, Answer: session.Take(""),
	})
	s.Require().ErrorIs(err, session.ErrDoorHeld, "the hold answers her, not the lock")
	var held *session.DoorHeldError
	s.Require().ErrorAs(err, &held)
	s.Equal("someone bars the door", held.Reason, "she cannot see the skeleton behind the door")
	for name, sentinel := range compositionSentinels {
		s.NotErrorIs(err, sentinel, "the composition's %s never leaks", name)
	}

	s.Empty(s.reactRow(mgr, "alice").ID, "the pause is closed, not left open")
	s.True(s.holdsTheDie(), "declined, the die is not spent")
	s.Equal("locked", s.gateStateInTheRecord(), "and the lock stands")

	// NOT FROZEN: the next door verb is asked of the door, not refused by a
	// window still waiting.
	// The next attempt meets the hold as an answer, not an error
	// (rpg-project#842): no window is left open to refuse it.
	next, err := mgr.Unlock(context.Background(), &session.UnlockInput{Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err, "no window is left open")
	s.Require().NotNil(next.Refused, "the next attempt meets the hold")
	s.Equal("skel-1", next.Refused.Holder)
	s.Equal("someone bars the door", next.Refused.Reason, "she still cannot see the skeleton")
}
