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

func (s SSOTracesExporter) pushTraceRecord(ctx context.Context, _ pcommon.Resource, span ptrace.Span) error {
	index := "default-index"
	sso := Sso{}
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
	payload, _ := json.Marshal(sso)
	return pushDocuments(ctx, s.logger, index, payload, s.bulkIndexer, s.maxAttempts)

}

func newSSOTracesExporter(logger *zap.Logger, cfg *Config) (tracesExporter, error) {
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
	}, nil
}
