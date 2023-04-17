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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/testdata"
)

func TestParsePrometheusRemoteWriteRequest(t *testing.T) {
	ctx := context.Background()
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	require.NotNil(t, reporter)
	parser, err := NewPrwOtelParser(context.Background(), reporter)
	require.Nil(t, err)

	sampleWriteRequests := testdata.GetWriteRequests()
	for _, sampleWriteRequest := range sampleWriteRequests {
		partitions, err := parser.partitionWriteRequest(ctx, sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.NotEmpty(t, key)
				assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			}
		}
	}
	sampleWriteRequestsMd := testdata.GetWriteRequestsWithMetadata()
	for _, sampleWriteRequest := range sampleWriteRequestsMd {
		partitions, err := parser.partitionWriteRequest(ctx, sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			}
		}
	}
}

func TestParseAndPartitionPrometheusRemoteWriteRequest(t *testing.T) {
	ctx := context.Background()
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	require.NotNil(t, reporter)
	parser, err := NewPrwOtelParser(context.Background(), reporter)
	require.Nil(t, err)

	sampleWriteRequests := testdata.GetWriteRequests()
	for _, sampleWriteRequest := range sampleWriteRequests {
		partitions, err := parser.partitionWriteRequest(ctx, sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.NotEmpty(t, key)
				assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			}
		}
	}
	sampleWriteRequestsMd := testdata.GetWriteRequestsWithMetadata()
	for _, sampleWriteRequest := range sampleWriteRequestsMd {
		partitions, err := parser.partitionWriteRequest(ctx, sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			}
		}
		results, err := parser.TransformPrwToOtel(context.Background(), partitions)
		assert.Nil(t, err)
		assert.NotNil(t, results)
	}
}

func TestParseAndPartitionMixedPrometheusRemoteWriteRequest(t *testing.T) {
	ctx := context.Background()
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	require.NotNil(t, reporter)
	parser, err := NewPrwOtelParser(context.Background(), reporter)
	require.Nil(t, err)

	sampleWriteRequests := testdata.FlattenWriteRequests(testdata.GetWriteRequests())
	noMdPartitions, err := parser.partitionWriteRequest(ctx, sampleWriteRequests)
	require.NoError(t, err)

	noMdMap := make(map[string]map[string][]MetricData)
	for key, partition := range noMdPartitions {
		require.Nil(t, noMdMap[key])
		noMdMap[key] = make(map[string][]MetricData)

		for _, md := range partition {
			assert.Equal(t, key, md.MetricMetadata.MetricFamilyName)

			noMdMap[key][md.MetricName] = append(noMdMap[key][md.MetricName], md)

			assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			assert.NotEmpty(t, md.MetricMetadata.Type)
			assert.NotEmpty(t, md.MetricMetadata.MetricFamilyName)

			// Help and Unit should only exist for things with metadata
			assert.Empty(t, md.MetricMetadata.Unit)
			assert.Empty(t, md.MetricMetadata.Help)
		}
	}

	sampleWriteRequestsMd := testdata.FlattenWriteRequests(testdata.GetWriteRequestsWithMetadata())
	mdPartitions, err := parser.partitionWriteRequest(ctx, sampleWriteRequestsMd)
	require.NoError(t, err)
	for key, partition := range mdPartitions {
		for _, md := range partition {
			assert.NotEmpty(t, key)
			assert.Equal(t, key, md.MetricMetadata.MetricFamilyName)
			assert.NotEmpty(t, md.MetricName)
			assert.Equalf(t, key, md.MetricMetadata.MetricFamilyName, "%s was not %s.  metricname: %s, metric type: %d", key, md.MetricMetadata.MetricFamilyName, md.MetricName, md.MetricMetadata.Type) // Huh, apparently 1 and 2 get coalesced to 3? is that expected?
			noMetadataItem := noMdMap[key][md.MetricName][0]
			noMdMap[key][md.MetricName] = noMdMap[key][md.MetricName][1:]
			if len(noMdMap[key][md.MetricName]) == 0 {
				delete(noMdMap[key], md.MetricName)
			}
			if len(noMdMap[key]) == 0 {
				delete(noMdMap, key)
			}
			assert.Equalf(t, noMetadataItem.MetricName, md.MetricName, "%s was not %s.  family: %s, metric type: %s", noMetadataItem.MetricName, md.MetricName, md.MetricMetadata.MetricFamilyName, md.MetricMetadata.Type.String()) // Huh, apparently 1 and 2 get coalesced to 3? is that expected?)
			assert.Equalf(t, noMetadataItem.MetricMetadata.Type, md.MetricMetadata.Type, "%s was not %s.  metricname: %s", noMetadataItem.MetricMetadata.Type, md.MetricMetadata.Type.String(), md.MetricName)                       // Huh, apparently 1 and 2 get coalesced to 3? is that expected?)
			assert.Equal(t, noMetadataItem.MetricMetadata.MetricFamilyName, md.MetricMetadata.MetricFamilyName)
			assert.NotEmpty(t, md.MetricMetadata.Help)
			assert.Equal(t, "unit", md.MetricMetadata.Unit)
		}
	}
	// We remove items one by one in above comparison
	assert.Empty(t, noMdMap)

	results, err := parser.TransformPrwToOtel(context.Background(), mdPartitions)
	assert.Nil(t, err)
	assert.NotNil(t, results)

}
func TestFromWriteRequest(t *testing.T) {
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	require.NotNil(t, reporter)
	parser, err := NewPrwOtelParser(context.Background(), reporter)
	require.Nil(t, err)

	sampleWriteRequests := testdata.FlattenWriteRequests(testdata.GetWriteRequests())
	metrics, err := parser.FromPrometheusWriteRequestMetrics(context.Background(), sampleWriteRequests)
	require.Nil(t, err)
	require.NotNil(t, metrics)
	assert.NotNil(t, metrics.ResourceMetrics())
	assert.Greater(t, metrics.DataPointCount(), 0)
}
