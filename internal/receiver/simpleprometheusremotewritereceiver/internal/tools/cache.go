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

type PrometheusMetricTypeCache struct {
	prwMdCache *cache.Cache[string, prompb.MetricMetadata]
}

func NewPrometheusMetricTypeCache(ctx context.Context, capacity int) *PrometheusMetricTypeCache {
	lruCache := cache.NewContext(ctx, cache.AsLRU[string, prompb.MetricMetadata](lru.WithCapacity(capacity)))
	return &PrometheusMetricTypeCache{
		prwMdCache: lruCache,
	}
}

func CoalesceMetadata(mds ...prompb.MetricMetadata) prompb.MetricMetadata {
	var result prompb.MetricMetadata
	for _, md := range mds {
		if result.Unit == "" {
			result.Unit = md.Unit
		}
		if result.Type == prompb.MetricMetadata_UNKNOWN {
			result.Type = md.Type
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
	// If metricFamily (as parsed from dealio) does not match expectd metadata.MetricFamilyName, we gotta cross link them
	// alternatively, we could do metricName (instead of metric family name) -> metricFamilyName
	// really we just wanna merge right? like upsert
	if metadata.MetricFamilyName == "" {
		metadata.MetricFamilyName = metricFamily
	}
	heuristic, _ := prwCache.Get(metricFamily)
	explicit, _ := prwCache.Get(metadata.MetricFamilyName)
	metadata = CoalesceMetadata(metadata, explicit, heuristic)
	if metadata.MetricFamilyName != metricFamily {
		prwCache.prwMdCache.Set(metadata.MetricFamilyName, metadata)
	}
	prwCache.prwMdCache.Set(metricFamily, metadata)
	cached, _ := prwCache.Get(metricFamily)
	return cached
}

func (prwCache *PrometheusMetricTypeCache) AddHeuristic(metricFamily string, metadata prompb.MetricMetadata) prompb.MetricMetadata {
	if metadata.MetricFamilyName == "" {
		metadata.MetricFamilyName = metricFamily
	}
	mf, exists := prwCache.prwMdCache.Get(metricFamily)
	if !exists {
		prwCache.prwMdCache.Set(metricFamily, metadata)
	} else {
		prwCache.prwMdCache.Set(metricFamily, CoalesceMetadata(mf, metadata))
	}
	result, _ := prwCache.Get(metricFamily)
	return result
}

func (prwCache *PrometheusMetricTypeCache) Get(metricFamily string) (prompb.MetricMetadata, bool) {
	return prwCache.prwMdCache.Get(metricFamily)
}
