package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestLayoutResourceBasic(t *testing.T) {
	randInt := acctest.RandIntRange(0, 1000)
	layoutID := fmt.Sprintf("tf-acc-%d", randInt)
	rname := fmt.Sprintf("tf-acc-%d", randInt)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccLayoutResourceConfig(layoutID, rname, "<html><body><div>{{content}}</div></body></html>"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue("novu_layout.test", tfjsonpath.New("layout_id"), knownvalue.StringExact(layoutID)),
						plancheck.ExpectKnownValue("novu_layout.test", tfjsonpath.New("email").AtMapKey("editor_type"), knownvalue.StringExact("html")),
						plancheck.ExpectUnknownValue("novu_layout.test", tfjsonpath.New("id")),
						plancheck.ExpectUnknownValue("novu_layout.test", tfjsonpath.New("is_default")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("novu_layout.test", tfjsonpath.New("layout_id"), knownvalue.StringExact(layoutID)),
					statecheck.ExpectKnownValue("novu_layout.test", tfjsonpath.New("name"), knownvalue.StringExact(rname)),
					statecheck.ExpectKnownValue("novu_layout.test", tfjsonpath.New("email"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"body":        knownvalue.StringExact("<html><body><div>{{content}}</div></body></html>"),
						"editor_type": knownvalue.StringExact("html"),
					})),
				},
			},
			{
				// update the body in place
				Config: testAccLayoutResourceConfig(layoutID, rname, "<html><body><section>{{content}}</section></body></html>"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("novu_layout.test", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("novu_layout.test", tfjsonpath.New("email").AtMapKey("body"), knownvalue.StringExact("<html><body><section>{{content}}</section></body></html>")),
				},
			},
			{
				ResourceName:                         "novu_layout.test",
				ImportState:                          true,
				ImportStateId:                        layoutID,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "layout_id",
			},
			{
				// re-apply the same config, expect no changes
				Config: testAccLayoutResourceConfig(layoutID, rname, "<html><body><section>{{content}}</section></body></html>"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testAccLayoutResourceConfig(layoutID string, name string, body string) string {
	return fmt.Sprintf(`
resource "novu_layout" "test" {
  layout_id = "%s"
  name      = "%s"
  email = {
    body = "%s"
  }
}
`, layoutID, name, body)
}
