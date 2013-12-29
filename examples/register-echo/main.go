package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cider/go-cider/cider/services/rpc"
	zrpc "github.com/cider/go-cider/cider/transports/zmq3/rpc"

	"github.com/joho/godotenv"
	zmq "github.com/pebbe/zmq3"
)

func main() {
	// Parse the command line.
	help := flag.Bool("h", false, "print help and exit")
	flag.Parse()

	if *help || flag.NArg() != 0 {
		Usage()
	}

	// Load the pre-configured environment that uses the default Cider endpoints.
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Make sure you are calling this program from its source directory.")
		log.Fatalln(err)
	}

	// Start catching signals.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Initialise RPC service from environmental variables.
	srv, err := rpc.NewService(func() (rpc.Transport, error) {
		config := new(zrpc.TransportConfig)
		config.MustFeedFromEnv("CIDER_ZMQ3_RPC_").MustBeComplete()
		return config.NewTransport("Echo#" + MustRandomString())
	})
	if err != nil {
		log.Fatalf("Failed to initialise RPC: %v\n", err)
	}

	// Start processing signals.
	interruptCh := make(chan struct{})
	go func() {
		select {
		case <-signalCh:
			log.Println("Signal received, terminating...")
			close(interruptCh)
		case <-srv.Closed():
			return
		}
	}()

	// Register Echo.
	//
	// No need to check for errors right now, <-srv.Closed() will be closed
	// if there is any fatal error during Register.
	srv.RegisterMethod("Echo", func(req rpc.RemoteRequest) {
		var args map[string]interface{}
		if err := req.UnmarshalArgs(&args); err != nil {
			req.Resolve(2, map[string]string{
				"error": err.Error(),
			})
			return
		}
		req.Resolve(0, args)
	})

	// Wait for an error or user interrupt.
	select {
	case <-srv.Closed():
	case <-interruptCh:
		if err := srv.Close(); err != nil {
			log.Fatalln(err)
		}
	}

	// Wait for the service to shut down and the messages to be sent.
	log.Printf("RPC service exited with error=%v\n", srv.Wait())
	zmq.Term()
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
Usage: ./register-echo

This demo program exports a single service called "Echo" that just returns
exactly the same payload that it receives. It can be used for benchmarking
Cider RPC framework as well since there is almost no overhead in the method
handler itself.

`))
	os.Exit(2)
}
