## http2server

`http2server` provides a minimal HTTP/2 server wrapper for VictoriaMetrics
components that need gRPC-compatible ingestion endpoints.

It currently serves OTLP/gRPC metrics ingestion for `vmagent` and `vminsert`.
The package intentionally keeps only the small subset of `lib/httpserver`
behavior needed by these dedicated listeners, such as server lifecycle handling
and TLS configuration loading.

Keep this package narrow. General-purpose HTTP server features should stay in
`lib/httpserver`.
