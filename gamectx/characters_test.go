// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/gamectx"
)

// CharacterRegistryTestSuite tests the BasicCharacterRegistry implementation
type CharacterRegistryTestSuite struct {
	suite.Suite
	registry *gamectx.BasicCharacterRegistry
}

func (s *CharacterRegistryTestSuite) SetupTest() {
	s.registry = gamectx.NewBasicCharacterRegistry()
}

func (s *CharacterRegistryTestSuite) TestAddAndRetrieveCharacter() {
	// Create a character with a one-handed weapon
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, nil)

	// Add to registry
	s.registry.Add("hero-1", weapons)

	// Retrieve and verify
	retrieved, ok := s.registry.Get("hero-1")
	s.Require().True(ok, "Character should be found in registry")
	s.Require().NotNil(retrieved)
	s.Equal(longsword, retrieved.MainHand())
}

func (s *CharacterRegistryTestSuite) TestGetNonexistentCharacter() {
	// Attempt to retrieve a character that doesn't exist
	retrieved, ok := s.registry.Get("nonexistent")
	s.False(ok, "Character should not be found")
	s.Nil(retrieved)
}

func (s *CharacterRegistryTestSuite) TestGetCharacterInterface() {
	// Test the CharacterRegistry interface method
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, nil)
	s.registry.Add("hero-1", weapons)

	// Retrieve via interface method
	retrieved := s.registry.GetCharacter("hero-1")
	s.Require().NotNil(retrieved)

	// Type assert to CharacterWeapons
	characterWeapons, ok := retrieved.(*gamectx.CharacterWeapons)
	s.Require().True(ok, "Retrieved value should be *CharacterWeapons")
	s.Equal(longsword, characterWeapons.MainHand())
}

func (s *CharacterRegistryTestSuite) TestGetCharacterInterfaceNotFound() {
	// Test the CharacterRegistry interface method with nonexistent character
	retrieved := s.registry.GetCharacter("nonexistent")
	s.Nil(retrieved)
}

func (s *CharacterRegistryTestSuite) TestReplaceCharacterWeapons() {
	// Add initial weapons
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons1 := gamectx.NewCharacterWeapons(longsword, nil)
	s.registry.Add("hero-1", weapons1)

	// Replace with different weapons
	greataxe := &gamectx.EquippedWeapon{
		ID:          "greataxe-1",
		Name:        "Greataxe",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: true,
		IsMelee:     true,
	}
	weapons2 := gamectx.NewCharacterWeapons(greataxe, nil)
	s.registry.Add("hero-1", weapons2)

	// Verify replacement
	retrieved, ok := s.registry.Get("hero-1")
	s.Require().True(ok)
	s.Equal(greataxe, retrieved.MainHand())
}

func TestCharacterRegistrySuite(t *testing.T) {
	suite.Run(t, new(CharacterRegistryTestSuite))
}

// CharacterWeaponsTestSuite tests the CharacterWeapons weapon query methods
type CharacterWeaponsTestSuite struct {
	suite.Suite
}

func (s *CharacterWeaponsTestSuite) TestMainHandReturnsMainHandWeapon() {
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, nil)

	mainHand := weapons.MainHand()
	s.Require().NotNil(mainHand)
	s.Equal("longsword-1", mainHand.ID)
	s.Equal("Longsword", mainHand.Name)
	s.Equal("main_hand", mainHand.Slot)
	s.False(mainHand.IsTwoHanded)
	s.True(mainHand.IsMelee)
}

func (s *CharacterWeaponsTestSuite) TestMainHandReturnsNilWhenEmpty() {
	weapons := gamectx.NewCharacterWeapons(nil, nil)

	mainHand := weapons.MainHand()
	s.Nil(mainHand, "MainHand should return nil when no weapon equipped")
}

func (s *CharacterWeaponsTestSuite) TestOffHandReturnsOffHandWeapon() {
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	dagger := &gamectx.EquippedWeapon{
		ID:          "dagger-1",
		Name:        "Dagger",
		Slot:        "off_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, dagger)

	offHand := weapons.OffHand()
	s.Require().NotNil(offHand)
	s.Equal("dagger-1", offHand.ID)
	s.Equal("Dagger", offHand.Name)
	s.Equal("off_hand", offHand.Slot)
}

func (s *CharacterWeaponsTestSuite) TestOffHandReturnsNilForShield() {
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Name:     "Shield",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, shield)

	offHand := weapons.OffHand()
	s.Nil(offHand, "OffHand should return nil when shield is equipped")
}

func (s *CharacterWeaponsTestSuite) TestOffHandReturnsNilWhenEmpty() {
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, nil)

	offHand := weapons.OffHand()
	s.Nil(offHand, "OffHand should return nil when no off-hand item equipped")
}

func (s *CharacterWeaponsTestSuite) TestAllEquippedReturnsSingleWeapon() {
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, nil)

	allWeapons := weapons.AllEquipped()
	s.Require().Len(allWeapons, 1)
	s.Equal("longsword-1", allWeapons[0].ID)
}

func (s *CharacterWeaponsTestSuite) TestAllEquippedReturnsTwoWeapons() {
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	dagger := &gamectx.EquippedWeapon{
		ID:          "dagger-1",
		Name:        "Dagger",
		Slot:        "off_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, dagger)

	allWeapons := weapons.AllEquipped()
	s.Require().Len(allWeapons, 2)
	s.Equal("longsword-1", allWeapons[0].ID)
	s.Equal("dagger-1", allWeapons[1].ID)
}

func (s *CharacterWeaponsTestSuite) TestAllEquippedExcludesShield() {
	longsword := &gamectx.EquippedWeapon{
		ID:          "longsword-1",
		Name:        "Longsword",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: false,
		IsMelee:     true,
	}
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Name:     "Shield",
		Slot:     "off_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons(longsword, shield)

	allWeapons := weapons.AllEquipped()
	s.Require().Len(allWeapons, 1, "AllEquipped should exclude shields")
	s.Equal("longsword-1", allWeapons[0].ID)
}

func (s *CharacterWeaponsTestSuite) TestAllEquippedReturnsEmptyWhenNoWeapons() {
	weapons := gamectx.NewCharacterWeapons(nil, nil)

	allWeapons := weapons.AllEquipped()
	s.Require().NotNil(allWeapons, "AllEquipped should return empty slice, not nil")
	s.Len(allWeapons, 0)
}

func (s *CharacterWeaponsTestSuite) TestAllEquippedReturnsEmptyWithOnlyShield() {
	shield := &gamectx.EquippedWeapon{
		ID:       "shield-1",
		Name:     "Shield",
		Slot:     "main_hand",
		IsShield: true,
	}
	weapons := gamectx.NewCharacterWeapons(shield, nil)

	allWeapons := weapons.AllEquipped()
	s.Require().NotNil(allWeapons)
	s.Len(allWeapons, 0, "AllEquipped should return empty when only shield equipped")
}

func (s *CharacterWeaponsTestSuite) TestTwoHandedWeapon() {
	greataxe := &gamectx.EquippedWeapon{
		ID:          "greataxe-1",
		Name:        "Greataxe",
		Slot:        "main_hand",
		IsShield:    false,
		IsTwoHanded: true,
		IsMelee:     true,
	}
	weapons := gamectx.NewCharacterWeapons(greataxe, nil)

	mainHand := weapons.MainHand()
	s.Require().NotNil(mainHand)
	s.True(mainHand.IsTwoHanded)

	offHand := weapons.OffHand()
	s.Nil(offHand)

	allWeapons := weapons.AllEquipped()
	s.Require().Len(allWeapons, 1)
	s.Equal("greataxe-1", allWeapons[0].ID)
}

func TestCharacterWeaponsSuite(t *testing.T) {
	suite.Run(t, new(CharacterWeaponsTestSuite))
}
