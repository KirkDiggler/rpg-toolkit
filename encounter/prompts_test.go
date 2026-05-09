package encounter_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/stretchr/testify/suite"
)

const (
	abilityDEX  = "DEX"
	thievesTool = "dnd5e:item:thieves-tools"
)

// fakeResolver is a hand-written CharacterResolver stub. It returns a
// flat ability modifier and a flat tool bonus; ok=false signals "no
// proficiency" which SubmitCheck treats as a zero contribution.
type fakeResolver struct {
	abilityMod int
	abilityOK  bool
	toolBonus  int
	toolOK     bool
}

func (f *fakeResolver) AbilityModifier(_ core.PlayerID, _ string) (int, bool) {
	return f.abilityMod, f.abilityOK
}

func (f *fakeResolver) ToolProficiencyBonus(_ core.PlayerID, _ string) (int, bool) {
	return f.toolBonus, f.toolOK
}

type PromptsSuite struct {
	suite.Suite
	transport *encounter.InMemoryTransport
	broker    *encounter.Broker
	resolver  *fakeResolver
}

func TestPromptsSuite(t *testing.T) {
	suite.Run(t, new(PromptsSuite))
}

func (s *PromptsSuite) SetupTest() {
	s.transport = encounter.NewInMemoryTransport()
	s.broker = encounter.NewBroker(s.transport)
	s.resolver = &fakeResolver{
		abilityMod: 3, abilityOK: true,
		toolBonus: 2, toolOK: true,
	}
}

func (s *PromptsSuite) TearDownTest() {
	_ = s.broker.Close()
	_ = s.transport.Close()
}

// newEncounterWithLockedDoor returns an encounter with one player, one
// locked door at a position the player can perceive, and the suite's
// resolver wired up.
func (s *PromptsSuite) newEncounterWithLockedDoor() *encounter.Encounter {
	e := encounter.New("enc-1", s.broker, encounter.WithCharacterResolver(s.resolver))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 5,
	}))
	e.AddDoor("door-1", core.Hex{Q: 1, R: 0, S: -1}, false)
	door := e.ToData().Doors["door-1"]
	door.Locked = true
	door.LockDC = 15
	door.LockAbility = abilityDEX
	door.LockTool = thievesTool
	return e
}

// --- Round-trip tests for new persisted state -------------------------------

// DoorData with Wave 2.9 lock fields round-trips through JSON unchanged.
func (s *PromptsSuite) TestRoundTrip_DoorDataLockFields() {
	e1 := encounter.New("enc-1", s.broker)
	e1.AddDoor("door-1", core.Hex{Q: 1, R: 0, S: -1}, false)
	door := e1.ToData().Doors["door-1"]
	door.Locked = true
	door.LockDC = 17
	door.LockAbility = abilityDEX
	door.LockTool = thievesTool

	payload, err := json.Marshal(e1.ToData())
	s.Require().NoError(err)

	var data encounter.Data
	s.Require().NoError(json.Unmarshal(payload, &data))

	got := data.Doors["door-1"]
	s.Require().NotNil(got)
	s.True(got.Locked)
	s.Equal(17, got.LockDC)
	s.Equal("DEX", got.LockAbility)
	s.Equal("dnd5e:item:thieves-tools", got.LockTool)
}

// Legacy DoorData (no lock fields) round-trips as an unlocked door.
// This guards backwards compatibility with Wave 2.7 persisted state.
func (s *PromptsSuite) TestRoundTrip_DoorDataLegacyUnlocked() {
	legacy := []byte(`{"id":"door-legacy","position":{"q":0,"r":0,"s":0},"open":false}`)
	var got encounter.DoorData
	s.Require().NoError(json.Unmarshal(legacy, &got))
	s.False(got.Locked)
	s.Equal(0, got.LockDC)
	s.Empty(got.LockAbility)
	s.Empty(got.LockTool)
}

// PendingPrompts round-trips through JSON with the prompt intact.
func (s *PromptsSuite) TestRoundTrip_PendingPrompts() {
	e1 := s.newEncounterWithLockedDoor()
	_, err := e1.AttemptUnlock("alice", "door-1")
	s.Require().NoError(err)

	payload, err := json.Marshal(e1.ToData())
	s.Require().NoError(err)

	var data encounter.Data
	s.Require().NoError(json.Unmarshal(payload, &data))

	prompt, ok := data.PendingPrompts["alice"]
	s.Require().True(ok, "PendingPrompts must round-trip")
	s.Require().NotNil(prompt)
	s.Equal(encounter.PromptKindSkillCheck, prompt.Kind)
	s.Equal(15, prompt.DC)
	s.Equal(abilityDEX, prompt.Ability)
	s.Equal(thievesTool, prompt.Tool)
	s.Equal(core.EntityID("door-1"), prompt.TriggeredBy)
	s.Equal("open", prompt.TriggeredAction)
}

// Empty PendingPrompts is omitted from the wire (omitempty) so legacy
// snapshots stay byte-identical.
func (s *PromptsSuite) TestRoundTrip_PendingPromptsOmitWhenEmpty() {
	e := encounter.New("enc-1", s.broker)
	payload, err := json.Marshal(e.ToData())
	s.Require().NoError(err)
	s.NotContains(string(payload), "pending_prompts")
}

// --- AttemptUnlock --------------------------------------------------------

func (s *PromptsSuite) TestAttemptUnlock_HappyPath() {
	e := s.newEncounterWithLockedDoor()

	issued, err := e.AttemptUnlock("alice", "door-1")
	s.Require().NoError(err)
	s.Equal(encounter.PromptKindSkillCheck, issued.Kind)
	s.Equal(15, issued.DC)
	s.Equal(abilityDEX, issued.Ability)
	s.Equal(thievesTool, issued.Tool)
	s.Equal(core.EntityID("door-1"), issued.TriggeredBy)
	s.Equal("open", issued.TriggeredAction)

	// Prompt is now pending on persisted state.
	s.Require().Contains(e.ToData().PendingPrompts, core.PlayerID("alice"))
}

func (s *PromptsSuite) TestAttemptUnlock_SecondCallRejected() {
	e := s.newEncounterWithLockedDoor()

	_, err := e.AttemptUnlock("alice", "door-1")
	s.Require().NoError(err)

	_, err = e.AttemptUnlock("alice", "door-1")
	s.Require().Error(err)
	s.Require().True(errors.Is(err, encounter.ErrPromptAlreadyPending))
}

func (s *PromptsSuite) TestAttemptUnlock_UnlockedDoor() {
	e := encounter.New("enc-1", s.broker, encounter.WithCharacterResolver(s.resolver))
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 5,
	}))
	e.AddDoor("door-unlocked", core.Hex{Q: 1, R: 0, S: -1}, false)

	_, err := e.AttemptUnlock("alice", "door-unlocked")
	s.Require().Error(err)
	s.Require().True(errors.Is(err, encounter.ErrDoorNotLocked))
}

func (s *PromptsSuite) TestAttemptUnlock_UnknownPlayer() {
	e := s.newEncounterWithLockedDoor()
	_, err := e.AttemptUnlock("nobody", "door-1")
	s.Require().Error(err)
}

func (s *PromptsSuite) TestAttemptUnlock_UnknownDoor() {
	e := s.newEncounterWithLockedDoor()
	_, err := e.AttemptUnlock("alice", "ghost-door")
	s.Require().Error(err)
}

// --- SubmitCheck ----------------------------------------------------------

// setupAttemptUnlocked is the common arrange step for the two
// SubmitCheck-after-AttemptUnlock paths (success / failure). Returns the
// encounter and alice's broker subscription.
func (s *PromptsSuite) setupAttemptUnlocked() (*encounter.Encounter, *encounter.Subscription) {
	e := s.newEncounterWithLockedDoor()
	sub, err := s.broker.Subscribe("enc-1", "alice")
	s.Require().NoError(err)
	_, err = e.AttemptUnlock("alice", "door-1")
	s.Require().NoError(err)
	return e, sub
}

// Success path: roll + ability + tool >= DC. The wired action is "open",
// so OpenDoor fires and the broker delivers DoorOpenedEvent (and
// HexRevealedEvent if any viewer's vision grew). Door is left unlocked
// and open; pending prompt is cleared.
func (s *PromptsSuite) TestSubmitCheck_SuccessOpensDoorAndEmits() {
	e, sub := s.setupAttemptUnlocked()

	// roll(15) + abilityMod(3) + toolBonus(2) = 20 >= DC(15) → success.
	res, err := e.SubmitCheck("alice", 15)
	s.Require().NoError(err)
	s.True(res.Success)
	s.Equal(20, res.Total)

	door := e.ToData().Doors["door-1"]
	s.True(door.Open, "door must be open after success")
	s.False(door.Locked, "door must be unlocked after success")
	s.NotContains(e.ToData().PendingPrompts, core.PlayerID("alice"))

	// Broker emitted DoorOpenedEvent for alice (she's the actor and
	// has LoS). HexRevealedEvent is optional — only emitted if her
	// vision grew.
	seen := collectTypes(sub, 500*time.Millisecond)
	s.Contains(seen, "*events.DoorOpenedEvent")
}

// Failure path: total < DC. No events emitted; door stays locked &
// closed; prompt is cleared.
func (s *PromptsSuite) TestSubmitCheck_FailureClearsPromptNoEvents() {
	e, sub := s.setupAttemptUnlocked()

	// roll(5) + abilityMod(3) + toolBonus(2) = 10 < DC(15) → failure.
	res, err := e.SubmitCheck("alice", 5)
	s.Require().NoError(err)
	s.False(res.Success)
	s.Equal(10, res.Total)

	door := e.ToData().Doors["door-1"]
	s.False(door.Open, "door must remain closed after failure")
	s.True(door.Locked, "door must remain locked after failure")
	s.NotContains(e.ToData().PendingPrompts, core.PlayerID("alice"))

	seen := collectTypes(sub, 100*time.Millisecond)
	s.Empty(seen, "failure path must not emit events")
}

// SubmitCheck without an outstanding prompt returns ErrNoPendingPrompt.
func (s *PromptsSuite) TestSubmitCheck_NoPendingPrompt() {
	e := s.newEncounterWithLockedDoor()
	_, err := e.SubmitCheck("alice", 15)
	s.Require().Error(err)
	s.Require().True(errors.Is(err, encounter.ErrNoPendingPrompt))
}

// SubmitCheck with a roll outside [1,20] returns ErrInvalidRoll. The
// pending prompt is preserved so the player can retry with a valid roll.
func (s *PromptsSuite) TestSubmitCheck_InvalidRoll() {
	e := s.newEncounterWithLockedDoor()
	_, err := e.AttemptUnlock("alice", "door-1")
	s.Require().NoError(err)

	for _, badRoll := range []int{0, -1, 21, 100} {
		_, err := e.SubmitCheck("alice", badRoll)
		s.Require().Errorf(err, "roll %d should be invalid", badRoll)
		s.Require().Truef(errors.Is(err, encounter.ErrInvalidRoll), "roll %d: want ErrInvalidRoll", badRoll)
	}

	// Prompt preserved across invalid attempts.
	s.Require().Contains(e.ToData().PendingPrompts, core.PlayerID("alice"))
}

// SubmitCheck without a CharacterResolver returns ErrNoCharacterResolver
// and PRESERVES the prompt — this is an input-validation error, not a
// resolution outcome, so the orchestrator can wire the resolver and
// retry without losing the player's pending state.
func (s *PromptsSuite) TestSubmitCheck_NoResolverConfigured() {
	e := s.newEncounterWithResolver(nil) // no resolver wired
	_, err := e.AttemptUnlock("alice", "door-1")
	s.Require().NoError(err)

	_, err = e.SubmitCheck("alice", 15)
	s.Require().Error(err)
	s.Require().True(errors.Is(err, encounter.ErrNoCharacterResolver))
	s.Require().Contains(e.ToData().PendingPrompts, core.PlayerID("alice"))
}

// SubmitCheck on a prompt whose Kind is not PromptKindSkillCheck
// returns ErrPromptKindMismatch and PRESERVES the prompt (the right
// verb resolves it later). Guards against silently resolving a
// dialogue / target-select prompt as a skill check.
func (s *PromptsSuite) TestSubmitCheck_PromptKindMismatch() {
	e := s.newEncounterWithLockedDoor()
	// Hand-set a non-SkillCheck prompt — same path future waves take
	// when they queue dialogue / target-select prompts.
	e.ToData().PendingPrompts["alice"] = &encounter.PendingPrompt{
		Kind:            encounter.PromptKindDialogue,
		TriggeredBy:     "npc-1",
		TriggeredAction: "respond",
	}

	_, err := e.SubmitCheck("alice", 15)
	s.Require().Error(err)
	s.Require().True(errors.Is(err, encounter.ErrPromptKindMismatch))
	s.Require().Contains(e.ToData().PendingPrompts, core.PlayerID("alice"))
}

// SubmitCheck on a prompt whose TriggeredAction is something other than
// "open" returns ErrUnsupportedPromptAction. The check itself resolves
// (success path), so the prompt is CLEARED to avoid stranded state.
// Hand-set via the Data accessor since AttemptUnlock currently only
// issues "open" prompts.
func (s *PromptsSuite) TestSubmitCheck_UnsupportedAction() {
	e := s.newEncounterWithLockedDoor()
	// Hand-set a prompt with an unsupported action via the persisted
	// shape — same path the orchestrator would take if a future wave
	// queued a non-"open" prompt before this wave updated the dispatch.
	e.ToData().PendingPrompts["alice"] = &encounter.PendingPrompt{
		Kind:            encounter.PromptKindSkillCheck,
		DC:              5,
		Ability:         abilityDEX,
		TriggeredBy:     "door-1",
		TriggeredAction: "speak-friend-and-enter", // not wired
	}

	// roll(20) easily clears DC=5; success path runs and dispatch fails.
	res, err := e.SubmitCheck("alice", 20)
	s.Require().Error(err)
	s.Require().True(errors.Is(err, encounter.ErrUnsupportedPromptAction))
	s.True(res.Success, "check should still report success")

	// Prompt must still be cleared.
	s.NotContains(e.ToData().PendingPrompts, core.PlayerID("alice"))
}

// SubmitCheck treats unknown ability / tool lookups as zero contributions
// rather than erroring — a player attempting a check without proficiency
// is normal play.
func (s *PromptsSuite) TestSubmitCheck_UnknownAbilityTreatedAsZero() {
	resolver := &fakeResolver{abilityMod: 0, abilityOK: false, toolBonus: 0, toolOK: false}
	e := s.newEncounterWithResolver(resolver)

	_, err := e.AttemptUnlock("alice", "door-1")
	s.Require().NoError(err)

	res, err := e.SubmitCheck("alice", 12)
	s.Require().NoError(err)
	s.Equal(12, res.Total)
	s.True(res.Success)
}

// newEncounterWithResolver mirrors newEncounterWithLockedDoor but with a
// caller-supplied resolver so tests can exercise the "no resolver" and
// "stub resolver" paths against the same fixture.
func (s *PromptsSuite) newEncounterWithResolver(r encounter.CharacterResolver) *encounter.Encounter {
	var opts []encounter.Option
	if r != nil {
		opts = append(opts, encounter.WithCharacterResolver(r))
	}
	e := encounter.New("enc-1", s.broker, opts...)
	s.Require().NoError(e.AddPlayer(encounter.PlayerInput{
		PlayerID: "alice", EntityID: aliceEntityID,
		Position: core.Hex{}, SightRange: 5,
	}))
	e.AddDoor("door-1", core.Hex{Q: 1, R: 0, S: -1}, false)
	d := e.ToData().Doors["door-1"]
	d.Locked = true
	d.LockDC = 10
	d.LockAbility = abilityDEX
	return e
}
