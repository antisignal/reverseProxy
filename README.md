# reverseProxy
A reverse proxy and load balancer implemented in Go, designed to explore reliability engineering, fault tolerance, observability, and concurrent systems programming.

This README was last edited in full after commit ebb1da2.

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