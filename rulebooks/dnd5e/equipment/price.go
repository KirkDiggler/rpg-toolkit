package equipment

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// PriceOf resolves id's catalog Cost into Money — the currency-aware sibling
// to ResolveEquipmentDetail, which returns Cost as an opaque, unparsed
// string. Lives here rather than in currency: currency stays
// dependency-free (a bare string in, Money out), and equipment already
// knows how to find any catalog item's Detail by ID.
//
// Returns rpgerr.CodeNotFound if id names no catalog entry (the same
// vocabulary GetByID uses), or wraps currency.ParseCost's error if a
// catalog entry's Cost string does not parse — a data defect in this
// module's own catalog, not a caller mistake.
func PriceOf(id shared.EquipmentID) (currency.Money, error) {
	detail := ResolveEquipmentDetail(id)
	if detail == nil {
		return currency.Money{}, rpgerr.Newf(rpgerr.CodeNotFound, "equipment %q not found", id)
	}

	price, err := currency.ParseCost(detail.Cost)
	if err != nil {
		return currency.Money{}, rpgerr.Newf(rpgerr.CodeInternal,
			"equipment %q: %v", id, err)
	}

	return price, nil
}
