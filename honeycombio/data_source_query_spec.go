package honeycombio

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/kvrhdn/go-honeycombio"
	"github.com/kvrhdn/terraform-provider-honeycombio/honeycombio/internal/hashcode"
)

func dataSourceHoneycombioQuerySpec() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceHoneycombioQuerySpecRead,

		Schema: map[string]*schema.Schema{
			"calculation": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"op": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice(calculationOpStrings(), false),
						},
						"column": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"filter": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"column": {
							Type:     schema.TypeString,
							Required: true,
						},
						"op": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice(filterOpStrings(), false),
						},
						"value": {
							Type:        schema.TypeString,
							Description: "Deprecated: use the explicitly typed `value_string` instead. This variant will break queries when used with non-string columns. Mutually exclusive with the other `value_*` options.",
							Optional:    true,
							Deprecated:  "Use of attribute `value` is discouraged and will break queries when used with non-string columns. The explicitly typed `value_*` variants are preferred instead.",
						},
						"value_string": {
							Type:        schema.TypeString,
							Description: "The value used for the filter when the column is a string. Mutually exclusive with `value` and the other `value_*` options",
							Optional:    true,
						},
						"value_integer": {
							Type:        schema.TypeInt,
							Description: "The value used for the filter when the column is an integer. Mutually exclusive with `value` and the other `value_*` options",
							Optional:    true,
						},
						"value_float": {
							Type:        schema.TypeFloat,
							Description: "The value used for the filter when the column is a float. Mutually exclusive with `value` and the other `value_*` options",
							Optional:    true,
						},
						"value_boolean": {
							Type:        schema.TypeBool,
							Description: "The value used for the filter when the column is a boolean. Mutually exclusive with `value` and the other `value_*` options",
							Optional:    true,
						},
					},
				},
			},
			"filter_combination": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "AND",
				ValidateFunc: validation.StringInSlice([]string{"AND", "OR"}, false),
			},
			"breakdowns": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"order": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"op": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(calculationOpStrings(), false),
						},
						"column": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"order": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(sortOrderStrings(), false),
						},
					},
				},
			},
			"limit": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntBetween(1, 1000),
			},
			"time_range": {
				Type:     schema.TypeInt,
				Optional: true,
				// Note: this field could have been set to be computed, but
				// when only the JSON output is used by another resource,
				// Terraform isn't able to set the computed value causing a
				// constant diff. By using a default value instead, we don't
				// need the feedback from the API.
				Default: 7200,
			},
			"start_time": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"end_time": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"granularity": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"json": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceHoneycombioQuerySpecRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	calculations, err := extractCalculations(d)
	if err != nil {
		return diag.FromErr(err)
	}

	filters, err := extractFilters(d)
	if err != nil {
		return diag.FromErr(err)
	}

	querySpec := &honeycombio.QuerySpec{
		Calculations:      calculations,
		Filters:           filters,
		FilterCombination: honeycombio.FilterCombination(d.Get("filter_combination").(string)),
		Breakdowns:        extractBreakdowns(d),
		Orders:            extractOrders(d),
		Limit:             extractOptionalInt(d, "limit"),
		TimeRange:         extractOptionalInt(d, "time_range"),
		StartTime:         extractOptionalInt64(d, "start_time"),
		EndTime:           extractOptionalInt64(d, "end_time"),
		Granularity:       extractOptionalInt(d, "granularity"),
	}

	if querySpec.TimeRange != nil && querySpec.StartTime != nil && querySpec.EndTime != nil {
		return diag.Errorf("specify at most two of time_range, start_time and end_time")
	}

	if querySpec.TimeRange != nil && querySpec.Granularity != nil {
		if *querySpec.Granularity > (*querySpec.TimeRange / 10) {
			return diag.Errorf("granularity can not be greater than time_range/10")
		}
		if *querySpec.Granularity < (*querySpec.TimeRange / 1000) {
			return diag.Errorf("granularity can not be less than time_range/1000")
		}
	}

	jsonQuery, err := encodeQuery(querySpec)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("json", jsonQuery)
	d.SetId(strconv.Itoa(hashcode.String(jsonQuery)))

	return nil
}

func extractCalculations(d *schema.ResourceData) ([]honeycombio.CalculationSpec, error) {
	calculationSchemas := d.Get("calculation").([]interface{})
	calculations := make([]honeycombio.CalculationSpec, len(calculationSchemas))

	for i := range calculationSchemas {
		calculation := &calculations[i]

		calculation.Op = honeycombio.CalculationOp(d.Get(fmt.Sprintf("calculation.%d.op", i)).(string))

		c, ok := d.GetOk(fmt.Sprintf("calculation.%d.column", i))
		if ok {
			calculations[i].Column = honeycombio.StringPtr(c.(string))
		}

		if calculation.Op == honeycombio.CalculationOpCount && calculation.Column != nil {
			return nil, fmt.Errorf("calculation op %s should not have an accompanying column", calculation.Op)
		} else if calculation.Op != honeycombio.CalculationOpCount && calculation.Column == nil {
			return nil, fmt.Errorf("calculation op %s is missing an accompanying column", calculation.Op)
		}
	}

	return calculations, nil
}

func extractFilters(d *schema.ResourceData) ([]honeycombio.FilterSpec, error) {
	filterSchemas := d.Get("filter").([]interface{})
	filters := make([]honeycombio.FilterSpec, len(filterSchemas))

	for i := range filterSchemas {
		honeyFilter, err := extractFilter(d, i)
		if err != nil {
			return nil, err
		}
		filters[i] = honeyFilter
	}
	return filters, nil
}

const multipleValuesError = "must choose one of 'value', 'value_string', 'value_integer', 'value_float', 'value_boolean'"

func extractFilter(d *schema.ResourceData, index int) (honeycombio.FilterSpec, error) {
	var filter honeycombio.FilterSpec

	filter.Column = d.Get(fmt.Sprintf("filter.%d.column", index)).(string)
	filter.Op = honeycombio.FilterOp(d.Get(fmt.Sprintf("filter.%d.op", index)).(string))

	valueSet := false
	v, vOk := d.GetOk(fmt.Sprintf("filter.%d.value", index))
	if vOk {
		filter.Value = v
		valueSet = true
	}
	vs, vsOk := d.GetOk(fmt.Sprintf("filter.%d.value_string", index))
	if vsOk {
		if valueSet {
			return filter, fmt.Errorf(multipleValuesError)
		}
		filter.Value = vs
		valueSet = true
	}
	vi, viOk := d.GetOk(fmt.Sprintf("filter.%d.value_integer", index))
	if viOk {
		if valueSet {
			return filter, fmt.Errorf(multipleValuesError)
		}
		filter.Value = vi
		valueSet = true
	}
	vf, vfOk := d.GetOk(fmt.Sprintf("filter.%d.value_float", index))
	if vfOk {
		if valueSet {
			return filter, fmt.Errorf(multipleValuesError)
		}
		filter.Value = vf
		valueSet = true
	}
	vb, vbOk := d.GetOkExists(fmt.Sprintf("filter.%d.value_boolean", index))
	if vbOk {
		if valueSet {
			return filter, fmt.Errorf(multipleValuesError)
		}
		filter.Value = vb
		valueSet = true
	}

	if filter.Op == honeycombio.FilterOpIn || filter.Op == honeycombio.FilterOpNotIn {
		vs, ok := filter.Value.(string)
		if !ok {
			return filter, fmt.Errorf("value must be a string if filter op is 'in' or 'not-in'")
		}
		filter.Value = strings.Split(vs, ",")
	}

	if filter.Op == honeycombio.FilterOpExists || filter.Op == honeycombio.FilterOpDoesNotExist {
		if filter.Value != nil {
			return filter, fmt.Errorf("filter operation %s must not contain a value", filter.Op)
		}
	} else {
		if filter.Value == nil {
			return filter, fmt.Errorf("filter operation %s requires a value", filter.Op)
		}
	}
	return filter, nil
}

func extractBreakdowns(d *schema.ResourceData) []string {
	breakdownsRaw := d.Get("breakdowns").([]interface{})
	breakdowns := make([]string, len(breakdownsRaw))

	for i, b := range breakdownsRaw {
		breakdowns[i] = b.(string)
	}

	return breakdowns
}

func extractOrders(d *schema.ResourceData) []honeycombio.OrderSpec {
	orderSchemas := d.Get("order").([]interface{})
	orders := make([]honeycombio.OrderSpec, len(orderSchemas))

	for i := range orderSchemas {
		order := &orders[i]

		op, ok := d.GetOk(fmt.Sprintf("order.%d.op", i))
		if ok {
			order.Op = honeycombio.CalculationOpPtr(honeycombio.CalculationOp(op.(string)))
		}

		c, ok := d.GetOk(fmt.Sprintf("order.%d.column", i))
		if ok {
			order.Column = honeycombio.StringPtr(c.(string))
		}

		so, ok := d.GetOk(fmt.Sprintf("order.%d.order", i))
		if ok {
			order.Order = honeycombio.SortOrderPtr(honeycombio.SortOrder(so.(string)))
		}

		// TODO: validation
	}

	return orders
}

func extractOptionalInt(d *schema.ResourceData, key string) *int {
	value, ok := d.GetOk(key)
	if !ok {
		return nil
	}
	return honeycombio.IntPtr(value.(int))
}

func extractOptionalInt64(d *schema.ResourceData, key string) *int64 {
	value, ok := d.GetOk(key)
	if !ok {
		return nil
	}
	return honeycombio.Int64Ptr(int64(value.(int)))
}
