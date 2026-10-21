// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// act_bodies.go is THE FAMILY BODIES (R16, R17, R26, Laws 6, 7, 10): what each
// act says, field by field. Every field's comment names the beat payload key
// it is read from; act_family.go is where they are read.
//
// THE OUTCOME LIVES INSIDE THE ACT (Law 6). A swing says hit, miss or warded
// in its own body; a line carries its answer; a cast carries each target's
// result. A change of standing the act caused — a fall, a broken
// concentration — is its own state entry under the same act id, because it
// happened to somebody else.

// WalkStep is one cell of a walk.
type WalkStep struct {
	// Cell is where the step landed (moved `position`).
	Cell spatial.Position
	// Doors are the doors the step went through (moved `doors`).
	Doors []string
}

// WalkAct is a walk: consecutive moved beats of one mover under one act.
type WalkAct struct {
	// Mover is who walked (moved `member`).
	Mover Party
	// Path is the walk's cells in order. A walk's later segment starts at
	// the cell its earlier segment left the mover on.
	Path []WalkStep
	// Cause is what moved them, when something did (moved `cause`).
	Cause string
}

// AttackOutcome is how a swing ended.
type AttackOutcome string

const (
	// AttackHit is a struck beat.
	AttackHit AttackOutcome = "hit"
	// AttackMiss is a missed beat.
	AttackMiss AttackOutcome = "miss"
	// AttackWarded is a warded beat: a ward stopped the swing before a roll.
	AttackWarded AttackOutcome = "warded"
)

// WardSave is the save a ward forced on an attacker.
type WardSave struct {
	// Source is the warding creature (warded `warded.source`).
	Source Party
	// Ability is the save's ability (warded `warded.save.ability`).
	Ability string
	// Roll is the save (warded `warded.save.roll`, `total`, `dc`,
	// `succeeded`, `calculation`).
	Roll Roll
}

// AttackAct is one swing, with its outcome (Law 6).
type AttackAct struct {
	// By is the attacker (outcome `actor`).
	By Party
	// To is the target (outcome `targets[0]`).
	To Party
	// Attack is what swung (outcome `attack`).
	Attack AttackRef
	// Outcome is hit, miss or warded, from the beat's name.
	Outcome AttackOutcome
	// Critical is a critical hit (struck `critical`).
	Critical bool
	// Damage is the damage dealt (struck `amount`).
	Damage int
	// DamageComponents are the damage's parts (struck `damage_components`).
	DamageComponents []DamageComponent
	// Roll is the attack roll (outcome `roll`, `total`, `against`,
	// `calculation`). Nil on a warded swing, which rolled nothing.
	Roll *Roll
	// Ward is the save the ward forced (warded `warded`). Nil otherwise.
	Ward *WardSave
	// Sequence is the multiattack the swing was inside (outcome `sequence`).
	Sequence *SequenceRef
	// Reaction is the reaction the swing was, for a reaction entry (outcome
	// `reaction`). Nil on an ordinary swing.
	Reaction *ReactionRef
	// PresentationID is the swing's shared-dice token (outcome
	// `presentation_id`).
	PresentationID string
}

// CastOutcome is how a cast ended for one target.
type CastOutcome string

const (
	// CastMissed is a cast_missed beat.
	CastMissed CastOutcome = "miss"
	// CastWarded is a cast_warded beat.
	CastWarded CastOutcome = "warded"
	// CastSaved is a saved beat whose save succeeded.
	CastSaved CastOutcome = "saved"
	// CastFailed is a saved beat whose save failed.
	CastFailed CastOutcome = "failed"
)

// CastResult is one target's result of a cast.
type CastResult struct {
	// Target is who the result is about (cast_missed and cast_warded
	// `target`, saved `saver`).
	Target Party
	// Outcome is the result.
	Outcome CastOutcome
	// Ability is the save's or ward's ability (`ability`).
	Ability string
	// Save is the save or ward roll (`roll`, `total`, `dc`, `succeeded`,
	// `calculation`). Nil on a miss.
	Save *Roll
	// Source is the warding creature on a warded result (cast_warded
	// `source`).
	Source Party
}

// CastAct is a cast: the cast beat and the result beats that follow it.
type CastAct struct {
	// By is the caster (cast `actor`).
	By Party
	// Spell is what was cast (cast `spell`).
	Spell SpellRef
	// Targets are the cast's targets (cast `target` or `targets`).
	Targets []Party
	// Results are each target's result, in record order.
	Results []CastResult
}

// SaveResult is a save an activation forced (saved).
type SaveResult struct {
	// Saver is who saved (saved `saver`).
	Saver Party
	// Ability is the save's ability (saved `ability`).
	Ability string
	// Source is what forced it (saved `source`).
	Source SpellRef
	// Roll is the save (saved `roll`, `total`, `dc`, `succeeded`,
	// `calculation`).
	Roll Roll
}

// Stabilization is a stabilized result's detail (activation-result
// `result.stabilization`).
type Stabilization struct {
	Before, After                      string
	HitPoints                          int
	Successes, Failures                int
	SuccessesNeeded, FailuresRemaining int
	Stabilized, Dead                   bool
}

// ActivationResult is one result of an activation (activation-result
// `result`).
type ActivationResult struct {
	// Kind is the result's kind (`result.kind`).
	Kind string
	// Target is who it landed on (`result.target`).
	Target Party
	// Ref and Name are the result's source identity (`result.ref`,
	// `result.name`).
	Ref, Name string
	// SourceID is a condition's source entity (`result.source_id`).
	SourceID string
	// Amount, Requested, Before, After are a healing's or a damage's numbers
	// (`result.amount`, `requested`, `before`, `after`).
	Amount, Requested, Before, After int
	// Roll is a healing's or a damage's roll (`result.calculation`).
	Roll *Roll
	// DamageType is a damage's type (`result.damage_type`).
	DamageType string
	// Detail is a condition's own detail (`result.detail`).
	Detail string
	// Description is a granted capacity (`result.description`).
	Description string
	// Moved and StoppedBy are a push's distance and what stopped it
	// (`result.moved`, `result.stopped_by`).
	Moved     int
	StoppedBy string
	// Stabilization is a stabilized result's detail (`result.stabilization`).
	Stabilization *Stabilization
}

// ActivateAct is an activation: the activated beat and its results. A
// removed condition is not a result here — it is a state entry under the
// same act.
type ActivateAct struct {
	// By is who activated (activated `actor`).
	By Party
	// Ability is what was activated (activated `ability`).
	Ability AbilityRef
	// Target is the selected target (activated `target`), the zero Party
	// when none was selected.
	Target Party
	// Results are the activation's results in record order
	// (activation-result `result`).
	Results []ActivationResult
	// Saves are the saves the activation forced (saved).
	Saves []SaveResult
}

// SpeakAct is a line spoken, and its answer.
type SpeakAct struct {
	// By is the speaker (spoke `actor`).
	By Party
	// To is who or what was spoken to (spoke `target`, or `prop`).
	To Party
	// Option is the line's option id (spoke `option`).
	Option string
	// Line is the line's text (spoke `line`).
	Line string
	// Skill is the line's skill (spoke `skill`).
	Skill string
	// Roll is the line's check (spoke `total`, `dc`, `beaten`,
	// `calculation`).
	Roll *Roll
	// Answer is the target's answer (the answered beat under this act).
	Answer *TableAnswer
}

// DeathSaveAct is one death save (death_save `death_save`).
type DeathSaveAct struct {
	// By is who rolled (death_save `actor`).
	By Party
	// Roll is the save (`death_save.roll`, `calculation`).
	Roll *Roll
	// Outcome is the save's outcome (`death_save.outcome`).
	Outcome string
	// The tallies, as the beat states them (`death_save.successes_added`,
	// `failures_added`, `successes`, `failures`, `successes_needed`,
	// `failures_remaining`).
	SuccessesAdded, FailuresAdded      int
	Successes, Failures                int
	SuccessesNeeded, FailuresRemaining int
	// Stabilized, Dead, Recovered are where it left them
	// (`death_save.stabilized`, `dead`, `recovered`).
	Stabilized, Dead, Recovered bool
	// HPRestored is hit points a natural 20 restored
	// (`death_save.hp_restored`).
	HPRestored int
	// Continuation is what the save does to the turn
	// (`death_save.continuation`).
	Continuation string
	// PresentationID is the save's shared-dice token
	// (`death_save.presentation_id`).
	PresentationID string
}

// LootAct is a body looted (looted).
type LootAct struct {
	// By is the looter (looted `member`).
	By Party
	// From is the body (looted `target`).
	From Party
}

// HoldAct is a prop taken in hand (held).
type HoldAct struct {
	// By is the holder (held `holder`).
	By Party
	// Prop is what was taken (held `prop`).
	Prop Party
}

// DropAct is a prop put down (dropped).
type DropAct struct {
	// By is who dropped it (dropped `member`).
	By Party
	// Prop is what was dropped (dropped `prop`).
	Prop Party
	// Cell is where it landed (dropped `position`).
	Cell spatial.Position
}

// DoorAct is a door changing (door).
type DoorAct struct {
	// By is who changed it (door `actor`), the zero Party for an
	// unattributed change.
	By Party
	// Door is the door (door `door`).
	Door string
	// State is what it is now (door `state`).
	State string
	// Roll is an unlock attempt's check (door `total`, `dc`, `beaten`,
	// `calculation`). Nil for an open or a close.
	Roll *Roll
}

// EquipAct is an item drawn or stowed (equipment-changed).
type EquipAct struct {
	// By is whose equipment changed (equipment-changed `member`).
	By Party
	// Slot is the slot (`slot`).
	Slot string
	// Item is the item (`item`).
	Item string
	// Change is draw or stow (`change`).
	Change string
}

// StayAct is a driven word that did nothing (stayed).
type StayAct struct {
	// Member is who stayed (stayed `member`).
	Member Party
	// Cause is what drove the word (stayed `cause`).
	Cause string
	// Why is the reason nothing happened (stayed `why`).
	Why string
}

// SneakAct is a sneaker's Stealth roll against one watcher (snuck).
type SneakAct struct {
	// By is the sneaker (snuck `member`).
	By Party
	// Watcher is the watcher, when the sneaker sees it (snuck `watcher`).
	Watcher Party
	// Roll is the check (snuck `total`, `dc`, `beaten`, `calculation`).
	Roll *Roll
}

// RestAct is a rest (rested). The concentrations and conditions the rest
// ended are state entries under the same act.
type RestAct struct {
	// By is who rested (rested `member`).
	By Party
	// Kind is short or long (rested `kind`).
	Kind string
	// The numbers, as the beat states them (rested `hit_points_restored`,
	// `hit_points`, `hit_dice_spent`, `hit_dice_returned`,
	// `hit_dice_remaining`).
	HitPointsRestored, HitPoints                    int
	HitDiceSpent, HitDiceReturned, HitDiceRemaining int
	// ResourcesRefilled are the resources refilled
	// (rested `resources_refilled`).
	ResourcesRefilled []string
	// Roll is the hit dice spent (rested `calculation`).
	Roll *Roll
}

// TableCandidate is one row a table roll could have landed on (answered
// `candidates`).
type TableCandidate struct {
	Entry, Weight, Percent, Loaded int
}

// Temper is a faction's temperament deal for one creature (tempered).
type Temper struct {
	// Faction is the die's owner (tempered `faction`).
	Faction Party
	// Temper is the dealt word (tempered `temper`).
	Temper string
	// Roll is the deal (tempered `roll`, `of`).
	Roll Roll
}

// TableAnswer is a table roll's answer: a creature's or a prop's pick, or a
// temperament deal. It never carries the answer's fact (R10).
type TableAnswer struct {
	// Who answered (answered `creature` as a member or `prop`; tempered
	// `member`).
	Who Party
	// Option is the option answered (answered `option`).
	Option string
	// Key is the table key (answered `key`).
	Key string
	// Word is the outcome word (answered `word`).
	Word string
	// Selector is the outcome's selector word (answered `selector`).
	Selector string
	// Say is what was said (answered `say`).
	Say string
	// Temperament is the temperament column the pick read (answered
	// `temper`).
	Temperament string
	// Entry is the row picked (answered `entry`).
	Entry int
	// Candidates are the rows it could have been (answered `candidates`).
	Candidates []TableCandidate
	// Roll is the table die (answered `roll`, `of`). Nil on a hold with
	// nothing eligible, which rolled nothing.
	Roll *Roll
	// Dealt is a temperament deal (tempered). Nil on a pick.
	Dealt *Temper
}

// StateKind is what a state entry changed.
type StateKind string

const (
	// StateDowned is a fall (down).
	StateDowned StateKind = "downed"
	// StateStance is two factions' stance turning (stance).
	StateStance StateKind = "stance"
	// StateConcentrationEnded is a concentration broken or let go
	// (concentration_ended, and rested `concentration_ended`).
	StateConcentrationEnded StateKind = "concentration_ended"
	// StateConditionRemoved is a condition lifted (an activation-result of
	// kind condition-removed, rested and exited `ended`, ended `ended`).
	StateConditionRemoved StateKind = "condition_removed"
	// StateExperience is experience paid out (experience_gained).
	StateExperience StateKind = "experience"
)

// Downed is a fall's detail.
type Downed struct {
	// Kind is the fallen member's kind (down `kind`).
	Kind string
}

// StanceTurn is a stance's detail.
type StanceTurn struct {
	// Between are the two factions (stance `between`).
	Between []Party
	// Stance is the new stance (stance `stance`).
	Stance string
	// Cause is what turned it (stance `cause`).
	Cause string
}

// ConditionLifted is a removed condition's detail.
type ConditionLifted struct {
	// Ref and Name are the condition (`ref`, `name`).
	Ref, Name string
	// SourceID is its source entity (`source_id`).
	SourceID string
	// Reason is why it ended (`reason`).
	Reason string
}

// ConcentrationEnded is a concentration's end.
type ConcentrationEnded struct {
	// Spell is the spell concentrated on (`spell`).
	Spell SpellRef
	// Reason is why it ended (`reason`).
	Reason string
}

// StateChange is a change of standing, under the act that caused it.
type StateChange struct {
	// Party is who changed: the fallen (down `member`), the caster
	// (`caster`), the condition's holder, the grant's source
	// (`experience.member`). A stance names no party.
	Party Party
	// Change is what changed.
	Change StateKind
	// Downed is set for StateDowned.
	Downed *Downed
	// Stance is set for StateStance.
	Stance *StanceTurn
	// Concentration is set for StateConcentrationEnded.
	Concentration *ConcentrationEnded
	// Condition is set for StateConditionRemoved.
	Condition *ConditionLifted
	// Experience is set for StateExperience: each character's share
	// (experience_gained `experience.grants`).
	Experience []ExperienceGrant
}

// Sense is how a perception entry was perceived.
type Sense string

const (
	// SenseSight is sight changing (sighted).
	SenseSight Sense = "sight"
	// SenseHearing is a sound (heard).
	SenseHearing Sense = "hearing"
	// SenseDiscovery is a search's check (discovery_checked).
	SenseDiscovery Sense = "discovery"
	// SenseReveal is a secret revealed (concealment_revealed).
	SenseReveal Sense = "reveal"
	// SenseCells is layout learned (cells_revealed, legacy room_revealed).
	SenseCells Sense = "cells"
)

// SightChange is what came into or out of sight (sighted).
type SightChange struct {
	// Gained, Lost, Changed are the members (sighted `gained`, `lost`,
	// `changed`).
	Gained, Lost, Changed []string
	// KnowledgeChanged is the observer's knowledge changing
	// (sighted `knowledge_changed`).
	KnowledgeChanged bool
}

// Sound is a sound heard (heard).
type Sound struct {
	// Kind is the sound (heard `kind`).
	Kind string
	// Cell is where it came from (heard `position`).
	Cell spatial.Position
	// Hops is how far it carried (heard `hops`).
	Hops int
}

// DiscoveryCheck is a search's check (discovery_checked).
type DiscoveryCheck struct {
	// Ability is the check's ability (discovery_checked `ability`).
	Ability string
	// Roll is the check (`total`, `beaten`, `calculation`).
	Roll *Roll
}

// SecretRevealed is a concealment's contents, revealed to the observer
// (concealment_revealed).
type SecretRevealed struct {
	// Concealment is the secret (`concealment`).
	Concealment string
	// Cells, Props, Doors, Regions, Boundaries, Segments, Sealed are what it
	// held (`cells`, `props`, `doors`, `regions`, `boundaries`, `segments`,
	// `sealed`).
	Cells      []spatial.Position
	Props      []AtlasProp
	Doors      []RevealedDoor
	Regions    []AtlasRegion
	Boundaries []AtlasBoundary
	Segments   []AtlasSegment
	Sealed     []spatial.Position
	// Structural rows and prop presentations (`structural_walls`,
	// `structural_doors`, `structural_wall_openings_replacements`,
	// `prop_presentations`).
	StructuralWalls                    []AtlasStructuralWall
	StructuralDoors                    []AtlasStructuralDoor
	StructuralWallOpeningsReplacements []StructuralWallOpeningsReplacement
	PropPresentations                  []PropPresentation
}

// CellsLearned is fixed layout the observer newly learned (cells_revealed,
// legacy room_revealed read the same way).
type CellsLearned struct {
	// Regions are the regions' newly known cells (`regions`; a legacy
	// room_revealed's `region`).
	Regions []AtlasRegion
	// Scenery, Props, Boundaries, Segments, Sealed, Doorways, Placed, Exits
	// are the layout (`scenery`, `props`, `boundaries`, `segments`,
	// `sealed`, `doorways`, `placed`, `exits`).
	Scenery    []spatial.Position
	Props      []AtlasProp
	Boundaries []AtlasBoundary
	Segments   []AtlasSegment
	Sealed     []spatial.Position
	Doorways   []AtlasDoorway
	Placed     []AtlasPlacedProp
	Exits      []AtlasExit
	// Structural rows and prop presentations, as SecretRevealed's.
	StructuralWalls                    []AtlasStructuralWall
	StructuralDoors                    []AtlasStructuralDoor
	StructuralWallOpeningsReplacements []StructuralWallOpeningsReplacement
	PropPresentations                  []PropPresentation
}

// Perception is what one observer newly perceived. It is folded from the
// observer's own entries, so it never reaches anybody else.
type Perception struct {
	// Observer is the recipient.
	Observer Party
	// Sense is how it was perceived; exactly the matching part is set.
	Sense     Sense
	Sighted   *SightChange
	Heard     *Sound
	Discovery *DiscoveryCheck
	Revealed  *SecretRevealed
	Cells     *CellsLearned
}

// ClockMark is which piece of the table's bookkeeping a clock entry is.
type ClockMark string

const (
	// ClockTick is the world clock advancing (tick).
	ClockTick ClockMark = "tick"
	// ClockTurnEnded is a turn passing (turn-ended).
	ClockTurnEnded ClockMark = "turn_ended"
	// ClockFightFormed is a fight forming (bubble-formed).
	ClockFightFormed ClockMark = "fight_formed"
	// ClockFightDissolved is a fight ending (bubble-dissolved).
	ClockFightDissolved ClockMark = "fight_dissolved"
	// ClockJoined is a member joining (joined).
	ClockJoined ClockMark = "joined"
	// ClockExited is a member leaving (exited).
	ClockExited ClockMark = "exited"
	// ClockArrived is a reserved placement entering (arrived).
	ClockArrived ClockMark = "arrived"
	// ClockSceneOpened is the scene opening (scene-opened).
	ClockSceneOpened ClockMark = "scene_opened"
	// ClockEnded is the run ending (ended).
	ClockEnded ClockMark = "ended"
	// ClockWindowOpened is a step stopping to ask somebody (window_opened).
	ClockWindowOpened ClockMark = "window_opened"
	// ClockRollWindowOpened is a roll stopping to ask its roller
	// (roll_window_opened).
	ClockRollWindowOpened ClockMark = "roll_window_opened"
)

// TurnPassed is a turn ending (turn-ended).
type TurnPassed struct {
	// Member is whose turn ended (turn-ended `member`).
	Member Party
	// Next is whose turn it is now (turn-ended `next`).
	Next Party
}

// FightFormed is a fight forming (bubble-formed).
type FightFormed struct {
	// Order is the initiative order (bubble-formed `order`).
	Order []Party
	// Surprised are the surprised (bubble-formed `surprised`).
	Surprised []Party
}

// FightDissolved is a fight ending (bubble-dissolved).
type FightDissolved struct {
	// Members are who it held (bubble-dissolved `members`).
	Members []Party
	// Cause is why it ended (bubble-dissolved `cause`).
	Cause string
}

// Departure is a member leaving (exited). The conditions the departure
// lifted are state entries.
type Departure struct {
	// Member is who left (exited `member`).
	Member Party
	// Holding is what left with them (exited `holding`).
	Holding []string
	// Exit is the authored exit they took (exited `exit`).
	Exit string
}

// Arrival is a reserved placement entering (arrived).
type Arrival struct {
	// ID, Kind, Cell are what arrived and where (arrived `id`, `kind`,
	// `cell`).
	ID, Kind string
	Cell     spatial.Position
}

// WindowOffer is one recipient of a window and what it offers (window_opened
// `windows`).
type WindowOffer struct {
	// Audience is who is asked (`audience`).
	Audience Party
	// Reaction is what they may do (`reaction`).
	Reaction ReactionRef
}

// WindowOpened is a step stopping to ask (window_opened).
type WindowOpened struct {
	// Member is the mover (window_opened `member`).
	Member Party
	// From and To are the step asked about (window_opened `from`, `to`).
	From, To spatial.Position
	// Windows are who is asked (window_opened `windows`).
	Windows []WindowOffer
}

// RollWindowOpened is a roll stopping to ask its roller (roll_window_opened).
type RollWindowOpened struct {
	// PresentationID is the roll's shared-dice token (`presentation_id`).
	PresentationID string
	// Audience is the roller (`audience`).
	Audience Party
	// Offer is what they may spend (`offer`).
	Offer ReactionRef
	// Roll is the roll in question (`roll`, `total`, `calculation`).
	Roll *Roll
}

// ClockEntry is the table's bookkeeping. A clock entry inside an act carries
// that act's id and closes the act's open segment.
type ClockEntry struct {
	// Mark is which entry this is; exactly the matching part below is set
	// (scene_opened carries none).
	Mark ClockMark
	// Tick is the world clock's reading (tick `tick`).
	Tick uint64
	// Turn is set for ClockTurnEnded.
	Turn *TurnPassed
	// Fight is set for ClockFightFormed.
	Fight *FightFormed
	// Dissolved is set for ClockFightDissolved.
	Dissolved *FightDissolved
	// Member is who joined (joined `member`).
	Member Party
	// Departure is set for ClockExited.
	Departure *Departure
	// Arrival is set for ClockArrived.
	Arrival *Arrival
	// Ending is the ending reached (ended `ending`).
	Ending string
	// Window is set for ClockWindowOpened.
	Window *WindowOpened
	// RollWindow is set for ClockRollWindowOpened.
	RollWindow *RollWindowOpened
}

func (WalkAct) isActBody()      {}
func (AttackAct) isActBody()    {}
func (CastAct) isActBody()      {}
func (ActivateAct) isActBody()  {}
func (SpeakAct) isActBody()     {}
func (DeathSaveAct) isActBody() {}
func (LootAct) isActBody()      {}
func (HoldAct) isActBody()      {}
func (DropAct) isActBody()      {}
func (DoorAct) isActBody()      {}
func (EquipAct) isActBody()     {}
func (StayAct) isActBody()      {}
func (SneakAct) isActBody()     {}
func (RestAct) isActBody()      {}
func (TableAnswer) isActBody()  {}
func (StateChange) isActBody()  {}
func (Perception) isActBody()   {}
func (ClockEntry) isActBody()   {}
