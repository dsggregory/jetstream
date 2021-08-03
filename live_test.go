package jetstream

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"

	. "github.com/smartystreets/goconvey/convey"
)

func logConsumers(js nats.JetStreamContext, cfg Config) {
	consumer, err := js.ConsumerInfo(PsStreamName, cfg.Name)
	if err != nil {
		logrus.WithError(err).Error("cannot get consumer for stream")
	}
	logrus.WithField("consumer", consumer).Debug("consumersInfo")
}

func TestLive(t *testing.T) {
	server := os.Getenv("NATS_URL")
	if server == "" {
		t.Skip("Requires live NATS 2.0 server. See README for docker details.")
	}

	logrus.SetLevel(logrus.TraceLevel)

	jscfg := &Config{
		Ctx:         context.Background(),
		Name:        PsDurableName,
		ClusterURLs: nats.DefaultURL,
		StreamConfig: &nats.StreamConfig{
			Name:      PsStreamName,
			Retention: nats.InterestPolicy, // msg removed after subscriber ACKs
			MaxAge:    10 * time.Minute,
		},
	}

	SkipConvey("Should pubsub with consumer group using chan", t, func() {
		pub, err := NewJetStreamPublisher(jscfg, PsFullSubject)
		So(err, ShouldBeNil)
		defer pub.Close()
		_ = pub.GetJetStreamContext().PurgeStream(PsStreamName)

		mc := make(chan *nats.Msg)
		/*
			sub, err := NewJetStreamSubscriberCB(jscfg, PsFullSubject, "psGroup", func(msg *nats.Msg) {
				mc <- msg
			})
		*/
		sub, err := NewJetStreamSubscriber(jscfg, PsFullSubject, "psGroup", mc)
		So(err, ShouldBeNil)
		defer sub.Close()
		logConsumers(sub.GetJetStreamContext(), *jscfg)

		msg := "msg1"
		ackChan := make(chan *PubAck)
		err = pub.Publish([]byte(msg), ackChan)
		So(err, ShouldBeNil)
		select {
		case ack := <-ackChan:
			So(ack.Err, ShouldBeNil)
		}

		select {
		case m := <-mc:
			So(string(m.Data), ShouldEqual, msg)
		case t := <-time.After(3 * time.Second):
			So(t, ShouldBeNil)
		}

		// next same subscriber should have nothing to read
		sub.Close()
		mc = make(chan *nats.Msg)
		sub2, err := NewJetStreamSubscriber(jscfg, PsFullSubject, "psGroup", mc)
		So(err, ShouldBeNil)
		logConsumers(sub2.GetJetStreamContext(), *jscfg)
		defer sub2.Close()
		select {
		case m := <-mc:
			So(string(m.Data), ShouldEqual, "") // should have nothing to read
		case <-time.After(1 * time.Second):
		}

	})
	Convey("PubAsync tests", t, func() {
		jsif, err := NewJetStream(jscfg)
		So(err, ShouldBeNil)
		defer jsif.Close()
		js := jsif.GetJetStreamContext()
		_ = js.PurgeStream(PsStreamName)

		Convey("with future", func() {
			f, err := js.PublishAsync(PsFullSubject, []byte("pub future"))
			So(err, ShouldBeNil)
			state := "unknown"
			select {
			case err = <-f.Err():
				// +OK {"stream": "PsTestStream", "seq": 91794}
				state = err.Error()
			case <-f.Ok():
				state = "ok"
			case <-time.After(time.Second):
				state = "ack time-out"
			}
			So(state, ShouldEqual, "ok")
		})
	})
}
