# OpenSearch Exporter

| Status                   |           |
| ------------------------ |-----------|
| Stability                | [dev]     |
| Distributions            | [contrib] |

This exporter supports sending OpenTelemetry signals to [OpenSearch](https://www.opensearch.org).

## Configuration options

- `endpoints`: List of OpenSearch URLs. If endpoints is missing, the
  OPENSEARCH_URL environment variable will be used.
- `num_workers` (optional): Number of workers publishing bulk requests concurrently.
- `traces_index`: The
  [index](https://opensearch.org/docs/latest/opensearch/index-data/)
  or [datastream](https://opensearch.org/docs/latest/opensearch/data-streams/)
  name to publish trace events to. The default value is `traces-generic-default`. Note: this value is only used when `mapping.mode = none`. In `SSO` mode, destination index (or datastream) is derived from `dataset` and `namespace` options.
- `dataset` (default=`default`) a user-provided label. It is used to construct the name of the destination index or data stream.
- `namespace` (default=`namespace`)a user-provided label. It is used to construct the name of the destination index or data stream.
- `flush`: Event bulk buffer flush settings
  - `bytes` (default=5242880): Write buffer flush limit.
  - `interval` (default=30s): Write buffer time limit.
- `retry`: Event retry settings
  - `enabled` (default=true): Enable/Disable event retry on error. Retry
    support is enabled by default.
  - `max_requests` (default=3): Number of HTTP request retries.
  - `initial_interval` (default=100ms): Initial waiting time if a HTTP request failed.
  - `max_interval` (default=1m): Max waiting time if a HTTP request failed.
- `mapping`: OpenTelemetry signals are encoded to JSON. The `mapping` section allows users to
  additional related options.
  - `mode` (default=none): Determines the JSON schema and related behaviour. Valid options are `none` or `sso`.
  - `dedup` (default=true): Applies only when `mode = none`. When `dedup` is `true`, the exporter will try to find and remove duplicate fields/attributes
    from events before publishing to OpenSearch. Some structured logging
    libraries can produce duplicate fields (for example zap). OpenSearch
    will reject documents that have duplicate fields.
  - `dedot` (default=true): When enabled attributes with `.` will be split into
    proper json objects.
### HTTP settings

- `read_buffer_size` (default=0): Read buffer size.
- `write_buffer_size` (default=0): Write buffer size used when.
- `timeout` (default=90s): HTTP request time limit.
- `headers` (optional): Headers to be send with each HTTP request.

### Security and Authentication settings

- `user` (optional): Username used for HTTP Basic Authentication.
- `password` (optional): Password used for HTTP Basic Authentication.

### TLS settings
- `ca_file` (optional): Root Certificate Authority (CA) certificate, for
  verifying the server's identity, if TLS is enabled.
- `cert_file` (optional): Client TLS certificate.
- `key_file` (optional): Client TLS key.
- `insecure` (optional): In gRPC when set to true, this is used to disable the client transport security. In HTTP, this disables verifying the server's certificate chain and host name.
- `insecure_skip_verify` (optional): Will enable TLS but not verify the certificate.
  is enabled.

## Example

```yaml
exporters:
  opensearch/trace:
    endpoints: [https://opensearch.example.com:9200]
    traces_index: trace_index
  opensearch/log:
    endpoints: [http://localhost:9200]
    logs_index: log_index
# ······
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [opensearch/log]
    traces:
      receivers: [otlp]
      exporters: [opensearch/trace]
      processors: [batch]
```
[beta]:https://github.com/open-telemetry/opentelemetry-collector#development
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib