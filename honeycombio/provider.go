package honeycombio

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/kvrhdn/go-honeycombio"
)

func init() {
	// set global description kind to markdown, as described in:
	// https://www.terraform.io/docs/extend/guides/v2-upgrade-guide.html#support-for-resource-level-and-field-level-descriptions
	schema.DescriptionKind = schema.StringMarkdown
}

// providerVersion represents the current version of the provider. It should be
// overwritten during the release process.
var providerVersion = "dev"

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("HONEYCOMBIO_APIKEY", nil),
				Sensitive:   true,
			},
			"api_url": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"debug": {
				Type:     schema.TypeBool,
				Optional: true,
				DefaultFunc: func() (interface{}, error) {
					b, _ := strconv.ParseBool(os.Getenv("HONEYCOMBIO_DEBUG"))
					return b, nil
				},
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"honeycombio_datasets":          dataSourceHoneycombioDatasets(),
			"honeycombio_query_spec":        dataSourceHoneycombioQuerySpec(),
			"honeycombio_trigger_recipient": dataSourceHoneycombioSlackRecipient(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"honeycombio_board":            newBoard(),
			"honeycombio_column":           newColumn(),
			"honeycombio_dataset":          newDataset(),
			"honeycombio_derived_column":   newDerivedColumn(),
			"honeycombio_marker":           newMarker(),
			"honeycombio_query":            newQuery(),
			"honeycombio_query_annotation": newQueryAnnotation(),
			"honeycombio_trigger":          newTrigger(),
		},
	}

	provider.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		config := &honeycombio.Config{
			APIKey:    d.Get("api_key").(string),
			APIUrl:    d.Get("api_url").(string),
			UserAgent: provider.UserAgent("terraform-provider-honeycombio", providerVersion),
			Debug:     d.Get("debug").(bool),
		}
		c, err := honeycombio.NewClient(config)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		log.Printf("Configured honeycombio client with debug = %t", config.Debug)
		log.Printf("To log requests and responses, set environment variable HONEYCOMBIO_DEBUG to true")

		return c, nil
	}

	return provider
}
