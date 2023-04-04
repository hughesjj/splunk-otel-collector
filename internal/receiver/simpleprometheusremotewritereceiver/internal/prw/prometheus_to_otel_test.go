package prw

import (
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"testing"
)


var sampleWriteRequestsNoMetadata = []*prompb.WriteRequest{
	// Counter
	{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "http_requests_total"},
					{Name: "method", Value: "GET"},
					{Name: "status", Value: "200"},
				},
				Samples: []prompb.Sample{
					{Value: 1024, Timestamp: 1633024800000},
				},
			},
		},
	},
	// Gauge
	{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "go_goroutines"},
				},
				Samples: []prompb.Sample{
					{Value: 42, Timestamp: 1633024800000},
				},
			},
		},
	},
	// Histogram
	{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "api_request_duration_seconds_bucket"},
					{Name: "le", Value: "0.1"},
				},
				Samples: []prompb.Sample{
					{Value: 500, Timestamp: 1633024800000},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "api_request_duration_seconds_bucket"},
					{Name: "le", Value: "0.2"},
				},
				Samples: []prompb.Sample{
					{Value: 1500, Timestamp: 1633024800000},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "api_request_duration_seconds_count"},
				},
				Samples: []prompb.Sample{
					{Value: 2500, Timestamp: 1633024800000},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "api_request_duration_seconds_sum"},
				},
				Samples: []prompb.Sample{
					{Value: 350, Timestamp: 1633024800000},
				},
			},
		},
	},
	// Summary
	{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "rpc_duration_seconds"},
					{Name: "quantile", Value: "0.5"},
				},
				Samples: []prompb.Sample{
					{Value: 0.25, Timestamp: 1633024800000},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "rpc_duration_seconds"},
					{Name: "quantile", Value: "0.9"},
				},
				Samples: []prompb.Sample{
					{Value: 0.35, Timestamp: 1633024800000},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "rpc_duration_seconds_sum"},
				},
				Samples: []prompb.Sample{
					{Value: 123.5, Timestamp: 1633024800000},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "rpc_duration_seconds_count"},
				},
				Samples: []prompb.Sample{
						{Value: 1500, Timestamp: 1633024800000},
					},
				},
			},
		},
	}



func TestParsePrometheusRemoteWriteRequest(t *testing.T) {
	parser := NewPrwOtelParser()

	sampleMetadata := []prompb.MetricMetadata
	sampleMetadata = append(sampleMetadata, prompb.MetricMetadata{
		MetricFamilyName: "foo",
	})

	sampleWriteRequest := prompb.WriteRequest{
		Metadata: sampleMetadata,
	}
	partitions, err := parser.partitionWriteRequest(sampleWriteRequest)
	assert.NoError(t, err)
	for key, partition := range partitions {
		for _, md := range partition {
			assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			// TODO hughesjj finish test
		}
	}
}
