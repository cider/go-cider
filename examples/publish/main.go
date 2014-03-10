package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"

	"github.com/cider/go-cider/cider/services/pubsub"
	zpubsub "github.com/cider/go-cider/cider/transports/zmq3/pubsub"

	"github.com/joho/godotenv"
	zmq "github.com/pebbe/zmq3"
)

func main() {
	log.SetFlags(0)

	var exitError error
	defer func() {
		// Make sure all messages are sent before exiting.
		log.Println("Waiting for the transport to dispatch queued messages...")
		zmq.Term()
		if exitError != nil {
			log.Fatalln(exitError)
		}
	}()

	// Check and parse the arguments.
	switch {
	case len(os.Args) != 3:
		fallthrough
	case len(os.Args[1]) == 0:
		Usage()
	}

	kind := os.Args[1]
	var event interface{}
	if err := json.Unmarshal([]byte(os.Args[2]), &event); err != nil {
		Usage()
	}

	// Load the pre-configured environment.
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Make sure you are calling this program from its source directory.")
		exitError = err
		return
	}

	// Initialise PubSub service from environmental variables.
	ps, err := pubsub.NewService(func() (pubsub.Transport, error) {
		factory := zpubsub.NewTransportFactory()
		factory.MustReadConfigFromEnv("CIDER_ZMQ3_PUBSUB_").MustBeFullyConfigured()
		return factory.NewTransport("Publisher#"+MustRandomString())
	})
	if err != nil {
		exitError = err
		return
	}
	defer ps.Close()

	// Publish the event.
	if err := ps.Publish(kind, event); err != nil {
		exitError = err
		return
	}
}

func MustRandomString() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func Usage() {
	os.Stderr.Write([]byte(`
Usage: ./publish EVENT_KIND EVENT_OBJECT

where
  EVENT_KIND is the event kind string to publish the object as, and
  EVENT_OBJECT is a json-encoded object to be published.

This small demo program just publishes the event defined by the arguments and
disconnects from Cider.
`))
	os.Exit(2)
}
