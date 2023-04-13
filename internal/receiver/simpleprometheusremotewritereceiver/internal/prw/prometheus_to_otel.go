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
	"context"
	"time"

	"github.com/jaegertracing/jaeger/pkg/multierror"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/tools"
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/transport"
)

func (prwParser *PrometheusRemoteOtelParser) FromPrometheusWriteRequestMetrics(ctx context.Context, request *prompb.WriteRequest) (pmetric.Metrics, error) {
	var otelMetrics pmetric.Metrics
	metricFamiliesAndData, err := prwParser.partitionWriteRequest(*request)
	if nil == err {
		otelMetrics, err = prwParser.TransformPrwToOtel(ctx, metricFamiliesAndData)
		if nil == err {
			prwParser.Reporter.OnMetricsProcessed(ctx, otelMetrics.DataPointCount(), err)
		} else {
			prwParser.Reporter.OnTranslationError(ctx, err)
		}
	} else {
		prwParser.Reporter.OnTranslationError(ctx, err)
	}
	return otelMetrics, err
}

type MetricData struct {
	Labels           *[]prompb.Label
	Samples          *[]prompb.Sample
	Exemplars        *[]prompb.Exemplar
	Histograms       *[]prompb.Histogram
	MetricFamilyName string
	MetricName       string
	MetricMetadata   prompb.MetricMetadata
}

type PrometheusRemoteOtelParser struct {
	metricTypesCache *tools.PrometheusMetricTypeCache
	Reporter         transport.Reporter
}

const maxCachedMetadata = 10000

func NewPrwOtelParser(reporter transport.Reporter) (PrometheusRemoteOtelParser, error) {
	return PrometheusRemoteOtelParser{
		metricTypesCache: tools.NewPrometheusMetricTypeCache(maxCachedMetadata),
		Reporter:         reporter,
	}, nil
}

// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/13bcae344506fe2169b59d213361d04094c651f6/receiver/prometheusreceiver/internal/util.go#L106

func (prwParser *PrometheusRemoteOtelParser) partitionWriteRequest(writeReq prompb.WriteRequest) (map[string][]MetricData, error) {
	partitions := make(map[string][]MetricData)

	if len(writeReq.Timeseries) >= maxCachedMetadata {
		// TODO hughesjj ruh Roh
	}

	// update cache with any metric data found
	for _, metadata := range writeReq.Metadata {
		prwParser.metricTypesCache.AddMetadata(metadata.MetricFamilyName, metadata)
	}

	for _, ts := range writeReq.Timeseries {
		metricName, filteredLabels := tools.GetMetricNameAndFilteredLabels(&ts.Labels)
		metricFamilyName := tools.GetBaseMetricFamilyName(metricName)

		// Determine metric type using cache if available, otherwise use label analysis
		metricMetadata, ok := prwParser.metricTypesCache.Get(metricName)
		if !ok {
			metricType := tools.GetMetricTypeByLabels(&ts.Labels)
			metricMetadata = prwParser.metricTypesCache.AddHeuristic(metricName, prompb.MetricMetadata{
				// TODO gotta add unit and help too I think?
				MetricFamilyName: metricFamilyName,
				Type:             metricType,
			})
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

func (prwParser *PrometheusRemoteOtelParser) TransformPrwToOtel(context context.Context, parsedPrwMetrics map[string][]MetricData) (pmetric.Metrics, error) {
	// TODO hughesjj oooffff... so do we shove it all into one "metrics" I suppose?
	metric := pmetric.NewMetrics()
	var errors []error
	for metricFamily, metrics := range parsedPrwMetrics {
		rm := metric.ResourceMetrics().AppendEmpty()
		err := prwParser.addMetrics(rm, metricFamily, metrics)
		if err != nil {
			// TODO hughesjj I think I need to better pass context?  Also no idea why this takes a context...
			errors = append(errors, err)
			prwParser.Reporter.OnTranslationError(context, err)
		}
	}
	if errors != nil {
		return metric, multierror.Wrap(errors)
	}
	return metric, nil
}

func (prwParser *PrometheusRemoteOtelParser) addMetrics(rm pmetric.ResourceMetrics, family string, metrics []MetricData) error {
	// TODO hughesjj refactor just getting it working for now
	// if type is gauge then add new scope metric

	// TODO hughesjj cast to int if essentially int... maybe?  idk they do it in sfx.gateway
	ilm := rm.ScopeMetrics().AppendEmpty()
	// TODO hughesjj should I set typeStr here?
	//ilm.Scope().SetName("simpleprometheusremotewritereceiver")
	for _, metricsData := range metrics {
		nm := ilm.Metrics().AppendEmpty()
		nm.SetUnit(metricsData.MetricMetadata.Unit)
		nm.SetDescription(metricsData.MetricMetadata.GetHelp())
		if metricsData.MetricName != "" {
			nm.SetName(metricsData.MetricName)
		} else {
			nm.SetName(family)
		}
		switch metricType := metricsData.MetricMetadata.Type; metricType {
		case prompb.MetricMetadata_GAUGE, prompb.MetricMetadata_UNKNOWN:
			gauge := nm.SetEmptyGauge()
			for _, sample := range *metricsData.Samples {
				dp := gauge.DataPoints().AppendEmpty()
				dp.SetDoubleValue(sample.Value)
				dp.SetTimestamp(pcommon.Timestamp(sample.Timestamp * int64(time.Millisecond)))
				for _, attr := range *metricsData.Labels {
					dp.Attributes().PutStr(attr.Name, attr.Value)
				}
				// Fairly certain gauge is byref here
			}
		case prompb.MetricMetadata_COUNTER:
			sumMetric := nm.SetEmptySum()
			// TODO hughesjj No idea how correct this is, but scraper always sets this way.  could totally see PRW being different
			sumMetric.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			// TODO hughesjj No idea how correct this is, but scraper always sets this way.  could totally see PRW being different
			sumMetric.SetIsMonotonic(true)
			for _, sample := range *metricsData.Samples {
				counter := nm.Sum().DataPoints().AppendEmpty()
				counter.SetDoubleValue(sample.Value)
				counter.SetTimestamp(pcommon.Timestamp(sample.Timestamp * int64(time.Millisecond)))
				for _, attr := range *metricsData.Labels {
					counter.Attributes().PutStr(attr.Name, attr.Value)
				}
				// Fairly certain counter is byref here
			}
		case prompb.MetricMetadata_HISTOGRAM:
			histogramDps := nm.SetEmptyHistogram()
			for _, sample := range *metricsData.Histograms {
				dp := histogramDps.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetStartTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetSum(sample.GetSum())
				dp.SetCount(sample.GetCountInt())
			}
		case prompb.MetricMetadata_SUMMARY:
			summaryDps := nm.SetEmptySummary()

			for _, sample := range *metricsData.Samples {
				dp := summaryDps.DataPoints().AppendEmpty()
				sample.Descriptor()
				dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetStartTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetSum(sample.GetValue())
				dp.SetCount(uint64(sample.Size()))
			}

		case prompb.MetricMetadata_INFO, prompb.MetricMetadata_STATESET:
			// set as SUM but not monotonic
			sumMetric := nm.SetEmptySum()
			sumMetric.SetIsMonotonic(false)
			sumMetric.SetAggregationTemporality(pmetric.AggregationTemporalityUnspecified)

			for _, sample := range *metricsData.Samples {
				dp := sumMetric.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetDoubleValue(sample.GetValue()) // TODO hughesjj maybe see if can be intvalue
			}

		default:
			// unsupported so obsreport only
			//prwParser.Reporter.OnTranslationError(prwParser.con)
			// TODO hughesjj figure out how to better obs report stuff
		}

	}
	return nil
}
