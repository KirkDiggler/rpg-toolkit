package currency_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/ammunition"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/items"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/packs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

func TestMoneyRoundTripsThroughJSON(t *testing.T) {
	m := currency.FromCopper(1234)

	raw, err := json.Marshal(m)
	require.NoError(t, err)
	require.JSONEq(t, `{"copper":1234}`, string(raw))

	var got currency.Money
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, m, got)
}

func TestFromDenominationConversions(t *testing.T) {
	require.Equal(t, currency.Money{Copper: 7}, currency.FromCopper(7))
	require.Equal(t, currency.Money{Copper: 30}, currency.FromSilver(3))
	require.Equal(t, currency.Money{Copper: 150}, currency.FromElectrum(3))
	require.Equal(t, currency.Money{Copper: 500}, currency.FromGold(5))
	require.Equal(t, currency.Money{Copper: 2000}, currency.FromPlatinum(2))
	require.Equal(t, currency.Money{}, currency.FromCopper(0))
}

// realEquipmentCosts collects every Cost string in the current equipment
// catalog — weapons, armor, tools, packs, ammunition, items — the same set
// design.md §1 verified by hand. Read from the live catalogs rather than
// transcribed, so this test tracks the real data instead of a snapshot of
// it: a future entry with a malformed Cost fails here, not silently later.
//
// weapons.UnarmedStrike is excluded by name, not by "skip empty Cost"
// generally: it is the one entry across every catalog with an empty Cost
// (verified directly against the live maps), because it is not a
// purchasable good — weapons.go's own comment notes it is "not
// equippable." Excluding it by ID rather than by a blanket empty-string
// rule means a FUTURE priced item that accidentally ships with an empty
// Cost still fails this test instead of silently passing.
func realEquipmentCosts(t *testing.T) map[string]string {
	t.Helper()
	costs := map[string]string{}

	for id, w := range weapons.All {
		if id == weapons.UnarmedStrike {
			continue
		}
		costs[fmt.Sprintf("weapons.%s", id)] = w.Cost
	}
	for id, a := range armor.All {
		costs[fmt.Sprintf("armor.%s", id)] = a.Cost
	}
	for id, tl := range tools.All {
		costs[fmt.Sprintf("tools.%s", id)] = tl.Cost
	}
	for id, p := range packs.All {
		costs[fmt.Sprintf("packs.%s", id)] = p.Cost
	}
	for id, it := range items.All {
		costs[fmt.Sprintf("items.%s", id)] = it.Cost
	}
	for id, am := range ammunition.StandardAmmunition {
		costs[fmt.Sprintf("ammunition.%s", id)] = am.Cost
	}

	require.NotEmpty(t, costs, "the real catalogs must not be empty, or this test proves nothing")
	return costs
}

// TestParseCostParsesEveryRealEquipmentCost is design.md §6's headline
// requirement: ParseCost must correctly parse every Cost string in the
// current equipment data, not a handful of hand-picked examples.
//
// While collecting this set, two entries (ammunition.Arrows50,
// ammunition.Bolts50) turned out to be "2 gp 5 sp" — a genuine compound,
// contradicting design.md §1's claim that no compound occurs anywhere in
// the catalog. Normalized to "25 sp" (250 cp = 25 sp exactly, a lossless
// single-denomination equivalent) in ammunition.go rather than teaching
// ParseCost to parse a compound for two data points — see that file's own
// comment.
func TestParseCostParsesEveryRealEquipmentCost(t *testing.T) {
	for label, cost := range realEquipmentCosts(t) {
		t.Run(label+"="+cost, func(t *testing.T) {
			_, err := currency.ParseCost(cost)
			require.NoError(t, err, "cost %q for %s", cost, label)
		})
	}
}

func TestParseCostExamples(t *testing.T) {
	tests := []struct {
		cost string
		want currency.Money
	}{
		{"15 gp", currency.FromGold(15)},
		{"4 cp", currency.FromCopper(4)},
		{"1 sp", currency.FromSilver(1)},
		{"0 gp", currency.Money{}},
		{"25 sp", currency.FromSilver(25)},
		{"1 ep", currency.FromElectrum(1)},
		{"1 pp", currency.FromPlatinum(1)},
	}
	for _, tc := range tests {
		t.Run(tc.cost, func(t *testing.T) {
			got, err := currency.ParseCost(tc.cost)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseCostRefusesMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		cost string
		want error
	}{
		{"compound", "1 gp 5 sp", currency.ErrMalformedCost},
		{"missing denomination", "15", currency.ErrMalformedCost},
		{"missing amount", "gp", currency.ErrMalformedCost},
		{"empty string", "", currency.ErrMalformedCost},
		{"non-integer amount", "fifteen gp", currency.ErrMalformedCost},
		{"negative amount", "-5 gp", currency.ErrMalformedCost},
		{"unknown denomination", "15 xp", currency.ErrUnknownDenomination},
		{"wrong case denomination", "15 GP", currency.ErrUnknownDenomination},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := currency.ParseCost(tc.cost)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

func TestAdd(t *testing.T) {
	require.Equal(t, currency.FromCopper(30), currency.FromGold(0).Add(currency.FromSilver(3)))
	require.Equal(t, currency.FromGold(5), currency.FromGold(2).Add(currency.FromGold(3)))
	require.Equal(t, currency.Money{}, currency.Money{}.Add(currency.Money{}))
}

func TestSub(t *testing.T) {
	t.Run("exact to zero", func(t *testing.T) {
		got, err := currency.FromGold(5).Sub(currency.FromGold(5))
		require.NoError(t, err)
		require.Equal(t, currency.Money{}, got)
	})

	t.Run("partial", func(t *testing.T) {
		got, err := currency.FromGold(5).Sub(currency.FromSilver(1))
		require.NoError(t, err)
		require.Equal(t, currency.FromCopper(490), got)
	})

	t.Run("zero minus zero", func(t *testing.T) {
		got, err := currency.Money{}.Sub(currency.Money{})
		require.NoError(t, err)
		require.Equal(t, currency.Money{}, got)
	})

	t.Run("one short is refused, not negative", func(t *testing.T) {
		_, err := currency.FromCopper(4).Sub(currency.FromCopper(5))
		require.ErrorIs(t, err, currency.ErrInsufficientFunds)
	})
}

func TestCanAfford(t *testing.T) {
	purse := currency.FromGold(10)

	require.True(t, purse.CanAfford(currency.FromGold(10)), "afford exactly")
	require.False(t, purse.CanAfford(currency.FromCopper(1001)), "afford one short")
	require.True(t, purse.CanAfford(currency.Money{}), "anything affords free")
	require.True(t, purse.CanAfford(currency.FromGold(9)))
	require.False(t, purse.CanAfford(currency.FromGold(11)))
}

func TestBreakdown(t *testing.T) {
	tests := []struct {
		name string
		cp   int
		want currency.Breakdown
	}{
		{"zero", 0, currency.Breakdown{}},
		{"design.md's own example", 1247, currency.Breakdown{Platinum: 1, Gold: 2, Silver: 4, Copper: 7}},
		{"pure copper", 7, currency.Breakdown{Copper: 7}},
		{"exact platinum", 3000, currency.Breakdown{Platinum: 3}},
		{
			"every denomination at once",
			1273, // 1000 (1pp) + 200 (2gp) + 50 (1ep) + 20 (2sp) + 3 (3cp)
			currency.Breakdown{Platinum: 1, Gold: 2, Electrum: 1, Silver: 2, Copper: 3},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := currency.FromCopper(tc.cp).Breakdown()
			require.Equal(t, tc.want, got)
		})
	}
}
