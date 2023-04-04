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
	"strconv"
	"testing"

	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
)

func TestCacheAccessPatterns(t *testing.T) {
	expectedCapacity := 10
	pmtCache := NewPrometheusMetricTypeCache(expectedCapacity)
	assert.NotNil(t, pmtCache)

	// empty cache should return nothing but throw no errors either
	_, notFound := pmtCache.Get("0")
	assert.True(t, notFound)

	// Add single element, ensure it's there
	pmtCache.AddMetadata("0", prompb.MetricMetadata{Type: prompb.MetricMetadata_HISTOGRAM})
	_, notFound = pmtCache.Get("0")
	assert.False(t, notFound)

	for i := 1; i <= expectedCapacity; i++ {
		pmtCache.AddMetadata(strconv.Itoa(i), prompb.MetricMetadata{Type: prompb.MetricMetadata_COUNTER})
	}

	// ensure eviction of least recently used
	_, notFound = pmtCache.Get("0")
	assert.False(t, notFound)

	// Ensure latest is on there
	value, notFound := pmtCache.Get(strconv.Itoa(expectedCapacity + 1))
	assert.Truef(t, notFound, "Missing most recently used from an LRU cache =(")
	assert.Equal(t, prompb.MetricMetadata_COUNTER, value)

	// Ensure heuristic doesn't override an explicitly set metadata
	value, ok := pmtCache.AddHeuristic(strconv.Itoa(expectedCapacity+1), prompb.MetricMetadata{Type: prompb.MetricMetadata_HISTOGRAM})
	assert.True(t, ok)
	assert.Equal(t, value, prompb.MetricMetadata_COUNTER)

	// as an initial value it's fine to add it
	value, notFound = pmtCache.AddHeuristic("HeursticFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_GAUGE})
	assert.False(t, notFound)
	assert.Equal(t, value, prompb.MetricMetadata_GAUGE)

	// It should be overridden by any Explicit metadata though
	value, notFound = pmtCache.AddMetadata("HeursticFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_HISTOGRAM})
	assert.False(t, notFound)
	assert.Equal(t, value, prompb.MetricMetadata_HISTOGRAM)

	// If they give us conflicting explicit metadata, we should trust their latest
	value, notFound = pmtCache.AddMetadata("HeursticFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_SUMMARY})
	assert.False(t, notFound)
	assert.Equal(t, value, prompb.MetricMetadata_SUMMARY)

	// Unless they give us literal junk
	value, notFound = pmtCache.AddMetadata("HeursticFirst", prompb.MetricMetadata{Type: prompb.MetricMetadata_UNKNOWN})
	assert.False(t, notFound)
	assert.Equal(t, value, prompb.MetricMetadata_SUMMARY)
}
