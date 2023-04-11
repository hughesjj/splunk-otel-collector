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

	//var sampleMetadata []prompb.MetricMetadata
	//sampleMetadata = append(sampleMetadata, prompb.MetricMetadata{
	//	MetricFamilyName: "foo",
	//})

	//sampleWriteRequest := prompb.WriteRequest{
	//	Metadata: sampleMetadata,
	//}
	sampleWriteRequests := testdata.GetWriteRequests()
	for _, sampleWriteRequest := range sampleWriteRequests {
		partitions, err := parser.partitionWriteRequest(*sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				mfn := getBaseMetricFamilyName(key)
				assert.Equal(t, md.MetricFamilyName, mfn)
			}
		}
	}
	sampleWriteRequestsMd := testdata.GetWriteRequestsWithMetadata()
	for _, sampleWriteRequest := range sampleWriteRequestsMd {
		partitions, err := parser.partitionWriteRequest(*sampleWriteRequest)
		require.NoError(t, err)
		for key, partition := range partitions {
			for _, md := range partition {
				mfn := getBaseMetricFamilyName(key)
				assert.Equal(t, md.MetricMetadata.MetricFamilyName, mfn)
			}
		}
	}
}
