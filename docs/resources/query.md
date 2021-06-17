# Resource: honeycombio_query

Creates a query in a dataset.

-> **Note** Queries can only be created or read. Any changes will result in a new query object being created, and destroying it does nothing.

## Example Usage

```hcl
variable "dataset" {
  type = string
}

data "honeycombio_query" "test_query" {
  calculation {
    op     = "AVG"
    column = "duration_ms"
  }

  filter {
    column = "duration_ms"
    op     = ">"
    value  = "200"
  }
}

resource "honeycombio_query" "test_query" {
  dataset = "%s"
  query_json = data.honeycombio_query.test_query.json
}
```

## Argument Reference

The following arguments are supported:

* `dataset` - (Required) The dataset this query is added to.
* `query_json` - (Required) A JSON object describing the query according to the [Query Specification](https://docs.honeycomb.io/api/query-specification/#fields-on-a-query-specification). While the JSON can be constructed manually, it is easiest to use the [`honeycombio_query`](terraform-provider-honeycombio/docs/data-sources/query_spec.md) data source.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - ID of the query. Useful for adding it to a board and/or creating a query annotation.

## Import

Queries cannot be imported.
