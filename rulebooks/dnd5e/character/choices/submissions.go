package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Submission represents what the player actually chose
type Submission struct {
	// Metadata about the choice
	Category shared.ChoiceCategory `json:"category"`
	Source   shared.ChoiceSource   `json:"source"`
	ChoiceID ChoiceID              `json:"choice_id"`           // References the requirement ID
	OptionID string                `json:"option_id,omitempty"` // Which option was selected (for equipment bundles)

	// The actual selection (just IDs, not full objects)
	Values []shared.SelectionID `json:"values"`
}

// Submissions represents all player choices
type Submissions struct {
	Choices []Submission `json:"choices"`

	// Quick lookups
	byCategory map[shared.ChoiceCategory][]Submission
	bySource   map[shared.ChoiceSource][]Submission
}

// NewSubmissions creates a new Submissions instance
func NewSubmissions() *Submissions {
	return &Submissions{
		Choices:    []Submission{},
		byCategory: make(map[shared.ChoiceCategory][]Submission),
		bySource:   make(map[shared.ChoiceSource][]Submission),
	}
}

// Add adds a submission
func (s *Submissions) Add(submission Submission) {
	s.Choices = append(s.Choices, submission)

	// Update lookups
	s.byCategory[submission.Category] = append(s.byCategory[submission.Category], submission)
	s.bySource[submission.Source] = append(s.bySource[submission.Source], submission)
}

// GetByCategory returns all submissions for a category
func (s *Submissions) GetByCategory(category shared.ChoiceCategory) []Submission {
	return s.byCategory[category]
}

// GetBySource returns all submissions from a source
func (s *Submissions) GetBySource(source shared.ChoiceSource) []Submission {
	return s.bySource[source]
}

// GetValues returns the values for a specific choice
func (s *Submissions) GetValues(source shared.ChoiceSource, choiceID ChoiceID) []string {
	submissions := s.bySource[source]
	for _, sub := range submissions {
		if sub.ChoiceID == choiceID {
			return sub.Values
		}
	}
	return nil
}

// HasChoice checks if a choice has been submitted
func (s *Submissions) HasChoice(source shared.ChoiceSource, choiceID ChoiceID) bool {
	submissions := s.bySource[source]
	for _, sub := range submissions {
		if sub.ChoiceID == choiceID {
			return true
		}
	}
	return false
}
