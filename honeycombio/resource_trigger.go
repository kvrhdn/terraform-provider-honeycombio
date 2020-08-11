package honeycombio

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	honeycombio "github.com/kvrhdn/go-honeycombio"
)

func newTrigger() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceTriggerCreate,
		ReadContext:   resourceTriggerRead,
		UpdateContext: resourceTriggerUpdate,
		DeleteContext: resourceTriggerDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"dataset": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"disabled": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"query_json": {
				Type:     schema.TypeString,
				Required: true,
				ValidateDiagFunc: validateQueryJSON(func(q *honeycombio.QuerySpec) diag.Diagnostics {
					if len(q.Calculations) != 1 {
						return diag.Errorf("Query of a trigger must have exactly one calculation")
					}
					return nil
				}),
			},
			"threshold": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"op": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{">", ">=", "<", "<="}, false),
						},
						"value": {
							Type:     schema.TypeFloat,
							Required: true,
						},
					},
				},
			},
			"frequency": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
				ValidateFunc: validation.All(
					validation.IntDivisibleBy(60),
					validation.IntBetween(60, 86400),
				),
			},
			"recipient": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					// TODO can we validate either id or type+target is set?
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"type": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringInSlice(validTriggerRecipientTypes, false),
						},
						"target": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

var validTriggerRecipientTypes []string = []string{"email", "marker", "pagerduty", "slack"}

func resourceTriggerCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)
	t, err := expandTrigger(d)
	if err != nil {
		return diag.FromErr(err)
	}

	t, err = client.Triggers.Create(dataset, t)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(t.ID)
	return resourceTriggerRead(ctx, d, meta)
}

func resourceTriggerRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)

	t, err := client.Triggers.Get(dataset, d.Id())
	if err != nil {
		if err == honeycombio.ErrNotFound {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	// API returns nil for filterCombination if set to the default value "AND"
	// To keep the Terraform config simple, we'll explicitly set "AND" ourself
	if t.Query.FilterCombination == nil {
		filterCombination := honeycombio.FilterCombinationAnd
		t.Query.FilterCombination = &filterCombination
	}

	d.SetId(t.ID)
	d.Set("name", t.Name)
	d.Set("description", t.Description)
	d.Set("disabled", t.Disabled)

	encodedQuery, err := encodeQuery(t.Query)
	if err != nil {
		return diag.FromErr(err)
	}
	d.Set("query_json", encodedQuery)

	err = d.Set("threshold", flattenTriggerThreshold(t.Threshold))
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("frequency", t.Frequency)

	err = d.Set("recipient", flattenTriggerRecipients(t.Recipients))
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceTriggerUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)
	t, err := expandTrigger(d)
	if err != nil {
		return diag.FromErr(err)
	}

	t, err = client.Triggers.Update(dataset, t)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(t.ID)
	return resourceTriggerRead(ctx, d, meta)
}

func resourceTriggerDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)

	err := client.Triggers.Delete(dataset, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func expandTrigger(d *schema.ResourceData) (*honeycombio.Trigger, error) {
	var query honeycombio.QuerySpec

	err := json.Unmarshal([]byte(d.Get("query_json").(string)), &query)
	if err != nil {
		return nil, err
	}

	trigger := &honeycombio.Trigger{
		ID:          d.Id(),
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Disabled:    d.Get("disabled").(bool),
		Query:       &query,
		Threshold:   expandTriggerThreshold(d.Get("threshold").([]interface{})),
		Frequency:   d.Get("frequency").(int),
		Recipients:  expandTriggerRecipients(d.Get("recipient").([]interface{})),
	}
	return trigger, nil
}

func expandTriggerThreshold(s []interface{}) *honeycombio.TriggerThreshold {
	d := s[0].(map[string]interface{})

	value := d["value"].(float64)

	return &honeycombio.TriggerThreshold{
		Op:    honeycombio.TriggerThresholdOp(d["op"].(string)),
		Value: &value,
	}
}

func expandTriggerRecipients(s []interface{}) []honeycombio.TriggerRecipient {
	triggerRecipients := make([]honeycombio.TriggerRecipient, len(s))

	for i, r := range s {
		rMap := r.(map[string]interface{})

		triggerRecipients[i] = honeycombio.TriggerRecipient{
			ID:     rMap["id"].(string),
			Type:   honeycombio.TriggerRecipientType(rMap["type"].(string)),
			Target: rMap["target"].(string),
		}
	}

	return triggerRecipients
}

func flattenTriggerThreshold(t *honeycombio.TriggerThreshold) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"op":    t.Op,
			"value": t.Value,
		},
	}
}

func flattenTriggerRecipients(rs []honeycombio.TriggerRecipient) []map[string]interface{} {
	result := make([]map[string]interface{}, len(rs))

	for i, r := range rs {
		result[i] = map[string]interface{}{
			"id":     r.ID,
			"type":   string(r.Type),
			"target": r.Target,
		}
	}

	return result
}
