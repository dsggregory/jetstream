package jetstream

import (
	"context"

	"github.com/nats-io/nats.go"
)

// Config configuration used for JetStream consumers and producers
type Config struct {
	// Ctx the go context
	Ctx context.Context
	// Name logical name of the client.
	//
	// For subscribers, it is used to make durable subscriptions and sets the consumer name. A consumer using this name cannot be both a push-based (NewJetStreamSubscriber()) and pull-based (NewPullSubscriber()).
	//
	// For producer-only connections, this is simply the application's name.
	Name string
	// ClusterURLs comma-sep list of nats cluster node URLs - nats://user:pass@localhost:4222.
	// A single URL may specify a host with round-robin DNS and will be expanded to a list of URLs.
	ClusterURLs string
	// StreamConfig configuration when creating a new stream when one doesn't exist.
	//
	// Requires at least StreamConfig.Name to specify the stream name.
	// StreamConfig.Subjects MUST include either `stream-name.*` or the EXACT subject your publisher/subscriber will use. If `stream-name.*` is used, the subscriber would specify a subject like `stream-name.mysubject`.
	//
	// WARN: changing this after the stream has been created is not supported. You would need to run `nats str rm <stream-name>` before running with the changed config.
	StreamConfig *nats.StreamConfig
}
