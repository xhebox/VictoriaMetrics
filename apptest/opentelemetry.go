package apptest

import (
	"fmt"
	"net/http"
	"testing"

	vmgrpc "github.com/VictoriaMetrics/VictoriaMetrics/lib/grpc"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	otlppb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func opentelemetryRecordsCount(md otlppb.MetricsData) int {
	var recordsCount int
	for _, rss := range md.ResourceMetrics {
		for _, sm := range rss.ScopeMetrics {
			recordsCount += len(sm.Metrics)
			for _, m := range sm.Metrics {
				if prommetadata.IsEnabled() {
					recordsCount += len(m.Metadata)
				}
			}
		}
	}
	return recordsCount
}

func getOTLPGRPCMetricsURL(addr string) string {
	if addr == "" {
		return ""
	}
	return fmt.Sprintf("http://%s/opentelemetry.proto.collector.metrics.v1.MetricsService/Export", addr)
}

func sendOpentelemetryGRPCMetrics(t *testing.T, cli *Client, url string, sendBlocking func(t *testing.T, numRecordsToSend int, send func()), md otlppb.MetricsData, opts QueryOpts) {
	t.Helper()
	if url == "" {
		t.Fatalf("OTLP/gRPC metrics URL is empty")
	}

	recordsCount := opentelemetryRecordsCount(md)
	data := vmgrpc.AppendMessageFrame(nil, md.MarshalProtobuf(nil), false)
	reqHeaders := opts.getHeaders()
	reqHeaders.Set("Content-Type", "application/grpc")
	sendBlocking(t, recordsCount, func() {
		body, statusCode, respHeaders, respTrailers := cli.PostHTTP2(t, url, data, reqHeaders)
		if statusCode != http.StatusOK {
			t.Fatalf("unexpected status code: got %d, want %d; response body: %q", statusCode, http.StatusOK, body)
		}
		if got, want := respTrailers.Get("grpc-status"), vmgrpc.StatusCodeOk; got != want {
			t.Fatalf("unexpected grpc-status trailer; got %q; want %q; response headers: %v; response body: %q", got, want, respHeaders, body)
		}
		message, compressed, err := vmgrpc.ParseMessageFrame(body)
		if err != nil {
			t.Fatalf("cannot parse OTLP/gRPC response frame: %s; response body: %q", err, body)
		}
		if compressed {
			t.Fatalf("unexpected compressed OTLP/gRPC response")
		}
		if len(message) != 0 {
			t.Fatalf("unexpected OTLP/gRPC response body; got %q; want empty body", message)
		}
	})
}
