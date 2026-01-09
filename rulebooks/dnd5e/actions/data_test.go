package actions

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

type LoadFromDataTestSuite struct {
	suite.Suite
}

func TestLoadFromDataSuite(t *testing.T) {
	suite.Run(t, new(LoadFromDataTestSuite))
}

func (s *LoadFromDataTestSuite) TestLoadFromData_NilRef() {
	s.Run("returns error when ref is nil", func() {
		data := ActionData{
			ID:      "test-action",
			OwnerID: "test-owner",
		}

		action, err := LoadFromData(data)
		s.Require().Error(err)
		s.Nil(action)
		s.Contains(err.Error(), "requires a ref")
	})
}

func (s *LoadFromDataTestSuite) TestLoadFromData_UnknownRef() {
	s.Run("returns error for unknown action type", func() {
		data := ActionData{
			Ref:     &core.Ref{Module: "dnd5e", Type: "actions", ID: "unknown"},
			ID:      "test-action",
			OwnerID: "test-owner",
		}

		action, err := LoadFromData(data)
		s.Require().Error(err)
		s.Nil(action)
		s.Contains(err.Error(), "unknown action type")
	})
}

func (s *LoadFromDataTestSuite) TestLoadFromData_Move() {
	s.Run("loads Move action successfully", func() {
		data := ActionData{
			Ref:     refs.Actions.Move(),
			ID:      "move-1",
			OwnerID: "char-1",
		}

		action, err := LoadFromData(data)
		s.Require().NoError(err)
		s.Require().NotNil(action)

		move, ok := action.(*Move)
		s.Require().True(ok, "should be a *Move")
		s.Equal("move-1", move.GetID())
		s.Equal("char-1", move.ownerID)
	})
}

func (s *LoadFromDataTestSuite) TestLoadFromData_Strike() {
	s.Run("loads Strike action successfully", func() {
		data := ActionData{
			Ref:      refs.Actions.Strike(),
			ID:       "strike-1",
			OwnerID:  "char-1",
			WeaponID: weapons.Longsword,
		}

		action, err := LoadFromData(data)
		s.Require().NoError(err)
		s.Require().NotNil(action)

		strike, ok := action.(*Strike)
		s.Require().True(ok, "should be a *Strike")
		s.Equal("strike-1", strike.GetID())
		s.Equal("char-1", strike.ownerID)
		s.Equal(weapons.Longsword, strike.weaponID)
	})
}

func (s *LoadFromDataTestSuite) TestLoadFromData_OffHandStrike() {
	s.Run("loads OffHandStrike action successfully", func() {
		data := ActionData{
			Ref:      refs.Actions.OffHandStrike(),
			ID:       "offhand-1",
			OwnerID:  "char-1",
			WeaponID: weapons.Dagger,
		}

		action, err := LoadFromData(data)
		s.Require().NoError(err)
		s.Require().NotNil(action)

		offhand, ok := action.(*OffHandStrike)
		s.Require().True(ok, "should be a *OffHandStrike")
		s.Equal("offhand-1", offhand.GetID())
		s.Equal("char-1", offhand.ownerID)
		s.Equal(weapons.Dagger, offhand.weaponID)
		// Capacity is tracked via ActionEconomy, not internally
		s.Equal(UnlimitedUses, offhand.UsesRemaining())
	})
}

func (s *LoadFromDataTestSuite) TestLoadFromData_FlurryStrike() {
	s.Run("loads FlurryStrike action successfully", func() {
		data := ActionData{
			Ref:     refs.Actions.FlurryStrike(),
			ID:      "flurry-1",
			OwnerID: "monk-1",
		}

		action, err := LoadFromData(data)
		s.Require().NoError(err)
		s.Require().NotNil(action)

		flurry, ok := action.(*FlurryStrike)
		s.Require().True(ok, "should be a *FlurryStrike")
		s.Equal("flurry-1", flurry.GetID())
		s.Equal("monk-1", flurry.ownerID)
		// Capacity is tracked via ActionEconomy, not internally
		s.Equal(UnlimitedUses, flurry.UsesRemaining())
	})
}

func (s *LoadFromDataTestSuite) TestLoadFromData_UnarmedStrike() {
	s.Run("loads UnarmedStrike as Strike with no weapon", func() {
		data := ActionData{
			Ref:     refs.Actions.UnarmedStrike(),
			ID:      "unarmed-1",
			OwnerID: "monk-1",
		}

		action, err := LoadFromData(data)
		s.Require().NoError(err)
		s.Require().NotNil(action)

		strike, ok := action.(*Strike)
		s.Require().True(ok, "should be a *Strike")
		s.Equal("unarmed-1", strike.GetID())
		s.Equal("monk-1", strike.ownerID)
		s.Equal(weapons.WeaponID(""), strike.weaponID, "should have no weapon")
	})
}
