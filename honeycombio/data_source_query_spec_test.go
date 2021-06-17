package honeycombio

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceHoneycombioQuerySpec_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          testAccPreCheck(t),
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccQueryConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckOutput("query_json", expectedJSON),
				),
			},
		},
	})
}

const testAccQueryConfig = `
data "honeycombio_query_spec" "test" {
    calculation {
        op     = "AVG"
        column = "duration_ms"
    }

    filter {
        column = "trace.parent_id"
        op     = "does-not-exist"
    }
    filter {
        column        = "duration_ms"
        op            = ">"
        value_integer = 100
    }
    filter {
        column = "app.tenant"
        op     = "="
        value  = "ThatSpecialTenant"
    }

    breakdowns = ["column_1"]

    order {
        op     = "AVG"
        column = "duration_ms"
    }
    order {
        column = "column_1"
        order  = "descending"
    }

    limit 	    = 250
	time_range  = 7200
	start_time  = 1577836800
    granularity = 30
}

output "query_json" {
    value = data.honeycombio_query_spec.test.json
}`

//Note: By default go encodes `<` and `>` for html, hence the `\u003e`
const expectedJSON string = `{
  "calculations": [
    {
      "op": "AVG",
      "column": "duration_ms"
    }
  ],
  "filters": [
    {
      "column": "trace.parent_id",
      "op": "does-not-exist"
    },
    {
      "column": "duration_ms",
      "op": "\u003e",
      "value": 100
    },
    {
      "column": "app.tenant",
      "op": "=",
      "value": "ThatSpecialTenant"
    }
  ],
  "filter_combination": "AND",
  "breakdowns": [
    "column_1"
  ],
  "orders": [
    {
      "op": "AVG",
      "column": "duration_ms"
    },
    {
      "column": "column_1",
      "order": "descending"
    }
  ],
  "limit": 250,
  "time_range": 7200,
  "start_time": 1577836800,
  "granularity": 30
}`

func TestAccDataSourceHoneycombioQuerySpec_validationChecks(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          testAccPreCheck(t),
		ProviderFactories: testAccProviderFactories,
		Steps: appendAllTestSteps(
			testStepsQueryValidationChecks_calculation,
			testStepsQueryValidationChecks_filter,
			testStepsQueryValidationChecks_limit(),
			testStepsQueryValidationChecks_time,
		),
	})
}

var testStepsQueryValidationChecks_calculation = []resource.TestStep{
	{
		Config: `
data "honeycombio_query_spec" "test" {
  calculation {
    op     = "COUNT"
    column = "we-should-not-specify-a-column-with-COUNT"
  }
}
`,
		ExpectError: regexp.MustCompile("calculation op COUNT should not have an accompanying column"),
	},
	{
		Config: `
data "honeycombio_query_spec" "test" {
  calculation {
    op     = "AVG"
  }
}
`,
		ExpectError: regexp.MustCompile("calculation op AVG is missing an accompanying column"),
	},
}

var testStepsQueryValidationChecks_filter = []resource.TestStep{
	{
		Config: `
data "honeycombio_query_spec" "test" {
  filter {
    column = "column"
    op     = "exists"
    value  = "this-value-should-not-be-here"
  }
}
`,
		ExpectError: regexp.MustCompile("filter operation exists must not contain a value"),
	},
	{
		Config: `
data "honeycombio_query_spec" "test" {
  filter {
    column = "column"
    op     = ">"
  }
}
`,
		ExpectError: regexp.MustCompile("filter operation > requires a value"),
	},
	{
		Config: `
data "honeycombio_query_spec" "test" {
  filter {
    column        = "column"
    op            = ">"
    value_string  = "1"
    value_integer = 10
  }
}
`,
		ExpectError: regexp.MustCompile(multipleValuesError),
	},
	{
		Config: `
data "honeycombio_query_spec" "test" {
  filter {
    column        = "column"
    op            = "in"
    value_integer = 10
  }
}
`,
		ExpectError: regexp.MustCompile("value must be a string if filter op is 'in' or 'not-in'"),
	},
}

func testStepsQueryValidationChecks_limit() []resource.TestStep {
	var queryLimitFmt = `
data "honeycombio_query_spec" "test" {
  limit = %d
}`
	return []resource.TestStep{
		{
			Config:      fmt.Sprintf(queryLimitFmt, 0),
			ExpectError: regexp.MustCompile("expected limit to be in the range \\(1 - 1000\\)"),
		},
		{
			Config:      fmt.Sprintf(queryLimitFmt, -5),
			ExpectError: regexp.MustCompile("expected limit to be in the range \\(1 - 1000\\)"),
		},
		{
			Config:      fmt.Sprintf(queryLimitFmt, 1200),
			ExpectError: regexp.MustCompile("expected limit to be in the range \\(1 - 1000\\)"),
		},
	}
}

var testStepsQueryValidationChecks_time = []resource.TestStep{
	{
		Config: `
data "honeycombio_query_spec" "test" {
  time_range = 7200
  start_time = 1577836800
  end_time   = 1577844000
}
`,
		ExpectError: regexp.MustCompile("specify at most two of time_range, start_time and end_time"),
	},
	{
		Config: `
data "honeycombio_query_spec" "test" {
  time_range  = 120
  granularity = 13
}
`,
		ExpectError: regexp.MustCompile("granularity can not be greater than time_range/10"),
	},
	{
		Config: `
data "honeycombio_query_spec" "test" {
  time_range  = 60000
  granularity = 59
}
`,
		ExpectError: regexp.MustCompile("granularity can not be less than time_range/1000"),
	},
}

func appendAllTestSteps(steps ...[]resource.TestStep) []resource.TestStep {
	var allSteps []resource.TestStep
	for _, s := range steps {
		allSteps = append(allSteps, s...)
	}
	return allSteps
}

func TestAccDataSourceHoneycombioQuerySpec_filterOpInAndNotIn(t *testing.T) {
	dataset := testAccDataset()

	resource.Test(t, resource.TestCase{
		PreCheck:          testAccPreCheck(t),
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "honeycombio_query_spec" "test" {
  calculation {
    op = "COUNT"
  }

  filter {
    column = "app.tenant"
    op     = "in"
    value  = "foo,bar"
  }
  filter {
    column = "app.tenant"
    op     = "not-in"
    value  = "fzz,bzz"
  }
}

resource "honeycombio_board" "test" {
  name = "terraform-provider-honeycombio - Test honeycombio-query - filter ops in/not-in"
  query {
    dataset    = "%v"
    query_json = data.honeycombio_query_spec.test.json
  }
}`, dataset),
			},
		},
	})
}
