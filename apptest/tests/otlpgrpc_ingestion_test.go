package tests

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	otlppb "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func TestSingleOTLPGRPCIngestion(t *testing.T) {
	fs.MustRemoveDir(t.Name())
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle",
		"-retentionPeriod=100y",
		"-otlpGRPCListenAddr=127.0.0.1:0",
	})
	sut.OpentelemetryGRPCMetrics(t, makeOTLPGRPCMetricsData("single"), apptest.QueryOpts{})
	sut.ForceFlush(t)

	assertOTLPGRPCMetric(t, tc, sut, "single")
}

func TestClusterOTLPGRPCIngestion(t *testing.T) {
	fs.MustRemoveDir(t.Name())
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})
	vminsert := tc.MustStartVminsert("vminsert", []string{
		"-storageNode=" + vmstorage.VminsertAddr(),
		"-otlpGRPCListenAddr=127.0.0.1:0",
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})

	vminsert.OpentelemetryGRPCMetrics(t, makeOTLPGRPCMetricsData("cluster"), apptest.QueryOpts{})
	vmstorage.ForceFlush(t)

	assertOTLPGRPCMetric(t, tc, vmselect, "cluster")
}

func TestVmagentOTLPGRPCIngestion(t *testing.T) {
	fs.MustRemoveDir(t.Name())
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingle := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir() + "/vmsingle",
		"-retentionPeriod=100y",
	})
	vmagent := tc.MustStartDefaultRWVmagent("vmagent", []string{
		fmt.Sprintf(`-remoteWrite.url=http://%s/api/v1/write`, vmsingle.HTTPAddr()),
		"-remoteWrite.tmpDataPath=" + tc.Dir() + "/vmagent",
		"-otlpGRPCListenAddr=127.0.0.1:0",
	})

	vmagent.OpentelemetryGRPCMetrics(t, makeOTLPGRPCMetricsData("vmagent"), apptest.QueryOpts{})
	vmsingle.ForceFlush(t)

	assertOTLPGRPCMetric(t, tc, vmsingle, "vmagent")
}

func makeOTLPGRPCMetricsData(suffix string) otlppb.MetricsData {
	tsNano := uint64(1707123456700 * 1e6) // 2024-02-05T08:57:36.700Z
	metricName := "otlp_grpc_" + suffix
	serviceName := "otlp-grpc-" + suffix
	value := 42.5
	transport := "grpc"
	return otlppb.MetricsData{
		ResourceMetrics: []*otlppb.ResourceMetrics{
			{
				Resource: &otlppb.Resource{
					Attributes: []*otlppb.KeyValue{
						{
							Key:   "service.name",
							Value: &otlppb.AnyValue{StringValue: &serviceName},
						},
					},
				},
				ScopeMetrics: []*otlppb.ScopeMetrics{
					{
						Scope: &otlppb.InstrumentationScope{
							Name: &suffix,
						},
						Metrics: []*otlppb.Metric{
							{
								Name: metricName,
								Gauge: &otlppb.Gauge{
									DataPoints: []*otlppb.NumberDataPoint{
										{
											DoubleValue:  &value,
											TimeUnixNano: tsNano,
											Attributes: []*otlppb.KeyValue{
												{
													Key:   "transport",
													Value: &otlppb.AnyValue{StringValue: &transport},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func assertOTLPGRPCMetric(t *testing.T, tc *apptest.TestCase, sut apptest.PrometheusQuerier, suffix string) {
	t.Helper()
	metricName := "otlp_grpc_" + suffix
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected OTLP/gRPC metric",
		Got: func() any {
			got := sut.PrometheusAPIV1Export(t, `{__name__="`+metricName+`"}`, apptest.QueryOpts{
				Start: "2024-02-05T08:50:00.700Z",
				End:   "2024-02-05T09:00:00.700Z",
			})
			got.Sort()
			return got
		},
		Want: &apptest.PrometheusAPIV1QueryResponse{Data: &apptest.QueryData{Result: []*apptest.QueryResult{
			{
				Metric: map[string]string{
					"__name__":      metricName,
					"service.name":  "otlp-grpc-" + suffix,
					"scope.name":    suffix,
					"scope.version": "unknown",
					"transport":     "grpc",
				},
				Samples: []*apptest.Sample{{Timestamp: 1707123456700, Value: 42.5}},
			},
		}}},
		CmpOpts: []cmp.Option{
			cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		},
	})
}
