package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SceneMove signals a guard detail to leave. When anchors are provided,
// visible party members relocate before the guards leave.
type SceneMove struct {
	Guards  []MemberID
	Anchors []spatial.Position
	Leave   FactID
}

func (move SceneMove) validate() error {
	if len(move.Guards) == 0 || move.Leave == "" {
		return fmt.Errorf("scene movement needs guards and a departure signal")
	}
	guards := make(map[MemberID]bool)
	for _, id := range move.Guards {
		if id == "" || guards[id] {
			return fmt.Errorf("duplicate or empty guard id %q", id)
		}
		guards[id] = true
	}
	anchors := make(map[spatial.Position]bool)
	for _, anchor := range move.Anchors {
		if anchors[anchor] {
			return fmt.Errorf("duplicate arrival anchor %v", anchor)
		}
		anchors[anchor] = true
	}
	return nil
}

func (e *Encounter) applySceneMove(move SceneMove, line recognitionLine) error {
	down, err := e.downNow()
	if err != nil {
		return err
	}
	guards := make([]MemberID, 0, len(move.Guards))
	for _, id := range move.Guards {
		if _, present := e.members[id]; present && !down[id] {
			guards = append(guards, id)
		}
	}
	if len(move.Anchors) > 0 && len(guards) > 0 {
		reach, err := e.placedSightReach()
		if err != nil {
			return err
		}
		moves := make([]Relocation, 0, len(move.Anchors))
		for _, player := range e.arrestParty(line.speaker) {
			for _, guard := range guards {
				if !line.sees(reach, guard, player) {
					continue
				}
				if len(moves) == len(move.Anchors) {
					return fmt.Errorf("not enough arrival anchors for the visible party")
				}
				moves = append(moves, Relocation{Member: player, To: move.Anchors[len(moves)]})
				break
			}
		}
		if len(moves) > 0 {
			if _, err := e.Relocate(&RelocateInput{
				Moves: moves,
				Cause: RelocationCause{
					Ref:   *core.MustNewRef(core.RefInput{Module: "site", Type: "consequences", ID: "scene-move"}),
					Words: "The guards take you to the shared cell.",
				},
				Reach: line.reach,
			}); err != nil {
				return err
			}
		}
	}
	for _, guard := range guards {
		if err := e.learnFact(guard, move.Leave, "new assignment", line.at); err != nil {
			return err
		}
	}
	return nil
}
