// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"errors"
	"fmt"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Mind names in the rulebook's own words — the vocabulary a monster
// definition's Mind field speaks (monster.MindRetaliator is the same word;
// session's tests pin the two together, since this module does not import
// its parent). A member whose sheet names none is driven by [Basic].
const (
	// MindRetaliator turns on whoever attacked it while the deed is fresh,
	// and otherwise goes for the closest standing player.
	MindRetaliator = "retaliator"
)

// ErrUnknownMind reports a member whose sheet names a mind this driver has
// never heard of. It fails loudly: a monster silently falling back to the
// basic driver would look like a design choice rather than a typo.
var ErrUnknownMind = errors.New("behavior: unknown mind")

// DefaultPatience is how many clock ticks old an attack deed may be before
// the Retaliator stops holding a grudge over it. A feel number, and one the
// first walk will tune.
const DefaultPatience uint64 = 2

// Minded is a [encounter.TurnDriver] that gives each member the mind its
// sheet names, and drives it through mind/behavior: the view's holdings
// become a situation, the mind ranks it, the ladder decides, and the
// decision maps onto the encounter's sealed intents. A member that names no
// mind gets [Basic]'s answer (rule A5).
//
// The driver never reaches live state (rule A2). Everything the mind asks —
// what a payload means, how far a place is, where a step lands — is answered
// from the [encounter.MonsterView] the encounter built from its canvas, by
// a [board] the driver refreshes at the top of every Act. The encounter is
// the stage (rule A3): the driver declares an attack or a step, and the
// encounter validates the one against truth and walks the other.
//
// Names persist across turns per member, in the game the driver holds; the
// perception store stays the encounter's (rule A1). Not safe for concurrent
// use: one driver serves one encounter, one turn at a time.
type Minded struct {
	basic    Basic
	board    *board
	game     *behavior.Game
	patience uint64
	ranged   func(item string) bool
	minded   map[core.EntityID]struct{}
}

// NewMindedInput configures the driver. Patience 0 means DefaultPatience.
type NewMindedInput struct {
	// Patience is how many clock ticks old an attack deed may be and still
	// be answered by a mind that holds grudges. 0 means DefaultPatience.
	Patience uint64

	// Ranged says whether an item id names a weapon that can shoot back —
	// the first authoring knob a mind takes, and it is really just data: the
	// same Retaliator answers a crossbow or shrugs off a thrown dagger
	// depending on what the driver was handed. Nil means the rulebook's own
	// weapon catalog.
	Ranged func(item string) bool
}

// NewMinded builds a driver with no minds assigned yet; minds are assigned
// the first time a member's view names one.
func NewMinded(in *NewMindedInput) (*Minded, error) {
	b := &board{}

	game, err := behavior.New(&behavior.NewInput{Reader: b, Space: b})
	if err != nil {
		return nil, fmt.Errorf("minded: %w", err)
	}

	patience := DefaultPatience
	if in != nil && in.Patience > 0 {
		patience = in.Patience
	}

	var ranged func(item string) bool
	if in != nil {
		ranged = in.Ranged
	}

	return &Minded{
		board:    b,
		game:     game,
		patience: patience,
		ranged:   ranged,
		minded:   make(map[core.EntityID]struct{}),
	}, nil
}

// Act implements [encounter.TurnDriver].
func (d *Minded) Act(view encounter.MonsterView) (encounter.TurnIntent, error) {
	if view.Mind == "" {
		return d.basic.Act(view)
	}

	d.board.view = &view

	if err := d.assign(view); err != nil {
		return nil, err
	}

	turn, err := d.game.Turn(&behavior.TurnInput{Actor: view.Self, At: view.At, Holdings: view.Holdings})
	if err != nil {
		return nil, fmt.Errorf("minded: %s: %w", view.Self, err)
	}

	return d.intent(view, turn), nil
}

// assign gives the member its mind, sheet, and place. The mind is assigned
// once; the sheet and place are the turn's own.
func (d *Minded) assign(view encounter.MonsterView) error {
	if _, done := d.minded[view.Self]; !done {
		mind, err := d.mindFor(view.Mind)
		if err != nil {
			return fmt.Errorf("minded: %s: %w", view.Self, err)
		}

		d.game.Mind(&behavior.MindInput{Actor: view.Self, Mind: mind})
		d.minded[view.Self] = struct{}{}
	}

	reach := 0
	for _, a := range view.Actions {
		reach = max(reach, encounter.CellsFromFeet(a.RangeFeet))
	}

	d.game.Sheet(&behavior.SheetInput{Actor: view.Self, Sheet: behavior.Sheet{Reach: reach}})
	d.game.Place(&behavior.PlaceInput{Actor: view.Self, Where: place(view.Position)})

	return nil
}

// mindFor is the rulebook's registry: the sheet's word, the mind it means.
func (d *Minded) mindFor(name string) (behavior.Mind, error) {
	switch name {
	case MindRetaliator:
		return &Retaliator{Space: d.board, Patience: d.patience, Ranged: d.ranged}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownMind, name)
	}
}

// intent maps the ladder's decision onto the encounter's sealed intents.
// Attack goes at the contact's bearer with the first action that reaches;
// Toward and Away take the one step the encounter precomputed; anything
// the budget cannot pay for is a pass.
func (d *Minded) intent(view encounter.MonsterView, turn *behavior.TurnOutput) encounter.TurnIntent {
	var target *behavior.Contact
	for i := range turn.Situation.Contacts {
		if c := &turn.Situation.Contacts[i]; c.Named && c.Name == turn.Intent.Target {
			target = c
			break
		}
	}

	if target == nil {
		return encounter.Pass{}
	}

	switch turn.Intent.Verb {
	case behavior.Attack:
		if view.Budget.AttacksLeft <= 0 {
			return encounter.Pass{}
		}

		for _, sm := range view.Seen {
			if sm.ID != target.Bearer {
				continue
			}

			for _, action := range view.Actions {
				if sm.InReach[action.Ref] {
					return encounter.Attack{Target: sm.ID, Action: action.Ref}
				}
			}
		}

		return encounter.Pass{}
	case behavior.Toward:
		if view.Budget.MovementFeet <= 0 {
			return encounter.Pass{}
		}

		if step, ok := d.board.step(target.Where(), false); ok {
			return encounter.Move{Path: []spatial.Position{step}}
		}

		return encounter.Pass{}
	case behavior.Away:
		if view.Budget.MovementFeet <= 0 {
			return encounter.Pass{}
		}

		if step, ok := d.board.step(target.Where(), true); ok {
			return encounter.Move{Path: []spatial.Position{step}}
		}

		return encounter.Pass{}
	case behavior.Pass:
		return encounter.Pass{}
	default:
		return encounter.Pass{}
	}
}

// place is the string a cell is known by to the mind. It is never parsed:
// the board answers every question about it by looking the cell up again.
func place(p spatial.Position) string {
	return p.String()
}

// board answers, from one turn's view and nothing else, what a mind may ask
// of the world: what a sight payload means ([behavior.Reader]) and how far
// apart two places are or where a step lands ([behavior.Space]). It is
// refreshed at the top of every Act and holds no state of its own.
type board struct {
	view *encounter.MonsterView
}

// Read decodes the encounter's sight testimony. A creature, to this
// rulebook's monsters, is a standing PLAYER: a thing this member would
// strike and be struck by. Monsters do not read each other as targets, and
// a downed player is a place, not a threat. Anything else reads as the zero
// reading.
func (b *board) Read(h perception.Holding) (*behavior.Reading, error) {
	if h.Channel != perception.Sight {
		return &behavior.Reading{}, nil
	}

	location, ok := encounter.DecodeSightTestimony(h.Payload)
	if !ok || location.State != encounter.LocationKnown {
		return &behavior.Reading{}, nil
	}

	reading := &behavior.Reading{Where: place(location.Position)}

	for _, sm := range b.view.Seen {
		if sm.ID == h.Subject {
			reading.Creature = sm.Kind == encounter.KindPlayer && sm.Standing
		}
	}

	return reading, nil
}

// Distance is the encounter's own DistanceCells for the member standing at
// the place asked about; unknown for a place nobody seen or remembered
// stands at, and for anything but the member's own place as origin.
func (b *board) Distance(in *behavior.DistanceInput) (*behavior.DistanceOutput, error) {
	if in.From != place(b.view.Position) {
		return &behavior.DistanceOutput{}, nil
	}

	for _, sm := range b.view.Seen {
		if place(sm.Position) == in.To {
			return &behavior.DistanceOutput{Steps: int(math.Round(sm.DistanceCells)), Known: true}, nil
		}
	}

	for _, rm := range b.view.Remembered {
		if place(rm.Position) == in.To {
			return &behavior.DistanceOutput{Steps: int(math.Round(rm.DistanceCells)), Known: true}, nil
		}
	}

	return &behavior.DistanceOutput{}, nil
}

// Toward is the first cell of the encounter's precomputed path to whoever
// stands at the place.
func (b *board) Toward(in *behavior.TowardInput) (*behavior.TowardOutput, error) {
	step, ok := b.step(in.To, false)

	return &behavior.TowardOutput{Next: place(step), Found: ok}, nil
}

// Away is the encounter's precomputed step away from whoever stands at the
// place; not found is a dead end, and the encounter is what knows.
func (b *board) Away(in *behavior.AwayInput) (*behavior.AwayOutput, error) {
	step, ok := b.step(in.AwayFrom, true)

	return &behavior.AwayOutput{Next: place(step), Found: ok}, nil
}

// step is the precomputed one-cell path toward, or away from, whoever
// stands at a place.
func (b *board) step(at string, away bool) (spatial.Position, bool) {
	for _, sm := range b.view.Seen {
		if place(sm.Position) != at {
			continue
		}

		path := sm.Path
		if away {
			path = sm.AwayPath
		}

		if len(path) == 0 {
			return spatial.Position{}, false
		}

		return path[0], true
	}

	if !away {
		for _, rm := range b.view.Remembered {
			if place(rm.Position) == at && len(rm.Path) > 0 {
				return rm.Path[0], true
			}
		}
	}

	return spatial.Position{}, false
}
