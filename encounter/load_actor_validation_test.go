package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter"
)

func TestLoadFromDataRejectsNullActorMapEntriesBeforeHydration(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{name: "null player", raw: `{"id":"nil-player","players":{"alice":null}}`, want: `validate player "alice": null player state`},
		{name: "missing player view", raw: `{"id":"nil-view","players":{"alice":{"id":"alice","entity_id":"alice"}}}`, want: `validate player "alice": view is required`},
		{name: "null monster", raw: `{"id":"nil-monster","monsters":{"goblin":null}}`, want: `validate monster "goblin": null monster state`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var data encounter.Data
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &data))
			transport := encounter.NewInMemoryTransport()
			broker := encounter.NewBroker(transport)
			t.Cleanup(func() { _ = broker.Close(); _ = transport.Close() })
			loaded, err := encounter.LoadFromData(context.Background(), &data, broker)
			require.Nil(t, loaded)
			require.EqualError(t, err, tc.want)
		})
	}
}
