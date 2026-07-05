package events_test

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/stretchr/testify/suite"
)

const (
	testDSPlayerAlice = "p-alice"
	testDSPlayerCarol = "p-carol"
)

// DeathSaveEventsSuite covers the two rpg-toolkit#741 event types:
// EntityStabilizedEvent and DeathSaveRolledEvent.
type DeathSaveEventsSuite struct {
	suite.Suite
}

func TestDeathSaveEventsSuite(t *testing.T) {
	suite.Run(t, new(DeathSaveEventsSuite))
}

// EntityStabilizedEvent satisfies EncounterEvent.
func (s *DeathSaveEventsSuite) TestEntityStabilized_SatisfiesInterface() {
	var _ events.EncounterEvent = (*events.EntityStabilizedEvent)(nil)
}

// DeathSaveRolledEvent satisfies EncounterEvent.
func (s *DeathSaveEventsSuite) TestDeathSaveRolled_SatisfiesInterface() {
	var _ events.EncounterEvent = (*events.DeathSaveRolledEvent)(nil)
}

// JSON round-trip preserves all fields including unexported encID/seq.
func (s *DeathSaveEventsSuite) TestEntityStabilized_JSONRoundTrip() {
	original := events.NewEntityStabilizedEvent(
		"enc-1", 42, "char-bob",
		map[core.PlayerID]events.EntityStabilizedSlice{
			testDSPlayerAlice: {Visible: true},
		},
	)

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.EntityStabilizedEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(42), decoded.Sequence())
	s.Equal(core.EntityID("char-bob"), decoded.EntityID)
	s.True(decoded.PerPlayer[testDSPlayerAlice].Visible)
}

// Audience derives from PerPlayer keys.
func (s *DeathSaveEventsSuite) TestEntityStabilized_Audience_DerivedFromPerPlayer() {
	evt := events.NewEntityStabilizedEvent(
		"enc-1", 1, "char-bob",
		map[core.PlayerID]events.EntityStabilizedSlice{
			testDSPlayerAlice: {Visible: true},
			testDSPlayerCarol: {Visible: false},
		},
	)
	s.ElementsMatch(
		events.AudienceSet{testDSPlayerAlice, testDSPlayerCarol},
		evt.Audience(),
	)
}

// JSON round-trip preserves all fields including unexported encID/seq and
// the full roll-detail field set (roll, running counters, crit flags,
// nat-20-revival fields).
func (s *DeathSaveEventsSuite) TestDeathSaveRolled_JSONRoundTrip() {
	original := events.NewDeathSaveRolledEvent(&events.NewDeathSaveRolledEventInput{
		EncID:                 "enc-1",
		Seq:                   7,
		EntityID:              "char-bob",
		Roll:                  17,
		Successes:             2,
		Failures:              1,
		IsCriticalFail:        false,
		IsCriticalSuccess:     false,
		Stabilized:            false,
		Dead:                  false,
		RegainedConsciousness: false,
		HPRestored:            0,
		PerPlayer: map[core.PlayerID]events.DeathSaveRolledSlice{
			testDSPlayerAlice: {Visible: true},
		},
	})

	payload, err := json.Marshal(original)
	s.Require().NoError(err)

	var decoded events.DeathSaveRolledEvent
	s.Require().NoError(json.Unmarshal(payload, &decoded))

	s.Equal(core.EncounterID("enc-1"), decoded.EncounterID())
	s.Equal(uint64(7), decoded.Sequence())
	s.Equal(core.EntityID("char-bob"), decoded.EntityID)
	s.Equal(17, decoded.Roll)
	s.Equal(2, decoded.Successes)
	s.Equal(1, decoded.Failures)
	s.False(decoded.IsCriticalFail)
	s.False(decoded.IsCriticalSuccess)
	s.False(decoded.Stabilized)
	s.False(decoded.Dead)
	s.False(decoded.RegainedConsciousness)
	s.Equal(0, decoded.HPRestored)
	s.True(decoded.PerPlayer[testDSPlayerAlice].Visible)
}

// Roll==0 (the damage-while-unconscious auto-fail convention) and the
// nat-20-revival field set round-trip correctly too — a separate case since
// zero values are the easiest place for a hand-rolled wire struct to
// silently drop a field.
func (s *DeathSaveEventsSuite) TestDeathSaveRolled_JSONRoundTrip_DamageDrivenAndRevival() {
	damageDriven := events.NewDeathSaveRolledEvent(&events.NewDeathSaveRolledEventInput{
		EncID: "enc-1", Seq: 1, EntityID: "char-bob",
		Roll: 0, Successes: 0, Failures: 3, Dead: true,
		PerPlayer: map[core.PlayerID]events.DeathSaveRolledSlice{testDSPlayerAlice: {Visible: true}},
	})
	payload, err := json.Marshal(damageDriven)
	s.Require().NoError(err)
	var decodedDamage events.DeathSaveRolledEvent
	s.Require().NoError(json.Unmarshal(payload, &decodedDamage))
	s.Equal(0, decodedDamage.Roll)
	s.Equal(3, decodedDamage.Failures)
	s.True(decodedDamage.Dead)

	revival := events.NewDeathSaveRolledEvent(&events.NewDeathSaveRolledEventInput{
		EncID: "enc-1", Seq: 2, EntityID: "char-bob",
		Roll: 20, IsCriticalSuccess: true, RegainedConsciousness: true, HPRestored: 1,
		PerPlayer: map[core.PlayerID]events.DeathSaveRolledSlice{testDSPlayerAlice: {Visible: true}},
	})
	payload, err = json.Marshal(revival)
	s.Require().NoError(err)
	var decodedRevival events.DeathSaveRolledEvent
	s.Require().NoError(json.Unmarshal(payload, &decodedRevival))
	s.Equal(20, decodedRevival.Roll)
	s.True(decodedRevival.IsCriticalSuccess)
	s.True(decodedRevival.RegainedConsciousness)
	s.Equal(1, decodedRevival.HPRestored)
}

// Audience derives from PerPlayer keys.
func (s *DeathSaveEventsSuite) TestDeathSaveRolled_Audience_DerivedFromPerPlayer() {
	evt := events.NewDeathSaveRolledEvent(&events.NewDeathSaveRolledEventInput{
		EncID: "enc-1", Seq: 1, EntityID: "char-bob", Roll: 12, Successes: 1,
		PerPlayer: map[core.PlayerID]events.DeathSaveRolledSlice{
			testDSPlayerAlice: {Visible: true},
			testDSPlayerCarol: {Visible: false},
		},
	})
	s.ElementsMatch(
		events.AudienceSet{testDSPlayerAlice, testDSPlayerCarol},
		evt.Audience(),
	)
}
