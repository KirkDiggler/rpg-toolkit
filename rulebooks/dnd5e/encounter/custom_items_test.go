// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// custom_items_test.go is the encounter's half of A SITE FILE SAYS CUSTOM
// ITEMS (rpg-project#850, rpg-project#1166): a member's equipment and a placed
// prop's item are checked at construction, KEPT, saved and loaded byte for
// byte — and read by nothing. The site is dungeonspec/testdata/custom-items.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const (
	cloakSite   = "dungeonspec/testdata/custom-items/site.yaml"
	cloakPlayer = encounter.MemberID("scout")
	cloakProp   = encounter.PropID("cloak-on-the-peg")
	cloakJSON   = `{"charges":{"max":1,"refills":{"on":["short rest","long rest"]}},` +
		`"lets":[{"cost":{"charges":1},"use":"second wind","with":{"level":1}}],` +
		`"name":"Lieutenant's cloak","slot":"cloak"}`
)

func cloakCompiled(t *testing.T) dungeonspec.Compiled {
	t.Helper()
	raw, err := os.ReadFile(cloakSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	c, err := dungeonspec.Load(raw)
	require.NoError(t, err)
	return c
}

// cloakMembers is the player and the site's creatures, handed over the way a
// host does: each placement's fields copied onto a member input. withEquipment
// is whether the host hands the placement's Equipment across; arrives makes the
// creature wait in reserve.
func cloakMembers(c dungeonspec.Compiled, withEquipment bool, arrives encounter.Trigger) []encounter.MemberInput {
	members := []encounter.MemberInput{{ID: cloakPlayer, Kind: encounter.KindPlayer, Position: c.PartyStart[0].At}}
	for _, m := range c.Monsters {
		in := encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Arrives: arrives,
		}
		if withEquipment {
			in.Equipment = m.Equipment
		}
		members = append(members, in)
	}
	return members
}

func cloakCapabilities(standing encounter.Participation) encounter.Capabilities {
	return encounter.Capabilities{
		Sight:         &sightList{fallback: 24},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        zeroSheets{},
		Participation: standing,
		Initiative:    orderAsGiven{},
		Driver:        passDriver{},
		Roller:        rollsLowest{},
		CheckResolver: findsNothing{},
		Witness:       nobodyPerceives{},
		Actors:        encounter.Actors{Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{}},
	}
}

func cloakSetup(c dungeonspec.Compiled, members []encounter.MemberInput, standing encounter.Participation) *encounter.SetupInput {
	return &encounter.SetupInput{
		Field:        c.Field,
		Members:      members,
		Endings:      []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Capabilities: cloakCapabilities(standing),
		Retention:    encounter.RetentionUnbounded,
		IDs:          newFixedIDs(),
	}
}

func openCloak(t *testing.T, c dungeonspec.Compiled, members []encounter.MemberInput, standing encounter.Participation) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(cloakSetup(c, members, standing))
	require.NoError(t, err)
	return enc
}

func saved(t *testing.T, enc *encounter.Encounter) []byte {
	t.Helper()
	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	return raw
}

func loadCloak(t *testing.T, raw []byte, standing encounter.Participation) *encounter.Encounter {
	t.Helper()
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: cloakCapabilities(standing), IDs: newFixedIDs(),
	})
	require.NoError(t, err)
	return enc
}

// Test 5: the encounter keeps what it was told, through ToData, JSON and Load,
// byte for byte; and one with neither has no new key at all.
func TestCustomItems_TheEncounterKeepsWhoHasThemAndWhichPropIsOne(t *testing.T) {
	c := cloakCompiled(t)
	standing := everyoneStanding{}
	enc := openCloak(t, c, cloakMembers(c, true, nil), standing)

	data := enc.ToData()
	var lieutenant *encounter.MemberData
	for i := range data.Members {
		if data.Members[i].ID == encounter.MemberID(c.Monsters[0].MemberID) {
			lieutenant = &data.Members[i]
		}
	}
	require.NotNil(t, lieutenant)
	require.Len(t, lieutenant.Equipment, 1)
	require.Equal(t, "lieutenants-cloak", lieutenant.Equipment[0].ID)
	require.Equal(t, cloakJSON, string(lieutenant.Equipment[0].Item))

	// The prop is part of the SITE, which a host stores as its own piece: it
	// compiles, writes, reads back and writes the same bytes.
	site, err := encounter.CompileSite(&encounter.CompileSiteInput{Field: &c.Field})
	require.NoError(t, err)
	siteData := site.Site.Data()
	var cloak *encounter.PlacedPropData
	for i := range siteData.Field.Placed {
		if siteData.Field.Placed[i].ID == cloakProp {
			cloak = &siteData.Field.Placed[i]
		}
	}
	require.NotNil(t, cloak)
	require.NotNil(t, cloak.Item)
	require.Equal(t, cloakJSON, string(cloak.Item.Item))
	siteRaw, err := json.Marshal(siteData)
	require.NoError(t, err)
	var back encounter.SiteData
	require.NoError(t, json.Unmarshal(siteRaw, &back))
	again, err := encounter.CompileSite(&encounter.CompileSiteInput{Data: &back})
	require.NoError(t, err)
	againRaw, err := json.Marshal(again.Site.Data())
	require.NoError(t, err)
	require.Equal(t, string(siteRaw), string(againRaw), "the site: written, read, written: byte for byte")
	require.Equal(t, site.Site.Hash(), again.Site.Hash())

	first := saved(t, enc)
	reloaded := saved(t, loadCloak(t, first, standing))
	require.Equal(t, string(first), string(reloaded), "saved, loaded and saved again: byte for byte")
	require.Contains(t, string(reloaded), `"equipment":[{"id":"lieutenants-cloak","name":"Lieutenant's cloak","item":`+cloakJSON+`}]`)
}

// The member who waits in reserve keeps its equipment across a save too: its
// facts are the same facts, and a save must not lose them.
func TestCustomItems_AMemberInReserveKeepsItsEquipment(t *testing.T) {
	c := cloakCompiled(t)
	standing := everyoneStanding{}
	enc := openCloak(t, c, cloakMembers(c, true, encounter.TriggerRound{Round: 6}), standing)

	data := enc.ToData()
	require.Len(t, data.Reserve, 1, "the lieutenant waits")
	require.Len(t, data.Reserve[0].Equipment, 1)
	require.Equal(t, cloakJSON, string(data.Reserve[0].Equipment[0].Item))

	first := saved(t, enc)
	require.Equal(t, string(first), string(saved(t, loadCloak(t, first, standing))))
}

// Nothing new is written for an encounter with no equipment and no item prop:
// the saved shape of every encounter before these words.
func TestCustomItems_AnEncounterWithNeitherWritesNoNewKey(t *testing.T) {
	c := cloakCompiled(t)
	// The same site, with the host not handing the equipment across and the
	// prop's item taken off the placement.
	for i := range c.Field.Placed {
		c.Field.Placed[i].Item = nil
	}
	enc := openCloak(t, c, cloakMembers(c, false, nil), everyoneStanding{})
	raw := string(saved(t, enc))
	require.NotContains(t, raw, `"equipment"`)
	require.NotContains(t, raw, "lieutenants-cloak")
	site, err := encounter.CompileSite(&encounter.CompileSiteInput{Field: &c.Field})
	require.NoError(t, err)
	siteRaw, err := json.Marshal(site.Site.Data())
	require.NoError(t, err)
	require.NotContains(t, string(siteRaw), `"item":{`, "the site writes no item key for a prop that is none")
	require.NotContains(t, string(siteRaw), "lieutenants-cloak")

	reserved := openCloak(t, c, cloakMembers(c, false, encounter.TriggerRound{Round: 6}), everyoneStanding{})
	require.NotContains(t, string(saved(t, reserved)), `"equipment"`)
}

// Test 6: construction refuses a grant nobody could author on purpose.
func TestCustomItems_ConstructionRefusesAMalformedGrant(t *testing.T) {
	c := cloakCompiled(t)
	good := encounter.ItemGrant{ID: "lieutenants-cloak", Item: json.RawMessage(cloakJSON)}
	for name, tc := range map[string]struct {
		equipment []encounter.ItemGrant
		message   string
	}{
		"an empty id":      {[]encounter.ItemGrant{{ID: "", Item: good.Item}}, "has an item with no id"},
		"empty item JSON":  {[]encounter.ItemGrant{{ID: "x", Item: nil}}, `item "x", whose authored block is not a JSON object`},
		"not an object":    {[]encounter.ItemGrant{{ID: "x", Item: json.RawMessage(`[1]`)}}, `item "x", whose authored block is not a JSON object`},
		"invalid JSON":     {[]encounter.ItemGrant{{ID: "x", Item: json.RawMessage(`{"a":`)}}, `item "x", whose authored block is not a JSON object`},
		"one id twice":     {[]encounter.ItemGrant{good, good}, `item "lieutenants-cloak" equipped twice`},
		"two ids are fine": {nil, ""},
	} {
		t.Run(name, func(t *testing.T) {
			members := cloakMembers(c, false, nil)
			members[1].Equipment = tc.equipment
			_, err := encounter.NewEncounter(cloakSetup(c, members, everyoneStanding{}))
			if tc.message == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, encounter.ErrBadMember)
			require.ErrorContains(t, err, tc.message)
		})
	}

	t.Run("a prop's item with no id", func(t *testing.T) {
		c := cloakCompiled(t)
		for i := range c.Field.Placed {
			if c.Field.Placed[i].ID == cloakProp {
				c.Field.Placed[i].Item = &encounter.ItemGrant{ID: "", Item: good.Item}
			}
		}
		_, err := encounter.NewEncounter(cloakSetup(c, cloakMembers(c, false, nil), everyoneStanding{}))
		require.ErrorIs(t, err, encounter.ErrNoField)
		require.ErrorContains(t, err, "has an item with no id")
	})
	t.Run("a prop's item that is not an object", func(t *testing.T) {
		c := cloakCompiled(t)
		for i := range c.Field.Placed {
			if c.Field.Placed[i].ID == cloakProp {
				c.Field.Placed[i].Item = &encounter.ItemGrant{ID: "x", Item: json.RawMessage(`"cloak"`)}
			}
		}
		_, err := encounter.NewEncounter(cloakSetup(c, cloakMembers(c, false, nil), everyoneStanding{}))
		require.ErrorIs(t, err, encounter.ErrNoField)
	})
}

// A saved grant that is malformed is refused at load, as at construction.
func TestCustomItems_LoadRefusesAMalformedGrant(t *testing.T) {
	c := cloakCompiled(t)
	raw := saved(t, openCloak(t, c, cloakMembers(c, true, nil), everyoneStanding{}))
	broken := strings.Replace(string(raw), `"equipment":[{"id":"lieutenants-cloak"`, `"equipment":[{"id":""`, 1)
	require.NotEqual(t, string(raw), broken)
	var data encounter.EncounterData
	require.NoError(t, json.Unmarshal([]byte(broken), &data))
	_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: cloakCapabilities(everyoneStanding{})})
	require.ErrorIs(t, err, encounter.ErrInvalidData)
}

// Test 7: NOTHING ELSE IN A RUN USES THEM. Trying to hold the item prop behaves
// exactly as it does for a prop that is not an item: the same answer, the same
// beats, the same saved bytes once the new facts are set aside. (Looting the
// equipped creature's body used to be here; it now gives the looter the items,
// rpg-project#1167, and is tested in looting_equipment_test.go.)
func TestCustomItems_NothingInARunUsesThem(t *testing.T) {
	c := cloakCompiled(t)
	lieutenant := encounter.MemberID(c.Monsters[0].MemberID)

	run := func(equipped bool) (hold string, story, state []byte) {
		cc := cloakCompiled(t)
		if !equipped {
			for i := range cc.Field.Placed {
				cc.Field.Placed[i].Item = nil
			}
		}
		down := &downList{}
		enc := openCloak(t, cc, cloakMembers(cc, equipped, nil), down)
		down.down = []encounter.MemberID{lieutenant}
		_, err := aRound(enc)
		require.NoError(t, err)

		holdOut, holdErr := enc.Hold(&encounter.HoldInput{Member: cloakPlayer, Target: cloakProp, Range: 50})
		hold = string(mustJSON(t, holdOut)) + "|" + errTextOrOK(holdErr)

		beats, err := enc.Story(&encounter.StoryInput{Audience: cloakPlayer})
		require.NoError(t, err)
		story = mustJSON(t, beats)

		// The saved bytes with the new facts set aside: the same encounter.
		var data encounter.EncounterData
		require.NoError(t, json.Unmarshal(saved(t, enc), &data))
		for i := range data.Members {
			data.Members[i].Equipment = nil
		}
		// The two sites differ by the item the prop is, and by nothing else: so the
		// site's hash differs, and the one beat that names it (the scene's content).
		data.SiteHash = ""
		kept := data.Log.Entries[:0:0]
		for _, entry := range data.Log.Entries {
			if !strings.Contains(string(entry.Payload), "site_hash") {
				kept = append(kept, entry)
			}
		}
		require.Len(t, kept, len(data.Log.Entries)-1, "exactly one beat names the site's hash")
		data.Log.Entries = kept
		state = mustJSON(t, data)
		return hold, story, state
	}

	holdWith, storyWith, stateWith := run(true)
	holdWithout, storyWithout, stateWithout := run(false)
	require.Equal(t, holdWithout, holdWith, "holding the prop: the same answer")
	require.Equal(t, string(storyWithout), string(storyWith), "the same beats")
	require.Equal(t, string(stateWithout), string(stateWith), "the same encounter, the new facts aside")
	require.Contains(t, holdWith, "|", "the hold was asked, and answered")
}

func errTextOrOK(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}
