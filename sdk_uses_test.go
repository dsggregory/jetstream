// Alternative uses of jetstream that may not include what is done in our pkg. Here to understand their SDK.
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
	StreamName  = "TestStream"
	Subject     = StreamName + ".TestSubject"
	DurableName = "testJS"
)

type MsgHandler struct {
	sub        PullSubIface
	newMsgChan chan *nats.Msg
	ack        bool
	msgs       []*nats.Msg
}

func newMsgHandler(ack bool) *MsgHandler {
	return &MsgHandler{
		newMsgChan: make(chan *nats.Msg),
		ack:        false,
		msgs:       nil,
	}
}
func newFetchMsgHandler(sub PullSubIface) *MsgHandler {
	return &MsgHandler{sub: sub, msgs: nil}
}

func (mh *MsgHandler) msgHandler(msg *nats.Msg) {
	mh.msgs = append(mh.msgs, msg)
	mh.newMsgChan <- msg
}
func (mh *MsgHandler) waitOnMsg(msg string, count int, d time.Duration) error {
	if mh.newMsgChan != nil {
		select {
		case m := <-mh.newMsgChan:
			if len(mh.msgs) != count {
				return fmt.Errorf("count mismatch, expected(%d) != count(%d)", count, len(mh.msgs))
			}
			if string(m.Data) != msg {
				return fmt.Errorf("value mismatch, expected(%s) != value(%s)", msg, string(m.Data))
			}
			return nil
		case <-time.After(d):
			return fmt.Errorf("msg receive timed-out")
		}
	}

	// pull-based subscriber
	msgs, err := mh.sub.Fetch(count, d)
	if err != nil {
		return err
	}
	if len(msgs) != count {
		return fmt.Errorf("count mismatch, expected(%d) != count(%d)", count, len(msgs))
	}
	found := false
	for _, m := range msgs {
		if string(m.Data) == msg {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("msg not found, expected(%s)", msg)
	}
	return nil
}

func TestLiveSubscriptionAlternatives(t *testing.T) {
	server := os.Getenv("NATS_URL")
	if server == "" {
		t.Skip("Requires live NATS 2.0 server. See README for docker details.")
	}

	logrus.SetLevel(logrus.TraceLevel)

	js, err := NewJetStream(&Config{
		Ctx:         context.Background(),
		Name:        DurableName,
		ClusterURLs: nats.DefaultURL,
		StreamConfig: &nats.StreamConfig{
			Name:      StreamName,
			Retention: nats.InterestPolicy, // msg removed after subscriber ACKs
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = js.GetJetStreamContext().PurgeStream(StreamName)
	jpub := &Publisher{js: js, publishSubject: Subject}

	defer js.Close()

	Convey("Subscription types", t, func() {
		// called after each Convey at this level and at final return
		defer func() {
			_ = js.GetJetStreamContext().PurgeStream(StreamName)
		}()
		So(err, ShouldBeNil)
		Convey("With durable consumer and subscription no-ack", func() {
			msg := "dur#1"
			err = jpub.Publish([]byte(msg), func(msg *nats.Msg, err error) {
				if err != nil {
					t.Fatal(err)
				}
			})
			So(err, ShouldBeNil)

			// create/reuse a durable consumer
			_, err = js.GetJetStreamContext().AddConsumer(StreamName, &nats.ConsumerConfig{
				// consumer's durable name
				Durable: DurableName,
				// we won't ACK, so the msg will no longer be available to this durable consumer
				AckPolicy: nats.AckNonePolicy,
				// this acts like a durability name and without it you get the same messages each time. Regardless if the consumer specifies `Durable` or the Subscription specifies nats.Durable().
				DeliverSubject: nats.InboxPrefix + DurableName,
			})

			mh := newMsgHandler(false)
			_, err := js.GetJetStreamContext().Subscribe(Subject, mh.msgHandler,
				// without this, the stream cannot be determined
				nats.BindStream(StreamName),
				// the name of the consumer which will pick up where it left off after a restart
				nats.Durable(DurableName),
				// automatic ack on rec'd msgs
				nats.AckNone(),
			)
			So(err, ShouldBeNil)
			So(mh.waitOnMsg(msg, 1, time.Second), ShouldBeNil)

			// publish 2nd msg
			msg = msg + ".2"
			err = jpub.Publish([]byte(msg), nil)
			So(err, ShouldBeNil)
			// wait on msgh
			So(mh.waitOnMsg(msg, 2, time.Second), ShouldBeNil)

			Convey("2nd consumer new conn should not consume what 1st did", func() {
				msg = "2nd consumer msg"
				j2, err := NewJetStream(&Config{
					Ctx:         context.Background(),
					Name:        DurableName,
					ClusterURLs: nats.DefaultURL,
					StreamConfig: &nats.StreamConfig{
						Name:      StreamName,
						Retention: nats.InterestPolicy, // msg removed after subscriber ACKs
					},
				})
				if err != nil {
					t.Fatal(err)
				}

				mh2 := newMsgHandler(false)
				_, err = j2.GetJetStreamContext().Subscribe(Subject, mh2.msgHandler,
					// without this, the stream cannot be determined
					nats.BindStream(StreamName),
					// the name of the consumer which will pick up where it left off after a restart
					nats.Durable(DurableName),
					// automatic ack on rec'd msgs
					nats.AckNone(),
				)
				So(err, ShouldBeNil)
				So(mh2.waitOnMsg(msg, 0, time.Second), ShouldNotBeNil)
			})
		})
		Convey("With orig jetstream, new non-durable subscriber, no-ack", func() {
			msg := "dur#2"
			err = jpub.Publish([]byte(msg), nil)
			So(err, ShouldBeNil)

			mh := newMsgHandler(false)
			_, err := js.GetJetStreamContext().Subscribe(Subject, mh.msgHandler,
				// without this, the stream cannot be determined
				nats.BindStream(StreamName),
				// the name of the consumer which will pick up where it left off after a restart
				//nats.Durable(DurableName),
				// automatic ack on rec'd msgs
				nats.AckNone(),
			)
			So(err, ShouldBeNil)
			So(mh.waitOnMsg(msg, 1, time.Second), ShouldBeNil)
		})

		Convey("two consumers same durable no-ack", func() {
			/*
									// WARN: I don't know what's going on here, but without nats.Durable() in SubOpts we get the same message
				// over and over in the msgHandler even though we only sent one message.
				So(len(mh.msgs), ShouldBeGreaterThan, 1)
				for _, m := range mh.msgs {
					So(string(m.Data), ShouldEqual, msg)
				}

			*/
		})
	})
}
