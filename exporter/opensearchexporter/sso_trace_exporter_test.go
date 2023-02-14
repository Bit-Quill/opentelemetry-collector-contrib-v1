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

	"github.com/antchfx/jsonquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

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
		configFunc func(*Config)
	}{
		{
			name:       "default routing",
			configFunc: func(config *Config) {},
			assertNode: func(t *testing.T, node *jsonquery.Node) {
				// no attributes property expected
				luckyNode := jsonquery.FindOne(node, "/attributes")
				assert.Nil(t, luckyNode)
			},
		},
		{
			name:       "datatset is ngnix",
			configFunc: func(config *Config) { config.Dataset = "ngnix" },
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				// no attributes property expected
				luckyNode := jsonquery.FindOne(doc, "/attributes/data_stream/dataset")
				assert.NotNil(t, luckyNode)
				assert.Equal(t, "ngnix", luckyNode.Value())
				validateDataStreamType(t, doc)
			},
		},
		{
			name:       "namespace is exceptions",
			configFunc: func(config *Config) { config.Namespace = "exceptions" },
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				// no attributes property expected
				luckyNode := jsonquery.FindOne(doc, "/attributes/data_stream/namespace")
				assert.NotNil(t, luckyNode)
				assert.Equal(t, "exceptions", luckyNode.Value())
				validateDataStreamType(t, doc)
			},
		},
		{
			name: "dataset is mysql, namespace is warnings",
			configFunc: func(config *Config) {
				config.Namespace = "warnings"
				config.Dataset = "mysql"
			},
			assertNode: func(t *testing.T, doc *jsonquery.Node) {
				// no attributes property expected
				namespaceNode := jsonquery.FindOne(doc, "/attributes/data_stream/namespace")
				assert.NotNil(t, namespaceNode)
				assert.Equal(t, "warnings", namespaceNode.Value())

				datasetNode := jsonquery.FindOne(doc, "/attributes/data_stream/dataset")
				assert.NotNil(t, datasetNode)
				assert.Equal(t, "mysql", datasetNode.Value())
				validateDataStreamType(t, doc)
			},
		},
	}
	logger := zaptest.NewLogger(t)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rec := newBulkRecorder()
			server := newRecTestServer(t, rec)
			exporter := newSSOTestTracesExporter(t, logger, server.URL, testCase.configFunc)
			err := exporter.pushTraceRecord(context.TODO(), sampleResource(), sampleSpan())
			require.NoError(t, err)
			rec.WaitItems(1)

			// Sanity test
			require.Equal(t, 1, rec.NumItems())

			// Sanity test
			require.Len(t, rec.Requests()[0], 1)

			doc, err := jsonquery.Parse(bytes.NewReader(rec.Requests()[0][0].Document))

			// Sanity test
			require.NoError(t, err)

			testCase.assertNode(t, doc)
		})
	}

}

// Test_sso_routing verifies that Dataset and Namespace configuration options determine how export data is routed
func Test_sso_routing(t *testing.T) {
	testCases := []struct {
		name          string
		expectedIndex string
		configFunc    func(config *Config)
	}{
		{
			name:          "default routing",
			expectedIndex: "sso_traces-default-namespace",
			configFunc:    func(config *Config) {},
		},
		{
			name: "dataset is webapp",
			configFunc: func(config *Config) {
				config.Dataset = "webapp"
			},
			expectedIndex: "sso_traces-webapp-namespace",
		},
		{
			name: "namespace is exceptions",
			configFunc: func(config *Config) {
				config.Namespace = "exceptions"
			},
			expectedIndex: "sso_traces-default-exceptions",
		},
		{
			name: "namespace is warnings, dataset is mysql",
			configFunc: func(config *Config) {
				config.Namespace = "warnings"
				config.Dataset = "mysql"
			},
			expectedIndex: "sso_traces-mysql-warnings",
		},
	}
	logger := zaptest.NewLogger(t)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rec := newBulkRecorder()
			server := newRecTestServer(t, rec)
			exporter := newSSOTestTracesExporter(t, logger, server.URL, testCase.configFunc)
			err := exporter.pushTraceRecord(context.TODO(), sampleResource(), sampleSpan())
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
