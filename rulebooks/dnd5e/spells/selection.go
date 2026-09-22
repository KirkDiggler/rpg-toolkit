package spells

// Selectable keeps executable spells and explicitly supported catalog-only
// entries. Character creation can teach an NYI spell without granting a cast.
func Selectable(ids []Spell) []Spell {
	result := make([]Spell, 0, len(ids))
	for _, id := range ids {
		data := GetData(id)
		if HasCastProfile(id) || (data != nil && data.NotYetImplemented) {
			result = append(result, id)
		}
	}
	return result
}
