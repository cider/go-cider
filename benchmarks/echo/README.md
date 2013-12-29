# Results #

Benchmarks run on MacBook Pro, i7, 4 cores, 4 GB RAM.

```
BenchmarkTransport__________inproc	  100000	     27724 ns/op
BenchmarkTransport_ZeroMQ3x____ipc	   10000	    128479 ns/op
BenchmarkTransport_ZeroMQ3x_inproc	   10000	    111898 ns/op
ok  	github.com/cider/go-cider/benchmarks/echo	11.762s
```
