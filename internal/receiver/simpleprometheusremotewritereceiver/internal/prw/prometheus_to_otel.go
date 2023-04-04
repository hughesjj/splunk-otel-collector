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
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/tools"
	"strings"
	//"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	//"github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func (prwParser *PrwOtelParser) FromPrometheusWriteRequestMetrics(request prompb.WriteRequest) (
	pmetric.Metrics,
	error,
) {
	var otelMetrics pmetric.Metrics
	metricFamiliesAndData, err := prwParser.partitionWriteRequest(request)
	if nil != err {
		// TODO hughesjj
	}
	for metricFamilyName, metricData := range metricFamiliesAndData {
		// TODO actually convert now
		prwParser.
	}

	return otelMetrics, nil
}

type MetricData struct {
	MetricMetadata   prompb.MetricMetadata
	MetricFamilyName string
	MetricName       string
	Labels           *[]prompb.Label
	Samples          *[]prompb.Sample
	Exemplars        *[]prompb.Exemplar
	Histograms       *[]prompb.Histogram
}

type PrwOtelParser struct {
	metricTypesCache tools.PrometheusMetricTypeCache
}

const maxCachedMetadata = 10000

func NewPrwOtelParser() *PrwOtelParser {
	return &PrwOtelParser{metricTypesCache: tools.NewPrometheusMetricTypeCache(maxCachedMetadata)}
}

func getMetricNameAndFilteredLabels(labels *[]prompb.Label) (string, *[]prompb.Label) {
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

func getMetricTypeByLabels(labels *[]prompb.Label) prompb.MetricMetadata_MetricType {
	// TODO actually assign the slice?
	metricName, _ := getMetricNameAndFilteredLabels(labels)

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

	return prompb.MetricMetadata_GAUGE
}

func (prwParser *PrwOtelParser) partitionWriteRequest(writeReq prompb.WriteRequest) (map[string][]MetricData, error) {
	partitions := make(map[string][]MetricData)

	if len(writeReq.Timeseries) >= maxCachedMetadata {
		// TODO hughesjj ruh Roh
	}

	// update cache with any metric data found
	for _, metadata := range writeReq.Metadata {
		prwParser.metricTypesCache.AddMetadata(metadata.MetricFamilyName, metadata)
	}

	for _, ts := range writeReq.Timeseries {
		metricName, filteredLabels := getMetricNameAndFilteredLabels(&ts.Labels)
		metricFamilyName := getBaseMetricFamilyName(metricName)

		// Determine metric type using cache if available, otherwise use label analysis
		metricMetadata, ok := prwParser.metricTypesCache.Get(metricName)
		if !ok {
			metricType := getMetricTypeByLabels(&ts.Labels)
			metricMetadata, ok = prwParser.metricTypesCache.AddHeuristic(metricName, prompb.MetricMetadata{Type: metricType})
		}

		// Add the parsed time-series data to the corresponding partition
		// Might be nice to freeze and assign MetricMetadata after this loop has had the chance to "maximally cache" it all
		partitions[metricFamilyName] = append(partitions[metricFamilyName], MetricData{
			Labels:           filteredLabels,
			Samples:          &ts.Samples,
			Exemplars:        &ts.Exemplars,
			Histograms:       &ts.Histograms,
			MetricFamilyName: metricFamilyName,
			MetricMetadata:   metricMetadata,
		})
	}

	for metricFamilyName, data := range partitions {
		// I think there's a slim but nonzero chance we got a better read on the heuristics later on for a given metricFamilyName
		metricMetadata, notFound := prwParser.metricTypesCache.Get(metricFamilyName)
		if !notFound {
			for _, partition := range data {
				partition.MetricMetadata = metricMetadata
			}
		}
	}

	return partitions, nil
}

func getBaseMetricFamilyName(metricName string) string {
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

func (prwParser *PrwOtelParser) transformPrwToOtel(parsedPrwMetrics map[string][]MetricData) pmetric.Metrics {
	metric := pmetric.NewMetrics()
	metric.ResourceMetrics().AppendEmpty().Resource()
	return metric
}

func (prwParser *PrwOtelParser) addHistogram() {

}
func (prwParser *PrwOtelParser) addGauge() {

}
func (prwParser *PrwOtelParser) addCounter() {

}
func (prwParser *PrwOtelParser) addSummary() {

}
