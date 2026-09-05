package currency

import (
	"fmt"
	"strconv"
	"strings"
)

// Money is a normalized D&D 5e currency amount, stored as copper — the
// smallest denomination — so arithmetic never reconciles mixed units.
type Money struct {
	Copper int `json:"copper"`
}

// SRD conversion: 1 pp = 10 gp = 100 sp = 1,000 cp; 1 ep = 5 sp = 50 cp.
const (
	copperPerSilver   = 10
	copperPerElectrum = 50
	copperPerGold     = 100
	copperPerPlatinum = 1000
)

// FromCopper returns cp copper pieces as Money.
func FromCopper(cp int) Money { return Money{Copper: cp} }

// FromSilver returns sp silver pieces as Money.
func FromSilver(sp int) Money { return Money{Copper: sp * copperPerSilver} }

// FromElectrum returns ep electrum pieces as Money.
func FromElectrum(ep int) Money { return Money{Copper: ep * copperPerElectrum} }

// FromGold returns gp gold pieces as Money.
func FromGold(gp int) Money { return Money{Copper: gp * copperPerGold} }

// FromPlatinum returns pp platinum pieces as Money.
func FromPlatinum(pp int) Money { return Money{Copper: pp * copperPerPlatinum} }

// denominationConverters maps a cost string's denomination token to the
// From* constructor that turns an amount of it into Money.
var denominationConverters = map[string]func(int) Money{
	"cp": FromCopper,
	"sp": FromSilver,
	"ep": FromElectrum,
	"gp": FromGold,
	"pp": FromPlatinum,
}

// ParseCost parses a single-denomination equipment cost string ("15 gp",
// "4 cp") into Money.
//
// Every Cost string in the current equipment data (weapons, armor, tools,
// packs, ammunition, items) is exactly this shape: a nonnegative integer, a
// space, and one of cp/sp/ep/gp/pp. A compound ("1 gp 5 sp"), a missing or
// extra token, a non-integer or negative amount (ErrMalformedCost), or an
// unrecognized denomination (ErrUnknownDenomination) is refused as a data
// defect rather than silently misparsed.
func ParseCost(cost string) (Money, error) {
	fields := strings.Fields(cost)
	if len(fields) != 2 {
		return Money{}, fmt.Errorf("%w: %q", ErrMalformedCost, cost)
	}

	amount, err := strconv.Atoi(fields[0])
	if err != nil || amount < 0 {
		return Money{}, fmt.Errorf("%w: %q", ErrMalformedCost, cost)
	}

	convert, ok := denominationConverters[fields[1]]
	if !ok {
		return Money{}, fmt.Errorf("%w: %q", ErrUnknownDenomination, cost)
	}

	return convert(amount), nil
}

// Add returns the sum of m and other.
func (m Money) Add(other Money) Money {
	return Money{Copper: m.Copper + other.Copper}
}

// Sub returns m minus other, or ErrInsufficientFunds if that would be
// negative — a purse cannot hold less than nothing, so this is refused
// rather than returning a value every caller would have to separately
// range-check.
func (m Money) Sub(other Money) (Money, error) {
	if m.Copper < other.Copper {
		return Money{}, fmt.Errorf("%w: have %d cp, need %d cp", ErrInsufficientFunds, m.Copper, other.Copper)
	}
	return Money{Copper: m.Copper - other.Copper}, nil
}

// CanAfford reports whether m holds at least cost.
func (m Money) CanAfford(cost Money) bool {
	return m.Copper >= cost.Copper
}

// Breakdown is the canonical coin-purse split of a Money amount, largest
// denomination first, each remainder carried down (e.g. 1247 copper -> 1
// pp, 2 gp, 4 sp, 7 cp) — what a display actually wants, not five
// independent "entirely in this one unit" conversions.
type Breakdown struct {
	Platinum, Gold, Electrum, Silver, Copper int
}

// Breakdown splits m into the canonical coin-purse denominations, greedily
// carrying each remainder down the chain platinum -> gold -> electrum ->
// silver -> copper (each divides the one before it: 1000, 100, 50, 10, 1
// copper).
func (m Money) Breakdown() Breakdown {
	remaining := m.Copper

	platinum := remaining / copperPerPlatinum
	remaining -= platinum * copperPerPlatinum

	gold := remaining / copperPerGold
	remaining -= gold * copperPerGold

	electrum := remaining / copperPerElectrum
	remaining -= electrum * copperPerElectrum

	silver := remaining / copperPerSilver
	remaining -= silver * copperPerSilver

	return Breakdown{
		Platinum: platinum, Gold: gold, Electrum: electrum, Silver: silver, Copper: remaining,
	}
}
