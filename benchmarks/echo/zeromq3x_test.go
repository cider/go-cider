package echo

import (
	// Stdlib
	"runtime"
	"testing"
	"time"

	// Cider broker
	"github.com/cider/cider/broker"
	"github.com/cider/cider/broker/exchanges/rpc/roundrobin"
	zmq3broker "github.com/cider/cider/broker/transports/zmq3/rpc"

	// Cider client
	"github.com/cider/go-cider/cider/services/rpc"
	zmq3client "github.com/cider/go-cider/cider/transports/zmq3/rpc"

	// Others
	log "github.com/cihub/seelog"
)

func BenchmarkTransport_ZeroMQ3x____ipc(b *testing.B) {
	benchmarkZeroMQ3x(b, "ipc://cider-rpc.ipc")
}

func BenchmarkTransport_ZeroMQ3x_inproc(b *testing.B) {
	benchmarkZeroMQ3x(b, "inproc://rnd"+mustRandomString())
}

func benchmarkZeroMQ3x(b *testing.B, endpoint string) {
	// Disable logging.
	log.ReplaceLogger(log.Disabled)

	// Run in parallel.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Instantiate a new broker and RPC exchange.
	var (
		bro      = broker.New()
		balancer = roundrobin.NewBalancer()
	)

	// Set up ZeroMQ 3.x IPC RPC endpoint.
	bro.RegisterEndpointFactory("zmq3_rpc", func() (broker.Endpoint, error) {
		config := zmq3broker.NewEndpointConfig()
		config.Endpoint = endpoint
		return zmq3broker.NewEndpoint(config, balancer)
	})
	bro.ListenAndServe()
	defer bro.Terminate()

	// Give broker some time to spawn the inproc endpoint. It must be running
	// before the client attempts to connect, otherwise it's connection refused.
	time.Sleep(time.Second)

	// Set up an RPC service instance using ZeroMQ 3.x IPC transport.
	srv, err := rpc.NewService(func() (rpc.Transport, error) {
		factory := zmq3client.NewTransportFactory()
		factory.Endpoint = endpoint
		return factory.NewTransport("Call#" + mustRandomString())
	})
	if err != nil {
		b.Fatal(err)
	}
	defer srv.Close()

	// Export Echo method.
	if err := srv.RegisterMethod("Echo", func(req rpc.RemoteRequest) {
		var payload []byte
		if err := req.UnmarshalArgs(&payload); err != nil {
			req.Resolve(2, err.Error())
			return
		}
		req.Resolve(0, payload)
	}); err != nil {
		b.Fatal(err)
	}

	payload := []byte("payload")
	calls := make([]*rpc.RemoteCall, b.N)
	b.ResetTimer()

	// Start shooting requests.
	for i := 0; i < b.N; i++ {
		calls[i] = srv.NewRemoteCall("Echo", payload).GoExecute()
	}

	for i := 0; i < b.N; i++ {
		if err := calls[i].Wait(); err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()

	// Set GOMAXPROCS to the original value.
	runtime.GOMAXPROCS(1)
}
