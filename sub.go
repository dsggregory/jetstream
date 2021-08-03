package jetstream

import (
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// SubIface an interface to a jetstream subscriber
type SubIface interface {
	Close()
	GetJetStreamContext() nats.JetStreamContext
}

// Subscriber a jetstream subscription
type Subscriber struct {
	js  JetStreamIface
	sub *nats.Subscription
}

// Close closes the nats jetstream subscription
func (jsub *Subscriber) Close() {
	// WARN: don't unsubscribe the subscription or new ones will start from the beginning
	if jsub.js != nil {
		jsub.js.Close()
	}
}

// GetJetStreamContext returns the nats.JetStreamContext for the caller to use as they see fit.
func (jsub *Subscriber) GetJetStreamContext() nats.JetStreamContext {
	return jsub.js.GetJetStreamContext()
}

// newStreamConsumer create the jetstream, consumer and subscription
func newStreamConsumer(cfg *Config, subject string, queueName string, recip interface{}) (*Subscriber, error) {
	var msgcb nats.MsgHandler
	var msgchannel chan *nats.Msg

	switch v := recip.(type) {
	case func(msg *nats.Msg): // nats.Msg()
		msgcb = v
	case chan *nats.Msg:
		msgchannel = v
	default:
		return nil, ErrUnsupportedType{"recip unsupported type. Expecting func or channel."}
	}

	j, err := NewJetStream(cfg)
	if err != nil {
		return nil, err
	}

	/* could create a named consumer (e.g. DeliverSubject) but likely unnecessary since one is created for you as needed.
	// create/reuse a durable consumer
	deliverSubj := nats.InboxPrefix + subject	// needs to be used as 1st arg to j.GetJetStreamContext().Subscribe()
	consumer, err := j.GetJetStreamContext().ConsumerInfo(cfg.StreamConfig.Name, cfg.Name)
	if consumer == nil {
		consumer, err = j.GetJetStreamContext().AddConsumer(cfg.StreamConfig.Name, &nats.ConsumerConfig{
			// consumer's durable name
			Durable: cfg.Name,
			// we won't ACK, so the msg will no longer be available to this durable consumer
			AckPolicy: nats.AckNonePolicy,
			// this acts like a durability name and without it you get the same messages each time. Regardless if the consumer specifies `Durable` or the Subscription specifies nats.Durable().
			DeliverSubject: deliverSubj,
		})
		if err != nil {
			j.Close()
			return nil, err
		}
	}
	*/

	// subscription options specify policies and are used to create a consumer if one does not already exist.
	opts := []nats.SubOpt{
		// without this, the stream cannot be determined
		nats.BindStream(cfg.StreamConfig.Name),
		// the name of the consumer which will pick up where it left off after a restart
		nats.Durable(cfg.Name),
		// automatic ack on rec'd msgs
		nats.AckNone(),
		// with no-ack, only ever deliver the same message once
		//nats.MaxDeliver(1),
		nats.DeliverAll(),
	}

	// subscribe which will create a consumer if one for the stream/subject/durable does not already exist.
	var sub *nats.Subscription
	if msgcb == nil {
		// This is effectively how ChanSubscribe() methods work and doing it this way we can keep the code simple.
		msgcb = func(msg *nats.Msg) {
			msgchannel <- msg
		}
	}
	if queueName != "" { // TODO unsure of the value of queueName
		sub, err = j.GetJetStreamContext().QueueSubscribe(subject, queueName, msgcb, opts...)
	} else {
		sub, err = j.GetJetStreamContext().Subscribe(subject, msgcb, opts...)
	}
	if err != nil {
		j.Close()
		return nil, err
	}

	jsub := &Subscriber{
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

// NewJetStreamSubscriber creates a new jetstream push-based subscription adding the stream from `cfg` if it doesn't exist. Creates a durable consumer that requires no ACK on received messages.
// The `subject` is that to subscribe, ex: "Domains.domain".
// For a Queue Group, set `queueName` to the name of the group. A message will only be delivered to one of the consumers in the group.
// The `ackRecip` may be nil if ACKs are not needed or either:
//   * a channel of type (chan *nats.Msg)
//   * a callback func of type nats.MsgHandler
func NewJetStreamSubscriber(cfg *Config, subject string, queueName string, ackRecip interface{}) (SubIface, error) {
	return nil, ErrNotImplemented{}
	// TODO this will cause the cluster to become unstable. It has something to do with the way we handle acknowlegements. See Push_Consumer_Issues.md
	//return newStreamConsumer(cfg, subject, queueName, ackRecip)
}
