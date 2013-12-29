package echo

import (
	// Stdlib
	"runtime"
	"testing"

	// Cider broker
	bro "github.com/cider/cider/broker"
	"github.com/cider/cider/broker/exchanges/rpc/roundrobin"

	// Cider Go client
	"github.com/cider/go-cider/cider/services/rpc"
	inproc "github.com/cider/go-cider/cider/transports/inproc/rpc"

	// Others
	log "github.com/cihub/seelog"
)

func BenchmarkTransport__________inproc(b *testing.B) {
	// Disable logging.
	log.ReplaceLogger(log.Disabled)

	// Run in parallel.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Instantiate a new broker and RPC exchange.
	var (
		broker   = bro.New()
		balancer = roundrobin.NewBalancer()
	)

	// Set up inproc RPC endpoint.
	transport := inproc.NewTransport(mustRandomString(), balancer)
	broker.RegisterEndpointFactory("inproc_rpc", func() (bro.Endpoint, error) {
		return transport.AsEndpoint(), nil
	})

	// Set up an RPC service instance using inproc.
	srv, err := rpc.NewService(func() (rpc.Transport, error) {
		return transport, nil
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

	// Start benchmarking.
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
