package testdata

import (
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBasicNoMd(t *testing.T) {
	wqs := GetWriteRequests()
	require.NotNil(t, wqs)
	for _, wq := range wqs {
		for _, ts := range wq.Timeseries {
			require.NotNil(t, ts)
			assert.NotEmpty(t, ts.Labels)
		}
		require.Empty(t, wq.Metadata)
	}
}
func TestBasicMd(t *testing.T) {
	wqs := AddMetadataScaffoldToWriteRequests(GetWriteRequestsWithMetadata())
	require.NotNil(t, wqs)
	for _, wq := range wqs {
		for _, ts := range wq.Timeseries {
			require.NotNil(t, ts)
			assert.NotEmpty(t, ts.Labels)
		}
		require.NotEmpty(t, wq.Metadata)
		for _, ts := range wq.Metadata {
			require.NotNil(t, ts)
			//assert.NotEmpty(t, ts.Type)
			assert.Equal(t, prompb.MetricMetadata_UNKNOWN, ts.Type)
			assert.NotEmpty(t, ts.MetricFamilyName)
			assert.Equal(t, ts.Unit, "unit")
			assert.NotEmpty(t, ts.Help)
		}
	}
}
func TestCoveringMd(t *testing.T) {
	wq := FlattenWriteRequests(GetWriteRequestsWithMetadata())
	require.NotNil(t, wq)
	for _, ts := range wq.Timeseries {
		require.NotNil(t, ts)
		assert.NotEmpty(t, ts.Labels)
	}
	require.NotEmpty(t, wq.Metadata)
	total := 0
	unknown := 0
	for _, ts := range wq.Metadata {
		total += 1
		require.NotNil(t, ts)
		assert.NotEmpty(t, ts.Type)
		if ts.Type == prompb.MetricMetadata_UNKNOWN {
			unknown += 1
		}
		assert.NotEmpty(t, ts.MetricFamilyName)
		assert.Equal(t, ts.Unit, "unit")
		assert.NotEmpty(t, ts.Help)
	}
	assert.Equal(t, total, total-unknown)
}
