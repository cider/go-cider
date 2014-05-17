// Copyright (c) 2013 The go-meeko AUTHORS
//
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package pubsub

import (
	"bytes"
	"encoding/binary"

	"github.com/meeko/meekod/broker/services/pubsub"
	"github.com/meeko/go-meeko/meeko/utils/codecs"
)

// Event implements pubsub.Event, which is the interface required by the exchange.
type Event struct {
	kind      []byte
	seq       []byte
	publisher []byte
	body      []byte
}

func newEvent(identity string, eventKind string, eventObject interface{}) (pubsub.Event, error) {
	var buf bytes.Buffer
	if err := codecs.MessagePack.Encode(&buf, eventObject); err != nil {
		return nil, err
	}

	return &Event{
		kind:      []byte(eventKind),
		publisher: []byte(identity),
		body:      buf.Bytes(),
	}, nil
}

func (event *Event) Publisher() []byte {
	return event.publisher
}

func (event *Event) Kind() []byte {
	return event.kind
}

func (event *Event) Seq() []byte {
	if event.seq == nil {
		panic("Event.Seq called before Event.SetSeq")
	}
	return event.seq
}

func (event *Event) SetSeq(seq pubsub.EventSeqNum) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, seq); err != nil {
		panic(err)
	}

	event.seq = buf.Bytes()
}

func (event *Event) Body() []byte {
	return event.body
}
