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
	"strconv"
	"testing"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
)

func TestCacheAccessPatterns(t *testing.T) {
	expectedCapacity := 10
	pmtCache := NewPrometheusMetricTypeCache(context.Background(), expectedCapacity)
	assert.NotNil(t, pmtCache)

	// empty cache should return nothing but throw no errors either
	_, exists := pmtCache.Get("0")
	assert.False(t, exists)

	// Add single element, ensure it's there
	pmtCache.AddMetadata("0", prompb.MetricMetadata{
		Type: prompb.MetricMetadata_HISTOGRAM,
	})
	_, exists = pmtCache.Get("0")
	assert.True(t, exists)

	for i := 1; i <= expectedCapacity; i++ {
		t := prompb.MetricMetadata_MetricType(i % 4)
		pmtCache.AddMetadata(strconv.Itoa(i), prompb.MetricMetadata{
			Type: t,
			Unit: "some unit",
		})
	}

	// TODO FUCK yo we put it into gauge

	// ensure eviction of least recently used
	_, exists = pmtCache.Get("0")
	assert.False(t, exists)

	// Ensure latest is on there
	value, exists := pmtCache.Get(strconv.Itoa(expectedCapacity))
	assert.Truef(t, exists, "Missing most recently used from an LRU cache =(")
	assert.Equal(t, prompb.MetricMetadata_COUNTER, value.Type)
	assert.NotEmpty(t, value.MetricFamilyName)
	assert.NotEmpty(t, value.MetricFamilyName)

	// Ensure heuristic doesn't override an explicitly set metadata
	value = pmtCache.AddHeuristic(strconv.Itoa(expectedCapacity), prompb.MetricMetadata{Type: prompb.MetricMetadata_HISTOGRAM})
	assert.Equal(t, prompb.MetricMetadata_COUNTER, value.Type)
	assert.NotEmpty(t, value.MetricFamilyName)

	// as an initial value it's fine to add it
	value = pmtCache.AddHeuristic("HeuristicFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_GAUGE})
	assert.Equal(t, prompb.MetricMetadata_GAUGE, value.Type)
	assert.NotEmpty(t, value.MetricFamilyName)

	// It should be overridden by any Explicit metadata though
	value = pmtCache.AddMetadata("HeuristicFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_HISTOGRAM})
	assert.Equal(t, prompb.MetricMetadata_HISTOGRAM, value.Type)
	assert.NotEmpty(t, value.MetricFamilyName)

	// If they give us conflicting explicit metadata, we should trust their latest
	value = pmtCache.AddMetadata("HeuristicFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_SUMMARY})
	assert.Equal(t, prompb.MetricMetadata_SUMMARY, value.Type)
	assert.NotEmpty(t, value.MetricFamilyName)
	assert.NotEmpty(t, value.Unit)
	assert.NotEmpty(t, value.Help)

	// Unless they give us literal junk
	value = pmtCache.AddMetadata("HeuristicFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_UNKNOWN})
	assert.Equal(t, prompb.MetricMetadata_SUMMARY, value.Type)
	assert.NotEmpty(t, value.MetricFamilyName)
}
