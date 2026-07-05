package character

import (
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// MonkResourcesTestSuite tests Monk-specific class resource initialization,
// in particular the Ki resource's level-2 gate (rpg-toolkit#746).
type MonkResourcesTestSuite struct {
	suite.Suite
}

func TestMonkResourcesSuite(t *testing.T) {
	suite.Run(t, new(MonkResourcesTestSuite))
}

// newTestMonkDraftAndCharacter builds a bare Draft/Character pair for
// exercising initializeClassResources directly at a given level. The public
// SetRace/SetClass/SetBackground/ToCharacter flow always produces a level 1
// character (leveling up isn't implemented yet), so this is the only way to
// exercise the level-2 Ki gate today.
func newTestMonkDraftAndCharacter(level int) (*Draft, *Character) {
	draft := &Draft{class: classes.Monk}
	char := &Character{
		id:        "test-monk",
		level:     level,
		resources: make(map[coreResources.ResourceKey]*combat.RecoverableResource),
	}
	return draft, char
}

func (s *MonkResourcesTestSuite) TestLevel1MonkHasNoKiResource() {
	draft, char := newTestMonkDraftAndCharacter(1)

	draft.initializeClassResources(char)

	_, ok := char.resources[resources.Ki]
	s.False(ok, "Level 1 Monk should not have a Ki resource (Ki starts at Monk level 2)")
}

func (s *MonkResourcesTestSuite) TestLevel2MonkHasKiResource() {
	draft, char := newTestMonkDraftAndCharacter(2)

	draft.initializeClassResources(char)

	kiResource, ok := char.resources[resources.Ki]
	s.Require().True(ok, "Level 2 Monk should have a Ki resource")
	s.Equal(2, kiResource.Maximum(), "Level 2 Monk should have 2 max Ki points")
	s.Equal(2, kiResource.Current(), "Ki should start at maximum")
}
