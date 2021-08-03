# NATS JetStream Convenience Library

NATS v2.0 (e.g. JetStream) is a replacement for the previous nats-streaming called "STAN". The golang package `github.com/nats-io/nats.go@1.11.1` supports version 2.0 of NATS.

This package provides convenience methods to create JetStream producers and consumers.

## Glossary
* __Stream__ - a name given to collect messages. Ex: `nats str <ls | info>`
* __Subject__ - a name within a `Stream` used to segment messages. Referred by NATS as the `Filter Subject`. In other techs this may be called a Queue or Topic.
* __Consumer__ - a persistent object tied to a subscription used to define durability and other policies. Ex: `nats con info <stream>`. A consumer is a construct that allows for a subscription to be pre-allocated so that producers can produce to a subject on a stream before a subscriber to that subject comes online.
* __Subscription__ - a connection to a stream used to read messages. Used in conjunction with an explicit or implicitly-named consumer.
* __Durable Subscription__ - this is a subscription that is persisted. The app can restart to pick up where it left off. `Config.Name` specifies the durable name to be used.
* __Queue Name__ - *unsure of this relationship when passed to Subscribe()*. NMI.

## Usage
### Full Example
See [example](./examples/pkg/main.go) to create a producer and consumer.

### StreamConfig
The StreamConfig struct passed along with the package's Config specify how to create a new stream if it doesn't already exist.

#### Subjects
Be careful with your selection of stream subject(s). In the Config you specify a nats.StreamConfig. Typically, you would specify these config Subjects as "mystream.*" and when publishing and subscribing to a stream you specify something like `mystream.mysubj`. It is REQUIRED that what the stream has configured for Subjects match what is specified when publishing or consuming. Without the checks we impose on subjects, if you got this wrong, the publish will fail to ACK and return a timeout or you'll get a callback error with:
```text
nats: no responders available for request
```
That is the reason for imposing the subject to be specified when creating the publisher and Publish() does not take a subject param.

#### Pull vs. Push Based consumers
Durable consumers are created by calls to NewJetStreamSubscriber() and NewPullSubscriber() based on Config.Name setting the name of the consumer. You may use `nats con info` to describe a consumer. A consumer with a `Delivery Subject` is a push-based consumer and CANNOT be used with NewPullSubscriber().

#### Retention Policy
For full details see [JetStream Retention Policy](https://github.com/nats-io/jetstream#stream-limits-retention-modes-and-discard-policy). The default is `InterestPolicy` meaning messages are retained only while a consumer is defined. This means that a consumer MUST be created before any messages are published. Messages produced on a subject are silently dropped unless an active subscriber is connected on the subject or a consumer has been created.

#### MaxAge
This stream config setting specifies how long a message is available in a stream for consumers. The larger the value, the more disk space required to persist messages until they expire. Expiration handling may lag and negatively impact cluster startup times when storage requirements are large (maxAge * numMessages).

This setting must be carefully considered and should be set to the smallest time possible for your use case.

### Quick Example

```go
package main

import (
	"fmt"
	"log"
	"time"
)

const Subject = "MyStreamName.MySubject"

func main() {
	jscfg := &Config{
		Ctx:         context.Background(),
		Name:        "MyDurableName",
		ClusterURLs: nats.DefaultURL,
		StreamConfig: &nats.StreamConfig{
			Name:      "MyStreamName",
			Retention: nats.InterestPolicy, // msg removed after subscriber ACKs
			Subjects:  []string{"MyStreamName.*"},
			MaxAge:    24 * time.Hour,
		},
	}

	// publish a message
	pub, err := NewJetStreamPublisher(jscfg, Subject)
	defer pub.Close()

	msg := "msg1"
	ackChan := make(chan *PubAck)
	err = pub.Publish([]byte(msg), ackChan)
	// wait on publish ack
	select {
	case ack := <-ackChan:
		So(ack.Err, ShouldBeNil)
	}

	// consume the message that was published
	mc := make(chan *nats.Msg)
	sub, err := NewPullSubscriber(jscfg, Subject)
	So(err, ShouldBeNil)
	defer sub.Close()
	// example to consume forever
	for {
		msgs, err := sub.Fetch(1, time.Second)
		So(err, ShouldBeNil)
		fmt.Println(string(msg.Data))
	}

}
```

## Troubleshooting
### Docker
See https://hub.docker.com/_/nats. DO NOT use the synadia/jsm:latest image as it is quite a bit behind and has errors. The version successfully tested was nats:2.3.2-alpine.
```shell
$ docker volume create jetstream-storage
$ docker run -d -ti -p 4222:4222 --name jetstream --mount source=jetstream-storage,target=/tmp/jsd nats -js -sd /tmp/jsd
```

Run the tests against a live nats server (a docker instance for example):
```shell
$ NATS_URL=nats://localhost:4222 go test -v ./...
```

### Nats Command-line
The nats tools are helpful when troubleshooting.
```shell
$ brew tap nats-io/nats-tools
$ brew install nats-io/nats-tools/nats
```

Useful commands:
* nats str ls
* nats str info <stream-name>
* nats str purge <stream-name> -f
* nats str rm <stream-name>
* nats con info <stream-name> <consumer-name>
* nats con sub <stream-name> <consumer-name>
* nats pub <stream-name> <msg>
* nats str rmm <stream-name> <msg-num> -f   # remove a message
* nats --server=domains-nats-0.domains-nats.domains.svc.cluster.local:4222 account info

## References
Beware of legacy docs relating to the early access period.
* https://docs.nats.io/jetstream/jetstream
* https://github.com/nats-io/nats.go#jetstream-basic-usage
* https://shijuvar.medium.com/building-distributed-event-streaming-systems-in-go-with-nats-jetstream-3938e6dc7a13
* https://github.com/nats-io/jetstream#readme
* https://github.com/nats-io/natscli
* [NATS Slack Group](https://natsio.slack.com/?redir=%2Fmessages%2FDBET737GV)
