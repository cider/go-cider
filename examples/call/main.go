package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/cider/go-cider/cider/services/rpc"
	zrpc "github.com/cider/go-cider/cider/transports/zmq3/rpc"

	"github.com/cihub/seelog"
	"github.com/joho/godotenv"
	zmq "github.com/pebbe/zmq3"
	"github.com/wsxiaoys/terminal/color"
)

func main() {
	log.SetFlags(0)

	verbose := flag.Bool("v", false, "enable verbose output to stderr")
	flag.Parse()

	// Check and parse the arguments.
	if flag.NArg() != 2 {
		log.Println(flag.NArg())
		Usage()
	}

	if !*verbose {
		seelog.ReplaceLogger(seelog.Disabled)
	}

	// Parse command line.
	method := flag.Arg(0)
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(flag.Arg(1)), &args); err != nil {
		log.Println("Error: failed to parse the method args",)
		Usage()
	}

	color.Printf("@{y}Args:@{|} %v\n", args)

	// Load the pre-configured environment that uses the default Cider endpoints.
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Make sure you are calling this program from its source directory.")
		log.Fatalln(err)
	}

	// Start catching signals, but do not start processing them yet.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// Initialise RPC service from environmental variables.
	srv, err := rpc.NewService(func() (rpc.Transport, error) {
		config := new(zrpc.TransportConfig)
		config.MustFeedFromEnv("CIDER_ZMQ3_RPC_").MustBeComplete()
		return config.NewTransport("Call#" + MustRandomString())
	})
	if err != nil {
		log.Fatalf("Failed to initialise RPC: %v\n", err)
	}

	// Send the request and print the results.
	call := srv.NewRemoteCall(method, args)
	call.Stdout = os.Stdout
	call.Stderr = os.Stderr
	call.GoExecute()

	color.Println("@{g}>>>>> Output")

Loop:
	for {
		select {
		case <-call.Resolved():
			if err := call.Wait(); err != nil {
				color.Fprintf(os.Stderr, "@{r}Error: remote call failed: %v\n", err)
				break Loop
			}

			color.Println("@{g}<<<<< Output")
			var returnValue interface{}
			if err := call.UnmarshalReturnValue(&returnValue); err != nil {
				color.Printf("@{r}Error: %v\n", err)
				break Loop
			}

			if call.ReturnCode() == 0 {
				color.Printf("@{y}Return code:@{|}  %v\n", call.ReturnCode())
				color.Printf("@{y}Return value:@{|} %v\n", returnValue)
			} else {
				color.Printf("@{r}Return code:@{|}  %v\n", call.ReturnCode())
				color.Printf("@{r}Return value:@{|} %v\n", returnValue)
			}

			break Loop

		case <-signalCh:
			if err := call.Interrupt(); err != nil {
				color.Fprintf(os.Stderr, "@{r}Error: failed to interrupt remote call: %v\n", err)
				log.Println("Terminating...")
				break Loop
			}
		}
	}

	// Close the service.
	if err := srv.Close(); err != nil {
		log.Fatal(err)
	}

	// Make sure that all pending 0MQ messages are sent.
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
Usage: ./call [-v] METHOD ARGS

where
  -v enables logging output to stderr
  METHOD is the remote method to be called, and
  ARGS is the arguments object (json-encoded) for the requested method.

This small demo program
  1. connects to Cider,
  2. sends the RPC request for the specified method,
  3. prints the output coming from the other side while waiting
     for the call to finish,
  4. prints the return code and return value,
  5. then exists.
`))
	os.Exit(2)
}
