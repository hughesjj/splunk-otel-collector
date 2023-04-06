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

	"github.com/Code-Hex/go-generics-cache/policy/lru"
	"github.com/prometheus/prometheus/prompb"

	cache "github.com/Code-Hex/go-generics-cache"
)

type PrometheusMetricTypeCache struct {
	prwMdCache *cache.Cache[string, prompb.MetricMetadata]
}

func NewPrometheusMetricTypeCache(capacity int) *PrometheusMetricTypeCache {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lruCache := cache.NewContext(ctx, cache.AsLRU[string, prompb.MetricMetadata](lru.WithCapacity(capacity)))
	return &PrometheusMetricTypeCache{
		prwMdCache: lruCache,
	}
}

func (prwCache *PrometheusMetricTypeCache) AddMetadata(metricFamily string, metadata prompb.MetricMetadata) (prompb.MetricMetadata, bool) {
	if metadata.Type != prompb.MetricMetadata_UNKNOWN || !prwCache.prwMdCache.Contains(metricFamily) {
		prwCache.prwMdCache.Set(metricFamily, metadata)
	}
	return prwCache.Get(metricFamily)
}

func (prwCache *PrometheusMetricTypeCache) AddHeuristic(metricFamily string, metadata prompb.MetricMetadata) (prompb.MetricMetadata, bool) {
	if !prwCache.prwMdCache.Contains(metricFamily) {
		prwCache.prwMdCache.Set(metricFamily, metadata)
	}
	return prwCache.Get(metricFamily)
}

func (prwCache *PrometheusMetricTypeCache) Get(metricFamily string) (prompb.MetricMetadata, bool) {
	return prwCache.prwMdCache.Get(metricFamily)
}
