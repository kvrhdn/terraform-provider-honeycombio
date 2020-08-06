package honeycombio

import (
	"encoding/json"
	"errors"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	honeycombio "github.com/kvrhdn/go-honeycombio"
)

func newBoard() *schema.Resource {
	return &schema.Resource{
		Create: resourceBoardCreate,
		Read:   resourceBoardRead,
		Update: resourceBoardUpdate,
		Delete: resourceBoardDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"style": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "list",
				ValidateFunc: validation.StringInSlice([]string{"list", "visual"}, false),
			},
			"query": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"caption": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"dataset": {
							Type:     schema.TypeString,
							Required: true,
						},
						"query_json": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: func(i interface{}, s string) ([]string, []error) {
								err := json.Unmarshal([]byte(i.(string)), &honeycombio.QuerySpec{})
								if err != nil {
									return nil, []error{errors.New("Value of query_json is not a valid query specification")}
								}
								return nil, nil
							},
						},
					},
				},
			},
		},
	}
}

func resourceBoardCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)

	b, err := expandBoard(d)
	if err != nil {
		return err
	}

	b, err = client.Boards.Create(b)
	if err != nil {
		return err
	}

	d.SetId(b.ID)
	return resourceBoardRead(d, meta)
}

func resourceBoardRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)

	b, err := client.Boards.Get(d.Id())
	if err != nil {
		if err == honeycombio.ErrNotFound {
			d.SetId("")
			return nil
		}
		return err
	}

	d.SetId(b.ID)
	d.Set("name", b.Name)
	d.Set("description", b.Description)
	d.Set("style", b.Style)

	//TODO queries
	//d.Set("queries", queries)

	return nil
}

func resourceBoardUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)

	b, err := expandBoard(d)
	if err != nil {
		return err
	}

	b, err = client.Boards.Update(b)
	if err != nil {
		return err
	}

	d.SetId(b.ID)
	return resourceBoardRead(d, meta)
}

func resourceBoardDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)
	return client.Boards.Delete(d.Id())
}

func expandBoard(d *schema.ResourceData) (*honeycombio.Board, error) {
	var queries []honeycombio.BoardQuery

	qs := d.Get("query").([]interface{})
	for _, q := range qs {
		m := q.(map[string]interface{})
		caption := m["caption"].(string)
		dataset := m["dataset"].(string)
		queryJson := m["query_json"].(string)
		var query honeycombio.QuerySpec
		err := json.Unmarshal([]byte(queryJson), &query)
		if err != nil {
			return nil, err
		}
		queries = append(queries, honeycombio.BoardQuery{
			Caption: caption,
			Dataset: dataset,
			Query:   query,
		})
	}

	board := &honeycombio.Board{
		ID:          d.Id(),
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Style:       honeycombio.BoardStyle(d.Get("style").(string)),
		Queries:     queries,
	}
	return board, nil
}