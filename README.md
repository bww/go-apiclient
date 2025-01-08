A Better REST API Client for Go
===============================

_Go API Client_ builds on the Go standard library [HTTP client](https://pkg.go.dev/net/http) to provide _fully-featured, batteries-included support for interacting with REST APIs_. It supports higher-level features you either need now or will probably need eventually:

* Support for **automatically retrying failed requests** and generally handling errors more gracefully,
* Support for **rate limiting** (as imposed either by the service or by your project)
* Support for **multiplexing concurrent requests** and managing responses,
* Features for improved **logging, debugging, and observability** of requests,
* Better ergonomics for making REST requests than the standard library.
