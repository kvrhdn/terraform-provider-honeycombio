# Data Source: honeycombio_query_spec

Construct a query specification that can be used in triggers and boards. For more information about the query specification, check out [Query Specification](https://docs.honeycomb.io/api/query-specification/).

The `json` attribute contains a serialized JSON representation which can be passed to the `query` field of [`honeycombio_trigger`](../resources/trigger.md) and [`honeycombio_board`](../resources/board.md).

## Example Usage

```hcl
data "honeycombio_query_spec" "example" {
  # zero or more calculation blocks
  calculation {
    op     = "AVG"
    column = "duration_ms"
  }

  # zero or more filter blocks
  filter {
    column = "trace.parent_id"
    op     = "does-not-exist"
  }

  filter {
    column = "app.tenant"
    op     = "="
    value  = "ThatSpecialTenant" 
  }

  filter_combination = "AND"

  breakdowns = ["app.tenant"]
    
  time_range = 28800 // in seconds, 8 hours
}

output "json_query" {
    value = data.honeycombio_query_spec.example.json
}
```

## Argument Reference

The following arguments are supported:

* `calculation` - (Optional) Zero or more configuration blocks (described below) with the calculations that should be displayed. If no calculations are specified, `COUNT` will be used.
* `filter` - (Optional) Zero or more configuration blocks (described below) with the filters that should be applied.
* `filter_combination` - (Optional) How to combine multiple filters, either `AND` (default) or `OR`.
* `breakdowns` - (Optional) A list of fields to group by.
* `order` - (Optional) Zero or more configuration blocks (described below) describing how to order the query results. Each term must appear in either `calculation` or `breakdowns`.
* `limit` - (Optional)  The maximum number of query results, must be between 1 and 1000.
* `time_range` - (Optional) The time range of the query in seconds, defaults to two hours.
* `start_time` - (Optional) The absolute start time of the query in Unix Time (= seconds since epoch).
* `end_time` - (Optional) The absolute end time of the query in Unix Time (= seconds since epoch).
* `granularity` - (Optional) The time resolution of the query’s graph, in seconds. Valid values must be in between the query’s time range /10 at maximum, and /1000 at minimum.

~> **NOTE** It is not allowed to specify all three of `time_range`, `start_time` and `end_time`. For more details about specifying time windows, check [Query specification: A caveat on time](https://docs.honeycomb.io/api/query-specification/#a-caveat-on-time).

Each query configuration may have zero or more `calculation` blocks, which each accept the following arguments:

* `op` - (Required) The operator to apply, see the supported list of calculation operators at [Calculation Operators](https://docs.honeycomb.io/api/query-specification/#calculation-operators).
* `column` - (Optional) The column to apply the operator to, not needed with `COUNT`.

Each query configuration may have zero or more `filter` blocks, which each accept the following arguments:

* `column` - (Required) The column to apply the filter to.
* `op` - (Required) The operator to apply, see the supported list of filter operators at [Filter Operators](https://docs.honeycomb.io/api/query-specification/#filter-operators). Not all operators require a value.
* `value_string` - (Optional) The value used for the filter when the column is a string. Mutually exclusive with `value` and the other `value_*` options.
* `value_integer` - (Optional) The value used for the filter when the column is an integer. Mutually exclusive with `value` and the other `value_*` options.
* `value_float` - (Optional) The value used for the filter when the column is a float. Mutually exclusive with `value` and the other `value_*` options.
* `value_boolean` - (Optional) The value used for the filter when the column is a boolean. Mutually exclusive with `value` and the other `value_*` options.
* `value` - (Optional) Deprecated: use the explicitly typed `value_string` instead. This variant will break queries when used with non-string columns. Mutually exclusive with the other `value_*` options.

-> **NOTE** The type of the filter value should match with the type of the column. To determine the type of a column visit the dataset settings page, all the columns and their type are listed under _Schema_. This provider will not be able to detect invalid combinations.

-> **NOTE** Filter op `in` and `not-in` expect an array of strings as value. Use the `value_string` attribute and pass the values in single string separated by `,` without additional spaces (similar to the query builder in the UI). For example: the list `foo`, `bar` becomes `foo,bar`.

Each query configuration may have zero or more `order` blocks, which each accept the following arguments. An order term can refer to a `calculation` or a column set in `breakdowns`. When referring to a `calculation`, `op` and `column` must be the same as the calculation.

* `op` - (Optional) The calculation operator to apply, see the supported list of calculation operators at [Calculation Operators](https://docs.honeycomb.io/api/query-specification/#calculation-operators).
* `column` - (Optional) Either a column present in `breakdown` or a column to `op` applies to.
* `order` - (Optional) The sort direction, if set must be `ascending` or `descending`.

## Attribute Reference

In addition to all arguments above, the following attributes are exported:

* `id` - ID of the trigger.
* `json` - JSON representation of the query according to the [Query Specification](https://docs.honeycomb.io/api/query-specification/#fields-on-a-query-specification), can be used as input for other resources.
