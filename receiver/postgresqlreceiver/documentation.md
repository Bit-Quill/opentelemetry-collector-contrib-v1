[comment]: <> (Code generated by mdatagen. DO NOT EDIT.)

# postgresqlreceiver

## Metrics

These are the metrics available for this scraper.

| Name | Description | Unit | Type | Attributes |
| ---- | ----------- | ---- | ---- | ---------- |
| **postgresql.backends** | The number of backends. | 1 | Sum(Int) | <ul> <li>database</li> </ul> |
| **postgresql.blocks_read** | The number of blocks read. | 1 | Sum(Int) | <ul> <li>database</li> <li>table</li> <li>source</li> </ul> |
| **postgresql.commits** | The number of commits. | 1 | Sum(Int) | <ul> <li>database</li> </ul> |
| **postgresql.db_size** | The database disk usage. | By | Sum(Int) | <ul> <li>database</li> </ul> |
| **postgresql.operations** | The number of db row operations. | 1 | Sum(Int) | <ul> <li>database</li> <li>table</li> <li>operation</li> </ul> |
| **postgresql.rollbacks** | The number of rollbacks. | 1 | Sum(Int) | <ul> <li>database</li> </ul> |
| **postgresql.rows** | The number of rows in the database. | 1 | Sum(Int) | <ul> <li>database</li> <li>table</li> <li>state</li> </ul> |

**Highlighted metrics** are emitted by default. Other metrics are optional and not emitted by default.
Any metric can be enabled or disabled with the following scraper configuration:

```yaml
metrics:
  <metric_name>:
    enabled: <true|false>
```

## Metric attributes

| Name | Description | Values |
| ---- | ----------- | ------ |
| database | The name of the database. |  |
| operation | The database operation. | ins, upd, del, hot_upd |
| source | The block read source type. | heap_read, heap_hit, idx_read, idx_hit, toast_read, toast_hit, tidx_read, tidx_hit |
| state | The tuple (row) state. | dead, live |
| table | The schema name followed by the table name. |  |