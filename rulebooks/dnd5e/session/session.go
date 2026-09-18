// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Config carries what a Manager needs from the host. Construct a Manager with
// NewManager; the zero Manager is unusable.
//
// Required fields are those without which no verb can run. Optional ones are
// capabilities: absent, the corresponding behaviour simply does not happen and
// nothing else degrades.
//
// Fields are added over time as waves need them (a character repository arrives
// with entities, for example). That direction is deliberate and safe: adding a
// field here is a compatible change a host adopts when it wants the capability,
// whereas removing one it has already implemented is not. They are therefore
// introduced when something calls them, never in anticipation.
type Config struct {
	// StaleTargetPolicy enables known-creature casts. Empty leaves those
	// offers explicitly unavailable; other spells remain usable. API/SDK
	// setup should choose StaleTargetRefuse unless the host wants paid attempts.
	StaleTargetPolicy StaleTargetPolicy
	// Sessions persists session state. Required.
	Sessions SessionRepository

	// Encounters persists the world. Required.
	Encounters EncounterRepository

	// Characters persists player characters. Required.
	//
	// Added in the entities wave, when something finally called it. A new
	// REQUIRED Config field is worth a note, because gorelease will call it a
	// compatible change and for a required field that verdict is wrong: it
	// compiles everywhere and fails every existing host at its first
	// NewManager, since construction is total (S8).
	//
	// It is free here only because no host has adopted this package yet. Once
	// one has, a new required field is a silent runtime break wearing a green
	// CI check — so every required field must land at or before the migration
	// wave, and after that the answer is a separately type-asserted capability
	// rather than a Config field.
	Characters CharacterRepository

	// Events is the live channel to connected clients. Required.
	//
	// Required because a verb's response tells the CALLER about the caller's
	// own action, and that is a small fraction of what a client must render.
	// Monsters acting, the clock advancing, another player crossing a doorway —
	// none of it appears in anyone's return value. A host without a stream is
	// reduced to polling Story, which is a recovery path rather than a design.
	//
	// This is not about multiplayer. A single-player game needs it for exactly
	// the same reason: things happen that the player did not ask for.
	//
	// Use DiscardEvents for tests, headless simulation, or any run that
	// genuinely wants no delivery — an explicit opt-out that reads as a
	// decision, rather than a nil that reads as an oversight.
	Events EventStream

	// Dice is where random numbers come from. Required.
	//
	// Required rather than optional because a fight now starts on its own, and
	// it starts inside an ordinary Move: absent a source of randomness, the
	// verb a player just called fails. There is no "the capability simply does
	// not happen" version of that — an unorderable fight is a broken world,
	// not a reduced one.
	//
	// The host supplies the dice and nothing else; the rule that turns a d20
	// into an initiative order is the rulebook's. See Roller.
	//
	// It lands at the same moment the composition made its own initiative
	// roller mandatory, and for the same reason: refuse at the door, never
	// guard at the use site.
	Dice Roller

	// PresentationIDs supplies opaque correlation tokens for explicit rolls.
	// Required. The token is shared by response and every story witness while
	// each recipient keeps its own numeric sequence.
	PresentationIDs PresentationIDGenerator

	// TurnDriver is the ONE driver that serves EVERY session this Manager
	// serves. Wire it when the driver is stateless; wire TurnDrivers instead
	// when it is not. Exactly one of the two, and NewManager refuses both or
	// neither.
	//
	// A fight can form — or a turn can end — with the clock landing on a
	// member nobody plays: initiative rolled the monster first, or the human
	// ahead of it just ended their own turn. What that member does is a game
	// rule, so it is asked for rather than assumed, exactly as Dice is
	// (rpg-toolkit#1162).
	//
	// The customers for this field are the drivers that hold nothing:
	// session.Pass{} for v1's whole behavior — every unplayed member's turn
	// ends the moment the clock reaches it — and session.Behavior() for the
	// reference driver. One value answers every session's turns because
	// neither of them remembers anything between two of them.
	//
	// A STATEFUL DRIVER WANTS TurnDrivers. session.Minded(nil) is a game with
	// per-member names and memory, and it is not safe for concurrent use:
	// wired here, one of them would serve every session in the process, and
	// two parties running the same authored dungeon would share a skeleton's
	// assigned mind and names (member ids are authored per dungeon, not minted
	// per run — rpg-api#980's caveat, rpg-toolkit#1734).
	TurnDriver TurnDriver

	// TurnDrivers hands over ONE driver PER SESSION. Wire it when the driver
	// is stateful; wire TurnDriver instead when it is not. Exactly one of the
	// two, and NewManager refuses both or neither.
	//
	// It is asked once per verb, for the session that verb is about, and its
	// answer serves that whole verb — writes and reads alike. An error fails
	// the verb: there is no fallback to a reference driver, because a monster
	// answered by somebody else's brain looks like a design choice rather than
	// the wiring fault it is.
	//
	// The customer is session.Minded(nil) and every authored mind after it.
	// The CACHE BEHIND IT IS THE HOST'S, deliberately: a session's lifetime is
	// the host's (a Redis TTL, a run ending) and this Manager is stateless per
	// verb with no session-end signal to evict on, so a cache here would have
	// no owner. See [TurnDriverSource] and rule A6 in
	// docs/ideas/mind/behavior/adoption.md.
	TurnDrivers TurnDriverSource
}

// Manager is the host's single point of contact with the toolkit.
//
// It holds the host's repositories and stream, and nothing else (S1). Every
// verb loads what it needs, acts, saves, and drops everything, so a Manager is safe to construct once at
// process start, share across goroutines that do not share a verb call, and
// keep for the life of the process. Nothing about a session is cached between
// calls, which is what allows several servers to serve the same session with
// no coordination.
type Manager struct {
	staleTargetPolicy StaleTargetPolicy
	sessions          SessionRepository
	encounters        EncounterRepository
	characters        CharacterRepository
	events            EventStream
	initiative        encounter.InitiativeRoller
	presentationIDs   PresentationIDGenerator

	// turnDrivers is where a verb's driver comes from, and the Manager holds
	// no driver of its own beside it. A host that wired the stateless
	// Config.TurnDriver gets a [staticTurnDrivers] around it here, so "which
	// driver serves this verb" is one question with one answer and no branch:
	// see [Manager.resolveTurnDriver].
	turnDrivers TurnDriverSource

	// targetPreflight is the one shared target gate used by offer projection
	// and regenerated Attack execution. It is a pure function seam rather than
	// host configuration: production always installs buildTargetPreflight, while
	// an internal mutation test can inject a refusal and prove both callers move
	// together.
	targetPreflight targetPreflightFunc

	// dice is the host's randomness, kept as well as wrapped: the initiative
	// seam needs it wrapped for the composition, and a resolution machine
	// needs it wrapped for the rulebook. One source, two adapters.
	dice Roller
}

// NewManager returns a Manager wired to what the host supplied.
//
// Construction is total (S8): every required field is checked here, and a
// missing one is named in the error. The alternative — discovering a nil
// dependency at call time — turns a wiring mistake into a panic in the middle
// of a player's turn, in production, instead of a startup failure a deployment
// can catch.
//
// Returns ErrNilConfig for a nil config, and ErrIncompleteConfig naming the
// first absent required field.
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("newmanager: %w", ErrNilConfig)
	}

	// Checked in a fixed order so the reported name is deterministic: a host
	// wiring several at once fixes them one predictable step at a time rather
	// than watching the message change between runs.
	required := []struct {
		name    string
		present bool
	}{
		{"Sessions", cfg.Sessions != nil},
		{"Encounters", cfg.Encounters != nil},
		{"Characters", cfg.Characters != nil},
		{"Events", cfg.Events != nil},
		{"Dice", cfg.Dice != nil},
		{"PresentationIDs", cfg.PresentationIDs != nil},
		// ONE ROW FOR THE PAIR, because either one satisfies the requirement
		// and a row naming only the first would send a host that meant to wire
		// the other to the wrong field.
		{"TurnDriver or TurnDrivers", cfg.TurnDriver != nil || cfg.TurnDrivers != nil},
	}
	for _, dep := range required {
		if !dep.present {
			return nil, fmt.Errorf("newmanager: %s: %w", dep.name, ErrIncompleteConfig)
		}
	}

	// EXACTLY ONE OF THE TWO, and the second half of that law is its own
	// refusal rather than a precedence rule. Two wired drivers are two answers
	// to "what does a member with no player do", and picking either silently
	// would leave a host watching the driver it did not mean to wire take
	// every turn in the process.
	if cfg.TurnDriver != nil && cfg.TurnDrivers != nil {
		return nil, fmt.Errorf("newmanager: TurnDriver and TurnDrivers: %w", ErrAmbiguousConfig)
	}

	if cfg.StaleTargetPolicy != "" && cfg.StaleTargetPolicy != StaleTargetRefuse && cfg.StaleTargetPolicy != StaleTargetAttempt {
		return nil, fmt.Errorf("newmanager: invalid StaleTargetPolicy: %w", ErrIncompleteConfig)
	}

	// One source either way: a host that wired the every-session driver gets
	// the static source built around it here, so no verb downstream has to ask
	// which of the two fields was set.
	drivers := cfg.TurnDrivers
	if drivers == nil {
		drivers = staticTurnDrivers{driver: cfg.TurnDriver}
	}

	return &Manager{
		staleTargetPolicy: cfg.StaleTargetPolicy,
		sessions:          cfg.Sessions,
		encounters:        cfg.Encounters,
		characters:        cfg.Characters,
		events:            cfg.Events,
		initiative:        initiativeSeam{dice: cfg.Dice},
		dice:              cfg.Dice,
		turnDrivers:       drivers,
		presentationIDs:   cfg.PresentationIDs,
		targetPreflight:   buildTargetPreflight,
	}, nil
}

// resolveTurnDriver asks the host which driver serves one session, and wraps
// it in the seam the composition speaks.
//
// CALLED ONCE PER VERB, at the two places a world is loaded:
// [Manager.openForWrite] for a write, which puts the answer on the scope every
// capability then reads it from, and [Manager.loadWorld] for a read.
// Resolution in two places rather than at each of the eleven sites that used
// to reach for the Manager's own driver is the point of the shape: one verb,
// one driver, and no way for two capabilities inside one call to be looking at
// two different brains.
//
// FAIL CLOSED, BOTH WAYS. A source that errors fails the verb with its own
// error wrapped — never a fallback to the reference driver, which would answer
// a monster's turn with somebody else's brain and read as a design choice. A
// source that reports success and hands over nothing has broken its contract,
// and is refused here rather than wrapped into a seam that would panic several
// frames later, in the middle of somebody's turn.
func (m *Manager) resolveTurnDriver(ctx context.Context, sessionID string) (encounter.Driver, error) {
	driver, err := m.turnDrivers.DriverFor(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("turn driver for session %q: %w", sessionID, err)
	}
	if driver == nil {
		return nil, fmt.Errorf("session %q: %w", sessionID, ErrNoTurnDriver)
	}

	// [Driver] — the creature's own table — is not wrapped but RECOGNISED, and
	// built one layer down around this session's dice. It cannot cross the
	// projecting seam: the twin view carries no table, no temperament and no
	// deeds, and a pick's arithmetic has nowhere to ride back on. See
	// [tableDriver] for the whole argument.
	return encounterDriverFor(driver, m.encounterDice()), nil
}

// encounterDice is this session's shared randomness in the shape the
// composition takes.
//
// ONE SOURCE, ONE ADAPTER SHAPE, A FRESH VALUE PER ASK. [diceSeam] remembers
// the first error its caller threw away, which is initiative's need and
// nobody else's; a value shared between the initiative order and a creature's
// table would carry one's failure into the other's account of itself. What is
// shared is the host's Roller underneath, which is the thing that has to be
// shared for a run to replay.
func (m *Manager) encounterDice() *diceSeam {
	return &diceSeam{roller: m.dice}
}
