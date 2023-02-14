// Copyright 2023, OpenTelemetry Authors
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

package opensearchexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/antchfx/jsonquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const sampleSchemaUrl string = "https://example.com/schema/1.0"

type itemRequest struct {
	Action   json.RawMessage
	Document json.RawMessage
}

type itemResponse struct {
	Status int `json:"status"`
}

type bulkResult struct {
	Took      int            `json:"took"`
	HasErrors bool           `json:"errors"`
	Items     []itemResponse `json:"items"`
}

type bulkHandler func([]itemRequest) ([]itemResponse, error)

type httpTestError struct {
	status  int
	message string
	cause   error
}

const opensearchVersion = "2.5.0"

func (e *httpTestError) Error() string {
	return fmt.Sprintf("http request failed (status=%v): %v", e.Status(), e.Message())
}

func (e *httpTestError) Status() int {
	if e.status == 0 {
		return http.StatusInternalServerError
	}
	return e.status
}

func (e *httpTestError) Message() string {
	var buf strings.Builder
	if e.message != "" {
		buf.WriteString(e.message)
	}
	if e.cause != nil {
		if buf.Len() > 0 {
			buf.WriteString(": ")
		}
		buf.WriteString(e.cause.Error())
	}
	return buf.String()
}

type bulkRecorder struct {
	mu         sync.Mutex
	cond       *sync.Cond
	recordings [][]itemRequest
}

func newBulkRecorder() *bulkRecorder {
	r := &bulkRecorder{}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *bulkRecorder) Record(bulk []itemRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recordings = append(r.recordings, bulk)
	r.cond.Broadcast()
}

func (r *bulkRecorder) WaitItems(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for n > r.countItems() {
		r.cond.Wait()
	}
}

func (r *bulkRecorder) Requests() [][]itemRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recordings
}

func (r *bulkRecorder) Items() (docs []itemRequest) {
	for _, rec := range r.Requests() {
		docs = append(docs, rec...)
	}
	return docs
}

func (r *bulkRecorder) NumItems() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countItems()
}

func (r *bulkRecorder) countItems() (count int) {
	for _, docs := range r.recordings {
		count += len(docs)
	}
	return count
}

func newTestServer(t *testing.T, bulkHandler bulkHandler) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleErr(func(w http.ResponseWriter, req *http.Request) error {

		enc := json.NewEncoder(w)
		return enc.Encode(map[string]interface{}{
			"version": map[string]interface{}{
				"number": opensearchVersion,
			},
		})
	}))

	mux.HandleFunc("/_bulk", handleErr(func(w http.ResponseWriter, req *http.Request) error {
		tsStart := time.Now()
		var items []itemRequest

		dec := json.NewDecoder(req.Body)
		for dec.More() {
			var action, doc json.RawMessage
			if err := dec.Decode(&action); err != nil {
				return &httpTestError{status: http.StatusBadRequest, cause: err}
			}
			if !dec.More() {
				return &httpTestError{status: http.StatusBadRequest, message: "action without document"}
			}
			if err := dec.Decode(&doc); err != nil {
				return &httpTestError{status: http.StatusBadRequest, cause: err}
			}

			items = append(items, itemRequest{Action: action, Document: doc})
		}

		resp, err := bulkHandler(items)
		if err != nil {
			return err
		}
		took := int(time.Since(tsStart) / time.Microsecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)
		return enc.Encode(bulkResult{Took: took, Items: resp, HasErrors: itemsHasError(resp)})
	}))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func handleErr(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			httpError := &httpTestError{}
			if errors.As(err, &httpError) {
				http.Error(w, httpError.Message(), httpError.Status())
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}
}

func (item *itemResponse) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	_, err := fmt.Fprintf(&buf, `{"create": {"status": %v}}`, item.Status)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func itemsAllOK(docs []itemRequest) ([]itemResponse, error) {
	return itemsReportStatus(docs, http.StatusOK)
}

func itemsReportStatus(docs []itemRequest, status int) ([]itemResponse, error) {
	responses := make([]itemResponse, len(docs))
	for i := range docs {
		responses[i].Status = status
	}
	return responses, nil
}

func itemsHasError(resp []itemResponse) bool {
	for _, r := range resp {
		if r.Status != http.StatusOK {
			return true
		}
	}
	return false
}

func assertJsonValueEqual(t *testing.T, expected interface{}, node *jsonquery.Node, jq string) {
	luckyNode := jsonquery.FindOne(node, jq)
	assert.NotNil(t, luckyNode)
	assert.Equal(t, expected, luckyNode.Value())
}

// singleSpanTestCase represents a test case that configures sso exporter with configMod,
// pushes givenResource and givenSpan and verifies the bulk request that sso exporter generates
// using assertNode
type singleSpanTestCase struct {
	name          string
	configMod     func(*Config)
	givenResource *pcommon.Resource
	givenSpan     *ptrace.Span
	assertNode    func(*testing.T, *jsonquery.Node)
	givenScope    *pcommon.InstrumentationScope
}

func executeTestCase(t *testing.T, testCase *singleSpanTestCase) {
	t.Run(testCase.name, func(t *testing.T) {
		logger := zaptest.NewLogger(t)

		ctx := context.TODO()
		rec := newBulkRecorder()
		server := newRecTestServer(t, rec)
		exporter := newSSOTestTracesExporter(t, logger, server.URL, testCase.configMod)
		err := exporter.pushTraceRecord(ctx, *testCase.givenResource, *testCase.givenScope, sampleSchemaUrl, *testCase.givenSpan)

		// Verify that push completed successfully
		require.NoError(t, err)

		// Wait for HTTP server to capture the bulk request
		rec.WaitItems(1)

		// Confirm there was only one bulk request
		require.Equal(t, 1, rec.NumItems())
		require.Len(t, rec.Requests()[0], 1)

		// Read the bulk request document
		doc, err := jsonquery.Parse(bytes.NewReader(rec.Requests()[0][0].Document))

		require.NoError(t, err)

		// Verify document matches expectations
		testCase.assertNode(t, doc)
	})
}

func sampleResource() *pcommon.Resource {
	r := pcommon.NewResource()
	// An attribute of each supported type
	r.Attributes().PutStr("service.name", "opensearchexporter")
	r.Attributes().PutBool("browser.mobile", false)
	r.Attributes().PutInt("process.pid", 300)
	return &r
}

func sampleScope() *pcommon.InstrumentationScope {
	sample := pcommon.NewInstrumentationScope()
	sample.Attributes().PutStr("scope.short_name", "com.example.some-sdk")
	sample.SetDroppedAttributesCount(7)
	sample.SetName("some-sdk by Example.com")
	sample.SetVersion("semver:2.5.0-SNAPSHOT")
	return &sample
}

func sampleSpan() *ptrace.Span {
	sp := ptrace.NewSpan()

	sp.SetDroppedAttributesCount(3)
	sp.SetDroppedEventsCount(5)
	sp.SetDroppedLinksCount(8)
	sp.SetSpanID(sampleSpanIdA)
	sp.SetParentSpanID(sampleSpanIdB)
	sp.SetTraceID(sampleTraceIdA)
	sp.SetKind(ptrace.SpanKindInternal)
	sp.TraceState().FromRaw(rawSampleTraceState)
	sp.Attributes().PutStr("net.transport", "ip_tcp")
	sp.SetStartTimestamp(pcommon.NewTimestampFromTime(sampleTimeA))
	sp.SetEndTimestamp(pcommon.NewTimestampFromTime(sampleTimeB))
	sp.SetName("sample-span")
	sp.Status().SetCode(ptrace.StatusCodeError)
	sp.Status().SetMessage("sample status message")
	return &sp
}

func withEventNoTimestamp(span *ptrace.Span) *ptrace.Span {
	event := span.Events().AppendEmpty()
	// This event lacks timestamp
	event.Attributes().PutStr("exception.message", "Invalid user input")
	event.Attributes().PutStr("exception.type", "ApplicationException")
	event.SetDroppedAttributesCount(42)
	event.SetName("exception")
	return span
}

func withEventWithTimestamp(span *ptrace.Span) *ptrace.Span {
	event := span.Events().AppendEmpty()
	event.Attributes().PutStr("cloudevents.event_id", "001")
	event.Attributes().PutStr("cloudevents.event_source", "example.com/example-service")
	event.Attributes().PutStr("cloudevents.event_type", "com.example.example-service.panic")
	event.SetName("cloudevent")
	event.SetDroppedAttributesCount(42)
	event.SetTimestamp(
		pcommon.NewTimestampFromTime(sampleTimeC))
	return span
}

func withSampleLink(span *ptrace.Span) *ptrace.Span {
	link := span.Links().AppendEmpty()
	link.SetDroppedAttributesCount(3)
	link.SetTraceID(sampleTraceIdB)
	link.SetSpanID(sampleSpanIdB)
	link.TraceState().FromRaw(rawSampleTraceState)
	link.Attributes().PutStr("link_attr", "remote-link")
	return span
}

var sampleTraceIdA = pcommon.TraceID{1, 0, 0, 0}
var sampleTraceIdB = pcommon.TraceID{1, 1, 0, 0}
var sampleSpanIdA = pcommon.SpanID{1, 0, 0, 0}
var sampleSpanIdB = pcommon.SpanID{1, 1, 0, 0}
var sampleTimeA = time.Date(1990, 1, 1, 1, 1, 1, 1, time.UTC)
var sampleTimeB = time.Date(1995, 1, 1, 1, 1, 1, 1, time.UTC)
var sampleTimeC = time.Date(1998, 1, 1, 1, 1, 1, 1, time.UTC)

// TraceState is defined by a W3C spec. Sample value take from https://www.w3.org/TR/trace-context/#examples-of-tracestate-http-headers
const rawSampleTraceState = "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE"
