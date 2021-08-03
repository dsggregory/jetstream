package jetstream

import (
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// JetStreamIface an abstraction for a JetStream
type JetStreamIface interface {
	GetJetStreamContext() nats.JetStreamContext
	Close()
}

// JetStream a nats jetstream connection as an implementation of JetStreamIface
type JetStream struct {
	config    *Config
	nc        *nats.Conn
	jsContext nats.JetStreamContext
}

// Close closes the nats connection
func (js *JetStream) Close() {
	if err := js.nc.FlushTimeout(5 * time.Second); err != nil {
		logrus.WithError(err).Warn("error while flushing jetstream during close operation")
	}
	js.nc.Close()
}

// GetJetStreamContext returns the JetStreamContext to be used any way you see fit
func (js *JetStream) GetJetStreamContext() nats.JetStreamContext {
	return js.jsContext
}

// Support round-robin hosts of a single href.
// If a single URL (no commas), look up the host and format the cluster URL list using the response A records names.
// This is to get all hosts in a cluster backed by one CNAME.
func expandClusterURL(href string) string {
	l := logrus.WithField("state", "jetstream.expandClusterURL")

	if strings.Contains(href, ",") {
		return href // already a cluster list
	}

	u, err := url.Parse(href)
	if err != nil {
		l.WithError(err).Warn("unable to parse cluster href. Using as-is.")
		return href
	}
	addrs, err := net.LookupHost(u.Hostname())
	if err != nil {
		l.WithField("host", u.Hostname()).WithError(err).Warn("unable to lookup host. Using as-is.")
		return href
	}
	if len(addrs) == 0 {
		l.WithField("host", u.Hostname()).Warn("no addrs found for host. Using as-is.")
		return href
	}

	// WARN: net.LookupHost() may return aliases and IPv6 versions of the same host. No easy way to canonicalize.
	cl := ""
	for i, nm := range addrs {
		if i > 0 {
			cl += ","
		}
		cl += u.Scheme + "://"
		if u.User != nil {
			cl += u.User.String()
			cl += "@"
		}
		cl += nm
		if u.Port() != "" {
			cl += ":" + u.Port()
		}
		if u.RawQuery != "" {
			cl += "?" + u.RawQuery
		}
	}

	logrus.WithFields(logrus.Fields{
		"url":      href,
		"expanded": cl,
	}).Info("expanded JetStream cluster URL")

	return cl
}

// NewJetStream create a connection to a nats `cluster` (comma-sep list of cluster URLs) and hook up JetStream. Creates the stream if necessary.
// Callers may wish to use the other convenience funcs from this package to instantiate producers and consumers. This method is available when you wish to interact with jetstream in other ways not exposed by this package.
func NewJetStream(cfg *Config) (JetStreamIface, error) {
	urls := expandClusterURL(cfg.ClusterURLs)
	nc, err := nats.Connect(urls,
		nats.Name(cfg.Name),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(36),
		nats.ReconnectWait(5*time.Second),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logrus.WithField("server", c.ConnectedAddr()).Warn("reconnected after lost connection to nats")
		}))
	if err != nil {
		logrus.WithField("cluster", urls).WithError(err).Error("initially failed to connect to nats, will retry asynchronously")
	}

	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	j := &JetStream{
		config:    cfg,
		nc:        nc,
		jsContext: js,
	}

	if err := j.createStream(); err != nil {
		nc.Close()
		return nil, err
	}

	return j, nil
}

// createStream creates a stream by using JetStreamContext
func (js *JetStream) createStream() error {
	if js.config.StreamConfig == nil {
		return ErrBadConfig{str: "StreamConfig is required"}
	}
	if js.config.StreamConfig.Name == "" {
		return ErrBadConfig{str: "StreamConfig.Name is required"}
	}
	subjs := js.config.StreamConfig.Subjects
	if subjs == nil {
		// Default subjects. ex. STREAM.*
		subjs = []string{js.config.StreamConfig.Name + ".*"}
		js.config.StreamConfig.Subjects = subjs
	}

	// Check if the stream already exists; if not, create it.
	var stream *nats.StreamInfo
	var err error
	stream, err = js.jsContext.StreamInfo(js.config.StreamConfig.Name)
	if err != nil {
		logrus.WithField("stream", js.config.StreamConfig.Name).WithError(err).Warn("stream may not exist")
	}
	if stream == nil {
		logrus.Infof("creating stream %q and subjects %q", js.config.StreamConfig.Name, subjs)
		stream, err = js.jsContext.AddStream(js.config.StreamConfig)
		if err != nil {
			return err
		}
	}
	if logrus.GetLevel() == logrus.DebugLevel {
		logrus.WithFields(logrus.Fields{
			"name":    stream.Config.Name,
			"storage": stream.Config.Storage.String(),
		}).Debug("connected to JetStream stream")
	}
	return nil
}
