package main

import (
	// Stdlib
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	// Cider
	"github.com/cider/go-cider/cider/services/pubsub"
	zpubsub "github.com/cider/go-cider/cider/transports/zmq3/pubsub"

	// Other
	"github.com/joho/godotenv"
	zmq "github.com/pebbe/zmq3"
	"github.com/wsxiaoys/terminal/color"
)

func main() {
	log.SetFlags(0)

	// Check the args.
	if len(os.Args) != 2 {
		Usage()
	}
	kindPrefix := os.Args[1]

	// Load the pre-configured environment.
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Make sure you are calling this program from its source directory.")
		log.Fatalln(err)
	}

	// Perform some cleanup and print the exit error if one is set.
	var exitError error
	defer func() {
		// Terminate the ZeroMQ context before exiting.
		log.Println("Waiting for all pending messages to be sent ...")
		zmq.Term()
		// Exit with a return code depending on whether there was any error.
		if exitError != nil {
			log.Fatalf("Error: %v", exitError)
		}
	}()

	// Set up a signal handler to close the client on any signal received.
	var (
		signalCh    = make(chan os.Signal, 1)
		closeCh     = make(chan struct{})
		interruptCh = make(chan struct{})
	)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-signalCh:
			log.Println("Signal received, terminating...")
			close(interruptCh)
		case <-closeCh:
			close(interruptCh)
		}
	}()

	// Initialise PubSub service from environmental variables.
	srv, err := pubsub.NewService(func() (pubsub.Transport, error) {
		config := zpubsub.NewTransportConfig()
		config.MustFeedFromEnv("CIDER_ZMQ3_PUBSUB_").MustBeComplete()
		return zpubsub.NewTransport("Subscriber#"+MustRandomString(), config)
	})
	if err != nil {
		exitError = err
		return
	}

	// Start to monitor the service for transport-unrelated errors.
	srvMonitor := make(chan error)
	srv.Monitor(srvMonitor)

	// Subscribe for the requested event kind prefix.
	_, err = srv.Subscribe(kindPrefix, func(event pubsub.Event) {
		// Unmarshal the event object itself.
		var eventObject map[string]string
		if err := event.Unmarshal(&eventObject); err != nil {
			log.Printf("[PUBSUB] Failed to unmarshal event %v: %v\n", event.Kind(), err)
			return
		}

		// Stringify it so that we can pretty-print it to the user.
		var eventString string
		if bs, err := json.MarshalIndent(eventObject, "", "  "); err != nil {
			log.Printf("[PUBSUB] Failed to stringify event %v: %v\n", event.Kind(), err)
			return
		} else {
			eventString = string(bs)
		}

		// Print the event received to the user.
		color.Println("@{w}==========")
		color.Printf("@{g}Kind:@{|} %v\n", event.Kind())
		color.Printf("@{g}Seq:@{|}  %v\n", event.Seq())
		color.Printf("@{g}Body:@{|}\n%v\n", eventString)
		color.Println("@{w}==========\n")
	})
	if err != nil {
		exitError = err
		return
	}

	log.Printf("Subscribed for '%v' event kind prefix\n", os.Args[1])

	// Process messages on relevant channels.
SelectLoop:
	for {
		select {
		case err := <-srvMonitor:
			ex := err.(*pubsub.ErrEventSequenceGap)
			color.Fprintf(os.Stderr, "@{r}Missed %v %v event(s)!\n",
				ex.ReceivedSeq-ex.ExpectedSeq, ex.EventKind)

		case <-interruptCh:
			if err := srv.Close(); err != nil {
				log.Fatalln(err)
			}
			break SelectLoop

		case <-srv.Closed():
			break SelectLoop
		}
	}

	close(closeCh)
	exitError = srv.Wait()
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
Usage: ./subscribe EVENT_KIND_PREFIX

where
  EVENT_KIND_PREFIX is the event kind prefix to subscribe for.

This small demo program subscribes for the requested event kind prefix and
prints all the received events into the console, all nicely formatted.

`))
	os.Exit(2)
}
