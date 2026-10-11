package dungeonspec

import (
	"strings"

	"gopkg.in/yaml.v3"
)

func (s *SingleRoomSourceSuite) TestAppearanceRefIsCarriedWithoutChangingRulesOrFaction() {
	const appearance = "dnd5e:npcs:kingdom:merchant-01"
	base, err := Load(s.raw)
	s.Require().NoError(err)
	raw := []byte(strings.Replace(string(s.raw), "ref: 'dnd5e:monsters:skeleton'", "ref: 'dnd5e:monsters:skeleton', appearanceRef: '"+appearance+"'", 1))
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Equal(appearance, decoded.Spec.Room.Gameplay.MonsterDeclarations[0].AppearanceRef)
	compiled, err := Load(raw)
	s.Require().NoError(err)
	base.Monsters[0].AppearanceRef = appearance
	s.Equal(base.Monsters, compiled.Monsters, "only the presentation ref differs")
	s.Equal(base.Field, compiled.Field, "appearance changes no geometry or faction policy")
	encoded, err := yaml.Marshal(decoded.Spec)
	s.Require().NoError(err)
	roundTrip, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	s.Require().NoError(err)
	s.Equal(appearance, roundTrip.Spec.Room.Gameplay.MonsterDeclarations[0].AppearanceRef)
}

func (s *SingleRoomSourceSuite) TestMalformedExplicitAppearanceRefNamesItsSourcePath() {
	for _, value := range []string{"null", "42", "''", "'not-a-ref'"} {
		s.Run(value, func() {
			raw := []byte(strings.Replace(string(s.raw), "ref: 'dnd5e:monsters:skeleton'", "ref: 'dnd5e:monsters:skeleton', appearanceRef: "+value, 1))
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
			s.Require().Error(err)
			s.Contains(err.Error(), "appearanceRef")
		})
	}
}
