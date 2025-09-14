package character

import (
	"math/bits"
)

// Progress tracks which character creation steps are complete
type Progress uint8

const (
	// ProgressNone indicates no steps completed
	ProgressNone Progress = 0

	// ProgressName indicates character name is set
	ProgressName Progress = 1 << 0 // 0x01 - Character name set
	// ProgressRace indicates race and race choices are complete
	ProgressRace Progress = 1 << 1 // 0x02 - Race and race choices complete
	// ProgressClass indicates class and class choices are complete
	ProgressClass Progress = 1 << 2 // 0x04 - Class and all class choices complete
	// ProgressBackground indicates background and background choices are complete
	ProgressBackground Progress = 1 << 3 // 0x08 - Background and background choices complete
	// ProgressAbilityScores indicates ability scores are assigned
	ProgressAbilityScores Progress = 1 << 4 // 0x10 - Ability scores assigned

	// ProgressComplete indicates all required steps are done
	ProgressComplete = ProgressName | ProgressRace | ProgressClass |
		ProgressBackground | ProgressAbilityScores // 0x1F
)

// Has checks if a specific step is complete
func (p Progress) Has(step Progress) bool {
	return p&step != 0
}

// Set marks a step as complete
func (p *Progress) Set(step Progress) {
	*p |= step
}

// Clear marks a step as incomplete
func (p *Progress) Clear(step Progress) {
	*p &^= step
}

// PercentComplete returns the completion percentage (0-100)
// Always out of 5 core steps
func (p Progress) PercentComplete() int {
	completed := bits.OnesCount8(uint8(p))
	return (completed * 100) / 5
}

// IsComplete returns true if all required steps are done
func (p Progress) IsComplete() bool {
	return p == ProgressComplete
}

// StepsComplete returns the number of completed steps
func (p Progress) StepsComplete() int {
	return bits.OnesCount8(uint8(p))
}

// StepsRemaining returns the number of steps left to complete
func (p Progress) StepsRemaining() int {
	return 5 - p.StepsComplete()
}

// String returns a human-readable progress summary
func (p Progress) String() string {
	switch p.StepsComplete() {
	case 0:
		return "Not started"
	case 5:
		return "Complete"
	default:
		return "In progress"
	}
}
