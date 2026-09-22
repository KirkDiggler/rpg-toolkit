package resolution

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

func fogMembershipCharacter(id string) *character.Data {
	return &character.Data{ID: id, Name: id, Level: 1, ClassID: "cleric", HitPoints: 10, MaxHitPoints: 10}
}

func fogMembershipRoom(t *testing.T, id string) spatial.Room {
	t.Helper()
	room := spatial.NewBasicRoom(spatial.BasicRoomConfig{ID: "fog-membership", Grid: spatial.NewHexGrid(spatial.HexGridConfig{Width: 20, Height: 20})})
	require.NoError(t, room.PlaceEntity(fogMember{id}, spatial.Position{X: 0, Y: 0}))
	return room
}

func fogMembershipArea(id string) encounter.SightAreaData {
	return encounter.SightAreaData{ID: id, SourceID: "caster-1", Name: "Fog Cloud", Ref: refs.Spells.FogCloud().String(), Center: encounter.PositionData{X: 0, Y: 0}, RadiusFeet: 20}
}

func fogMembershipParticipantData(out *FogMembershipOutput) []Participant {
	participants := make([]Participant, 0, len(out.DirtyCharacters)+len(out.DirtyMonsters))
	for _, data := range out.DirtyCharacters {
		participants = append(participants, Participant{Character: data})
	}
	for _, data := range out.DirtyMonsters {
		participants = append(participants, Participant{Monster: data})
	}
	return participants
}

func TestReconcileFogMembershipEntersAndLeaves(t *testing.T) {
	ctx := context.Background()
	hero := fogMembershipCharacter("hero-1")
	room := fogMembershipRoom(t, hero.ID)
	participants := []Participant{{Character: hero}}
	area := fogMembershipArea("fog-a")

	entered, err := ReconcileFogMembership(ctx, &FogMembershipInput{Participants: participants, Room: room, Areas: []encounter.SightAreaData{area}, Roller: dice.NewRoller()})
	require.NoError(t, err)
	require.Len(t, entered.DirtyCharacters, 1)
	require.Len(t, entered.DirtyCharacters[0].Conditions, 1)

	unchanged, err := ReconcileFogMembership(ctx, &FogMembershipInput{Participants: fogMembershipParticipantData(entered), Room: room, Areas: []encounter.SightAreaData{area}, Roller: dice.NewRoller()})
	require.NoError(t, err)
	require.Empty(t, unchanged.DirtyCharacters)

	left, err := ReconcileFogMembership(ctx, &FogMembershipInput{Participants: fogMembershipParticipantData(entered), Room: room, Areas: nil, Roller: dice.NewRoller()})
	require.NoError(t, err)
	require.Len(t, left.DirtyCharacters, 1)
	require.Empty(t, left.DirtyCharacters[0].Conditions)
}

func TestReconcileFogMembershipKeepsOverlappingCloudsIndependent(t *testing.T) {
	ctx := context.Background()
	hero := fogMembershipCharacter("hero-1")
	room := fogMembershipRoom(t, hero.ID)
	participants := []Participant{{Character: hero}}
	areas := []encounter.SightAreaData{fogMembershipArea("fog-a"), fogMembershipArea("fog-b")}

	entered, err := ReconcileFogMembership(ctx, &FogMembershipInput{Participants: participants, Room: room, Areas: areas, Roller: dice.NewRoller()})
	require.NoError(t, err)
	require.Len(t, entered.DirtyCharacters, 1)
	require.Len(t, entered.DirtyCharacters[0].Conditions, 2)

	oneLeft, err := ReconcileFogMembership(ctx, &FogMembershipInput{Participants: fogMembershipParticipantData(entered), Room: room, Areas: []encounter.SightAreaData{areas[1]}, Roller: dice.NewRoller()})
	require.NoError(t, err)
	require.Len(t, oneLeft.DirtyCharacters, 1)
	require.Len(t, oneLeft.DirtyCharacters[0].Conditions, 1)

	allLeft, err := ReconcileFogMembership(ctx, &FogMembershipInput{Participants: fogMembershipParticipantData(oneLeft), Room: room, Areas: nil, Roller: dice.NewRoller()})
	require.NoError(t, err)
	require.Len(t, allLeft.DirtyCharacters, 1)
	require.Empty(t, allLeft.DirtyCharacters[0].Conditions)
}

type fogMember struct{ id string }

func (e fogMember) GetID() string            { return e.id }
func (e fogMember) GetType() core.EntityType { return core.EntityType("character") }
