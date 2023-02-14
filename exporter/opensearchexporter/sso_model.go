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
	"time"
)

type DataStream struct {
	Dataset   string `json:"dataset,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Type      string `json:"type,omitempty"`
}

type SSOSpanEvent struct {
	Attributes             map[string]any `json:"attributes"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
	Name                   string         `json:"name"`
	ObservedTimestamp      *time.Time     `json:"observedTimestamp,omitempty"`
	Timestamp              *time.Time     `json:"@timestamp,omitempty"`
}

type SSOSpanLinks struct {
	Attributes             map[string]any `json:"attributes,omitempty"`
	SpanID                 string         `json:"spanId,omitempty"`
	TraceID                string         `json:"traceId,omitempty"`
	TraceState             string         `json:"traceState,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount,omitempty"`
}

type SSOSpan struct {
	Attributes             map[string]any `json:"attributes,omitempty"`
	DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
	DroppedEventsCount     uint32         `json:"droppedEventsCount"`
	DroppedLinksCount      uint32         `json:"droppedLinksCount"`
	EndTime                time.Time      `json:"endTime"`
	Events                 []SSOSpanEvent `json:"events,omitempty"`
	InstrumentationScope   struct {
		Attributes             map[string]any `json:"attributes,omitempty"`
		DroppedAttributesCount uint32         `json:"droppedAttributesCount"`
		Name                   string         `json:"name"`
		SchemaURL              string         `json:"schemaUrl"`
		Version                string         `json:"version"`
	} `json:"instrumentationScope,omitempty"`
	Kind         string         `json:"kind"`
	Links        []SSOSpanLinks `json:"links,omitempty"`
	Name         string         `json:"name"`
	ParentSpanID string         `json:"parentSpanId"`
	Resource     map[string]any `json:"resource,omitempty"`
	SpanID       string         `json:"spanId"`
	StartTime    time.Time      `json:"startTime"`
	Status       struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
	TraceID    string `json:"traceId"`
	TraceState string `json:"traceState"`
}
