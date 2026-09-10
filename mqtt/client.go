// Package mqtt is a thin wrapper around paho.mqtt.golang that adds
// automatic resubscription after a reconnect and a simplified,
// string-based publish/subscribe API.
package mqtt

import (
	"crypto/tls"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// MessageHandler is called for every message received on a subscribed
// topic, with the exact topic the message arrived on (useful when
// subscribing with a wildcard) and its payload as a string.
type MessageHandler func(topic string, payload string)

// Options configures a new Client.
type Options struct {
	BrokerURL string
	ClientID  string
	Username  string
	Password  string
}

type subscription struct {
	topic   string
	qos     byte
	handler MessageHandler
}

// Client wraps a paho MQTT client. Subscriptions registered with
// Subscribe are automatically renewed whenever the client reconnects.
type Client struct {
	paho paho.Client

	mu              sync.Mutex
	subscriptions   []subscription
	connectedBefore bool
}

// connectAttemptTimeout bounds how long a single connection attempt is
// given before it's treated as failed and retried. Kept well under
// connectRetryInterval so a hung attempt doesn't stall the next retry.
const connectAttemptTimeout = 10 * time.Second

// connectRetryInterval is how long Connect waits between failed initial
// connection attempts.
const connectRetryInterval = 5 * time.Second

// Connect creates a new MQTT client and connects it, retrying - with a
// log line on every attempt - until it succeeds. This is deliberately
// its own loop rather than paho's built-in ConnectRetry: that option
// retries silently (nothing is logged unless paho's own debug/error
// loggers are wired up), so a broker that's slow to come up or
// unreachable at a misconfigured address looks identical to the
// process having hung.
func Connect(opts Options) (*Client, error) {
	c := &Client{}

	clientOpts := paho.NewClientOptions().
		AddBroker(opts.BrokerURL).
		SetClientID(opts.ClientID).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(_ paho.Client) {
			log.Printf("mqtt: connected to %s", opts.BrokerURL)
			c.onConnect()
		}).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			log.Printf("mqtt: connection lost: %v", err)
		})

	if opts.Username != "" {
		clientOpts.SetUsername(opts.Username)
	}
	if opts.Password != "" {
		clientOpts.SetPassword(opts.Password)
	}
	if strings.HasPrefix(opts.BrokerURL, "ssl://") || strings.HasPrefix(opts.BrokerURL, "tls://") {
		clientOpts.SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12})
	}

	c.paho = paho.NewClient(clientOpts)

	for attempt := 1; ; attempt++ {
		log.Printf("mqtt: connecting to %s (attempt %d)...", opts.BrokerURL, attempt)

		token := c.paho.Connect()
		var err error
		if !token.WaitTimeout(connectAttemptTimeout) {
			err = fmt.Errorf("timed out after %s", connectAttemptTimeout)
		} else {
			err = token.Error()
		}

		if err == nil {
			return c, nil
		}
		log.Printf("mqtt: connect attempt %d to %s failed: %v; retrying in %s", attempt, opts.BrokerURL, err, connectRetryInterval)
		time.Sleep(connectRetryInterval)
	}
}

// Subscribe subscribes to topic (which may include MQTT wildcards, e.g.
// "cmd/m33/#") and calls handler for every message received. The
// subscription is remembered and automatically re-established if the
// client reconnects.
func (c *Client) Subscribe(topic string, qos byte, handler MessageHandler) error {
	s := subscription{topic: topic, qos: qos, handler: handler}

	c.mu.Lock()
	c.subscriptions = append(c.subscriptions, s)
	c.mu.Unlock()

	return c.subscribeOne(s)
}

func (c *Client) subscribeOne(s subscription) error {
	callback := func(_ paho.Client, msg paho.Message) {
		s.handler(msg.Topic(), string(msg.Payload()))
	}

	token := c.paho.Subscribe(s.topic, s.qos, callback)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt: subscribe to %s: %w", s.topic, token.Error())
	}
	log.Printf("mqtt: subscribed to %s", s.topic)
	return nil
}

// onConnect fires on every successful (re)connect. The very first
// connect is a no-op here: the caller establishes its subscriptions
// itself via Subscribe once Connect returns, and resubscribing here too
// would race with that and double-subscribe. Only actual reconnects -
// where Subscribe's direct call from before is long done - need their
// subscriptions renewed.
func (c *Client) onConnect() {
	c.mu.Lock()
	first := !c.connectedBefore
	c.connectedBefore = true
	subs := make([]subscription, len(c.subscriptions))
	copy(subs, c.subscriptions)
	c.mu.Unlock()

	if first {
		return
	}

	for _, s := range subs {
		if err := c.subscribeOne(s); err != nil {
			log.Printf("mqtt: resubscribe to %s failed: %v", s.topic, err)
		}
	}
}

// Publish publishes payload to topic.
func (c *Client) Publish(topic string, qos byte, retained bool, payload string) error {
	token := c.paho.Publish(topic, qos, retained, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt: publish to %s: %w", topic, token.Error())
	}
	return nil
}

// Disconnect cleanly disconnects from the broker, waiting up to 250ms
// for in-flight work to finish.
func (c *Client) Disconnect() {
	c.paho.Disconnect(250)
}
