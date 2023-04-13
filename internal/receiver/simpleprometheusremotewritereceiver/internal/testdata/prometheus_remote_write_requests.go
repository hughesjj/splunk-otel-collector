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

package testdata

import (
	"fmt"

	"github.com/prometheus/prometheus/prompb"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/tools"
)

var sampleCounterTs = []prompb.TimeSeries{
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
}
var sampleCounterWq = &prompb.WriteRequest{Timeseries: sampleCounterTs}

var sampleGaugeTs = []prompb.TimeSeries{
	{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "go_goroutines"},
		},
		Samples: []prompb.Sample{
			{Value: 42, Timestamp: 1633024800000},
		},
	},
}
var sampleGaugeWq = &prompb.WriteRequest{Timeseries: sampleGaugeTs}

var sampleHistogramTs = []prompb.TimeSeries{
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
}

var sampleHistogramWq = &prompb.WriteRequest{
	Timeseries: sampleHistogramTs,
}

var sampleSummaryTs = []prompb.TimeSeries{
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
}
var sampleSummaryWq = &prompb.WriteRequest{
	Timeseries: sampleSummaryTs,
}

func GetWriteRequests() []*prompb.WriteRequest {
	var sampleWriteRequestsNoMetadata = []*prompb.WriteRequest{
		// Counter
		sampleCounterWq,
		// Gauge
		sampleGaugeWq,
		// Histogram
		sampleHistogramWq,
		// Summary
		sampleSummaryWq,
	}
	return sampleWriteRequestsNoMetadata
}

func AddMetadataScaffoldToWriteRequests(writeRequests []*prompb.WriteRequest, types []prompb.MetricMetadata_MetricType) []*prompb.WriteRequest {
	var writeRequestsWithMetadata []*prompb.WriteRequest

	for index, wr := range writeRequests {
		// Create a new WriteRequest with the same Timeseries as the original WriteRequest.
		newWr := &prompb.WriteRequest{
			Timeseries: wr.Timeseries,
		}

		// Generate metadata for each unique metric name in the WriteRequest.
		var metricNames []string
		for _, ts := range wr.Timeseries {
			for _, label := range ts.Labels {
				if label.Name == "__name__" {
					metricNames = append(metricNames, label.Value)
				}
			}
		}

		// Add metadata for each unique metric name.
		for _, name := range metricNames {
			name = tools.GetBaseMetricFamilyName(name)
			md := prompb.MetricMetadata{
				MetricFamilyName: name,
				//Type:             prompb.MetricMetadata_UNKNOWN,
				Type: types[index],
				Help: fmt.Sprintf("Help text for %s", name),
				Unit: "unit",
			}
			newWr.Metadata = append(
				newWr.Metadata,
				md,
			)
		}

		writeRequestsWithMetadata = append(writeRequestsWithMetadata, newWr)
	}

	return writeRequestsWithMetadata
}

func GetWriteRequestsWithMetadata() []*prompb.WriteRequest {
	wrq := GetWriteRequests()
	wrqMd := AddMetadataScaffoldToWriteRequests(wrq, []prompb.MetricMetadata_MetricType{
		prompb.MetricMetadata_COUNTER, prompb.MetricMetadata_GAUGE, prompb.MetricMetadata_HISTOGRAM, prompb.MetricMetadata_SUMMARY,
	})
	// Counter, Gauge, Histogram, Summary
	// While the rest may be sentinels, we should really fix any metric types where possible
	//for _, wq := range wrqMd {
	//	var mdType prompb.MetricMetadata_MetricType
	//	// lol alright this doesn't work does it
	//	// TODO hughesjj lol figure out how to actually test this
	//	switch &wq.Timeseries {
	//	case &sampleSummaryTs:
	//		mdType = prompb.MetricMetadata_SUMMARY
	//	case &sampleCounterTs:
	//		mdType = prompb.MetricMetadata_COUNTER
	//	case &sampleHistogramTs:
	//		mdType = prompb.MetricMetadata_HISTOGRAM
	//	case &sampleGaugeTs:
	//		mdType = prompb.MetricMetadata_GAUGE
	//	default:
	//		mdType = prompb.MetricMetadata_UNKNOWN
	//	}
	//	for _, md := range wq.Metadata {
	//		md.Type = mdType
	//	}
	//}
	return wrqMd
}

func FlattenWriteRequests(request []*prompb.WriteRequest) *prompb.WriteRequest {
	var ts []prompb.TimeSeries
	for _, req := range request {
		for _, t := range req.Timeseries {
			ts = append(ts, t)
		}
	}
	var md []prompb.MetricMetadata
	for _, req := range request {
		for _, t := range req.Metadata {
			md = append(md, t)
		}
	}
	return &prompb.WriteRequest{
		Timeseries: ts,
		Metadata:   md,
	}
}
