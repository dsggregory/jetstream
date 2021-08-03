package jetstream

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	PubAckDur = 30 * time.Second
)

// PubIface an interface to a jetstream publisher
type PubIface interface {
	Publish(data []byte, ackif interface{}) error
	PublishSubject(subject string, data []byte, ackif interface{}) error
	Close()
	GetJetStreamContext() nats.JetStreamContext
}

// Publisher an instance of a jetstream interface
type Publisher struct {
	// js the NATS connection on a jetstream interface
	js JetStreamIface
	// publishSubject is the subject used when publishing every message using Publish() and MUST match one of the cfg.StreamConfig.Subjects. We do this because otherwise the jetstream SDK silently ignores a publish to a subject not configured on the stream.
	publishSubject string
}

// Close closes the nats connection
func (jp *Publisher) Close() {
	if jp.js != nil {
		jp.js.Close()
	}
}

// GetJetStreamContext returns the nats.JetStreamContext for the caller to use as they see fit.
func (jp *Publisher) GetJetStreamContext() nats.JetStreamContext {
	return jp.js.GetJetStreamContext()
}

// SubjectMatchesStreamConfig verify that the subject they intend to publish matches one of the subjects on the stream config
func SubjectMatchesStreamConfig(cfg *Config, publishSubject string) bool {
	sok := false
	for _, s := range cfg.StreamConfig.Subjects {
		if sok, _ = regexp.MatchString(s, publishSubject); sok {
			break
		}
	}
	return sok
}

// NewJetStreamPublisher creates a new jetstream publisher adding the stream from `cfg` if it doesn't exist.
// The `publishSubject` is the subject used when publishing every message using Publish() and MUST match one of the cfg.StreamConfig.Subjects. We do this because otherwise the jetstream SDK silently ignores a publish to a subject not configured on the stream.
func NewJetStreamPublisher(cfg *Config, publishSubject string) (PubIface, error) {
	if !SubjectMatchesStreamConfig(cfg, publishSubject) {
		return nil, ErrSubjectConfigMismatch{str: fmt.Sprintf("publish subject \"%s\" does not match stream config %v", publishSubject,
			cfg.StreamConfig.Subjects)}
	}

	j, err := NewJetStream(cfg)
	if err != nil {
		return nil, err
	}

	jp := &Publisher{js: j, publishSubject: publishSubject}
	return jp, nil
}

// PubAck the ack for each attempted publish
type PubAck struct {
	// Msg the nats message being published
	Msg *nats.Msg
	// Err is not nil, the Msg was not or may not have been published
	Err error
}

type PubAckHandler func(msg *nats.Msg, err error)

func (jp *Publisher) publish(subject string, data []byte, ackIf interface{}) error {
	var ackcb PubAckHandler
	var ackchannel chan *PubAck

	if ackIf != nil {
		switch v := ackIf.(type) {
		case func(msg *nats.Msg, err error): // PubAckHandler
			ackcb = v
		case chan *PubAck:
			ackchannel = v
		default:
			return ErrUnsupportedType{str: fmt.Sprintf("ackif unsupported type %T. Expecting func or channel.", v)}
		}
	}

	f, err := jp.js.GetJetStreamContext().PublishAsync(subject, data)
	if err != nil {
		return err
	}

	// wait on the ack in a go routine
	go func() {
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), PubAckDur)
		defer cancel()
		select {
		case err = <-f.Err():
		case <-f.Ok():
		case <-ctx.Done():
			if ctx.Err() != nil {
				err = ctx.Err()
			}
		}
		if ackchannel != nil {
			ackchannel <- &PubAck{
				Msg: f.Msg(),
				Err: err,
			}
		} else if ackcb != nil {
			ackcb(f.Msg(), err)
		}
	}()

	return nil
}

// Publish publishes data to the subject defined when the Publisher was created. It asynchronously writes to the `ackif` once it has been
// acknowledged by the server or times out. The `ackif` may be nil if ACKs are not needed or either:
//   * a channel of type (chan *PubAck)
//   * a callback func of type PubAckHandler
func (jp *Publisher) Publish(data []byte, ackIf interface{}) error {
	return jp.publish(jp.publishSubject, data, ackIf)
}

// PublishSubject like Publish() but uses a different subject which MUST match the StreamConfig.Subjects.
// There are no safeguards to protect against an erroneous subject whose data would be silently dropped.
// To be safe, call SubjectMatchesStreamConfig() before ever calling this.
func (jp *Publisher) PublishSubject(subject string, data []byte, ackIf interface{}) error {
	return jp.publish(subject, data, ackIf)
}
