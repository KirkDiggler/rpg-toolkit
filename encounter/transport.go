package encounter

// Transport is the pluggable pub/sub interface the Broker uses to
// distribute event bytes across processes (or within one).
//
// Channel keys are opaque to the Transport — the Broker chooses the key
// scheme (e.g., "enc:<encID>"). Payloads are opaque bytes — encoding is
// the Broker's concern.
type Transport interface {
	Publish(channel string, payload []byte) error
	Subscribe(channel string) (TransportSubscription, error)
}

// TransportSubscription is the per-call return from Subscribe. It exposes
// a receive channel and a Close that releases resources.
type TransportSubscription interface {
	Payloads() <-chan []byte
	Close() error
}
