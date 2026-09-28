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
		for _, req := range mods.AdditionalCantrips {
			allowed[req.ID] = shared.ChoiceCantrips
		}
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
		case shared.ChoiceCantrips:
			record.SpellSelection = values
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

// validateSubclassCantrips requires a bonus pick to add a spell. It compares
// canonical spell IDs across all sources, so selection order cannot permit a
// duplicate cleric/druid spell such as Guidance or Resistance.
func (d *Draft) validateSubclassCantrips() error {
	known := map[string]bool{}
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceCantrips && choice.Source != shared.SourceSubclass {
			for _, id := range choice.SpellSelection {
				known[id] = true
			}
		}
	}
	for _, choice := range d.choices {
		if choice.Category == shared.ChoiceCantrips && choice.Source == shared.SourceSubclass {
			for _, id := range choice.SpellSelection {
				if known[id] {
					return rpgerr.Newf(rpgerr.CodeInvalidArgument, "subclass cantrip %s is already known", id)
				}
				known[id] = true
			}
		}
	}
	return nil
}
