package castletest

import (
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

func TestCastlePrisonerAnswersWithCountedThoughtfulWithoutChangingTheLine(t *testing.T) {
	for _, beaten := range []bool{false, true} {
		name := "failure"
		if beaten {
			name = "success"
		}
		t.Run(name, func(t *testing.T) {
			compiled := loadContent(t, kingFixture)
			var at spatial.Position
			placedFound := false
			for _, placed := range compiled.Monsters {
				if placed.MemberID == "prisoner-1" {
					at = placed.At
					placedFound = true
					break
				}
			}
			require.True(t, placedFound, "the canonical prisoner is placed")
			scene := openTheCastle(t, spatial.Position{X: at.X + 1, Y: at.Y})
			scene.reload()
			scene.said("prisoner-1", "where-is-the-king", "persuasion", beaten)
			story, err := scene.enc.Story(&encounter.StoryInput{Audience: hero})
			require.NoError(t, err)
			found := false
			for _, record := range story {
				var beat map[string]any
				require.NoError(t, json.Unmarshal(record.Payload, &beat))
				if beat["beat"] != encounter.BeatAnswered || beat["option"] != "where-is-the-king" {
					continue
				}
				found = true
				require.Equal(t, map[string]any{"ref": "dnd5e:activities:thoughtful", "cycles": float64(2)}, beat["activity"])
				if beaten {
					require.Contains(t, beat["say"], "Through the great hall")
					require.Equal(t, "king-in-throne-room", beat["fact"])
				} else {
					require.Equal(t, "Why should I tell you? Get me out first.", beat["say"])
				}
			}
			require.True(t, found)
		})
	}
}
