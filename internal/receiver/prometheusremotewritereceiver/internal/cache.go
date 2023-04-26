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

package internal

import (
	"context"
	"sync"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Code-Hex/go-generics-cache/policy/lru"
	mapset "github.com/deckarep/golang-set/v2"
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

type HeuristicTransition int64
type HeuristicTransitionMap []HeuristicTransition

func GetMask(metricType prompb.MetricMetadata_MetricType) HeuristicTransition {
	return 1 << metricType
}

func makeHeuristicTransitions() HeuristicTransitionMap {
	// Creates a type mask where the i_th bit is 1 iff we can transition to that state
	// indexed by numeric representation of the prompb type
	typeMask := make([]HeuristicTransition, prompb.MetricMetadata_STATESET+1)
	// Seed with ability to transition into self (makes reuse easier)
	for i := 0; i < len(typeMask); i++ {
		typeMask[i] = 1 << i
	}
	typeMask[prompb.MetricMetadata_HISTOGRAM] |= GetMask(prompb.MetricMetadata_GAUGEHISTOGRAM)
	typeMask[prompb.MetricMetadata_COUNTER] |= typeMask[prompb.MetricMetadata_HISTOGRAM] | GetMask(prompb.MetricMetadata_SUMMARY)
	typeMask[prompb.MetricMetadata_GAUGE] |= typeMask[prompb.MetricMetadata_COUNTER]
	typeMask[prompb.MetricMetadata_UNKNOWN] |= typeMask[prompb.MetricMetadata_GAUGE] | GetMask(prompb.MetricMetadata_INFO) | GetMask(prompb.MetricMetadata_STATESET)
	return typeMask
}

var (
	HeuristicTransitions = makeHeuristicTransitions()
)

func CanTransition(from prompb.MetricMetadata_MetricType, to prompb.MetricMetadata_MetricType) bool {
	return HeuristicTransitions[from]&GetMask(to) > 0
}

type PrometheusMetricTypeCache struct {
	prwMdCache *cache.Cache[string, MetadataCache]
	// Basically used for disjoint set pattern/datastructure.  Makes maintaining single source of truth easier.
	metricToFamilyMapping *sync.Map
	frozenKeys            mapset.Set[string]
}

func NewPrometheusMetricTypeCache(ctx context.Context, capacity int) *PrometheusMetricTypeCache {
	lruCache := cache.NewContext(ctx, cache.AsLRU[string, MetadataCache](lru.WithCapacity(capacity)))
	return &PrometheusMetricTypeCache{
		prwMdCache:            lruCache,
		metricToFamilyMapping: &sync.Map{},
		frozenKeys:            mapset.NewSet[string](),
	}
}

func CoalesceAndMergeMetadata(mds ...MetadataCache) MetadataCache {
	var result MetadataCache
	for _, md := range mds {
		if result.Type == prompb.MetricMetadata_UNKNOWN || (result.MetadataSource <= md.MetadataSource) && CanTransition(result.Type, md.Type) {
			result.Type = md.Type
			result.MetadataSource = md.MetadataSource
		}
		if result.MetricFamilyName == "" || len(result.MetricFamilyName) > len(md.MetricFamilyName) {
			result.MetricFamilyName = md.MetricFamilyName
		}
		if result.Unit == "" {
			result.Unit = md.Unit
		}
		if result.Help == "" {
			result.Help = md.Help
		}
	}
	return result
}

func (prwCache *PrometheusMetricTypeCache) AddMetadata(metricName string, metadata prompb.MetricMetadata) prompb.MetricMetadata {
	return prwCache.storeMetadata(metricName, metadata, Metadata)
}

func (prwCache *PrometheusMetricTypeCache) storeMetadata(metricName string, metadata prompb.MetricMetadata, from setFrom) prompb.MetricMetadata {
	explicitNameMetadata, found := prwCache.prwMdCache.Get(metricName)
	if metadata.MetricFamilyName == "" {
		if found && explicitNameMetadata.MetricFamilyName != "" {
			metadata.MetricFamilyName = explicitNameMetadata.MetricFamilyName
		} else {
			metadata.MetricFamilyName = GetBaseMetricFamilyName(metricName)
		}
	}
	if metadata.MetricFamilyName != explicitNameMetadata.MetricFamilyName && explicitNameMetadata.MetricFamilyName == "" {
		explicitNameMetadata.MetricFamilyName = metadata.MetricFamilyName
	}
	var toMerge = []MetadataCache{explicitNameMetadata}

	familyName := prwCache.findRoot(metadata.MetricFamilyName)
	prwCache.connectMetricToFamily(metricName, familyName)

	familyNameMetadata, found := prwCache.prwMdCache.Get(familyName)
	if found {
		toMerge = append(toMerge, familyNameMetadata)
	}
	if prwCache.frozenKeys.Contains(metricName) || prwCache.frozenKeys.Contains(familyName) {
		if !found {
			prwCache.frozenKeys.Remove(metricName)
			prwCache.frozenKeys.Remove(familyName)
			// TODO hughesjj maybe return error?
		} else {
			return explicitNameMetadata.MetricMetadata
		}
	}
	md := MetadataCache{
		MetricMetadata: metadata,
		MetadataSource: from,
	}
	toMerge = append(toMerge, md)
	merged := CoalesceAndMergeMetadata(toMerge...)
	prwCache.prwMdCache.Set(merged.MetricFamilyName, merged)
	return merged.MetricMetadata
}

func (prwCache *PrometheusMetricTypeCache) AddHeuristic(metricName string, metadata prompb.MetricMetadata) prompb.MetricMetadata {
	return prwCache.storeMetadata(metricName, metadata, Heuristic)
}

func (prwCache *PrometheusMetricTypeCache) Get(metricName string) (prompb.MetricMetadata, bool) {
	metricDataRoot := prwCache.findRoot(metricName)
	parentMd, found := prwCache.prwMdCache.Get(metricDataRoot)
	return parentMd.MetricMetadata, found
}

// findRoot implement the find_root part of the disjoint set pattern/datastructure.
// Base case: identifier has parent (is root node).  This is denoted as the metricName and metricFamilyName being the same.
// Calling this will recursively walk up the tree & reduce the height of any chains
func (prwCache *PrometheusMetricTypeCache) findRoot(metricName string) string {
	// implements disjoint set findroot method
	parent, loaded := prwCache.metricToFamilyMapping.LoadOrStore(metricName, metricName)
	if !loaded || parent.(string) == metricName {
		return metricName
	}
	parentRoot := prwCache.findRoot(parent.(string))
	if parentRoot != parent.(string) {
		prwCache.metricToFamilyMapping.Store(metricName, parentRoot)
	}
	return parentRoot
}

// connect part of disjointset pattern.  Here we take advantage of familyName always having a shorter tree to avoid
// storing ranks etc. for balancing.  In practice should never be of height more than 2
func (prwCache *PrometheusMetricTypeCache) connectMetricToFamily(metricName string, familyName string) string {
	metricRoot := prwCache.findRoot(metricName)
	familyRoot := prwCache.findRoot(familyName)
	if metricRoot == familyRoot {
		return metricRoot
	}
	prwCache.metricToFamilyMapping.Store(metricName, familyRoot)
	return prwCache.findRoot(metricName)
}
