package session

import (
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAimPreviewDoesNotRevealUnseenOrRememberedMembers(t *testing.T) {
	holdings := []perception.Holding{
		{Subject: "visible-ally", CurrentVia: []perception.Channel{perception.Sight}},
		{Subject: "remembered", Channel: perception.Sight},
		{Subject: "heard", CurrentVia: []perception.Channel{"hearing"}},
		{Subject: "visible-enemy", CurrentVia: []perception.Channel{perception.Sight}},
	}
	require.Equal(t, []string{"visible-enemy", "visible-ally"}, visibleAimMembers("caster", []string{"unseen", "remembered", "visible-enemy", "heard", "visible-ally"}, holdings))
	require.Empty(t, visibleAimMembers("caster", []string{"unseen"}, holdings))
}
