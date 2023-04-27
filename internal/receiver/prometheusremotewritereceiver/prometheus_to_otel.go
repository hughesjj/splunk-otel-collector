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

package prometheusremotewritereceiver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jaegertracing/jaeger/pkg/multierror"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/prometheusremotewritereceiver/internal"
)

func (prwParser *PrometheusRemoteOtelParser) FromPrometheusWriteRequestMetrics(ctx context.Context, request *prompb.WriteRequest) (pmetric.Metrics, error) {
	var otelMetrics pmetric.Metrics
	metricFamiliesAndData, err := prwParser.partitionWriteRequest(ctx, request)
	if nil == err {
		otelMetrics, err = prwParser.TransformPrwToOtel(ctx, metricFamiliesAndData)
		if nil == err {
			prwParser.Reporter.OnMetricsProcessed(ctx, otelMetrics.DataPointCount(), err)
		} else {
			prwParser.Reporter.OnError(ctx, "prometheus_translation", err)
		}
	} else {
		prwParser.Reporter.OnError(ctx, "prometheus_translation", err)
	}
	return otelMetrics, err
}

type MetricData struct {
	MetricName     string
	Labels         *[]prompb.Label
	Samples        *[]prompb.Sample
	Exemplars      *[]prompb.Exemplar
	Histograms     *[]prompb.Histogram
	MetricMetadata prompb.MetricMetadata
}

type PrometheusRemoteOtelParser struct {
	metricTypesCache *internal.PrometheusMetricTypeCache
	Reporter         reporter
	context.Context
}

func NewPrwOtelParser(ctx context.Context, reporter reporter, capacity int) (*PrometheusRemoteOtelParser, error) {
	return &PrometheusRemoteOtelParser{
		metricTypesCache: internal.NewPrometheusMetricTypeCache(ctx, capacity),
		Reporter:         reporter,
		Context:          ctx,
	}, nil
}

// The ordering of the timeseries in the WriteRequest(s) matters significantly.
// Ex for histograms you need the metrics with an le attribute to appear first
// This function is written to prefer idempotence/maintainability over efficiency, at least until we vet the methodology
func (prwParser *PrometheusRemoteOtelParser) partitionWriteRequest(_ context.Context, writeReq *prompb.WriteRequest) (map[string][]MetricData, error) {
	partitions := make(map[string][]MetricData)

	// update cache with any metric data found.  Do this before processing any data to avoid incorrect heuristics.
	// TODO hughesjj so wait.... Are there any guarantees about the length of Metadata w.r.t TimeSeries?
	for _, metadata := range writeReq.Metadata {
		prwParser.metricTypesCache.AddMetadata(metadata.MetricFamilyName, metadata)
	}

	indexToMetricNameMapping := make([]MetricData, len(writeReq.Timeseries))
	for index, ts := range writeReq.Timeseries {
		metricName, filteredLabels := internal.GetMetricNameAndFilteredLabels(ts.Labels)
		metricFamilyName := internal.GetBaseMetricFamilyName(metricName)

		// Determine metric type using cache if available, otherwise use label heuristics
		metricType := internal.GuessMetricTypeByLabels(ts.Labels)
		prwParser.metricTypesCache.AddHeuristic(metricName, prompb.MetricMetadata{
			// Note we cannot intuit Help nor Unit from the labels
			MetricFamilyName: metricFamilyName,
			Type:             metricType,
		})
		indexToMetricNameMapping[index] = MetricData{
			Labels:     &filteredLabels,
			Samples:    &writeReq.Timeseries[index].Samples,
			Exemplars:  &writeReq.Timeseries[index].Exemplars,
			Histograms: &writeReq.Timeseries[index].Histograms,
			MetricName: metricName,
		}
	}
	for index, metricData := range indexToMetricNameMapping {
		// Add the parsed time-series data to the corresponding partition
		// Might be nice to freeze and assign MetricMetadata after this loop has had the chance to "maximally cache" it all
		metricMetaData, found := prwParser.metricTypesCache.Get(metricData.MetricName)
		if !found {
			return nil, errors.New("either cache too small to handle write request or bug in code prometheus remote write receiver metadata caching")
		}
		indexToMetricNameMapping[index].MetricMetadata = metricMetaData
		metricData = indexToMetricNameMapping[index]
		partitions[metricData.MetricMetadata.MetricFamilyName] = append(partitions[metricData.MetricMetadata.MetricFamilyName], metricData)
	}

	// TODO hughesjj okay what about if it isn't a valid write request as per conventions?

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
			prwParser.Reporter.OnError(context, "prometheus_translation", err)
		}
	}
	if translationErrors != nil {
		return metric, multierror.Wrap(translationErrors)
	}
	return metric, nil
}

// This actually converts from a prometheus prompdb.MetaDataType to the closest equivalent otel type
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/13bcae344506fe2169b59d213361d04094c651f6/receiver/prometheusreceiver/internal/util.go#L106
func (prwParser *PrometheusRemoteOtelParser) addMetrics(prwContext context.Context, rm pmetric.ResourceMetrics, family string, metrics []MetricData) error {
	// TODO hughesjj cast to int if essentially int... maybe?  idk they do it in sfx.gateway
	ilm := rm.ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(internal.TypeStr)
	ilm.Scope().SetVersion("0.1")

	metricsMetadata, found := prwParser.metricTypesCache.Get(family)
	if !found {
		return fmt.Errorf("could not find metadata for family %s", family)
	}

	var translationErrors []error
	switch metricsMetadata.Type {
	case prompb.MetricMetadata_GAUGE, prompb.MetricMetadata_UNKNOWN:
		prwParser.addGaugeMetrics(ilm, family, &metrics, metricsMetadata)
	case prompb.MetricMetadata_COUNTER:
		prwParser.addCounterMetrics(ilm, family, &metrics, metricsMetadata)
	case prompb.MetricMetadata_HISTOGRAM:
		prwParser.addHistogramMetrics(prwParser, ilm, family, &metrics, metricsMetadata)
	case prompb.MetricMetadata_SUMMARY:
		prwParser.addSummaryMetrics(ilm, family, &metrics, metricsMetadata)
	case prompb.MetricMetadata_INFO, prompb.MetricMetadata_STATESET:
		prwParser.addInfoStateset(ilm, family, &metrics, metricsMetadata)
	default:
		err := fmt.Errorf("unsupported type %s for metric family %s", metricsMetadata.Type, family)
		translationErrors = append(translationErrors, err)
		prwParser.Reporter.OnError(prwContext, "prometheus_translation", err)
	}
	if translationErrors != nil {
		return multierror.Wrap(translationErrors)
	}
	return nil
}

func (prwParser *PrometheusRemoteOtelParser) scaffoldNewMetric(ilm pmetric.ScopeMetrics, family string, metricsMetadata prompb.MetricMetadata) pmetric.Metric {
	nm := ilm.Metrics().AppendEmpty()
	nm.SetUnit(metricsMetadata.Unit)
	nm.SetDescription(metricsMetadata.GetHelp())
	nm.SetName(family)
	return nm
}

func (prwParser *PrometheusRemoteOtelParser) addHistogramMetrics(prwContext context.Context, ilm pmetric.ScopeMetrics, family string, metrics *[]MetricData, metadata prompb.MetricMetadata) {
	nm := prwParser.scaffoldNewMetric(ilm, family, metadata)

	for _, metricsData := range *metrics {
		if metricsData.MetricName != "" {
			nm.SetName(metricsData.MetricName)
		}
		histogramDps := nm.SetEmptyHistogram()
		for _, sample := range *metricsData.Histograms {
			dp := histogramDps.DataPoints().AppendEmpty()
			// TODO get max (likely)
			dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
			// TODO get min
			dp.SetStartTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
			dp.SetSum(sample.GetSum())
			dp.SetCount(sample.GetCountInt())
			for _, attr := range *metricsData.Labels {
				if attr.Name == "le" {
					val, err := strconv.Atoi(attr.Value)
					if err != nil {
						prwParser.Reporter.OnError(prwContext, "prometheus_translation_histogram", err)
					} else {
						// Okay is this actual observed max or simply upperbound?
						dp.SetMax(float64(val))
					}
				} else {
					dp.Attributes().PutStr(attr.Name, attr.Value)
				}
			}
			for _, sample := range *metricsData.Samples {
				dp := histogramDps.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				dp.SetStartTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
				for _, attr := range *metricsData.Labels {
					if attr.Name == "le" {
						val, err := strconv.Atoi(attr.Value)
						if err != nil {
							prwParser.Reporter.OnError(prwContext, "prometheus_translation_histogram", err)
						} else {
							// Okay is this actual observed max or simply upperbound?
							dp.SetMax(float64(val))
						}
					} else {
						dp.Attributes().PutStr(attr.Name, attr.Value)
					}
				}
			}
		}
	}
}

func (prwParser *PrometheusRemoteOtelParser) addSummaryMetrics(ilm pmetric.ScopeMetrics, family string, metrics *[]MetricData, metadata prompb.MetricMetadata) {
	// TODO hughesjj better way to access
	nm := prwParser.scaffoldNewMetric(ilm, family, metadata)
	for _, metricsData := range *metrics {
		if metricsData.MetricName != "" {
			nm.SetName(metricsData.MetricName)
		}
		summaryDps := nm.SetEmptySummary()

		for _, sample := range *metricsData.Samples {
			dp := summaryDps.DataPoints().AppendEmpty()
			sample.Descriptor()
			dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
			dp.SetStartTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
			dp.SetSum(sample.GetValue())
			dp.SetCount(uint64(sample.Size()))
			for _, attr := range *metricsData.Labels {
				dp.Attributes().PutStr(attr.Name, attr.Value)
			}
		}
	}
}

func (prwParser *PrometheusRemoteOtelParser) addGaugeMetrics(ilm pmetric.ScopeMetrics, family string, metrics *[]MetricData, metadata prompb.MetricMetadata) {
	// TODO hughesjj better way to access
	nm := prwParser.scaffoldNewMetric(ilm, family, metadata)
	for _, metricsData := range *metrics {
		if metricsData.MetricName != "" {
			nm.SetName(metricsData.MetricName)
		}
		gauge := nm.SetEmptyGauge()
		for _, sample := range *metricsData.Samples {
			dp := gauge.DataPoints().AppendEmpty()
			dp.SetDoubleValue(sample.Value)
			dp.SetTimestamp(pcommon.Timestamp(sample.Timestamp * int64(time.Millisecond)))
			for _, attr := range *metricsData.Labels {
				dp.Attributes().PutStr(attr.Name, attr.Value)
			}
		}
	}
}

func (prwParser *PrometheusRemoteOtelParser) addCounterMetrics(ilm pmetric.ScopeMetrics, family string, metrics *[]MetricData, metadata prompb.MetricMetadata) {
	// TODO hughesjj better way to access
	nm := prwParser.scaffoldNewMetric(ilm, family, metadata)
	for _, metricsData := range *metrics {
		// TODO yeah no
		if metricsData.MetricName != "" {
			nm.SetName(metricsData.MetricName)
		}
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
	}
}

func (prwParser *PrometheusRemoteOtelParser) addInfoStateset(ilm pmetric.ScopeMetrics, family string, metrics *[]MetricData, metadata prompb.MetricMetadata) {
	nm := prwParser.scaffoldNewMetric(ilm, family, metadata)
	for _, metricsData := range *metrics {
		// TODO better way to do this
		if metricsData.MetricName != "" {
			nm.SetName(metricsData.MetricName)
		}

		// set as SUM but not monotonic
		sumMetric := nm.SetEmptySum()
		sumMetric.SetIsMonotonic(false)
		sumMetric.SetAggregationTemporality(pmetric.AggregationTemporalityUnspecified)

		for _, sample := range *metricsData.Samples {
			dp := sumMetric.DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.Timestamp(sample.GetTimestamp() * int64(time.Millisecond)))
			dp.SetDoubleValue(sample.GetValue()) // TODO hughesjj maybe see if can be intvalue
			for _, attr := range *metricsData.Labels {
				dp.Attributes().PutStr(attr.Name, attr.Value)
			}
		}
	}
}

// TODO  hughesjj okay here let's factor out the above cases into different type translations
// We've already partitioned on family name at this point so should be more but not totally easy to do this
// TODO for future could
// 1. set up a different receiver just for metadata and offer a config option to hit that up & export to such from self?
// 2. allow providing config.go option or even file in config.go to "seed" metadata and the likes

// TODO hughesjj okay so next steps
// Alright probably should either do tests, add the cache for buckets and/or quantiles, or just converge your histogram impl
// probably best to make tests more stable and port over the other older ones first before going at it