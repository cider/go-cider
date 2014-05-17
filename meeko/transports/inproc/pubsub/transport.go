// Copyright (c) 2013 The go-meeko AUTHORS
//
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package pubsub

import (
	// Meeko
	"github.com/meeko/meekod/broker"
	"github.com/meeko/meekod/broker/services/pubsub"
	client "github.com/meeko/go-meeko/meeko/services/pubsub"

	// Other
	"github.com/dmotylev/nutrition"
)

// Transport implements client.Transport so it can be used as the underlying
// transport for a Meeko client instance.
//
// This particular transport is a bit special. It implements inproc transport
// that uses channels. It is suitable for clients that intend to live in the
// same process as Meeko.
//
// For using Transport as a service Endpoint for the Meeko broker, check the
// AsEndpoint method.
type Transport struct {
	exchange pubsub.Exchange
	identity string

	// endpoint makes Transport look like pubsub.Endpoint
	// It just delegates calls to Transport.
	endpoint *endpointAdapter

	// Channel for receiving publish requests from the exchange.
	// This channel is returned by EventChan interface method, so actually
	// Transport is not using the data running through this channel at all,
	// it is being processes by the pubsub client using this Transport.
	eventCh chan pubsub.Event

	// Termination management
	closeCh chan struct{}
}

// AsTransport returns an adapter that makes Transport look like pubsub.Endpoint.
func (t *Transport) AsEndpoint() pubsub.Endpoint {
	return t.endpoint
}

// TransportConfig serves as a factory for Transport. Once its fields are set
// to requested values, NewTransport method can be used to construct a Transport
// for the chosen configuration.
//
// FeedFromEnv can be used to load config from environmental variables.
type TransportConfig struct {
	ChannelCapacity uint
}

// NewTransportConfig creates a new TransportConfig with some reasonable defaults.
func NewTransportConfig() *TransportConfig {
	return &TransportConfig{
		ChannelCapacity: 1000,
	}
}

// FeedFromEnv loads config from the environment.
//
// It checks variables looking like prefix + capitalizedFieldName.
func (config *TransportConfig) FeedFromEnv(prefix string) error {
	return nutrition.Env(prefix).Feed(config)
}

func (config *TransportConfig) MustFeedFromEnv(prefix string) *TransportConfig {
	if err := config.FeedFromEnv(prefix); err != nil {
		panic(err)
	}
	return config
}

// NewTransport creates a new Transport instance for the given configuration.
func (config *TransportConfig) NewTransport(identity string, exchange pubsub.Exchange) *Transport {
	t := &Transport{
		exchange: exchange,
		identity: identity,
		// Publish call from the exchange must try to avoid blocking as much as
		// possible since blocking here means that the whole exchange is blocked.
		eventCh: make(chan pubsub.Event, config.ChannelCapacity),
		closeCh: make(chan struct{}),
	}

	t.endpoint = &endpointAdapter{t}
	exchange.RegisterEndpoint(t.endpoint)

	return t
}

// client.Transport interface --------------------------------------------------

func (t *Transport) Publish(eventKind string, eventObject interface{}) error {
	event, err := newEvent(t.identity, eventKind, eventObject)
	if err != nil {
		return err
	}

	t.exchange.Publish(event)
	return nil
}

func (t *Transport) Subscribe(eventKindPrefix string) error {
	return nil
}

func (t *Transport) Unsubscribe(eventKindPrefix string) error {
	return nil
}

func (t *Transport) EventChan() <-chan pubsub.Event {
	return t.eventCh
}

func (t *Transport) EventSeqTableChan() <-chan client.EventSeqTable {
	return nil
}

func (t *Transport) ErrorChan() <-chan error {
	return nil
}

func (t *Transport) Close() error {
	select {
	case <-t.closeCh:
	default:
		t.exchange.UnregisterEndpoint(t.endpoint)
		close(t.closeCh)
	}
	return nil
}

func (t *Transport) Closed() <-chan struct{} {
	return t.closeCh
}

func (t *Transport) Wait() error {
	<-t.closeCh
	return nil
}

// pubsub.Endpoint adapter ----------------------------------------------------

type endpointAdapter struct {
	t *Transport
}

func (adapter *endpointAdapter) Publish(event pubsub.Event) error {
	select {
	case adapter.t.eventCh <- event:
	case <-adapter.t.Closed():
		return &broker.ErrTerminated{"inproc endpoint"}
	}

	return nil
}

func (adapter *endpointAdapter) ListenAndServe() error {
	return adapter.t.Wait()
}

func (adapter *endpointAdapter) Close() error {
	return adapter.t.Close()
}
