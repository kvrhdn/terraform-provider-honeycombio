package honeycombio

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/kvrhdn/go-honeycombio"
	"github.com/stretchr/testify/assert"
)

func TestAccHoneycombioQuery_update(t *testing.T) {
	dataset := testAccDataset()
	firstDuration := 20
	secondDuration := 40

	resource.Test(t, resource.TestCase{
		PreCheck:          testAccPreCheck(t),
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceQueryConfig(dataset, firstDuration),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQueryExists(t, dataset, "honeycombio_query.test", firstDuration),
				),
			},
			{
				Config: testAccResourceQueryConfig(dataset, secondDuration),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckQueryExists(t, dataset, "honeycombio_query.test", secondDuration),
				),
			},
		},
	})
}

func testAccResourceQueryConfig(dataset string, duration int) string {
	return fmt.Sprintf(`
data "honeycombio_query_spec" "test" {
  calculation {
    op     = "AVG"
    column = "duration_ms"
  }

  filter {
    column = "duration_ms"
    op     = ">"
    value  = %d
  }
}

resource "honeycombio_query" "test" {
  dataset = "%s"
  query_json = data.honeycombio_query_spec.test.json
}
`, duration, dataset)
}

func testAccCheckQueryExists(t *testing.T, dataset string, name string, duration int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}

		client := testAccClient(t)
		createdQuery, err := client.Queries.Get(context.Background(), dataset, resourceState.Primary.ID)
		if err != nil {
			return fmt.Errorf("could not find created query: %w", err)
		}

		expectedQuery := &honeycombio.QuerySpec{
			ID: &resourceState.Primary.ID,
			Calculations: []honeycombio.CalculationSpec{
				{
					Op:     honeycombio.CalculationOpAvg,
					Column: honeycombio.StringPtr("duration_ms"),
				},
			},
			Filters: []honeycombio.FilterSpec{
				{
					Column: "duration_ms",
					Op:     ">",
					Value:  strconv.Itoa(duration),
				},
			},
			TimeRange: honeycombio.IntPtr(7200),
		}

		ok = assert.Equal(t, expectedQuery, createdQuery)
		if !ok {
			return errors.New("created query did not match expected query")
		}
		return nil
	}
}
