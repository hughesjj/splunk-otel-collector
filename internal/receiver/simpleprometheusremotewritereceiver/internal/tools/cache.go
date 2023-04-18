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
	"context"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lru"
	"github.com/prometheus/prometheus/prompb"
)

type setFrom uint8

// iota in ascending order of confidence
const (
	Unset setFrom = iota
	Heuristic
	Metadata
)

type MetadataCache struct {
	prompb.MetricMetadata
	MetadataSource setFrom
}

type PrometheusMetricTypeCache struct {
	prwMdCache *cache.Cache[string, MetadataCache]
}

func NewPrometheusMetricTypeCache(ctx context.Context, capacity int) *PrometheusMetricTypeCache {
	lruCache := cache.NewContext(ctx, cache.AsLRU[string, MetadataCache](lru.WithCapacity(capacity)))
	return &PrometheusMetricTypeCache{
		prwMdCache: lruCache,
	}
}

func CoalesceAndMergeMetadata(mds ...MetadataCache) MetadataCache {
	var result MetadataCache
	for _, md := range mds {
		if result.Unit == "" {
			result.Unit = md.Unit
		}
		if result.Type == prompb.MetricMetadata_UNKNOWN || result.MetadataSource <= md.MetadataSource && (result.Type == prompb.MetricMetadata_GAUGE || result.Type == prompb.MetricMetadata_COUNTER) && (md.Type == prompb.MetricMetadata_HISTOGRAM || md.Type == prompb.MetricMetadata_GAUGEHISTOGRAM || md.Type == prompb.MetricMetadata_SUMMARY) {
			result.Type = md.Type
			result.MetadataSource = md.MetadataSource
		}
		if result.Help == "" {
			result.Help = md.Help
		}
		if result.MetricFamilyName == "" {
			result.MetricFamilyName = md.MetricFamilyName
		}
	}
	return result
}

func (prwCache *PrometheusMetricTypeCache) AddMetadata(metricFamily string, metadata prompb.MetricMetadata) prompb.MetricMetadata {
	// TODO hughesjj If the old metricfamily is bunk, gotta update I suppose?   link them?
	// If metricFamily (as parsed from dealio) does not match expected metadata.MetricFamilyName, we gotta cross link them
	// alternatively, we could do metricName (instead of metric family name) -> metricFamilyName
	// really we just wanna merge right? like upsert
	if metadata.MetricFamilyName == "" {
		metadata.MetricFamilyName = metricFamily
	}
	md := MetadataCache{
		MetricMetadata: metadata,
		MetadataSource: Metadata,
	}
	heuristic, _ := prwCache.prwMdCache.Get(metricFamily)
	explicit, _ := prwCache.prwMdCache.Get(metadata.MetricFamilyName)
	md = CoalesceAndMergeMetadata(md, explicit, heuristic)
	if md.MetricMetadata.MetricFamilyName != metricFamily {
		prwCache.prwMdCache.Set(metadata.MetricFamilyName, md)
	}
	prwCache.prwMdCache.Set(metricFamily, md)
	cached, _ := prwCache.Get(metricFamily)
	return cached
}

func (prwCache *PrometheusMetricTypeCache) AddHeuristic(metricFamily string, metadata prompb.MetricMetadata) prompb.MetricMetadata {
	// TODO what if metricFamily is not metdata.MetricFamilyName?
	if metadata.MetricFamilyName == "" {
		metadata.MetricFamilyName = metricFamily
	}
	mf, exists := prwCache.prwMdCache.Get(metricFamily)
	md := MetadataCache{
		MetricMetadata: metadata,
		MetadataSource: Heuristic,
	}
	if !exists {
		prwCache.prwMdCache.Set(metricFamily, md)
	} else {
		prwCache.prwMdCache.Set(metricFamily, CoalesceAndMergeMetadata(mf, md))
	}
	result, _ := prwCache.Get(metricFamily)
	return result
}

func (prwCache *PrometheusMetricTypeCache) Get(metricFamily string) (prompb.MetricMetadata, bool) {
	a, exists := prwCache.prwMdCache.Get(metricFamily)
	return a.MetricMetadata, exists
}
