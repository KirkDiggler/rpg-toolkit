package session

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRollWindowBodyPreservesPresentationIDAndLegacyAbsence(t *testing.T) {
	for _, tc := range []struct{ name, extra, want string }{
		{name: "current", extra: `,"presentation_id":"opaque~roll"`, want: "opaque~roll"},
		{name: "legacy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(`{"audience":"fighter","offer":{"ref":"dnd5e:conditions:inspired","name":"Bardic Inspiration"},"roll":8,"total":12` + tc.extra + `}`)
			body, ok := rollWindowOpenedBody(payload).(RollWindowOpenedBody)
			require.True(t, ok)
			require.Equal(t, tc.want, body.PresentationID)
			require.Equal(t, 8, body.Roll)
			require.Equal(t, 12, body.Total)
		})
	}
}
