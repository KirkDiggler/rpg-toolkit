// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func rosterPtr[T any](value T) *T {
	return &value
}

func rosterAppearance() *customization.Appearance {
	return &customization.Appearance{
		Hair: &customization.HairCustomization{
			Scalp: &customization.StyleSelection{
				Kind:     customization.StyleSelectionStyle,
				StyleRef: "provider:hair:38",
			},
			FacialHair: &customization.StyleSelection{Kind: customization.StyleSelectionNone},
			ColorSRGB:  rosterPtr(uint32(0)),
			Roughness:  rosterPtr(float32(0.25)),
		},
		Outfit: &customization.OutfitCustomization{
			PrimaryColorSRGB:   rosterPtr(uint32(0)),
			SecondaryColorSRGB: rosterPtr(uint32(0xFFFFFF)),
		},
	}
}

func rosterCharacter(id, name, player string, appearance *customization.Appearance) *character.Data {
	return &character.Data{
		ID:               id,
		PlayerID:         player,
		Name:             name,
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			"str": 16, "dex": 14, "con": 14,
			"int": 10, "wis": 12, "cha": 8,
		},
		HitPoints: 24, MaxHitPoints: 28, ArmorClass: 16,
		Appearance: appearance,
	}
}

type rosterFixture struct {
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	manager    *session.Manager
}

func newRosterFixture(t *testing.T) *rosterFixture {
	t.Helper()

	alice := rosterCharacter("alice", "Fresh Alice", "player-alice", rosterAppearance())
	bob := rosterCharacter("bob", "Fresh Bob", "player-bob", nil)
	characters := newFakeCharacters(alice, bob)
	sessions := newFakeSessions()
	encounters := newFakeEncounters()
	world := rosterWorld(t)
	encounters.byID["world"] = world
	sessions.byID["sess"] = &session.SessionData{
		ID:        "sess",
		Encounter: "world",
		NPCs: []monster.Data{{
			ID: "skel-1", Name: "Skeleton", Ref: refs.Monsters.Skeleton(),
		}},
	}

	manager, err := session.NewManager(&session.Config{
		Sessions: sessions, Encounters: encounters, Characters: characters,
		Events: session.DiscardEvents{}, Dice: testDice{}, TurnDriver: session.Pass{},
	})
	require.NoError(t, err)

	return &rosterFixture{
		sessions: sessions, encounters: encounters, characters: characters,
		manager: manager,
	}
}

func rosterWorld(t *testing.T) *encounter.EncounterData {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{},
		TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Name: "Authored Alice", Position: spatial.Position{X: 0, Y: 0}},
			{ID: "bob", Kind: encounter.KindPlayer, Name: "Authored Bob", Position: spatial.Position{X: 1, Y: 0}},
			{ID: "skel-1", Kind: encounter.KindMonster, Name: "Authored Skeleton", Position: spatial.Position{X: 2, Y: 0}},
			{ID: "vendor-1", Kind: encounter.KindWorld, Name: "Vendor", Position: spatial.Position{X: 3, Y: 0}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()
	return &data
}

func TestRosterProjectsMixedRoster(t *testing.T) {
	fixture := newRosterFixture(t)

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess",
		Player:  "player-alice",
	})
	require.NoError(t, err)
	require.Equal(t, []session.PublicMember{
		{
			ID: "alice", Kind: session.KindPlayer, Name: "Fresh Alice",
			ClassRef: "fighter", RaceRef: "human",
			Customization: session.Customization{
				Hair: &session.HairCustomization{
					Scalp:      &session.StyleSelection{Kind: "style", StyleRef: "provider:hair:38"},
					FacialHair: &session.StyleSelection{Kind: "none"},
					ColorSRGB:  rosterPtr(uint32(0)), Roughness: rosterPtr(float32(0.25)),
				},
				Outfit: &session.OutfitCustomization{
					PrimaryColorSRGB:   rosterPtr(uint32(0)),
					SecondaryColorSRGB: rosterPtr(uint32(0xFFFFFF)),
				},
			},
		},
		{
			ID: "bob", Kind: session.KindPlayer, Name: "Fresh Bob",
			ClassRef: "fighter", RaceRef: "human",
			Customization: session.Customization{},
		},
		{
			ID: "skel-1", Kind: session.KindMonster, Name: "Skeleton",
			MonsterRef:    "dnd5e:monsters:skeleton",
			Customization: session.Customization{},
		},
	}, out.Members)
}

func TestRosterIsReadOnlyAndPreservesEncounterOrder(t *testing.T) {
	fixture := newRosterFixture(t)

	_, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.NoError(t, err)
	require.Zero(t, fixture.sessions.saves)
	require.Zero(t, fixture.encounters.saves)

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"alice", "bob", "skel-1"}, []string{
		out.Members[0].ID, out.Members[1].ID, out.Members[2].ID,
	})
}

func TestRosterReloadsCurrentCharacterIdentityOnEveryCall(t *testing.T) {
	fixture := newRosterFixture(t)
	input := &session.RosterInput{Session: "sess", Player: "player-alice"}

	first, err := fixture.manager.Roster(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "Fresh Alice", first.Members[0].Name)

	fixture.characters.byID["alice"].Name = "Renamed Alice"
	second, err := fixture.manager.Roster(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "Renamed Alice", second.Members[0].Name)
}

func TestRosterValidatesInputAndDependencies(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		call func(*rosterFixture) error
		want error
	}{
		{
			name: "nil input",
			call: func(f *rosterFixture) error {
				_, err := f.manager.Roster(ctx, nil)
				return err
			},
			want: session.ErrNilInput,
		},
		{
			name: "empty session",
			call: func(f *rosterFixture) error {
				_, err := f.manager.Roster(ctx, &session.RosterInput{Player: "player-alice"})
				return err
			},
			want: session.ErrNoSessionID,
		},
		{
			name: "empty player",
			call: func(f *rosterFixture) error {
				_, err := f.manager.Roster(ctx, &session.RosterInput{Session: "sess"})
				return err
			},
			want: session.ErrNoMemberID,
		},
		{
			name: "missing session",
			call: func(f *rosterFixture) error {
				_, err := f.manager.Roster(ctx, &session.RosterInput{Session: "missing", Player: "player-alice"})
				return err
			},
			want: session.ErrNoSession,
		},
		{
			name: "missing encounter",
			call: func(f *rosterFixture) error {
				f.sessions.byID["orphan"] = &session.SessionData{ID: "orphan", Encounter: "missing"}
				_, err := f.manager.Roster(ctx, &session.RosterInput{Session: "orphan", Player: "player-alice"})
				return err
			},
			want: session.ErrNoEncounter,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(newRosterFixture(t))
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRosterRefusesAnUnauthenticatedPrincipal(t *testing.T) {
	fixture := newRosterFixture(t)

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-nobody",
	})
	require.ErrorIs(t, err, session.ErrNotSeated)
	require.Nil(t, out)
}

func TestRosterRefusesMissingCharacter(t *testing.T) {
	fixture := newRosterFixture(t)
	delete(fixture.characters.byID, "bob")

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrNoCharacter)
	require.Nil(t, out)
}

func TestRosterRefusesCorruptCharacter(t *testing.T) {
	fixture := newRosterFixture(t)
	fixture.characters.byID["alice"].Appearance.Hair.ColorSRGB = rosterPtr(uint32(0x1000000))

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrBadCharacter)
	require.Nil(t, out)
}

func TestRosterRefusesACharacterReturnedUnderTheWrongID(t *testing.T) {
	fixture := newRosterFixture(t)
	fixture.characters.byID["bob"].ID = "not-bob"

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrBadRepository)
	require.Nil(t, out)
}

func TestRosterRefusesANilCharacterRepositoryResult(t *testing.T) {
	fixture := newRosterFixture(t)
	manager, err := session.NewManager(&session.Config{
		Sessions: fixture.sessions, Encounters: fixture.encounters,
		Characters: nilRosterRepository{}, Events: session.DiscardEvents{},
		Dice: testDice{}, TurnDriver: session.Pass{},
	})
	require.NoError(t, err)

	out, err := manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrBadRepository)
	require.Nil(t, out)
}

type nilRosterRepository struct{}

func (nilRosterRepository) GetCharacter(context.Context, string) (*character.Data, error) {
	return nil, nil
}

func (nilRosterRepository) SaveCharacter(context.Context, *character.Data) error {
	return nil
}

func TestRosterRefusesMissingMonsterSheet(t *testing.T) {
	fixture := newRosterFixture(t)
	fixture.sessions.byID["sess"].NPCs = nil

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrNoSheet)
	require.Nil(t, out)
}

func TestRosterRefusesDuplicateMonsterSheet(t *testing.T) {
	fixture := newRosterFixture(t)
	fixture.sessions.byID["sess"].NPCs = append(
		fixture.sessions.byID["sess"].NPCs,
		fixture.sessions.byID["sess"].NPCs[0],
	)

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrBadNPC)
	require.Nil(t, out)
}

func TestRosterRefusesCorruptMonsterIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*monster.Data)
	}{
		{
			name: "missing name",
			edit: func(data *monster.Data) { data.Name = "" },
		},
		{
			name: "missing ref",
			edit: func(data *monster.Data) { data.Ref = nil },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newRosterFixture(t)
			tc.edit(&fixture.sessions.byID["sess"].NPCs[0])

			out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
				Session: "sess", Player: "player-alice",
			})
			require.ErrorIs(t, err, session.ErrBadNPC)
			require.Nil(t, out)
		})
	}
}

func TestRosterRefusesUnknownMemberKind(t *testing.T) {
	fixture := newRosterFixture(t)
	for i := range fixture.encounters.byID["world"].Members {
		if fixture.encounters.byID["world"].Members[i].ID == "bob" {
			fixture.encounters.byID["world"].Members[i].Kind = encounter.MemberKind("summoned")
		}
	}

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrInvalidWorld)
	require.Nil(t, out)
}

func TestRosterOutputHasNoPrivateSheetOrPlacementFields(t *testing.T) {
	privateNames := []string{"PlayerID", "Position", "HitPoints", "MaxHitPoints", "Inventory", "Level", "ArmorClass"}
	for _, value := range []any{
		session.PublicMember{}, session.Customization{}, session.HairCustomization{},
		session.OutfitCustomization{}, session.StyleSelection{},
	} {
		typeOfValue := reflect.TypeOf(value)
		for _, name := range privateNames {
			_, present := typeOfValue.FieldByName(name)
			require.False(t, present, "%s must not appear on %s", name, typeOfValue.Name())
		}
	}
	require.Equal(t,
		[]string{"ID", "Kind", "Name", "ClassRef", "RaceRef", "MonsterRef", "Customization"},
		rosterFieldNames(session.PublicMember{}),
	)
}

func rosterFieldNames(value any) []string {
	typeOfValue := reflect.TypeOf(value)
	fields := make([]string, typeOfValue.NumField())
	for i := range fields {
		fields[i] = typeOfValue.Field(i).Name
	}
	return fields
}

func TestRosterReturnedCustomizationIsDetached(t *testing.T) {
	fixture := newRosterFixture(t)
	storedAppearance := customization.CloneAppearance(fixture.characters.byID["alice"].Appearance)

	out, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.NoError(t, err)

	out.Members[0].Name = "mutated public name"
	out.Members[0].Customization.Hair.Scalp.StyleRef = "mutated hair"
	*out.Members[0].Customization.Hair.ColorSRGB = 0x123456
	*out.Members[0].Customization.Hair.Roughness = 1
	*out.Members[0].Customization.Outfit.PrimaryColorSRGB = 0x654321
	*out.Members[0].Customization.Outfit.SecondaryColorSRGB = 0
	out.Members[2].Name = "mutated monster name"

	require.Equal(t, storedAppearance, fixture.characters.byID["alice"].Appearance)
	require.Equal(t, "Skeleton", fixture.sessions.byID["sess"].NPCs[0].Name)

	again, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.NoError(t, err)
	require.Equal(t, "Fresh Alice", again.Members[0].Name)
	require.Equal(t, "provider:hair:38", again.Members[0].Customization.Hair.Scalp.StyleRef)
	require.Equal(t, "Skeleton", again.Members[2].Name)
}

func TestRosterSurvivesManagerRestartFromCopiedPersistence(t *testing.T) {
	fixture := newRosterFixture(t)
	first, err := fixture.manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.NoError(t, err)

	storedSession, err := copyOf(fixture.sessions.byID["sess"])
	require.NoError(t, err)
	storedEncounter, err := copyOf(fixture.encounters.byID["world"])
	require.NoError(t, err)
	alice, err := copyOf(fixture.characters.byID["alice"])
	require.NoError(t, err)
	bob, err := copyOf(fixture.characters.byID["bob"])
	require.NoError(t, err)

	restartedSessions := newFakeSessions()
	restartedSessions.byID["sess"] = storedSession
	restartedEncounters := newFakeEncounters()
	restartedEncounters.byID["world"] = storedEncounter
	restartedCharacters := newFakeCharacters(alice, bob)
	restarted, err := session.NewManager(&session.Config{
		Sessions: restartedSessions, Encounters: restartedEncounters,
		Characters: restartedCharacters, Events: session.DiscardEvents{},
		Dice: testDice{}, TurnDriver: session.Pass{},
	})
	require.NoError(t, err)

	afterRestart, err := restarted.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.NoError(t, err)
	require.Equal(t, first, afterRestart)

	// The copied stores, not the original manager or its loaded values, are the
	// source of truth after the restart.
	require.NotSame(t, fixture.sessions.byID["sess"], restartedSessions.byID["sess"])
	require.NotSame(t, fixture.encounters.byID["world"], restartedEncounters.byID["world"])
}

func TestRosterRejectsMalformedNPCRefWithoutLeakingCoreErrors(t *testing.T) {
	// A direct repository is needed here because the shared JSON fake correctly
	// rejects malformed core.Ref values while copying them. The roster must
	// still translate the in-memory corrupt identity to its own sentinel.
	fixture := newRosterFixture(t)
	sessions := newDirectRosterSessions(&session.SessionData{
		ID: "sess", Encounter: "world", NPCs: []monster.Data{{
			ID: "skel-1", Name: "Skeleton", Ref: &core.Ref{Module: "", Type: "monsters", ID: "skeleton"},
		}},
	})
	manager, err := session.NewManager(&session.Config{
		Sessions: sessions, Encounters: fixture.encounters, Characters: fixture.characters,
		Events: session.DiscardEvents{}, Dice: testDice{}, TurnDriver: session.Pass{},
	})
	require.NoError(t, err)

	out, err := manager.Roster(context.Background(), &session.RosterInput{
		Session: "sess", Player: "player-alice",
	})
	require.ErrorIs(t, err, session.ErrBadNPC)
	require.Nil(t, out)
}

type directRosterSessions struct {
	data *session.SessionData
}

func newDirectRosterSessions(data *session.SessionData) *directRosterSessions {
	return &directRosterSessions{data: data}
}

func (s *directRosterSessions) GetSession(_ context.Context, id string) (*session.SessionData, error) {
	if id != s.data.ID {
		return nil, session.ErrNotFound
	}
	return s.data, nil
}

func (s *directRosterSessions) SaveSession(_ context.Context, data *session.SessionData) error {
	s.data = data
	return nil
}
