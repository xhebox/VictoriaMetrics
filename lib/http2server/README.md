## http2server

`http2server` package is ported from [`github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver`](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/lib/httpserver),
with HTTP/2 support enabled. Only core features are preserved.

It is intended as an interim package for OTLP/gRPC ingestion over HTTP/2.
It should eventually be merged into `lib/httpserver` so that both VictoriaMetrics
and VictoriaTraces can share a single HTTP/2-capable server implementation and
avoid maintaining duplicate copies.
