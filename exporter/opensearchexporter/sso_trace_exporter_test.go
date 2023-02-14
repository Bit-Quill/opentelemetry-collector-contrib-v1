// Copyright The OpenTelemetry Authors
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
	"net/http/httptest"
	"testing"
	"time"

	"github.com/antchfx/jsonquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func Test_span_sample(t *testing.T) {
	testCase := &singleSpanTestCase{
		name:          "sample span",
		configMod:     func(config *Config) {},
		givenResource: sampleResource(),
		givenSpan:     sampleSpan(),
		givenScope:    sampleScope(),
		assertNode: func(t *testing.T, doc *jsonquery.Node) {
			assertJsonValueEqual(t, "ip_tcp", doc, "/attributes/net.transport")
			assertJsonValueEqual(t, float64(3), doc, "/droppedAttributesCount")
			assertJsonValueEqual(t, float64(5), doc, "/droppedEventsCount")
			assertJsonValueEqual(t, float64(8), doc, "/droppedLinksCount")
			assertJsonValueEqual(t, sampleTimeB.Format(time.RFC3339Nano), doc, "/endTime")
			assertJsonValueEqual(t, "Internal", doc, "/kind")
			assertJsonValueEqual(t, "sample-span", doc, "/name")
			assertJsonValueEqual(t, sampleSpanIdB.String(), doc, "/parentSpanId")
			assertJsonValueEqual(t, sampleSpanIdA.String(), doc, "/spanId")
			assertJsonValueEqual(t, "Error", doc, "/status/code")
			assertJsonValueEqual(t, "sample status message", doc, "/status/message")
			assertJsonValueEqual(t, sampleTimeA.Format(time.RFC3339Nano), doc, "/startTime")
			assertJsonValueEqual(t, rawSampleTraceState, doc, "/traceState")
			assertJsonValueEqual(t, sampleTraceIdA.String(), doc, "/traceId")
		},
	}

	executeTestCase(t, testCase)
}

func Test_instrumentation_scope(t *testing.T) {
	testCases := []*singleSpanTestCase{
		{
			name:          "sample scope",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenSpan:     sampleSpan(),
			givenScope:    sampleScope(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				scope := jsonquery.FindOne(doc, "/instrumentationScope")
				assert.NotNil(t, scope)
				assert.NotNil(t, jsonquery.FindOne(scope, "name"))
				assert.NotNil(t, jsonquery.FindOne(scope, "version"))
				assertJsonValueEqual(t, sampleSchemaUrl, scope, "/schemaUrl")
				assertJsonValueEqual(t, "com.example.some-sdk", scope, "/attributes/scope.short_name")
			},
		},
	}

	for _, testCase := range testCases {
		executeTestCase(t, testCase)
	}
}

func Test_links(t *testing.T) {
	testCases := []*singleSpanTestCase{
		{
			name:          "no links",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenSpan:     sampleSpan(),
			givenScope:    sampleScope(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				links := jsonquery.FindOne(doc, "/links")
				assert.Nil(t, links)
			},
		},

		{
			name:          "sample link",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenSpan:     withSampleLink(sampleSpan()),
			givenScope:    sampleScope(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				links := jsonquery.FindOne(doc, "/links/*")

				assertJsonValueEqual(t, sampleTraceIdB.String(), links, "/traceId")
				assertJsonValueEqual(t, sampleSpanIdB.String(), links, "/spanId")
				assertJsonValueEqual(t, float64(3), links, "droppedAttributesCount")
				assertJsonValueEqual(t, rawSampleTraceState, links, "traceState")
				assertJsonValueEqual(t, "remote-link", links, "/attributes/link_attr")
			},
		},
	}

	for _, testCase := range testCases {
		executeTestCase(t, testCase)
	}
}

// Test_events verifies that events are mapped as expected.
func Test_events(t *testing.T) {
	testCases := []*singleSpanTestCase{
		{
			name:          "no events",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     sampleSpan(),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				eventsNode := jsonquery.FindOne(doc, "/events")
				assert.Nil(t, eventsNode)
			},
		},
		{
			name:          "event with timestamp",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     withEventWithTimestamp(sampleSpan()),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				eventsNode := jsonquery.FindOne(doc, "/events/*")
				assertJsonValueEqual(t, "cloudevent", eventsNode, "name")
				// JSON Go encodes all numbers as floats, see https://pkg.go.dev/encoding/json#Marshal
				assertJsonValueEqual(t, float64(42), eventsNode, "droppedAttributesCount")
				assert.NotNil(t, eventsNode.SelectElement("@timestamp"))
				assert.Nil(t, eventsNode.SelectElement("observedTimestamp"))
			},
		},
		{
			name:          "event, no timestamp, observedTimestamp added",
			configMod:     func(config *Config) {},
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     withEventNoTimestamp(sampleSpan()),
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				eventsNode := jsonquery.FindOne(doc, "/events/*")
				assertJsonValueEqual(t, "exception", eventsNode, "name")
				assert.NotNil(t, eventsNode.SelectElement("observedTimestamp"))
				assert.Nil(t, eventsNode.SelectElement("@timestamp"))
			},
		},
	}

	for _, testCase := range testCases {
		executeTestCase(t, testCase)
	}
}

// Test_attributes_data_stream verifies that Dataset and Namespace configuration options are
// recorded in the `.attributes.data_stream` object of an exported span
func Test_attributes_data_stream(t *testing.T) {
	// validateDataStreamType asserts that /attributes/data_stream/type exists and is equal to "span"
	validateDataStreamType := func(t *testing.T, node *jsonquery.Node) {
		assert.Equal(t, "span", jsonquery.FindOne(node, "/attributes/data_stream/type").Value())
	}
	testCases := []struct {
		name       string
		assertNode func(*testing.T, *jsonquery.Node)
		configMod  func(*Config)
	}{
		{
			name:      "default routing",
			configMod: func(config *Config) {},
			assertNode: func(t *testing.T, node *jsonquery.Node) {
				// no data_stream attribute expected
				luckyNode := jsonquery.FindOne(node, "/attributes/data_stream")
				assert.Nil(t, luckyNode)
			},
		},
		{
			name:      "datatset is ngnix",
			configMod: func(config *Config) { config.Dataset = "ngnix" },
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				assertJsonValueEqual(t, "ngnix", doc, "/attributes/data_stream/dataset")
				validateDataStreamType(t, doc)
			},
		},
		{
			name:      "namespace is exceptions",
			configMod: func(config *Config) { config.Namespace = "exceptions" },
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				// no attributes property expected
				assertJsonValueEqual(t, "exceptions", doc, "/attributes/data_stream/namespace")
				validateDataStreamType(t, doc)
			},
		},
		{
			name: "dataset is mysql, namespace is warnings",
			configMod: func(config *Config) {
				config.Namespace = "warnings"
				config.Dataset = "mysql"
			},
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				// no attributes property expected
				assertJsonValueEqual(t, "warnings", doc, "/attributes/data_stream/namespace")
				assertJsonValueEqual(t, "mysql", doc, "/attributes/data_stream/dataset")

				validateDataStreamType(t, doc)
			},
		},
	}

	for _, sampleTestCase := range testCases {
		testCase := &singleSpanTestCase{
			name:          sampleTestCase.name,
			configMod:     sampleTestCase.configMod,
			givenResource: sampleResource(),
			givenScope:    sampleScope(),
			givenSpan:     sampleSpan(),
			assertNode:    sampleTestCase.assertNode,
		}
		executeTestCase(t, testCase)
	}

}

// Test_sso_routing verifies that Dataset and Namespace configuration options determine how export data is routed
func Test_sso_routing(t *testing.T) {
	testCases := []struct {
		name          string
		expectedIndex string
		configMod     func(config *Config)
	}{
		{
			name:          "default routing",
			expectedIndex: "sso_traces-default-namespace",
			configMod:     func(config *Config) {},
		},
		{
			name: "dataset is webapp",
			configMod: func(config *Config) {
				config.Dataset = "webapp"
			},
			expectedIndex: "sso_traces-webapp-namespace",
		},
		{
			name: "namespace is exceptions",
			configMod: func(config *Config) {
				config.Namespace = "exceptions"
			},
			expectedIndex: "sso_traces-default-exceptions",
		},
		{
			name: "namespace is warnings, dataset is mysql",
			configMod: func(config *Config) {
				config.Namespace = "warnings"
				config.Dataset = "mysql"
			},
			expectedIndex: "sso_traces-mysql-warnings",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			rec := newBulkRecorder()
			server := newRecTestServer(t, rec)
			exporter := newSSOTestTracesExporter(t, logger, server.URL, testCase.configMod)
			err := exporter.pushTraceRecord(context.TODO(), *sampleResource(), *sampleScope(), sampleSchemaUrl, *sampleSpan())
			require.NoError(t, err)
			rec.WaitItems(1)

			// Sanity test
			require.Equal(t, 1, rec.NumItems())

			// Sanity test
			require.Len(t, rec.Requests()[0], 1)

			doc, err := jsonquery.Parse(bytes.NewReader(rec.Requests()[0][0].Action))

			// Sanity test
			require.NoError(t, err)

			inx := jsonquery.FindOne(doc, "/create/_index")
			assert.Equal(t, testCase.expectedIndex, inx.Value())
		})
	}
}

// newSSOTestTracesExporter creates a traces exporter that sends data in Simple Schema for Observability schema.
// See https://github.com/opensearch-project/observability for details
func newSSOTestTracesExporter(t *testing.T, logger *zap.Logger, url string, fns ...func(*Config)) *SSOTracesExporter {
	exporter, err := newSSOTracesExporter(logger, withTestTracesExporterConfig(fns...)(url))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, exporter.Shutdown(context.TODO()))
	})
	return exporter
}

// newRecTestServer creates a http webserver that records all the requests in the provided bulkRecorder
func newRecTestServer(t *testing.T, rec *bulkRecorder) *httptest.Server {
	server := newTestServer(t, func(docs []itemRequest) ([]itemResponse, error) {
		rec.Record(docs)
		return itemsAllOK(docs)
	})
	t.Cleanup(func() {
		server.Close()
	})
	return server
}
