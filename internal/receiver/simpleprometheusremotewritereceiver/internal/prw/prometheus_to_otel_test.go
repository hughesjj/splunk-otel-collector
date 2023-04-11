// Copyright Splunk, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prw

import (
	"testing"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
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
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	parser, err := NewPrwOtelParser(reporter)

	var sampleMetadata []prompb.MetricMetadata
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
