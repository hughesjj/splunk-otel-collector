package transport

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/config/confignet"
	"io"
	"net/http"
	"time"
)

type MockPrwClient struct {
	confignet.NetAddr
	Path    string
	Timeout time.Duration
}

func (prwc *MockPrwClient) SendWriteRequest(wr *prompb.WriteRequest) error {
	data, err := proto.Marshal(wr)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	remoteWriteURL := fmt.Sprintf("http://%s/%s", prwc.NetAddr.Endpoint, prwc.Path)
	req, err := http.NewRequest(http.MethodPost, remoteWriteURL, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "gzip")

	client := &http.Client{Timeout: prwc.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return errors.New(string(body))
	}

	return nil
}
