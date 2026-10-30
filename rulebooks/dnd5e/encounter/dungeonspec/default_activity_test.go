package dungeonspec

import (
	"strings"

	"gopkg.in/yaml.v3"
)

func (s *SingleRoomSourceSuite) TestDefaultActivityRefIsCarriedWithoutChangingRulesAppearanceOrFaction() {
	const activity = "dnd5e:activities:thoughtful"
	const appearance = "dnd5e:npcs:goblin:prisoner-02"
	const original = "ref: 'dnd5e:monsters:skeleton'"
	withAppearance := original + ", appearanceRef: '" + appearance + "'"
	source := []byte(strings.Replace(string(s.raw), original, withAppearance, 1))
	base, err := Load(source)
	s.Require().NoError(err)
	raw := []byte(strings.Replace(string(source), withAppearance, withAppearance+", defaultActivityRef: '"+activity+"'", 1))
	s.Require().NotEqual(source, raw)
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	s.Require().NoError(err)
	s.Equal(activity, decoded.Spec.Room.Gameplay.MonsterDeclarations[0].DefaultActivityRef)
	compiled, err := Load(raw)
	s.Require().NoError(err)
	base.Monsters[0].DefaultActivityRef = activity
	s.Equal(base.Monsters, compiled.Monsters, "only the default presentation choice differs")
	s.Equal(base.Field, compiled.Field, "activity changes no geometry or faction policy")
	encoded, err := yaml.Marshal(decoded.Spec)
	s.Require().NoError(err)
	roundTrip, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	s.Require().NoError(err)
	s.Equal(activity, roundTrip.Spec.Room.Gameplay.MonsterDeclarations[0].DefaultActivityRef)
}

func (s *SingleRoomSourceSuite) TestMalformedExplicitDefaultActivityRefNamesItsSourcePath() {
	for _, value := range []string{"null", "42", "''", "'not-a-ref'"} {
		s.Run(value, func() {
			raw := []byte(strings.Replace(string(s.raw), "ref: 'dnd5e:monsters:skeleton'", "ref: 'dnd5e:monsters:skeleton', defaultActivityRef: "+value, 1))
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
			s.Require().Error(err)
			s.Contains(err.Error(), "defaultActivityRef")
		})
	}
}
