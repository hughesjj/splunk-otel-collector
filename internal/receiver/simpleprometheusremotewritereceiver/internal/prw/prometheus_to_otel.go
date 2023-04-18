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
	"fmt"
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
	metricFamiliesAndData, err := prwParser.partitionWriteRequest(ctx, request)
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
	MetricName     string
	Labels         []prompb.Label
	Samples        []prompb.Sample
	Exemplars      []prompb.Exemplar
	Histograms     []prompb.Histogram
	MetricMetadata prompb.MetricMetadata
	FromMd         bool
}

type PrometheusRemoteOtelParser struct {
	metricTypesCache *tools.PrometheusMetricTypeCache
	Reporter         transport.Reporter
	context.Context
}

const maxCachedMetadata = 10000

func NewPrwOtelParser(ctx context.Context, reporter transport.Reporter) (*PrometheusRemoteOtelParser, error) {
	return &PrometheusRemoteOtelParser{
		metricTypesCache: tools.NewPrometheusMetricTypeCache(ctx, maxCachedMetadata),
		Reporter:         reporter,
		Context:          ctx,
	}, nil
}

// The ordering of the timeseries in the WriteRequest(s) matters significantly.
// Ex for histograms you need the metrics with an le attribute to appear first
func (prwParser *PrometheusRemoteOtelParser) partitionWriteRequest(ctx context.Context, writeReq *prompb.WriteRequest) (map[string][]MetricData, error) {
	partitions := make(map[string][]MetricData)

	if len(writeReq.Timeseries) >= maxCachedMetadata {
		err := fmt.Errorf("CANNOT CACHE ALL METADATA -- REQUEST TOO LARGE.  Max %d but given %d", maxCachedMetadata, len(writeReq.Metadata))
		prwParser.Reporter.OnTranslationError(ctx, err)
		return nil, err
	}

	// update cache with any metric data found.  Do this before processing any data to avoid incorrect heuristics.
	for _, metadata := range writeReq.Metadata {
		prwParser.metricTypesCache.AddMetadata(metadata.MetricFamilyName, metadata)
	}
	// TODO hughesjj so wait.... Are there any guarantees about the length of Metadata w.r.t TimeSeries?
	for _, ts := range writeReq.Timeseries {
		metricName, filteredLabels := tools.GetMetricNameAndFilteredLabels(ts.Labels)
		metricFamilyName := tools.GetBaseMetricFamilyName(metricName)

		// Determine metric type using cache if available, otherwise use label heuristics
		metricType := tools.GuessMetricTypeByLabels(ts.Labels)
		metricMetadata := prwParser.metricTypesCache.AddHeuristic(metricName, prompb.MetricMetadata{
			// Note we cannot intuit Help nor Unit from the labels
			MetricFamilyName: metricFamilyName,
			Type:             metricType,
		})

		// Add the parsed time-series data to the corresponding partition
		// Might be nice to freeze and assign MetricMetadata after this loop has had the chance to "maximally cache" it all
		partitions[metricFamilyName] = append(partitions[metricFamilyName], MetricData{
			Labels:         filteredLabels,
			Samples:        ts.Samples,
			Exemplars:      ts.Exemplars,
			Histograms:     ts.Histograms,
			MetricName:     metricName,
			MetricMetadata: metricMetadata,
		})
	}
	// TODO hughesjj so... should we sort on quantiles etc here?
	for metricFamilyName, data := range partitions {
		// TODO hughesjj wait do we even want this?
		// I think there's a slim but nonzero chance we got a better read on the heuristics later on for a given metricFamilyName
		metricMetadata, found := prwParser.metricTypesCache.Get(metricFamilyName)
		if found {
			for index := range data {
				partitions[metricFamilyName][index].MetricMetadata = metricMetadata
			}
		}
	}

	return partitions, nil
}

func (prwParser *PrometheusRemoteOtelParser) TransformPrwToOtel(context context.Context, parsedPrwMetrics map[string][]MetricData) (pmetric.Metrics, error) {
	metric := pmetric.NewMetrics()
	var translationErrors []error
	for metricFamily, metrics := range parsedPrwMetrics {
		rm := metric.ResourceMetrics().AppendEmpty()
		err := prwParser.addMetrics(context, rm, metricFamily, metrics)
		if err != nil {
			translationErrors = append(translationErrors, err)
			prwParser.Reporter.OnTranslationError(context, err)
		}
	}
	if translationErrors != nil {
		return metric, multierror.Wrap(translationErrors)
	}
	return metric, nil
}

// This actually converts from a prometheus prompdb.MetaDataType to the closest equivalent otel type
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/13bcae344506fe2169b59d213361d04094c651f6/receiver/prometheusreceiver/internal/util.go#L106
func (prwParser *PrometheusRemoteOtelParser) addMetrics(context context.Context, rm pmetric.ResourceMetrics, family string, metrics []MetricData) error {
	// TODO hughesjj cast to int if essentially int... maybe?  idk they do it in sfx.gateway
	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(tools.TypeStr)
	ilm.Scope().SetVersion("0.1")

	var translationErrors []error

	for _, metricsData := range metrics {
		nm := ilm.Metrics().AppendEmpty()
		nm.SetUnit(metricsData.MetricMetadata.Unit)
		nm.SetDescription(metricsData.MetricMetadata.GetHelp())
		if metricsData.MetricName != "" {
			nm.SetName(metricsData.MetricName)
		} else {
			// TODO hughesjj should prolly warn on this
			nm.SetName(family)
		}
		switch metricType := metricsData.MetricMetadata.Type; metricType {
		case prompb.MetricMetadata_GAUGE, prompb.MetricMetadata_UNKNOWN:
			gauge := nm.SetEmptyGauge()
			for _, sample := range metricsData.Samples {
				dp := gauge.DataPoints().AppendEmpty()
				dp.SetDoubleValue(sample.Value)
				dp.SetTimestamp(pcommon.Timestamp(sample.Timestamp * int64(time.Millisecond)))
				for _, attr := range metricsData.Labels {
					dp.Attributes().PutStr(attr.Name, attr.Value)
				}
			}
		case prompb.MetricMetadata_COUNTER:
			sumMetric := nm.SetEmptySum()
			// TODO hughesjj No idea how correct this is, but scraper always sets this way.  could totally see PRW being different
			sumMetric.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			// TODO hughesjj No idea how correct this is, but scraper always sets this way.  could totally see PRW being different
			sumMetric.SetIsMonotonic(true)
			for _, sample := range metricsData.Samples {
				counter := nm.Sum().DataPoints().AppendEmpty()
				counter.SetDoubleValue(sample.Value)
				counter.SetTimestamp(pcommon.Timestamp(sample.Timestamp * int64(time.Millisecond)))
				for _, attr := range metricsData.Labels {
					counter.Attributes().PutStr(attr.Name, attr.Value)
				}
				// Fairly certain counter is byref here
			}
		case prompb.MetricMetadata_HISTOGRAM:
			histogramDps := nm.SetEmptyHistogram()
			for _, sample := range metricsData.Histograms {
				dp := histogramDps.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetStartTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetSum(sample.GetSum())
				dp.SetCount(sample.GetCountInt())
			}
		case prompb.MetricMetadata_SUMMARY:
			summaryDps := nm.SetEmptySummary()

			for _, sample := range metricsData.Samples {
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

			for _, sample := range metricsData.Samples {
				dp := sumMetric.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetDoubleValue(sample.GetValue()) // TODO hughesjj maybe see if can be intvalue
			}

		default:
			err := fmt.Errorf("unsupported type %s", metricType)
			translationErrors = append(translationErrors, err)
			prwParser.Reporter.OnTranslationError(context, err)

		}

	}
	if translationErrors != nil {
		return multierror.Wrap(translationErrors)
	}
	return nil
}
