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
	"context"
	"encoding/json"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type SSOTracesExporter struct {
	logger *zap.Logger

	maxAttempts int

	client      *osClientCurrent
	bulkIndexer osBulkIndexerCurrent
	Namespace   string
	Dataset     string
}

func (s SSOTracesExporter) Shutdown(ctx context.Context) error {
	return s.bulkIndexer.Close(ctx)

}

func (s SSOTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if err := s.pushTraceRecord(ctx, resource, spans.At(k)); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}

func defaultIfEmpty(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func (s SSOTracesExporter) pushTraceRecord(ctx context.Context, resource pcommon.Resource, span ptrace.Span) error {
	sso := SSOSpan{}
	sso.Name = span.Name()
	sso.TraceID = span.TraceID().String()
	sso.TraceState = span.TraceState().AsRaw()
	sso.ParentSpanID = span.ParentSpanID().String()
	sso.StartTime = span.StartTimestamp().AsTime()
	sso.EndTime = span.EndTimestamp().AsTime()
	sso.Kind = span.Kind().String()
	sso.DroppedAttributesCount = span.DroppedAttributesCount()
	sso.DroppedEventsCount = span.DroppedEventsCount()
	sso.DroppedLinksCount = span.DroppedLinksCount()
	sso.Status.Code = span.Status().Code().String()
	sso.Status.Message = span.Status().Message()
	sso.Resource = resource.Attributes().AsRaw()
	sso.Attributes = span.Attributes().AsRaw()

	dataStream := DataStream{}
	if s.Dataset != "" {
		dataStream.Dataset = s.Dataset
	}

	if s.Namespace != "" {
		dataStream.Namespace = s.Namespace
	}

	if dataStream != (DataStream{}) {
		dataStream.Type = "span"
		sso.Attributes["data_stream"] = dataStream
	}
	payload, _ := json.Marshal(sso)

	index := strings.Join([]string{"sso_traces", defaultIfEmpty(s.Dataset, "default"), defaultIfEmpty(s.Namespace, "namespace")}, "-")
	return pushDocuments(ctx, s.logger, index, payload, s.bulkIndexer, s.maxAttempts)

}

func newSSOTracesExporter(logger *zap.Logger, cfg *Config) (*SSOTracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newOpenSearchClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	bulkIndexer, err := newBulkIndexer(logger, client, cfg)
	if err != nil {
		return nil, err
	}

	maxAttempts := GetMaxAttempts(cfg)
	return &SSOTracesExporter{
		logger:      logger,
		maxAttempts: maxAttempts,
		client:      client,
		bulkIndexer: bulkIndexer,
		Namespace:   cfg.Namespace,
		Dataset:     cfg.Dataset,
	}, nil
}
