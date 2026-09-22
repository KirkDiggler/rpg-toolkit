package character

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character/choices"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/languages"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// compileSubclassChoices admits only choices declared by this subclass. Host
// supplied source tags cannot promote a class skill to a subclass bonus.
func compileSubclassChoices(subclass classes.Subclass, submitted []choices.Submission) ([]choices.ChoiceData, error) {
	mods := choices.GetSubclassModifications(subclass)
	allowed := map[choices.ChoiceID]shared.ChoiceCategory{}
	if mods != nil {
		if mods.AdditionalSkills != nil {
			allowed[mods.AdditionalSkills.ID] = shared.ChoiceSkills
		}
		for _, req := range mods.AdditionalLanguages {
			allowed[req.ID] = shared.ChoiceLanguages
		}
	}
	seen := map[choices.ChoiceID]bool{}
	result := make([]choices.ChoiceData, 0, len(submitted))
	for _, sub := range submitted {
		category, ok := allowed[sub.ChoiceID]
		if !ok || category != sub.Category || seen[sub.ChoiceID] {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid subclass choice %s", sub.ChoiceID)
		}
		seen[sub.ChoiceID] = true
		record := choices.ChoiceData{Source: shared.SourceSubclass, Category: category, ChoiceID: sub.ChoiceID}
		values := append([]shared.SelectionID(nil), sub.Values...)
		switch category {
		case shared.ChoiceSkills:
			record.SkillSelection = values
		case shared.ChoiceLanguages:
			record.LanguageSelection = values
		}
		result = append(result, record)
	}
	return result, nil
}

// Domain language choices must add languages, not spend a pick on a language
// learned from ancestry/background. Validate against the final draft so order
// of selection cannot change the result.
func (d *Draft) validateSubclassLanguages(racial []languages.Language) error {
	known := map[languages.Language]bool{}
	for _, language := range racial {
		known[language] = true
	}
	for _, choice := range d.choices {
		if choice.Source != shared.SourceSubclass {
			for _, language := range choice.LanguageSelection {
				known[language] = true
			}
		}
	}
	for _, choice := range d.choices {
		if choice.Source == shared.SourceSubclass {
			for _, language := range choice.LanguageSelection {
				if known[language] {
					return rpgerr.Newf(rpgerr.CodeInvalidArgument, "subclass language %s is already known", language)
				}
				known[language] = true
			}
		}
	}
	return nil
}
