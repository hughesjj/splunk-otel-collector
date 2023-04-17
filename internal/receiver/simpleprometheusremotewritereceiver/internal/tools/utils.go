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

package tools

import (
	"strings"

	"github.com/prometheus/prometheus/prompb"
)

const TypeStr = "simpleprometheusremotewrite"

func GetBaseMetricFamilyName(metricName string) string {
	// Remove known suffixes for Sum/Counter, Histogram and Summary metric types.
	// While not strictly enforced in the protobuf, prometheus does not support "colliding"
	// "metric family names" in the same write request, so this should be safe
	// https://prometheus.io/docs/practices/naming/
	// https://prometheus.io/docs/concepts/metric_types/
	suffixes := []string{"_count", "_sum", "_bucket", "_created"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(metricName, suffix) {
			metricName = strings.TrimSuffix(metricName, suffix)
			break
		}
	}

	return metricName
}
func GetMetricNameAndFilteredLabels(labels *[]prompb.Label) (string, []prompb.Label) {
	metricName := ""
	var filteredLabels []prompb.Label

	for _, label := range *labels {
		if label.Name == "__name__" {
			metricName = label.Value
		} else {
			filteredLabels = append(filteredLabels, label)
		}
	}

	return metricName, filteredLabels
}

// GuessMetricTypeByLabels This is a 'best effort' heuristic applying guidance from the latest OpenMetrics specification
// See: https://raw.githubusercontent.com/OpenObservability/OpenMetrics/main/specification/OpenMetrics.md
// As this is a heuristic process, the order of operations is SIGNIFICANT.
func GuessMetricTypeByLabels(labels *[]prompb.Label) prompb.MetricMetadata_MetricType {
	metricName, filteredLabels := GetMetricNameAndFilteredLabels(labels)

	for _, label := range filteredLabels {
		if label.Name == "le" {
			return prompb.MetricMetadata_HISTOGRAM
		}
		if label.Name == "quantile" {
			return prompb.MetricMetadata_SUMMARY
		}
		if label.Name == metricName {
			// The OpenMetrics spec ABNF examples directly conflict with their own given summary, TODO inform them
			return prompb.MetricMetadata_STATESET
		}
	}
	if strings.HasSuffix(metricName, "_gsum") || strings.HasSuffix(metricName, "_gcount") {
		// Of note is that 'le' should never appear in a metric label for this, but such is checked above
		// ... also the ABNF part of the OpenMetrics spec directly conflicts with their leading summary
		return prompb.MetricMetadata_GAUGEHISTOGRAM
	}
	if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_count") || strings.HasSuffix(metricName, "_counter") || strings.HasSuffix(metricName, "_created") {
		return prompb.MetricMetadata_COUNTER
	}
	if strings.HasSuffix(metricName, "_bucket") {
		// While bucket may exist for a gauge histogram or Summary, we've checked such above
		return prompb.MetricMetadata_HISTOGRAM
	}
	if strings.HasSuffix(metricName, "_info") {
		return prompb.MetricMetadata_INFO
	}
	// TODO hughesjj okay should we ever return unknown?
	return prompb.MetricMetadata_GAUGE
}
