// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package npc_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/npc"
)

type NPCSuite struct {
	suite.Suite

	ref *core.Ref
}

func TestNPCSuite(t *testing.T) {
	suite.Run(t, new(NPCSuite))
}

func (s *NPCSuite) SetupTest() {
	ref, err := core.NewRef(core.RefInput{Module: "dnd5e", Type: "npcs", ID: "merchant"})
	s.Require().NoError(err)
	s.ref = ref
}

func (s *NPCSuite) data() *npc.Data {
	return &npc.Data{
		Ref:               s.ref,
		DisplayName:       "Merchant",
		Capabilities:      []npc.Capability{npc.CapabilityVendor},
		CombatPolicy:      npc.CombatPolicyNonCombatant,
		ObservationPolicy: npc.ObservationPolicySubjectOnly,
		DispositionPolicy: npc.DispositionPolicyNeutral,
		MovementPolicy:    npc.MovementPolicyBlocking,
	}
}

func (s *NPCSuite) TestNewDefaultsFirstVendorShape() {
	n, err := npc.New(npc.Config{
		Ref:          s.ref,
		DisplayName:  "Merchant",
		Capabilities: []npc.Capability{npc.CapabilityVendor},
	})
	s.Require().NoError(err)

	s.True(n.Ref().Equals(s.ref))
	s.Equal("Merchant", n.DisplayName())
	s.Equal([]npc.Capability{npc.CapabilityVendor}, n.Capabilities())
	s.Equal(npc.CombatPolicyNonCombatant, n.CombatPolicy())
	s.Equal(npc.ObservationPolicySubjectOnly, n.ObservationPolicy())
	s.Equal(npc.DispositionPolicyNeutral, n.DispositionPolicy())
	s.Equal(npc.MovementPolicyBlocking, n.MovementPolicy())
}

func (s *NPCSuite) TestLoadRequiresPersistedPolicyValues() {
	cases := []struct {
		name string
		edit func(*npc.Data)
		want error
	}{
		{name: "combat policy", edit: func(d *npc.Data) { d.CombatPolicy = "" }, want: npc.ErrNoCombatPolicy},
		{name: "observation policy", edit: func(d *npc.Data) { d.ObservationPolicy = "" }, want: npc.ErrNoObservationPolicy},
		{name: "disposition policy", edit: func(d *npc.Data) { d.DispositionPolicy = "" }, want: npc.ErrNoDispositionPolicy},
		{name: "movement policy", edit: func(d *npc.Data) { d.MovementPolicy = "" }, want: npc.ErrNoMovementPolicy},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			data := s.data()
			tc.edit(data)

			_, err := npc.Load(data)

			s.ErrorIs(err, tc.want)
		})
	}
}

func (s *NPCSuite) TestValidationRejectsMissingAndUnknownValues() {
	cases := []struct {
		name string
		edit func(*npc.Data)
		want error
	}{
		{name: "nil data", edit: nil, want: npc.ErrNoData},
		{name: "nil ref", edit: func(d *npc.Data) { d.Ref = nil }, want: npc.ErrNoRef},
		{
			name: "invalid ref",
			edit: func(d *npc.Data) { d.Ref = &core.Ref{Module: "dnd5e", Type: "npcs"} },
			want: npc.ErrInvalidRef,
		},
		{name: "empty display name", edit: func(d *npc.Data) { d.DisplayName = "  " }, want: npc.ErrNoDisplayName},
		{
			name: "empty capability",
			edit: func(d *npc.Data) { d.Capabilities = []npc.Capability{""} },
			want: npc.ErrEmptyCapability,
		},
		{
			name: "unknown combat policy",
			edit: func(d *npc.Data) { d.CombatPolicy = "violent" },
			want: npc.ErrUnknownCombatPolicy,
		},
		{
			name: "unknown observation policy",
			edit: func(d *npc.Data) { d.ObservationPolicy = "smells" },
			want: npc.ErrUnknownObservationPolicy,
		},
		{
			name: "unknown disposition policy",
			edit: func(d *npc.Data) { d.DispositionPolicy = "friendly" },
			want: npc.ErrUnknownDispositionPolicy,
		},
		{
			name: "unknown movement policy",
			edit: func(d *npc.Data) { d.MovementPolicy = "allied_passable" },
			want: npc.ErrUnknownMovementPolicy,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			var data *npc.Data
			if tc.edit != nil {
				data = s.data()
				tc.edit(data)
			}

			_, err := npc.Load(data)

			s.Require().Error(err)
			s.True(errors.Is(err, tc.want), "expected %v, got %v", tc.want, err)
		})
	}
}

func (s *NPCSuite) TestUnknownCapabilitiesRoundTrip() {
	data := s.data()
	data.Capabilities = []npc.Capability{npc.CapabilityVendor, "blacksmith"}

	n, err := npc.Load(data)
	s.Require().NoError(err)

	s.Equal([]npc.Capability{npc.CapabilityVendor, "blacksmith"}, n.Capabilities())
	s.Equal([]npc.Capability{npc.CapabilityVendor, "blacksmith"}, n.ToData().Capabilities)
}

func (s *NPCSuite) TestPassableMovementPolicyRoundTrips() {
	data := s.data()
	data.MovementPolicy = npc.MovementPolicyPassable

	n, err := npc.Load(data)
	s.Require().NoError(err)

	s.Equal(npc.MovementPolicyPassable, n.MovementPolicy())
	s.Equal(npc.MovementPolicyPassable, n.ToData().MovementPolicy)
}

func (s *NPCSuite) TestMovementPolicyMapsToCurrentSpatialBlockingSeam() {
	cases := []struct {
		name   string
		policy npc.MovementPolicy
		want   bool
	}{
		{name: "blocking", policy: npc.MovementPolicyBlocking, want: true},
		{name: "passable", policy: npc.MovementPolicyPassable, want: false},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := tc.policy.BlocksMovement()

			s.Require().NoError(err)
			s.Equal(tc.want, got)
		})
	}
}

func (s *NPCSuite) TestMovementPolicyRefusesUnknownBinaryMapping() {
	_, err := npc.MovementPolicy("allied_passable").BlocksMovement()
	s.ErrorIs(err, npc.ErrUnknownMovementPolicy)

	_, err = npc.MovementPolicy("").BlocksMovement()
	s.ErrorIs(err, npc.ErrNoMovementPolicy)
}

func (s *NPCSuite) TestCapabilitiesAreCopiedAtBoundaries() {
	data := s.data()
	data.Capabilities = []npc.Capability{npc.CapabilityVendor}

	n, err := npc.Load(data)
	s.Require().NoError(err)

	data.Capabilities[0] = "mutated-input"
	s.Equal([]npc.Capability{npc.CapabilityVendor}, n.Capabilities())

	fromAccessor := n.Capabilities()
	fromAccessor[0] = "mutated-accessor"
	s.Equal([]npc.Capability{npc.CapabilityVendor}, n.Capabilities())

	fromData := n.ToData()
	fromData.Capabilities[0] = "mutated-output"
	s.Equal([]npc.Capability{npc.CapabilityVendor}, n.Capabilities())
}

func (s *NPCSuite) TestRefsAreCopiedAtBoundaries() {
	n, err := npc.Load(s.data())
	s.Require().NoError(err)

	s.ref.ID = "mutated-source"
	s.Equal(core.ID("merchant"), n.Ref().ID)

	ref := n.Ref()
	ref.ID = "mutated-accessor"
	s.Equal(core.ID("merchant"), n.Ref().ID)

	fromData := n.ToData()
	fromData.Ref.ID = "mutated-output"
	s.Equal(core.ID("merchant"), n.Ref().ID)
}

// TestInventoryIsOpaqueAndUnvalidated pins the whole point of the field
// (rpg-toolkit#1444): this package carries it and never reads it. Malformed,
// non-JSON bytes are exactly as legal as well-formed ones — the same
// treatment Capability strings get, not the treatment a policy enum gets.
func (s *NPCSuite) TestInventoryIsOpaqueAndUnvalidated() {
	data := s.data()
	data.Inventory = json.RawMessage(`not valid json at all`)

	n, err := npc.Load(data)

	s.Require().NoError(err, "an opaque field is never validated, malformed or not")
	s.Equal(json.RawMessage(`not valid json at all`), n.Inventory())
}

// TestNewCarriesInventoryFromConfig proves the field travels through New,
// not only through Load.
func (s *NPCSuite) TestNewCarriesInventoryFromConfig() {
	n, err := npc.New(npc.Config{
		Ref:          s.ref,
		DisplayName:  "Merchant",
		Capabilities: []npc.Capability{npc.CapabilityVendor},
		Inventory:    json.RawMessage(`{"stock":["longsword"]}`),
	})
	s.Require().NoError(err)

	s.Equal(json.RawMessage(`{"stock":["longsword"]}`), n.Inventory())
	s.Equal(json.RawMessage(`{"stock":["longsword"]}`), n.ToData().Inventory)
}

// TestInventoryRoundTripsByteForByte is the load-bearing round-trip: Load
// then ToData must reproduce the exact bytes, not merely "non-nil".
func (s *NPCSuite) TestInventoryRoundTripsByteForByte() {
	data := s.data()
	data.Inventory = json.RawMessage(`{"entries":[{"id":"longbow"},{"id":"arrows-20"}]}`)

	n, err := npc.Load(data)
	s.Require().NoError(err)

	s.Equal(data.Inventory, n.Inventory())
	s.Equal(data.Inventory, n.ToData().Inventory)
}

// TestNoInventoryRoundTripsAsNilAndOmitsFromJSON proves the omitempty
// contract directly — the key must be absent from marshaled JSON, not
// merely present-and-null.
func (s *NPCSuite) TestNoInventoryRoundTripsAsNilAndOmitsFromJSON() {
	n, err := npc.Load(s.data())
	s.Require().NoError(err)

	s.Nil(n.Inventory())

	marshaled, err := json.Marshal(n.ToData())
	s.Require().NoError(err)

	var raw map[string]interface{}
	s.Require().NoError(json.Unmarshal(marshaled, &raw))
	s.NotContains(raw, "inventory", "omitempty must drop a nil Inventory from the JSON entirely")
}

// TestInventoryIsCopiedAtBoundaries is Capabilities' own mutation-proof
// standard, applied to the new field.
func (s *NPCSuite) TestInventoryIsCopiedAtBoundaries() {
	data := s.data()
	data.Inventory = json.RawMessage(`{"stock":["longsword"]}`)

	n, err := npc.Load(data)
	s.Require().NoError(err)

	data.Inventory[0] = 'X'
	s.Equal(json.RawMessage(`{"stock":["longsword"]}`), n.Inventory())

	fromAccessor := n.Inventory()
	fromAccessor[0] = 'X'
	s.Equal(json.RawMessage(`{"stock":["longsword"]}`), n.Inventory())

	fromData := n.ToData()
	fromData.Inventory[0] = 'X'
	s.Equal(json.RawMessage(`{"stock":["longsword"]}`), n.Inventory())
}
