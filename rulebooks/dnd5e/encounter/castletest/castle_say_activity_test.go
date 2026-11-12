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
			require.Equal(t, 1, scene.linesSaid(hero, "where-is-the-king"), "the hero asked once")
			var answer map[string]any
			var fact any
			for _, record := range story {
				var beat map[string]any
				require.NoError(t, json.Unmarshal(record.Payload, &beat))
				switch {
				case beat["beat"] == encounter.BeatAnswered && beat["creature"] == "prisoner-1":
					answer = beat
				case beat["beat"] == encounter.BeatSpeech && beat["speaker"] == "prisoner-1":
					fact = beat["fact"]
				}
			}
			require.NotNil(t, answer, "the prisoner answered")
			require.Equal(t, map[string]any{"ref": "dnd5e:activities:thoughtful", "cycles": float64(2)}, answer["activity"])
			said := scene.saidBy(hero, "prisoner-1")
			require.NotEmpty(t, said, "the hero caught the prisoner's line")
			if beaten {
				require.Contains(t, said[len(said)-1], "Through the great hall")
				require.Equal(t, "king-in-throne-room", fact, "the fact travels with the words")
			} else {
				require.Equal(t, "Why should I tell you? Get me out first.", said[len(said)-1])
				require.Nil(t, fact)
			}
		})
	}
}
