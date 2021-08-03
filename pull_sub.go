package jetstream

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// PullSubIface an interface to a jetstream pull-based subscriber
type PullSubIface interface {
	Fetch(count int, timeout time.Duration) ([]*nats.Msg, error)
	Close()
	GetJetStreamContext() nats.JetStreamContext
}

// PullSubscriber an instance of a pull-based subscription on a jetstream stream
type PullSubscriber struct {
	js  JetStreamIface
	sub *nats.Subscription
}

// Close closes the nats jetstream subscription
func (jsub *PullSubscriber) Close() {
	// WARN: don't unsubscribe the subscription or new ones will start from the beginning
	if jsub.js != nil {
		jsub.js.Close()
	}
}

// GetJetStreamContext returns the nats.JetStreamContext for the caller to use as they see fit.
func (jsub *PullSubscriber) GetJetStreamContext() nats.JetStreamContext {
	return jsub.js.GetJetStreamContext()
}

// Fetch fetches the number of messages from a pull-based subscription. Messages are acknowledged by this function.
//
// Returns error of ErrTimeout on timeout error, or a low level nats error otherwise.
func (jsub *PullSubscriber) Fetch(count int, timeout time.Duration) ([]*nats.Msg, error) {
	msgs, err := jsub.sub.Fetch(count, nats.MaxWait(timeout))
	if err != nil {
		if err.Error() == nats.ErrTimeout.Error() {
			return nil, ErrTimeout{}
		}
		return nil, err
	}
	// ack the messages for the caller
	for _, m := range msgs {
		_ = m.Ack()
	}
	return msgs, nil
}

// NewPullSubscriber creates a new pull-based JetStream subscription where the user calls Fetch() to read next messages.
//
// Warn: cfg.Name is used for the durable consumer name. Ensure pull-based consumers created by NewJetStreamSubscriber() use a different cfg.Name.
//
// The `subject` is that to subscribe, ex: "Domains.domain".
func NewPullSubscriber(cfg *Config, subject string) (PullSubIface, error) {
	j, err := NewJetStream(cfg)
	if err != nil {
		return nil, err
	}

	// subscription options specify policies and are used to create a consumer if one does not already exist.
	opts := []nats.SubOpt{
		// without this, the stream cannot be determined
		nats.BindStream(cfg.StreamConfig.Name),
		nats.DeliverAll(),
	}

	// a durable (cfg.Name) pull subscription
	sub, err := j.GetJetStreamContext().PullSubscribe(subject, cfg.Name, opts...)
	if err != nil {
		j.Close()
		if err.Error() == nats.ErrPullSubscribeToPushConsumer.Error() {
			err = fmt.Errorf("%w; possibly an existing push-based consumer exists. Try `nats con info %s %s`",
				err, cfg.StreamConfig.Name, cfg.Name)
		}
		return nil, err
	}

	jsub := &PullSubscriber{
		js:  j,
		sub: sub,
	}
	consumer, _ := sub.ConsumerInfo()
	logrus.WithFields(logrus.Fields{
		"stream":          cfg.StreamConfig.Name,
		"deliverySubject": consumer.Config.DeliverSubject,
		"filterSubject":   subject,
		"numWaiting":      consumer.NumWaiting,
		"queue":           jsub.sub.Queue,
	}).Debug("subscribed to jetstream")

	return jsub, nil
}
