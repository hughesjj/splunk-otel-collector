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

package prometheus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	// TODO replace golib/datapoint with pmetricdata equivalents...?
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/signalfx/golib/datapoint"
	"github.com/signalfx/golib/datapoint/dptest"
	"github.com/signalfx/golib/event"
	"github.com/signalfx/golib/pointer"
	"github.com/signalfx/golib/web"
	"github.com/smartystreets/goconvey/convey"
)

func getPayload(incoming *prompb.WriteRequest) []byte {
	if incoming == nil {
		incoming = getWriteRequest()
	}
	bytes, err := proto.Marshal(incoming)
	convey.So(err, convey.ShouldBeNil)
	return snappy.Encode(nil, bytes)
}

func getWriteRequest() *prompb.WriteRequest {
	return &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "key", Value: "value"},
					{Name: model.MetricNameLabel, Value: "process_cpu_seconds_total"}, // counter
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: time.Now().Unix() * 1000},
				},
			},
		},
	}
}

func TestGetStuff(t *testing.T) {
	convey.Convey("test metric types", t, func() {
		convey.So(getMetricType("im_a_counter_bucket"), convey.ShouldEqual, datapoint.Counter)
		convey.So(getMetricType("im_a_counter_count"), convey.ShouldEqual, datapoint.Counter)
		convey.So(getMetricType("everything_else"), convey.ShouldEqual, datapoint.Gauge)
	})
}

func TestErrorCases(t *testing.T) {
	convey.Convey("test listener", t, func() {
		callCount := int64(0)
		conf := &Config{
			ListenAddr: pointer.String("127.0.0.1:0"),
			HTTPChain: func(ctx context.Context, rw http.ResponseWriter, r *http.Request, next web.ContextHandler) {
				atomic.AddInt64(&callCount, 1)
				next.ServeHTTPC(ctx, rw, r)
			},
		}
		sendTo := dptest.NewBasicSink()
		convey.Convey("test same port", func() {
			conf.ListenAddr = pointer.String("127.0.0.1:99999999r")
			_, err := NewListener(sendTo, conf)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

type errSink struct{}

func (e *errSink) AddDatapoints(ctx context.Context, points []*datapoint.Datapoint) error {
	return errors.New("nope")
}

func (e *errSink) AddEvents(ctx context.Context, points []*event.Event) error {
	return errors.New("nope")
}

func TestListener(t *testing.T) {
	convey.Convey("test listener", t, func() {
		callCount := int64(0)
		conf := &Config{
			ListenAddr: pointer.String("127.0.0.1:0"),
			HTTPChain: func(ctx context.Context, rw http.ResponseWriter, r *http.Request, next web.ContextHandler) {
				atomic.AddInt64(&callCount, 1)
				next.ServeHTTPC(ctx, rw, r)
			},
		}
		sendTo := dptest.NewBasicSink()
		listener, err := NewListener(sendTo, conf)
		convey.So(err, convey.ShouldBeNil)
		client := &http.Client{}
		baseURL := fmt.Sprintf("http://%s/write", listener.server.Addr)
		convey.Convey("Should expose health check", func() {
			req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/healthz", listener.server.Addr), nil)
			convey.So(err, convey.ShouldBeNil)
			resp, err := client.Do(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusOK)
			convey.So(atomic.LoadInt64(&callCount), convey.ShouldEqual, 1)
		})
		convey.Convey("Should be able to receive datapoints", func() {
			sendTo.Resize(10)
			req, err := http.NewRequest("POST", baseURL, bytes.NewReader(getPayload(nil)))
			convey.So(err, convey.ShouldBeNil)
			req.Header.Set("Content-Type", "application/x-protobuf")
			resp, err := client.Do(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusOK)
			datapoints := <-sendTo.PointsChan
			convey.So(len(datapoints), convey.ShouldEqual, 1)
		})
		convey.Convey("Is a Collector", func() {
			dps := listener.Datapoints()
			convey.So(len(dps), convey.ShouldEqual, 13)
		})
		convey.Convey("test getDatapoints edge cases", func() {
			ts := getWriteRequest().Timeseries[0]
			convey.Convey("check nan", func() {
				ts.Samples[0].Value = math.NaN()
				dps := listener.decoder.getDatapoints(ts)
				convey.So(len(dps), convey.ShouldEqual, 0)
				for atomic.LoadInt64(&listener.decoder.TotalNaNs) != 1 {
					runtime.Gosched()
				}
			})
			convey.Convey("check float conversion", func() {
				ts.Samples[0].Value = 1.71245
				dps := listener.decoder.getDatapoints(ts)
				convey.So(len(dps), convey.ShouldEqual, 1)
				convey.So(dps[0].Value, convey.ShouldResemble, datapoint.NewFloatValue(1.71245))
			})
		})
		convey.Reset(func() {
			convey.So(listener.Close(), convey.ShouldBeNil)
		})
	})
}

func TestBad(t *testing.T) {
	convey.Convey("test listener", t, func() {
		callCount := int64(0)
		conf := &Config{
			ListenAddr: pointer.String("127.0.0.1:0"),
			HTTPChain: func(ctx context.Context, rw http.ResponseWriter, r *http.Request, next web.ContextHandler) {
				atomic.AddInt64(&callCount, 1)
				next.ServeHTTPC(ctx, rw, r)
			},
		}
		sendTo := dptest.NewBasicSink()
		listener, err := NewListener(sendTo, conf)
		convey.So(err, convey.ShouldBeNil)
		client := &http.Client{}
		baseURL := fmt.Sprintf("http://%s/write", listener.server.Addr)
		sendTo.Resize(10)
		convey.Convey("Should bork on nil data", func() {
			req, err := http.NewRequest("POST", baseURL, nil)
			convey.So(err, convey.ShouldBeNil)
			req.Header.Set("Content-Type", "application/x-protobuf")
			resp, err := client.Do(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusBadRequest)
		})
		convey.Convey("Should bork bad readAll", func() {
			listener.decoder.readAll = func(r io.Reader) ([]byte, error) {
				return nil, errors.New("nope")
			}
			req, err := http.NewRequest("POST", baseURL, bytes.NewReader(getPayload(nil)))
			convey.So(err, convey.ShouldBeNil)
			req.Header.Set("Content-Type", "application/x-protobuf")
			resp, err := client.Do(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusInternalServerError)
		})
		convey.Convey("Should bork on bad data", func() {
			jeff := []byte("blarg")
			osnap := snappy.Encode(nil, jeff)
			req, err := http.NewRequest("POST", baseURL, bytes.NewReader(osnap))
			convey.So(err, convey.ShouldBeNil)
			req.Header.Set("Content-Type", "application/x-protobuf")
			resp, err := client.Do(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusBadRequest)
		})
		convey.Convey("count bad datapoint", func() {
			incoming := getWriteRequest()
			incoming.Timeseries[0].Labels = []prompb.Label{}
			req, err := http.NewRequest("POST", baseURL, bytes.NewReader(getPayload(incoming)))
			convey.So(err, convey.ShouldBeNil)
			req.Header.Set("Content-Type", "application/x-protobuf")
			resp, err := client.Do(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusOK)
			for atomic.LoadInt64(&listener.decoder.TotalBadDatapoints) != 1 {
				runtime.Gosched()
			}
		})
		convey.Convey("sink throws an error", func() {
			listener.decoder.SendTo = new(errSink)
			req, err := http.NewRequest("POST", baseURL, bytes.NewReader(getPayload(nil)))
			convey.So(err, convey.ShouldBeNil)
			req.Header.Set("Content-Type", "application/x-protobuf")
			resp, err := client.Do(req)
			convey.So(err, convey.ShouldBeNil)
			convey.So(resp.StatusCode, convey.ShouldEqual, http.StatusInternalServerError)
		})
		convey.Reset(func() {
			convey.So(listener.Close(), convey.ShouldBeNil)
		})
	})
}
