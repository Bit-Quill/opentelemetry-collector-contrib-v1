// Copyright 2020, OpenTelemetry Authors
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

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr            = "opensearch"
	defaultLogsIndex   = "logs-generic-default"
	defaultTracesIndex = "traces-generic-default"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for OpenSearch exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, stability),
		exporter.WithTraces(createTracesExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: HTTPClientSettings{
			Timeout: 90 * time.Second,
		},
		LogsIndex:   defaultLogsIndex,
		TracesIndex: defaultTracesIndex,
		Retry: RetrySettings{
			Enabled:         true,
			MaxRequests:     3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Minute,
		},
		Mapping: MappingsSettings{
			Mode:  "sso",
			Dedup: true,
			Dedot: true,
		},
	}
}

// createLogsExporter creates a new exporter for logs.
//
// Logs are directly indexed into OpenSearch.
func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	logsExporter, err := newLogsExporter(set.Logger, cfg.(*Config))
	if err != nil {
		return nil, fmt.Errorf("cannot configure OpenSearch logs logsExporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		logsExporter.pushLogsData,
		exporterhelper.WithShutdown(logsExporter.Shutdown),
	)
}

func createTracesExporter(ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Traces, error) {

	tracesExporter, err := newTracesExporter(set.Logger, cfg.(*Config))
	if err != nil {
		return nil, fmt.Errorf("cannot configure OpenSearch traces tracesExporter: %w", err)
	}
	return exporterhelper.NewTracesExporter(ctx, set, cfg, tracesExporter.pushTraceData,
		exporterhelper.WithShutdown(tracesExporter.Shutdown))
}