# reverseProxy
This project implements an HTTP reverse proxy and load balancer in Go. It explores concepts common to production infrastructure software, including concurrent request handling, health monitoring, structured logging, retries, and fault injection for testing failure scenarios.

This project has the following features:
- load balancing (currently round-robin)
- structured logging, with different levels depending on severity (INFO, ERROR, FATAL)
- a status page where the health of backends can be checked
- a health checker which flags unhealthy backends so they can be omitted from the load balancer's pool of available backends
- retries after backend failure, if a backend dies before the health checker flags it
- a fault injection framework which simulates degraded conditions (502 errors, dead and unhealthy backends)
- a CLI where args like the number of simulated backends and preferred port ranges for backends can be selected

The reverse proxy has the following architecture:
![Reverse Proxy Architecture Diagram](https://i.imgur.com/tFQVSUU.png)

Some major engineering decisions include:
- Debug flags are provided via a debugInfo struct, for the sake of rapid prototyping
- Apart from google/uuid, all imports are from the standard library to reduce dependency complexity, simplify builds, and avoid unnecessary third-party packages
- Backends are implemented and run in the same package as opposed to being hosted externally to ease testing, so that the fault injection framework/chaos monkey can take them down at the same time the other components start running
- Structured logs use an enum for events and reasons so that it's harder to accidentally mistype an event or reason name, which would lead to inconsistency in the logs
- Backends are stored as their url + liveness + listener, so that the load balancer has easy access to health checker information and the fault injection framework can close the backend easily
- The load balancer provides the next backend for the current request directly, to minimize the chance of data races due to direct concurrent external access to the backend list
- Latency is stored for applicable operations (requests, getting the next backend, etc) so that a future observability dashboard can track it as a metric
- The origin IP, listening port, start for backend ports, and number of backends can be passed as arguments to the CLI so the program can be used even if default ports are in use
- When backends are created, ports that are in use are skipped over automatically, and port exhaustion gives an error, for robustness
- Only HTTP, TCP, and IPv4 is supported for simplicity; this is one limitation of the program
- The health checker, chaos engine, reverse proxy, and the backends are all run concurrently in the same program for simplicity
- The program will panic if no backends are provided (invariant)
- JSON is used to provide information on the status page for simplicity of marshaling it and displaying it as hypertext

The request lifecycle (happy path) is as follows:
- client sends request to reverse proxy
- reverse proxy chooses a backend (marked alive - if the next backend is dead, it's skipped in favor of another one)
- proxy alters the request to have the selected backend as a destination
- proxy forwards the request to the backend
- proxy receives response
- proxy forwards response to client

In the future, I would like to add the following:
- Grafana/Prometheus metrics
- HTTP/2, HTTP/3 support
- least-connections load balancing
- weighted round-robin load balancing
- external backend selection

Lessons learned:
- Concurrency is hard and primarily about ownership. Even innocent seeming concurrent access can easily lead to race conditions. Most of concurrency is determining who owns mutable shared state and who accesses it when.
- Failure is inevitable and needs to be handled at every stage. Most of the complexity in this project deals with failure management.
- Engineering is largely about tradeoffs and working under constraints. Most of this project involved balancing things like simplicity and extensibility and prioritization of these was key.