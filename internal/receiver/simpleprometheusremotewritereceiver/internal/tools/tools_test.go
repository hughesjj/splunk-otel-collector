package tools

import (
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetMetricFamilyName(t *testing.T) {
	assert.Equal(t, "rpc_duration_seconds", GetBaseMetricFamilyName("rpc_duration_seconds"))
	assert.Equal(t, "rpc_duration_seconds", GetBaseMetricFamilyName("rpc_duration_seconds_count"))
	//assert.Equal(t, "rpc_duration_seconds", GetBaseMetricFamilyName("rpc_duration_seconds_total"))
	assert.Equal(t, "rpc_duration_seconds", GetBaseMetricFamilyName("rpc_duration_seconds_sum"))
	assert.Equal(t, "rpc_duration_seconds", GetBaseMetricFamilyName("rpc_duration_seconds_bucket"))
}

func TestGetMetricTypeByLabels(t *testing.T) {
	testCases := []struct {
		labels     []prompb.Label
		metricType prompb.MetricMetadata_MetricType
	}{
		{
			labels: []prompb.Label{
				{Name: "__name__", Value: "http_requests_total"},
				{Name: "method", Value: "GET"},
				{Name: "status", Value: "200"},
			},
			metricType: prompb.MetricMetadata_COUNTER,
		},
		{
			labels: []prompb.Label{
				{Name: "__name__", Value: "go_goroutines"},
			},
			metricType: prompb.MetricMetadata_GAUGE,
		},
		{
			labels: []prompb.Label{
				{Name: "__name__", Value: "api_request_duration_seconds_bucket"},
				{Name: "le", Value: "0.1"},
			},
			metricType: prompb.MetricMetadata_HISTOGRAM,
		},
		{
			labels: []prompb.Label{
				{Name: "__name__", Value: "rpc_duration_seconds"},
				{Name: "quantile", Value: "0.5"},
			},
			metricType: prompb.MetricMetadata_SUMMARY,
		},
	}

	for _, tc := range testCases {
		metricType := GetMetricTypeByLabels(&tc.labels)
		if metricType != tc.metricType {
			t.Errorf("GetMetricTypeByLabels(%v) = %v; want %v", tc.labels, metricType, tc.metricType)
		}
	}
}
