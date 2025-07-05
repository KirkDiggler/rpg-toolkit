// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"fmt"
	"time"
)

// Duration represents how long something lasts in the game.
type Duration interface {
	// Type returns the type of duration.
	Type() DurationType

	// IsExpired checks if the duration has expired.
	IsExpired(currentTime time.Time, currentRound int) bool

	// Description returns a human-readable description.
	Description() string
}

// DurationType represents different ways to measure duration.
type DurationType int

const (
	// DurationTypePermanent never expires.
	DurationTypePermanent DurationType = iota

	// DurationTypeRounds expires after a number of combat rounds.
	DurationTypeRounds

	// DurationTypeMinutes expires after a number of minutes.
	DurationTypeMinutes

	// DurationTypeHours expires after a number of hours.
	DurationTypeHours

	// DurationTypeEncounter expires when combat ends.
	DurationTypeEncounter

	// DurationTypeConcentration expires when concentration is broken.
	DurationTypeConcentration

	// DurationTypeShortRest expires after a short rest.
	DurationTypeShortRest

	// DurationTypeLongRest expires after a long rest.
	DurationTypeLongRest

	// DurationTypeUntilDamaged expires when damaged.
	DurationTypeUntilDamaged

	// DurationTypeUntilSave expires when a save is made.
	DurationTypeUntilSave
)

// String returns the string representation of the duration type.
func (d DurationType) String() string {
	switch d {
	case DurationTypePermanent:
		return "permanent"
	case DurationTypeRounds:
		return "rounds"
	case DurationTypeMinutes:
		return "minutes"
	case DurationTypeHours:
		return "hours"
	case DurationTypeEncounter:
		return "encounter"
	case DurationTypeConcentration:
		return "concentration"
	case DurationTypeShortRest:
		return "short_rest"
	case DurationTypeLongRest:
		return "long_rest"
	case DurationTypeUntilDamaged:
		return "until_damaged"
	case DurationTypeUntilSave:
		return "until_save"
	default:
		return "unknown"
	}
}

// PermanentDuration never expires.
type PermanentDuration struct{}

// Type returns the duration type.
func (d *PermanentDuration) Type() DurationType {
	return DurationTypePermanent
}

// IsExpired always returns false for permanent durations.
func (d *PermanentDuration) IsExpired(_ time.Time, _ int) bool {
	return false
}

// Description returns a human-readable description.
func (d *PermanentDuration) Description() string {
	return "Permanent"
}

// RoundsDuration expires after a specific number of combat rounds.
type RoundsDuration struct {
	Rounds       int
	StartRound   int
	IncludeStart bool // If true, the start round counts as round 1
}

// Type returns the duration type.
func (d *RoundsDuration) Type() DurationType {
	return DurationTypeRounds
}

// IsExpired checks if the rounds have elapsed.
func (d *RoundsDuration) IsExpired(_ time.Time, currentRound int) bool {
	elapsedRounds := currentRound - d.StartRound
	if d.IncludeStart {
		elapsedRounds++
	}
	return elapsedRounds > d.Rounds
}

// Description returns a human-readable description.
func (d *RoundsDuration) Description() string {
	if d.Rounds == 1 {
		return "1 round"
	}
	return fmt.Sprintf("%d rounds", d.Rounds)
}

// MinutesDuration expires after a specific number of minutes.
type MinutesDuration struct {
	Minutes   int
	StartTime time.Time
}

// Type returns the duration type.
func (d *MinutesDuration) Type() DurationType {
	return DurationTypeMinutes
}

// IsExpired checks if the minutes have elapsed.
func (d *MinutesDuration) IsExpired(currentTime time.Time, _ int) bool {
	elapsed := currentTime.Sub(d.StartTime)
	return elapsed.Minutes() >= float64(d.Minutes)
}

// Description returns a human-readable description.
func (d *MinutesDuration) Description() string {
	if d.Minutes == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", d.Minutes)
}

// HoursDuration expires after a specific number of hours.
type HoursDuration struct {
	Hours     int
	StartTime time.Time
}

// Type returns the duration type.
func (d *HoursDuration) Type() DurationType {
	return DurationTypeHours
}

// IsExpired checks if the hours have elapsed.
func (d *HoursDuration) IsExpired(currentTime time.Time, _ int) bool {
	elapsed := currentTime.Sub(d.StartTime)
	return elapsed.Hours() >= float64(d.Hours)
}

// Description returns a human-readable description.
func (d *HoursDuration) Description() string {
	if d.Hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", d.Hours)
}

// EncounterDuration expires when combat ends.
type EncounterDuration struct {
	EncounterActive bool
}

// Type returns the duration type.
func (d *EncounterDuration) Type() DurationType {
	return DurationTypeEncounter
}

// IsExpired checks if the encounter has ended.
func (d *EncounterDuration) IsExpired(_ time.Time, _ int) bool {
	return !d.EncounterActive
}

// Description returns a human-readable description.
func (d *EncounterDuration) Description() string {
	return "Until end of encounter"
}

// ConcentrationDuration expires when concentration is broken.
type ConcentrationDuration struct {
	ConcentrationBroken bool
}

// Type returns the duration type.
func (d *ConcentrationDuration) Type() DurationType {
	return DurationTypeConcentration
}

// IsExpired checks if concentration has been broken.
func (d *ConcentrationDuration) IsExpired(_ time.Time, _ int) bool {
	return d.ConcentrationBroken
}

// Description returns a human-readable description.
func (d *ConcentrationDuration) Description() string {
	return "Concentration"
}

// ShortRestDuration expires after a short rest.
type ShortRestDuration struct {
	ShortRestTaken bool
}

// Type returns the duration type.
func (d *ShortRestDuration) Type() DurationType {
	return DurationTypeShortRest
}

// IsExpired checks if a short rest has been taken.
func (d *ShortRestDuration) IsExpired(_ time.Time, _ int) bool {
	return d.ShortRestTaken
}

// Description returns a human-readable description.
func (d *ShortRestDuration) Description() string {
	return "Until short rest"
}

// LongRestDuration expires after a long rest.
type LongRestDuration struct {
	LongRestTaken bool
}

// Type returns the duration type.
func (d *LongRestDuration) Type() DurationType {
	return DurationTypeLongRest
}

// IsExpired checks if a long rest has been taken.
func (d *LongRestDuration) IsExpired(_ time.Time, _ int) bool {
	return d.LongRestTaken
}

// Description returns a human-readable description.
func (d *LongRestDuration) Description() string {
	return "Until long rest"
}

// UntilDamagedDuration expires when the entity takes damage.
type UntilDamagedDuration struct {
	DamageTaken bool
}

// Type returns the duration type.
func (d *UntilDamagedDuration) Type() DurationType {
	return DurationTypeUntilDamaged
}

// IsExpired checks if damage has been taken.
func (d *UntilDamagedDuration) IsExpired(_ time.Time, _ int) bool {
	return d.DamageTaken
}

// Description returns a human-readable description.
func (d *UntilDamagedDuration) Description() string {
	return "Until damaged"
}

// UntilSaveDuration expires when a specific save is made.
type UntilSaveDuration struct {
	Ability   string
	DC        int
	SaveMade  bool
	EndOfTurn bool // If true, save happens at end of turn
}

// Type returns the duration type.
func (d *UntilSaveDuration) Type() DurationType {
	return DurationTypeUntilSave
}

// IsExpired checks if the save has been made.
func (d *UntilSaveDuration) IsExpired(_ time.Time, _ int) bool {
	return d.SaveMade
}

// Description returns a human-readable description.
func (d *UntilSaveDuration) Description() string {
	when := "start"
	if d.EndOfTurn {
		when = "end"
	}
	return fmt.Sprintf("Until %s save (DC %d) at %s of turn", d.Ability, d.DC, when)
}
