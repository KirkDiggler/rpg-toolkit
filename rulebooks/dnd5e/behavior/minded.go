// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"errors"
	"fmt"
	"maps"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Mind names in the rulebook's own words — the vocabulary a monster
// definition's Mind field speaks (monster.MindRetaliator is the same word;
// session's tests pin the two together, since this module does not import
// its parent). A member whose sheet names none is driven by [Basic].
//
// All three are one [Retaliator] under three profiles, which is the whole
// lesson of rpg-toolkit#1745: a mind is a shape with fields, and a monster
// tuned differently is not a new type. It is still a new WORD and a toolkit
// release, because the profile a word means lives in [presets] and not on
// the definition — the cost authored data would remove.
const (
	// MindRetaliator turns on whoever attacked it while the deed is fresh
	// and they still hold something that could shoot back, and otherwise
	// goes for the closest standing player.
	MindRetaliator = "retaliator"
	// MindBerserker turns on whoever attacked it and does not care what
	// they are holding: only the clock talks it off a grudge.
	MindBerserker = "berserker"
	// MindCoward holds no grudge at all: it keeps its room, stepping away
	// from whatever closes on it, and answers the closest standing player
	// with whatever it holds — a blade-only coward backs off and fights
	// cornered. It is also the one mind a THREAT works on: frighten it and
	// it runs from whoever did while it can still see them
	// (rpg-project#454).
	MindCoward = "coward"
)

// ErrUnknownMind reports a member whose sheet names a mind this driver has
// never heard of. It fails loudly: a monster silently falling back to the
// basic driver would look like a design choice rather than a typo.
var ErrUnknownMind = errors.New("behavior: unknown mind")

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
	basic  Basic
	board  *board
	game   *behavior.Game
	ranged func(item string) bool
	minds  map[string]behavior.Mind
	minded map[core.EntityID]struct{}
}

// NewMindedInput configures the driver.
//
// There is no patience knob here, and that is the point of
// rpg-toolkit#1745: how long a mind holds a grudge is the MIND's, named by
// the word a monster's sheet says, and a host that set one number for every
// mind in the process could not have a berserker and a bow skeleton on the
// same board.
type NewMindedInput struct {
	// Ranged says whether an item id names a weapon that can shoot back.
	// It is the CATALOG knob and not a profile field: what a bow IS is data
	// about the world, while whether a mind cares is [Grudge.Excuse]. One
	// answer serves every mind this driver builds, which is why it lives
	// here and the profile does not. Nil means the rulebook's own weapon
	// catalog.
	Ranged func(item string) bool

	// Minds is the door an authored mind comes through: the word a member's
	// sheet names, and the mind it means. The built-ins are what the rulebook
	// ships and they stay — a caller adds to the vocabulary rather than
	// replacing it — and an entry under a built-in's word wins, which is how
	// a game tries its own Retaliator without a rulebook release.
	//
	// One mind answers for every member that names it. A mind is four
	// judgments and no state, so that is the shape, not a limit.
	Minds map[string]behavior.Mind
}

// NewMinded builds a driver with no minds assigned yet; minds are assigned
// the first time a member's view names one.
func NewMinded(in *NewMindedInput) (*Minded, error) {
	b := &board{}

	game, err := behavior.New(&behavior.NewInput{Reader: b, Space: b})
	if err != nil {
		return nil, fmt.Errorf("minded: %w", err)
	}

	var (
		ranged func(item string) bool
		minds  map[string]behavior.Mind
	)

	if in != nil {
		ranged = in.Ranged
		minds = maps.Clone(in.Minds)
	}

	return &Minded{
		board:  b,
		game:   game,
		ranged: ranged,
		minds:  minds,
		minded: make(map[core.EntityID]struct{}),
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

// preset is one word's tuning of the one [Retaliator] — the fields a mind
// has, which is what this slice exists to find out before any authored-data
// format is chosen. Space and Ranged are the driver's and not a preset's:
// geometry belongs to the board, and what counts as a bow is a catalog.
type preset struct {
	Grudge Grudge
	Fear   Fear
	Room   int
}

// presets are the profiles the rulebook's own words mean. Every number here
// is a FEEL number, tuned by a walk and derived from nothing.
//
//	word        patience  excuse   provokes           room  fear
//	retaliator  3         unarmed  attack             0     none
//	berserker   10        never    attack, intimidate 0     none
//	coward      0 (none)  -        —                  2     3
//
// The retaliator's 3 is #1725's behaviour unchanged: a deed stays worth
// answering on the tick it lands and the two after it, which is exactly
// what that slice's "older than 2 is stale" arithmetic did before Patience
// became a span. A tick is a fight ROUND (walk 2 of #1725), so it answers a
// shot for about two rounds after the last one.
//
// The berserker's 10 is longer than any fight at this table: within one
// fight it never forgets, and a shot it answered ten rounds later is a
// monster nobody will ever meet. It is a number rather than an infinity
// because "never forgets" is a claim no walk has paid for, and a span is
// the shape the field already has.
//
// The coward's 2 is two steps of room, which the ladder's rung 0 reads as
// "back away from a live creature nearer than two" — in practice, anything
// adjacent. One step would mean only a creature sharing its cell, which is
// nobody. Its grudge is the zero value and says so: a coward does not
// answer attacks, it leaves.
//
// # What a threat is worth, per word (rpg-project#454)
//
// The provocation column is the debt scenarios.md named — "an attack on me
// is hardcoded" — paid by the first verb that made it matter.
//
//   - The BERSERKER counts a threat as a swing. Its excuse is
//     [ExcuseNever], so nothing about the threatener's hands talks it down;
//     the thug charges whoever threatened it, whoever else is closer.
//   - The RETALIATOR does not. Its excuse is about hands and a threat is
//     not a swing, so `intimidate` is simply absent from its list: it holds
//     the deed, [Judge] still attaches it to the right figure, and its
//     ranking is exactly what it was.
//   - The COWARD answers no verb at all — its grudge is still the zero —
//     and reads the threat through [Fear] instead. Patience 3 is the
//     retaliator's number for the retaliator's reason: a fight's length.
//     The 2 steps of room STAY: intimidation is fear added on top of the
//     flinch, not a replacement for it (Kirk, rpg-project#454).
var presets = map[string]preset{
	MindRetaliator: {Grudge: Grudge{
		Patience: 3, Excuse: ExcuseUnarmed,
		Provokes: []string{encounter.DeedAttack},
	}},
	MindBerserker: {Grudge: Grudge{
		Patience: 10, Excuse: ExcuseNever,
		Provokes: []string{encounter.DeedAttack, encounter.DeedIntimidate},
	}},
	MindCoward: {Room: 2, Fear: Fear{Patience: 3}},
}

// mindFor is the rulebook's registry: the sheet's word, the mind it means.
// The caller's own minds are asked first, so an authored mind extends the
// vocabulary and may override a word the rulebook ships.
func (d *Minded) mindFor(name string) (behavior.Mind, error) {
	if mind, ok := d.minds[name]; ok {
		return mind, nil
	}

	if p, ok := presets[name]; ok {
		return &Retaliator{Space: d.board, Grudge: p.Grudge, Fear: p.Fear, Room: p.Room, Ranged: d.ranged}, nil
	}

	return nil, fmt.Errorf("%w: %q", ErrUnknownMind, name)
}

// intent maps the ladder's decision onto the encounter's sealed intents.
// Attack goes at the member the contact is about with the first action that
// reaches; Toward and Away take the one step the encounter precomputed;
// anything the budget cannot pay for is a pass.
//
// The member is [memberID] and deliberately not the contact's Bearer. The
// bearer is the subject mind/behavior recorded the name on, which for a
// figure first met as a ghost-plus-deed is the deeds handle — no member of
// this encounter — and it stays the bearer on every later turn that finds
// the word already there. The name is likewise not read as an id: what a
// mind calls a contact is its author's business, and this driver takes minds
// from its caller.
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

		id, ok := memberID(target.Holdings)
		if !ok {
			return encounter.Pass{}
		}

		for _, sm := range view.Seen {
			if sm.ID != id {
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

// memberID is the plain encounter member id a set of bundled holdings is
// about: the subject sight files a figure under, else the actor a deed names
// as its own testimony. False is holdings about nobody this encounter can put
// an id to.
//
// It is never the deeds handle, and that is the whole point of the function.
// A handle is the store's filing system — deed's own rule is that a mind
// reads testimony and not handles — and it is also the sorted-first subject
// of a ghost-plus-deed bundle, so a name or a target taken from it matches no
// member the encounter will ever offer. Both places that need an id for a
// contact ask here.
func memberID(holdings []behavior.Holding) (core.EntityID, bool) {
	for _, h := range holdings {
		if h.Channel == perception.Sight {
			return h.Subject, true
		}
	}

	for _, h := range holdings {
		if h.Channel != deed.Channel {
			continue
		}

		if d, err := deed.Decode(h.Payload); err == nil && d.Actor != "" {
			return d.Actor, true
		}
	}

	return "", false
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
