package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"log"
	"os"
	"os/exec"
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
		factory := zrpc.NewTransportFactory()
		factory.MustReadConfigFromEnv("CIDER_ZMQ3_RPC_").MustBeFullyConfigured()
		return factory.NewTransport("Basher#" + MustRandomString())
	})
	if err != nil {
		log.Fatalf("Failed to initialise RPC: %v\n", err)
	}

	// Register Bash.RunScript method.
	//
	// No need to check for errors, if there is any. srv.Closed() will be
	// closed immediately and the whole process will terminate because the
	// following select will not block at all.
	srv.RegisterMethod("Bash.RunScript", func(req rpc.RemoteRequest) {
		var args struct {
			Script string `codec:"script"`
			Cwd    string `codec:"cwd"`
		}
		if err := req.UnmarshalArgs(&args); err != nil {
			req.Resolve(2, err.Error())
			return
		}
		if args.Script == "" {
			req.Resolve(2, "script argument is missing")
			return
		}

		cmd := exec.Command("bash", "-c", args.Script)
		cmd.Dir = args.Cwd
		cmd.Stdout = req.Stdout()
		cmd.Stderr = req.Stderr()
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Run()
		}()

		select {
		case err := <-waitCh:
			if err != nil {
				req.Resolve(1, err.Error())
			} else {
				req.Resolve(0, nil)
			}
		case <-req.Interrupted():
			cmd.Process.Signal(os.Interrupt)
			<-waitCh
			req.Resolve(3, "interrupted")
		}
	})

	// Wait for an error or user interrupt.
	select {
	case <-srv.Closed():
	case <-signalCh:
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
Usage: ./register-bash

This demo program exports a single method - Bash.RunScript - that other apps
can start calling remotely. The args object is

  {
	  "commands": string
  }

where "commands" key is supposed to contain a valid string that can be
interpreted by Bash.

The output is, of course, being streamed back and it will appear in the console
as it is printed by bash on the other side.

Sending SIGINT to register-bash will send interrupt to the remote bash.
`))
	os.Exit(2)
}
