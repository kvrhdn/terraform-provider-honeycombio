package honeycombio

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/kvrhdn/go-honeycombio"
)

func TestAccHoneycombioTrigger_basic(t *testing.T) {
	var triggerBefore, triggerAfter honeycombio.Trigger

	dataset := testAccDataset()

	resource.Test(t, resource.TestCase{
		PreCheck:          testAccPreCheck(t),
		ProviderFactories: testAccProviderFactories,
		IDRefreshName:     "honeycombio_trigger.test",
		Steps: []resource.TestStep{
			{
				Config: testAccTriggerConfigWithFrequency(dataset, 900),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTriggerExists(t, "honeycombio_trigger.test", &triggerBefore),
					testAccCheckTriggerAttributes(&triggerBefore),
					resource.TestCheckResourceAttr("honeycombio_trigger.test", "frequency", "900"),
				),
			},
			{
				Config: testAccTriggerConfigWithFrequency(dataset, 300),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTriggerExists(t, "honeycombio_trigger.test", &triggerAfter),
					resource.TestCheckResourceAttr("honeycombio_trigger.test", "frequency", "300"),
				),
			},
			{
				Config: testAccTriggerConfigWithCount(dataset),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckTriggerExists(t, "honeycombio_trigger.test", &triggerAfter),
				),
			},
			{
				ResourceName:        "honeycombio_trigger.test",
				ImportStateIdPrefix: fmt.Sprintf("%v/", dataset),
				ImportState:         true,
				ImportStateVerify:   true,
			},
		},
	})
}

func testAccCheckTriggerExists(t *testing.T, name string, trigger *honeycombio.Trigger) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("not found: %s", name)
		}

		client := testAccClient(t)
		createdTrigger, err := client.Triggers.Get(context.Background(), resourceState.Primary.Attributes["dataset"], resourceState.Primary.ID)
		if err != nil {
			return fmt.Errorf("could not find created trigger: %w", err)
		}

		*trigger = *createdTrigger
		return nil
	}
}

func testAccCheckTriggerAttributes(t *honeycombio.Trigger) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if t.Name != "Test trigger from terraform-provider-honeycombio" {
			return fmt.Errorf("bad name: %s", t.Name)
		}

		if t.Frequency != 900 {
			return fmt.Errorf("bad frequency: %d", t.Frequency)
		}

		return nil
	}
}

// add a trigger recipient by ID to verify the diff is stable
func TestAccHoneycombioTrigger_triggerRecipientById(t *testing.T) {
	dataset := testAccDataset()

	trigger, deleteFn := createTriggerWithRecipient(t, dataset, honeycombio.TriggerRecipient{
		Type:   honeycombio.TriggerRecipientTypeEmail,
		Target: "acctest@example.com",
	})
	defer deleteFn()

	resource.Test(t, resource.TestCase{
		PreCheck:          testAccPreCheck(t),
		ProviderFactories: testAccProviderFactories,
		IDRefreshName:     "honeycombio_trigger.test",
		Steps: []resource.TestStep{
			{
				Config: testAccTriggerConfigWithRecipientID(dataset, trigger.Recipients[0].ID),
			},
		},
	})
}

func TestAccHoneycombioTrigger_validationErrors(t *testing.T) {
	dataset := testAccDataset()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          testAccPreCheck(t),
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccTriggerConfigWithQuery(dataset, `{]`),
				ExpectError: regexp.MustCompile("value of query_json is not a valid query specification"),
			},
			{
				Config:      testAccTriggerConfigWithQuery(dataset, `{"calculations":"bar"}`),
				ExpectError: regexp.MustCompile("value of query_json is not a valid query specification"),
			},
			{
				Config: testAccTriggerConfigWithQuery(dataset, `
{
    "calculations": [
        {"op": "COUNT"},
        {"op": "AVG", "column": "duration_ms"}
    ]
}`),
				ExpectError: regexp.MustCompile("a trigger query should contain exactly one calculation"),
			},
			{
				Config: testAccTriggerConfigWithQuery(dataset, `
{
    "calculations": [
        {"op": "HEATMAP"}
    ]
}`),
				ExpectError: regexp.MustCompile("a trigger query may not contain a HEATMAP calculation"),
			},
			{
				Config: testAccTriggerConfigWithQuery(dataset, `
{
    "calculations": [
        {"op": "AVG", "column": "duration_ms"}
    ],
    "orders": [
        {"column": "column_1"}
    ]
}`),
				ExpectError: regexp.MustCompile("orders is not allowed in a trigger query"),
			},
			{
				Config: testAccTriggerConfigWithQuery(dataset, `
{
    "calculations": [
        {"op": "AVG", "column": "duration_ms"}
    ],
    "limit": 100
}`),
				ExpectError: regexp.MustCompile("limit is not allowed in a trigger query"),
			},
			{
				ResourceName:      "honeycombio_trigger.test",
				ImportStateId:     "someId",
				ImportState:       true,
				ImportStateVerify: true,
				ExpectError:       regexp.MustCompile("invalid import ID, supplied ID must be written as <dataset>/<trigger ID>"),
			},
		},
	})
}

func testAccTriggerConfigWithFrequency(dataset string, frequency int) string {
	return fmt.Sprintf(`
data "honeycombio_query_spec" "test" {
  calculation {
    op     = "AVG"
    column = "duration_ms"
  }
  time_range = 1200
}

resource "honeycombio_trigger" "test" {
  name    = "Test trigger from terraform-provider-honeycombio"
  dataset = "%s"

  query = data.honeycombio_query_spec.test.json

  threshold {
    op    = ">"
    value = 100
  }

  frequency = %d

  recipient {
    type   = "email"
    target = "hello@example.com"
  }

  recipient {
    type   = "email"
    target = "bye@example.com"
  }
}`, dataset, frequency)
}

func testAccTriggerConfigWithCount(dataset string) string {
	return fmt.Sprintf(`
data "honeycombio_query_spec" "test" {
  calculation {
    op     = "COUNT"
  }
  time_range = 1200
}

resource "honeycombio_trigger" "test" {
  name    = "Test trigger from terraform-provider-honeycombio"
  dataset = "%s"

  query = data.honeycombio_query_spec.test.json

  threshold {
    op    = ">"
    value = 100
  }
}`, dataset)
}

func testAccTriggerConfigWithQuery(dataset, query string) string {
	return fmt.Sprintf(`
resource "honeycombio_trigger" "test" {
  name    = "Test trigger from terraform-provider-honeycombio"
  dataset = "%s"

  query = <<EOF
%s
EOF
    
  threshold {
    op    = ">"
    value = 100
  }
}`, dataset, query)
}

func testAccTriggerConfigWithRecipientID(dataset, recipientID string) string {
	return fmt.Sprintf(`
data "honeycombio_query_spec" "test" {
  calculation {
    op     = "AVG"
    column = "duration_ms"
  }
  time_range = 1800
}

resource "honeycombio_trigger" "test" {
  name    = "Test trigger from terraform-provider-honeycombio"
  dataset = "%s"

  query = data.honeycombio_query_spec.test.json
        
  threshold {
    op    = ">"
    value = 100
  }

  recipient {
    id = "%s"
  }
}`, dataset, recipientID)
}
