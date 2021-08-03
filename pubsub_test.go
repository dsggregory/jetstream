package jetstream

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/nats-io/nats.go"
	. "github.com/smartystreets/goconvey/convey"
)

const (
	PsStreamName  = "PsTestStream"
	PsSubject     = "TestSubject"
	PsFullSubject = PsStreamName + "." + PsSubject
	PsDurableName = "PsTestJS"
)

func TestSubjectMatchesStreamConfig(t *testing.T) {
	jscfg := &Config{
		Ctx:         context.Background(),
		Name:        PsDurableName,
		ClusterURLs: nats.DefaultURL,
		StreamConfig: &nats.StreamConfig{
			Name:      PsStreamName,
			Retention: nats.InterestPolicy, // msg removed after subscriber ACKs
			MaxAge:    10 * time.Minute,
			Subjects:  []string{PsStreamName + ".*"},
		},
	}

	Convey("Should match subject", t, func() {
		So(SubjectMatchesStreamConfig(jscfg, PsFullSubject), ShouldBeTrue)
	})
	Convey("Should not match subject", t, func() {
		So(SubjectMatchesStreamConfig(jscfg, PsSubject), ShouldBeFalse)

		jscfg.StreamConfig.Subjects = []string{}
		So(SubjectMatchesStreamConfig(jscfg, PsFullSubject), ShouldBeFalse)
	})
	Convey("Should not be implemented", t, func() {
		_, err := NewJetStreamSubscriber(jscfg, PsFullSubject, "", nil)
		_, ok := err.(ErrNotImplemented)
		So(ok, ShouldBeTrue)
	})
}

func TestPubSubLive(t *testing.T) {
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
			Subjects:  []string{PsStreamName + ".*"},
		},
	}

	queueGroup := "queueGroup"

	Convey("Should pub sub", t, func() {
		Convey("with consumer group using chan", func() {
			pub, err := NewJetStreamPublisher(jscfg, PsFullSubject)
			So(err, ShouldBeNil)
			defer pub.Close()
			_ = pub.GetJetStreamContext().PurgeStream(PsStreamName)

			msg := "msg1"
			ackChan := make(chan *PubAck)
			err = pub.Publish([]byte(msg), ackChan)
			So(err, ShouldBeNil)
			select {
			case ack := <-ackChan:
				So(ack.Err, ShouldBeNil)
			}

			mc := make(chan *nats.Msg)
			sub, err := newStreamConsumer(jscfg, PsFullSubject, queueGroup, mc)
			So(err, ShouldBeNil)
			defer sub.Close()
			select {
			case m := <-mc:
				So(string(m.Data), ShouldEqual, msg)
			case t := <-time.After(5 * time.Second):
				So(t, ShouldBeNil)
				So(false, ShouldEqual, true)
			}
			Convey("next same subscriber should have nothing to read", func() {
				sub.Close()
				mc := make(chan *nats.Msg)
				sub, err = newStreamConsumer(jscfg, PsFullSubject, queueGroup, mc)
				So(err, ShouldBeNil)
				defer sub.Close()
				select {
				case m := <-mc:
					So(m, ShouldBeNil) // should have nothing to read
				case <-time.After(1 * time.Second):
				}
			})
		})
		Convey("with consumer group using CB version", func() {
			pub, err := NewJetStreamPublisher(jscfg, PsFullSubject)
			So(err, ShouldBeNil)
			defer pub.Close()
			_ = pub.GetJetStreamContext().PurgeStream(PsStreamName)

			msg := "msg1"
			err = pub.Publish([]byte(msg), func(msg *nats.Msg, err error) {
				if err != nil {
					t.Fatal(err)
				}
				logrus.Debug("pub cb success")
			})
			So(err, ShouldBeNil)

			mh := newMsgHandler(false)
			sub, err := newStreamConsumer(jscfg, PsFullSubject, queueGroup, mh.msgHandler)
			So(err, ShouldBeNil)
			defer sub.Close()
			So(mh.waitOnMsg(msg, 1, time.Second), ShouldBeNil)
		})
		Convey("with diff durable name to read what 1st did", func() {
			mh := newMsgHandler(false)
			cfg := *jscfg
			cfg.Name += "_2"
			sub, err := newStreamConsumer(&cfg, PsFullSubject, "psGroup2", mh.msgHandler)
			So(err, ShouldBeNil)
			defer sub.Close()
			So(mh.waitOnMsg("msg1", 1, time.Second), ShouldBeNil)
		})
		Convey("with pull subscription", func() {
			pub, err := NewJetStreamPublisher(jscfg, PsFullSubject)
			So(err, ShouldBeNil)
			defer pub.Close()
			_ = pub.GetJetStreamContext().PurgeStream(PsStreamName)

			pubAck := func(msg *nats.Msg, err error) {
				if err != nil {
					t.Fatal(err)
				}
				logrus.Debug("pub cb success")
			}
			msg := "fetch-msg1"
			err = pub.Publish([]byte(msg), pubAck)
			So(err, ShouldBeNil)

			// needs to get a new consumer created for a pull. Would fail if the durable name was on an already created push-based consumer - which has a `Delivery Subject` from `nats con info`.
			cfg := *jscfg
			cfg.Name += "_pull"
			sub, err := NewPullSubscriber(&cfg, PsFullSubject)
			So(err, ShouldBeNil)
			defer sub.Close()
			mh := newFetchMsgHandler(sub)
			So(mh.waitOnMsg(msg, 1, time.Second), ShouldBeNil)

			// nothing else to read
			_, err = sub.Fetch(1, 250*time.Millisecond)
			_, ok := err.(ErrTimeout)
			So(ok, ShouldBeTrue)

			// one more publish
			msg = "fetch-msg2"
			err = pub.Publish([]byte(msg), pubAck)
			So(err, ShouldBeNil)

			// should consume only one but not timeout when asked for more than 1
			msgs, err := sub.Fetch(2, time.Second)
			_, ok = err.(ErrTimeout)
			So(ok, ShouldBeFalse)
			So(len(msgs), ShouldEqual, 1)
		})
	})
	Convey("Hammer", t, func(c C) {
		logrus.SetLevel(logrus.InfoLevel)

		pub, err := NewJetStreamPublisher(jscfg, PsFullSubject)
		So(err, ShouldBeNil)
		defer pub.Close()
		_ = pub.GetJetStreamContext().PurgeStream(PsStreamName)

		maxMsgs := 1000

		type subResults struct {
			nMsgs   int
			lastMsg *nats.Msg
			err     error
		}
		subDone := make(chan subResults)
		go func(c C) {
			sr := subResults{}
			mc := make(chan *nats.Msg)
			sub, err := newStreamConsumer(jscfg, PsFullSubject, "psGroup", mc)
			c.So(err, ShouldBeNil)
			defer sub.Close()
		loop:
			for {
				select {
				case m := <-mc:
					sr.nMsgs++
					sr.lastMsg = m
					if sr.nMsgs == maxMsgs {
						break loop
					}
				case <-time.After(5 * time.Second):
					sr.err = fmt.Errorf("msg select timed out")
					break loop
				}
			}
			subDone <- sr
		}(c)

		go func(c C) {
			ackChan := make(chan *PubAck)
			for i := 0; i < maxMsgs; i++ {
				err = pub.Publish([]byte(fmt.Sprintf("%d", i+1)), ackChan)
				c.So(err, ShouldBeNil)
				select {
				case ack := <-ackChan:
					c.So(ack.Err, ShouldBeNil)
				}
			}
		}(c)

		sr := <-subDone
		So(sr.err, ShouldBeNil)
		So(sr.nMsgs, ShouldEqual, maxMsgs)
	})
}
