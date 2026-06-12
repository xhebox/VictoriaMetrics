package apptest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prommetadata"
	otlppb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

func sendOpentelemetryGRPCMetrics(t *testing.T, addr string, sendBlocking func(t *testing.T, numRecordsToSend int, send func()), md otlppb.MetricsData, opts QueryOpts) {
	t.Helper()
	if addr == "" {
		t.Fatalf("OTLP/gRPC metrics addr is empty")
	}

	recordsCount := opentelemetryRecordsCount(md)
	req := pmetricotlp.NewExportRequest()
	if err := req.UnmarshalProto(md.MarshalProtobuf(nil)); err != nil {
		t.Fatalf("cannot create OTLP/gRPC export request: %s", err)
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithNoProxy())
	if err != nil {
		t.Fatalf("cannot create OTLP/gRPC client for %q: %s", addr, err)
	}
	defer conn.Close()
	grpcClient := pmetricotlp.NewGRPCClient(conn)
	headers := opts.getHeaders()
	sendBlocking(t, recordsCount, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if len(headers) > 0 {
			ctx = metadata.NewOutgoingContext(ctx, metadataFromHTTPHeaders(headers))
		}
		if _, err := grpcClient.Export(ctx, req); err != nil {
			t.Fatalf("cannot send OTLP/gRPC export request: %s", err)
		}
	})
}

func metadataFromHTTPHeaders(headers http.Header) metadata.MD {
	md := metadata.MD{}
	for k, values := range headers {
		md.Append(k, values...)
	}
	return md
}
