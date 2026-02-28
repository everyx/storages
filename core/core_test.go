//go:build !wasm && !wasi

package core

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/pierrec/lz4/v4"
)

func TestReadResponse_Concurrency(t *testing.T) {
	// 1. Create a sample HTTP response
	bodyContent := "This is a recurring body content for concurrency test. "
	for i := 0; i < 10; i++ {
		bodyContent += bodyContent
	}

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewBufferString(bodyContent)),
		ContentLength: int64(len(bodyContent)),
	}
	resp.Header.Set("Content-Type", "text/plain")

	var buf bytes.Buffer
	if err := resp.Write(&buf); err != nil {
		t.Fatalf("Failed to write response: %v", err)
	}
	fullRawResponse := buf.Bytes()

	// 2. Compress the response
	compressed := new(bytes.Buffer)
	writer := lz4.NewWriter(compressed)
	if _, err := writer.Write(fullRawResponse); err != nil {
		t.Fatalf("Failed to compress: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	cachedData := compressed.Bytes()
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	// CONCURRENCY TEST: 20 goroutines reading same cachedData concurrently
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			gotResp, err := readResponse(cachedData, req)
			if err != nil {
				t.Errorf("Goroutine %d: readResponse failed: %v", idx, err)
				return
			}
			defer gotResp.Body.Close()
			gotBody, err := io.ReadAll(gotResp.Body)
			if err != nil {
				t.Errorf("Goroutine %d: failed to read body: %v", idx, err)
				return
			}
			if string(gotBody) != bodyContent {
				t.Errorf("Goroutine %d: Body mismatch. Expected length %d, got %d", idx, len(bodyContent), len(gotBody))
			}
		}(i)
	}
	wg.Wait()
}
