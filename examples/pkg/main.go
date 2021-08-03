// --+build notforlib

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"bitbucket.phishlabs.com/golib/jetstream"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

const (
	StreamName = "Test"
	Subject    = "Test.test"
)

type Options struct {
	pub     bool
	pull    bool
	server  string
	stream  string
	subject string
	queue   string
	ttl     time.Duration
	count   int
	quiet   bool
}

func main() {
	o := Options{}

	flag.BoolVar(&o.pub, "p", false, "publish a random message, otherwise poll-subscribe")
	flag.BoolVar(&o.pull, "f", false, "pull-subscribe, default is poll-subscribe")
	flag.BoolVar(&o.quiet, "q", false, "be quiet")
	flag.StringVar(&o.server, "server", nats.DefaultURL, "NATS cluster comma-sep URLs, or single node URL")
	flag.StringVar(&o.stream, "stream", StreamName, "Stream to use")
	flag.StringVar(&o.subject, "subject", Subject, "Subject to subscribe")
	flag.StringVar(&o.queue, "qname", "", "QueueName to subscribe")
	flag.DurationVar(&o.ttl, "ttl", 24*7*time.Hour, "MaxAge of stream messages when stream is created")
	flag.IntVar(&o.count, "c", 1, "Number of messages to read or write")

	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Publish random messages or subscribe to a JetStream stream.")
		flag.PrintDefaults()
	}
	flag.Parse()

	//args := flag.Args()

	cfg := jetstream.Config{
		Ctx:         context.Background(),
		Name:        "test",
		ClusterURLs: o.server,
		StreamConfig: &nats.StreamConfig{
			Name:      o.stream,
			Retention: nats.InterestPolicy, // msg removed after subscriber ACKs
			Subjects:  []string{o.stream + ".*"},
			MaxAge:    o.ttl,
		},
	}

	if o.pub {
		p, err := jetstream.NewJetStreamPublisher(&cfg, o.subject)
		if err != nil {
			logrus.WithError(err).Fatal("cannot init publisher")
		}
		c := make(chan error)
		for i := 0; i < o.count; i++ {
			mstr := fmt.Sprintf("%s - message #%d", time.Now().String(), i)
			err = p.Publish([]byte(mstr), func(msg *nats.Msg, err error) {
				if !o.quiet {
					logrus.WithFields(logrus.Fields{
						"msg": string(msg.Data),
						"err": err,
					}).Info("published message")
				}
				c <- err
			})
			if err != nil {
				logrus.WithError(err).Fatal("publish error")
			}
			select {
			case err := <-c:
				if err != nil {
					logrus.WithError(err).Error("PublishAsyncCB failed")
				} else if !o.quiet {
					logrus.Info("PublishAsyncCB success")
				}
			case <-time.After(10 * time.Second):
				logrus.Error("main: timed out waiting on PublishAsyncCB")
			}
		}
		p.Close()
	} else if o.pull {
		logrus.Info("New pull-based consumer")
		cfg.Name += "_pull" // specific durable name to create a new consumer
		s, err := jetstream.NewPullSubscriber(&cfg, o.subject)
		if err != nil {
			logrus.WithError(err).Fatal("subscribe error")
		}
		fmt.Printf("Waiting on msgs. Press ^C to interrupt\n")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		n := 0
	pullloop:
		for n < o.count {
			select {
			case <-c:
				break pullloop
			default:
			}
			msgs, err := s.Fetch(5, time.Second)
			if err != nil {
				logrus.WithError(err).Error("fetch error")
				break
			}
			n += len(msgs)
			if !o.quiet {
				for _, msg := range msgs {
					logrus.WithField("msg", string(msg.Data)).Info("got a msg")
				}
			}
		}

		logrus.WithField("count", n).Info("finished consuming")
		s.Close()
	} else {
		logrus.Info("New push-based consumer")
		mc := make(chan *nats.Msg)
		s, err := jetstream.NewJetStreamSubscriber(&cfg, o.subject, o.queue, mc)
		if err != nil {
			logrus.WithError(err).Fatal("subscribe error")
		}
		fmt.Printf("Waiting on msgs. Press ^C to interrupt\n")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		n := 0
	loop:
		for n = 0; n < o.count; n++ {
			select {
			case msg := <-mc:
				if !o.quiet {
					logrus.WithField("msg", string(msg.Data)).Info("got a msg")
				}
			case <-c:
				break loop
			}
		}
		logrus.WithField("count", n).Info("finished consuming")
		s.Close()
	}
}
