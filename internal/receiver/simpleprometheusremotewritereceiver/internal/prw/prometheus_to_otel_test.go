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
	"github.com/signalfx/splunk-otel-collector/internal/receiver/simpleprometheusremotewritereceiver/internal/testdata"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePrometheusRemoteWriteRequest(t *testing.T) {
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	require.NotNil(t, reporter)
	parser, err := NewPrwOtelParser(reporter)
	require.Nil(t, err)

	sampleWriteRequests := testdata.GetWriteRequests()
	for _, sampleWriteRequest := range sampleWriteRequests {
		partitions, err := parser.partitionWriteRequest(*sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.NotEmpty(t, key)
				assert.Equal(t, md.MetricFamilyName, key)
			}
		}
	}
	sampleWriteRequestsMd := testdata.GetWriteRequestsWithMetadata()
	for _, sampleWriteRequest := range sampleWriteRequestsMd {
		partitions, err := parser.partitionWriteRequest(*sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			}
		}
	}
}

func TestParseAndPartitionPrometheusRemoteWriteRequest(t *testing.T) {
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	require.NotNil(t, reporter)
	parser, err := NewPrwOtelParser(reporter)
	require.Nil(t, err)

	sampleWriteRequests := testdata.GetWriteRequests()
	for _, sampleWriteRequest := range sampleWriteRequests {
		partitions, err := parser.partitionWriteRequest(*sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.NotEmpty(t, key)
				assert.Equal(t, md.MetricFamilyName, key)
			}
		}
	}
	sampleWriteRequestsMd := testdata.GetWriteRequestsWithMetadata()
	for _, sampleWriteRequest := range sampleWriteRequestsMd {
		partitions, err := parser.partitionWriteRequest(*sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			}
		}
		results, err := parser.TransformPrwToOtel(partitions)
		assert.Nil(t, err)
		assert.NotNil(t, results)
	}
}

func TestParseAndPartitionMixedPrometheusRemoteWriteRequest(t *testing.T) {
	expectedCalls := 1
	reporter := NewMockReporter(expectedCalls)
	require.NotNil(t, reporter)
	parser, err := NewPrwOtelParser(reporter)
	require.Nil(t, err)

	sampleWriteRequests := testdata.FlattenWriteRequests(testdata.GetWriteRequests())
	noMdPartitions, err := parser.partitionWriteRequest(*sampleWriteRequests)
	require.NoError(t, err)
	var noMdMd []MetricData
	for key, partition := range noMdPartitions {
		for _, md := range partition {
			assert.Equal(t, key, md.MetricFamilyName)
			assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			noMdMd = append(noMdMd, md)
			assert.Equal(t, md.MetricMetadata.MetricFamilyName, key)
			assert.NotEmpty(t, md.MetricMetadata.Type)
			assert.NotEmpty(t, md.MetricMetadata.MetricFamilyName)
			// Help and Unit should only exist for things with metadata
			assert.Empty(t, md.MetricMetadata.Unit)
			assert.Empty(t, md.MetricMetadata.Help)
		}
	}

	sampleWriteRequestsMd := testdata.FlattenWriteRequests(testdata.GetWriteRequestsWithMetadata())
	mdPartitions, err := parser.partitionWriteRequest(*sampleWriteRequestsMd)
	require.NoError(t, err)
	var mdMd []MetricData
	for key, partition := range mdPartitions {
		for index, md := range partition {
			assert.NotEmpty(t, key)
			assert.Equal(t, key, md.MetricFamilyName)
			assert.Equal(t, key, md.MetricMetadata.MetricFamilyName)
			assert.Equal(t, noMdPartitions[key][index].MetricMetadata.Type, md.MetricMetadata.Type)
			assert.Equal(t, noMdPartitions[key][index].MetricMetadata.MetricFamilyName, md.MetricMetadata.MetricFamilyName)
			assert.NotEmpty(t, md.MetricMetadata.Help)
			assert.Equal(t, "unit", md.MetricMetadata.Unit)
			mdMd = append(mdMd, md)
		}
	}
	assert.ElementsMatch(t, mdMd, noMdMd)

	results, err := parser.TransformPrwToOtel(mdPartitions)
	assert.Nil(t, err)
	assert.NotNil(t, results)

}
