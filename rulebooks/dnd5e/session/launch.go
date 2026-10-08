// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// Launch (rpg-project#542, "Launch", R7).
//
// One call builds the whole board, seats the party and lets the fight form,
// in one load-act-save under the session's guard and each party character's
// guard. It replaces StartSession and Spawn as host verbs; Join stays for a
// rejoin, and PlaceNPC for the demo vendor until authored world NPCs (R11).
//
// # What the host hands over, and what it no longer does
//
// The session id (the host mints it), the dungeon the toolkit's compiler
// produced (monster member ids already minted by the compile), the key the
// host registered it under, and the party in seat order. The host
// re-projects nothing: the authored cells, the endings the dungeon declares,
// the scenarios it binds and the boss it names are read here, off the
// compiled dungeon.
//
// # The order is the law
//
//  1. Everything that can be refused is refused before anything is written:
//     a party bigger than the seats, an id claimed twice, a character another
//     run holds, any sheet or monster or faction that cannot be resolved.
//  2. The whole board stands before anyone is seated: every monster is
//     placed on the world in memory.
//  3. Each party character's first-admission long rest is saved, then its
//     seat — the rested and seated sheets land before the run that holds
//     them (seats.go says why that order).
//  4. The party is placed, in seat order, onto the finished board; the fight
//     forms then and holds everyone it should.
//  5. The run is saved; one report names every aggregate written or failed.
//
// Placement is sequential, garrison first, and that is enough for the board
// law: the composition forms a fight only around a player in contact, so
// monsters that see each other while the board stands — two authored
// factions hostile to each other included — form nothing until the party
// arrives, and the party's arrival forms the one fight with everybody in it.

// Ending keys every launched run declares beside the dungeon's own.
const (
	// EndingWithdrawn is the party withdrawing: an external ending a host
	// fires with End.
	EndingWithdrawn = "withdrawn"

	// EndingBossDown fires when the dungeon's one boss placement goes down.
	// Declared only for a dungeon that names a boss.
	EndingBossDown = "boss-down"
)

// LaunchInput starts a run of an authored dungeon with a party.
type LaunchInput struct {
	// Session is the run's id, minted by the host. The run's world is stored
	// under the same id.
	Session string

	// DungeonKey is the key the host registered the dungeon under, written
	// on the session record so a client can fetch the room's appearance from
	// the same registry entry.
	DungeonKey string

	// Dungeon is the authored dungeon as the toolkit's compiler produced it,
	// monster member ids already minted. Shared vocabulary the host holds
	// from its registry, not a persistence shape.
	Dungeon *dungeonspec.Compiled

	// Party is the party's character ids in seat order: the first takes the
	// dungeon's first seat. Each is also the character's member id.
	Party []string
}

// LaunchOutput reports the launched run.
type LaunchOutput struct {
	// Session is the launched run's id.
	Session string

	// Members is every member placed on the board — monsters in authored
	// order, then the party in seat order. A monster held in reserve is not
	// on the board and not here.
	Members []Member

	// Formed is present if the party's arrival started a fight.
	Formed *Formed

	// Saved names every aggregate written: each party character, each seat,
	// the run's world and its session record.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// launchMonster is one compiled placement resolved into what placing it needs.
type launchMonster struct {
	placement dungeonspec.MonsterPlacement
	sheet     *monster.Data
	table     encounter.Table
	temper    encounter.Temper
}

// Launch starts a run: the whole board, the party seated and rested, the fight
// formed last, one load-act-save (see the top of this file).
//
// Returns ErrNilInput, ErrNoSessionID, ErrInvalidWorld (no dungeon, a dungeon
// whose world or scenarios cannot be built, a party bigger than the seats),
// ErrNoMemberID, ErrDuplicateMember (an id claimed twice), ErrSessionExists,
// ErrSeatedElsewhere, ErrNoCharacter, ErrBadCharacter, ErrNoRef, ErrBadRef,
// ErrNoLoader, ErrUnknownContent, ErrNoFaction, ErrBadPosition, or
// ErrSaveFailed with a populated report.
func (m *Manager) Launch(ctx context.Context, in *LaunchInput) (*LaunchOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("launch: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Session == "" {
		return nil, fmt.Errorf("launch: %w", ErrNoSessionID)
	}
	if err := validateLaunch(in); err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	if err := m.refuseExistingSession(ctx, in.Session); err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}

	monsters, err := resolveLaunchMonsters(in.Dungeon)
	if err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	world, err := launchWorld(in.Dungeon)
	if err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}

	data := &SessionData{ID: in.Session, Encounter: in.Session, Dungeon: in.DungeonKey}
	scope, err := m.openScope(ctx, data, world, in.Party...)
	if err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	scope.touched = true

	// EACH PARTY CHARACTER'S GUARD, after the session's, in id order.
	releaseParty, err := m.acquireCharactersFor(ctx, scope, in.Party...)
	if err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	defer releaseParty()

	// 1. Refused before anything is written: every sheet resolves, nobody is
	// seated elsewhere, and each first-admission rest resolves (in memory).
	sheets := m.sheetsFor(scope)
	rested := make([]*character.Data, 0, len(in.Party))
	for _, id := range in.Party {
		if err := m.refuseSeatedElsewhere(ctx, in.Session, id); err != nil {
			return nil, fmt.Errorf("launch: %w", err)
		}
		record, err := sheets.load(ctx, "party member", id)
		if err != nil {
			return nil, fmt.Errorf("launch: %w", err)
		}
		resolved, err := firstAdmissionRest(ctx, record)
		if err != nil {
			return nil, fmt.Errorf("launch: %w", err)
		}
		rested = append(rested, resolved)
	}

	// 2. The whole board stands, in memory, before anyone is seated. A
	// faction or cell the world refuses stops the launch here, with nothing
	// written.
	var members []Member
	for _, monster := range monsters {
		placed, err := m.placeLaunchMonster(scope, in.Dungeon.Field.Canvas.Orientation, monster)
		if err != nil {
			return nil, fmt.Errorf("launch: monster %q: %w", monster.placement.MemberID, err)
		}
		if !placed.Reserved {
			members = append(members, projectMember(placed.Member))
		}
	}

	// 3. Rested, then seated: both land before the run that holds them.
	for _, record := range rested {
		if err := sheets.save(ctx, record); err != nil {
			return nil, fmt.Errorf("launch: %w", err)
		}
	}
	for _, id := range in.Party {
		if err := m.writeSeat(ctx, scope, id, in.Session); err != nil {
			return nil, fmt.Errorf("launch: %w", err)
		}
	}

	// 4. The party arrives onto the finished board, in seat order.
	var (
		formed   *encounter.FormedBubble
		formedBy string
	)
	for i, record := range rested {
		projected, err := projectCharacter(ctx, record.ID, record)
		if err != nil {
			return nil, fmt.Errorf("launch: %w", saveErrorAfterWrites(scope, "", err))
		}
		seat := in.Dungeon.PartyStart[i].At
		cell := encounter.HexCellAt(in.Dungeon.Field.Canvas.Orientation, int(seat.X), int(seat.Y))
		placed, err := place(scope, record.ID, KindPlayer, projected.Sheet.Name, cell,
			false, nil, "", nil, socialPlacement{})
		if err != nil {
			return nil, fmt.Errorf("launch: party member %q: %w", record.ID, saveErrorAfterWrites(scope, "", err))
		}
		members = append(members, projectMember(placed.Member))
		if placed.Formed != nil {
			formed, formedBy = placed.Formed, record.ID
		}
	}

	// 5. The run.
	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	return &LaunchOutput{
		Session:  in.Session,
		Members:  members,
		Formed:   projectFormedFor(scope, formedBy, formed),
		Saved:    report,
		Delivery: delivery,
	}, nil
}

// validateLaunch refuses a launch that is wrong in itself, before any read: no
// dungeon, no party, a party bigger than the seats, an empty id, or an id
// claimed twice across the party and the compiled monsters.
func validateLaunch(in *LaunchInput) error {
	if in.Dungeon == nil {
		return fmt.Errorf("no dungeon: %w", ErrInvalidWorld)
	}
	if len(in.Party) == 0 {
		return fmt.Errorf("no party: %w", ErrNoMemberID)
	}
	if len(in.Party) > len(in.Dungeon.PartyStart) {
		return fmt.Errorf("a party of %d and a dungeon that seats %d: %w",
			len(in.Party), len(in.Dungeon.PartyStart), ErrInvalidWorld)
	}
	claimed := make(map[string]string, len(in.Party)+len(in.Dungeon.Monsters))
	claim := func(id, who string) error {
		if id == "" {
			return fmt.Errorf("%s has no id: %w", who, ErrNoMemberID)
		}
		if prev, taken := claimed[id]; taken {
			return fmt.Errorf("member id %q is claimed twice, by %s and by %s: %w", id, prev, who, ErrDuplicateMember)
		}
		claimed[id] = who
		return nil
	}
	for i, id := range in.Party {
		if err := claim(id, fmt.Sprintf("party member %d", i)); err != nil {
			return err
		}
	}
	for _, placement := range in.Dungeon.Monsters {
		if err := claim(placement.MemberID, fmt.Sprintf("monster %s", placement.Ref)); err != nil {
			return err
		}
	}
	return nil
}

// refuseExistingSession refuses a session id already in use, as StartSession
// does: the id names a game in progress.
func (m *Manager) refuseExistingSession(ctx context.Context, id string) error {
	existing, err := m.sessions.GetSession(ctx, id)
	switch {
	case err == nil && existing != nil:
		return fmt.Errorf("%q: %w", id, ErrSessionExists)
	case err == nil:
		return fmt.Errorf("%q: GetSession reported success with no data: %w", id, ErrBadRepository)
	case errors.Is(err, ErrNotFound):
		return nil
	default:
		return fmt.Errorf("checking for an existing session: %w", err)
	}
}

// resolveLaunchMonsters builds every compiled placement's sheet, folds its
// table and resolves its temper, refusing the first that cannot be resolved —
// all before the world exists, so nothing is placed for a launch that fails.
func resolveLaunchMonsters(dungeon *dungeonspec.Compiled) ([]launchMonster, error) {
	out := make([]launchMonster, 0, len(dungeon.Monsters))
	for _, placement := range dungeon.Monsters {
		sheet, err := instantiate(placement.MemberID, placement.Ref, placement.Actions)
		if err != nil {
			return nil, fmt.Errorf("monster %q: %w", placement.MemberID, err)
		}
		table, err := foldedTable(sheet.Ref, placement.Table)
		if err != nil {
			return nil, fmt.Errorf("monster %q: %w", placement.MemberID, err)
		}
		temper, err := resolvedTemper(placement.Temper)
		if err != nil {
			return nil, fmt.Errorf("monster %q: %w", placement.MemberID, err)
		}
		out = append(out, launchMonster{placement: placement, sheet: sheet, table: table, temper: temper})
	}
	return out, nil
}

// launchWorld builds the run's empty world from the compiled field and the
// endings the dungeon declares: the party withdrawing, the boss going down
// when it names one, every scenario it binds, and its own authored endings.
// Nobody is on it yet.
func launchWorld(dungeon *dungeonspec.Compiled) (*encounter.EncounterData, error) {
	endings, err := launchEndings(dungeon)
	if err != nil {
		return nil, err
	}
	setup := encounter.CompileOnlySetup(dungeon.Field, endings)
	setup.Retention = encounter.RetentionUnbounded
	enc, err := encounter.NewEncounter(setup)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWorld, err)
	}
	data := enc.ToData()
	return &data, nil
}

// launchEndings is every ending a launched run declares, in a fixed order.
func launchEndings(dungeon *dungeonspec.Compiled) ([]encounter.EndingInput, error) {
	endings := []encounter.EndingInput{{Key: EndingWithdrawn, Trigger: encounter.TriggerExternal{}}}

	var boss string
	for _, placement := range dungeon.Monsters {
		if !placement.Boss {
			continue
		}
		if boss != "" {
			return nil, fmt.Errorf("the dungeon names two bosses, %q and %q: %w",
				boss, placement.MemberID, ErrInvalidWorld)
		}
		boss = placement.MemberID
	}
	if boss != "" {
		endings = append(endings, encounter.EndingInput{
			Key: EndingBossDown, Trigger: encounter.TriggerMemberDown{Member: encounter.MemberID(boss)},
		})
	}

	ids := make([]string, 0, len(dungeon.Scenarios))
	for id := range dungeon.Scenarios {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	facts := scenarios.FactsFrom(dungeon.Field)
	for _, id := range ids {
		scenario, known := scenarios.Lookup(id)
		if !known {
			return nil, fmt.Errorf("scenario %q: none by that name: %w", id, ErrInvalidWorld)
		}
		declared, err := scenario.New(dungeon.Scenarios[id], facts)
		if err != nil {
			return nil, fmt.Errorf("scenario %q: %w: %v", id, ErrInvalidWorld, err)
		}
		endings = append(endings, declared.Endings...)
	}
	return append(endings, dungeon.Endings...), nil
}

// placeLaunchMonster records one resolved monster's sheet in the session and
// places it at its authored cell, exactly as Spawn places one: the sheet goes
// in before the member so the member's own sight refresh can read it.
func (m *Manager) placeLaunchMonster(
	scope *writeScope, orientation encounter.Orientation, monster launchMonster,
) (*encounter.JoinOutput, error) {
	scope.data.NPCs = append(scope.data.NPCs, *monster.sheet)
	at := monster.placement.At
	cell := encounter.HexCellAt(orientation, int(at.X), int(at.Y))
	return place(scope, monster.placement.MemberID, KindMonster, monster.sheet.Name, cell,
		false, monster.placement.Holds, monster.placement.Faction, monster.placement.Arrives,
		socialPlacement{
			Intimidate: monster.placement.Intimidate, Persuade: monster.placement.Persuade,
			Table: monster.table, Temper: monster.temper,
		})
}
