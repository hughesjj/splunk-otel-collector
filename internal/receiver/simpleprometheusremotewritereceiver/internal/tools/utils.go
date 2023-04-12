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
	"github.com/prometheus/prometheus/prompb"
	"strings"
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
func GetMetricNameAndFilteredLabels(labels *[]prompb.Label) (string, *[]prompb.Label) {
	metricName := ""
	var filteredLabels []prompb.Label

	for _, label := range *labels {
		if label.Name == "__name__" {
			metricName = label.Value
		} else {
			filteredLabels = append(filteredLabels, label)
		}
	}

	return metricName, &filteredLabels
}

func GetMetricTypeByLabels(labels *[]prompb.Label) prompb.MetricMetadata_MetricType {
	// TODO hughesjj actually assign the slice?
	metricName, _ := GetMetricNameAndFilteredLabels(labels)

	if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_count") || strings.HasSuffix(metricName, "_counter") {
		return prompb.MetricMetadata_COUNTER
	}

	for _, label := range *labels {
		if label.Name == "le" {
			return prompb.MetricMetadata_HISTOGRAM
		}

		if label.Name == "quantile" {
			return prompb.MetricMetadata_SUMMARY
		}
	}
	// TODO hughesjj okay should we ever return unknown?
	return prompb.MetricMetadata_GAUGE
}
