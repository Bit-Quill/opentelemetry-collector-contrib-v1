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

type Sso struct {
	Attributes struct {
		DataStream struct {
			Dataset   string `json:"dataset"`
			Namespace string `json:"namespace"`
			Type      string `json:"type"`
		} `json:"data_stream"`
	} `json:"attributes"`
	DroppedAttributesCount uint32    `json:"droppedAttributesCount"`
	DroppedEventsCount     uint32    `json:"droppedEventsCount"`
	DroppedLinksCount      uint32    `json:"droppedLinksCount"`
	EndTime                time.Time `json:"endTime"`
	Events                 []struct {
		Name              string    `json:"name"`
		Timestamp         time.Time `json:"@timestamp"`
		ObservedTimestamp time.Time `json:"observedTimestamp"`
	} `json:"events"`
	InstrumentationScope struct {
		Name                   string `json:"name"`
		Version                string `json:"version"`
		DroppedAttributesCount uint32 `json:"droppedAttributesCount"`
		SchemaURL              string `json:"schemaUrl"`
	} `json:"instrumentationScope"`
	Kind  string `json:"kind"`
	Links struct {
		SpanID     string `json:"spanId"`
		TraceID    string `json:"traceId"`
		TraceState string `json:"traceState"`
	} `json:"links"`
	Name         string `json:"name"`
	ParentSpanID string `json:"parentSpanId"`
	Resource     struct {
		Type string `json:"type"`
	} `json:"resource"`

	StartTime time.Time `json:"startTime"`
	Status    struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
	TraceID    string `json:"traceId"`
	TraceState string `json:"traceState"`
}
