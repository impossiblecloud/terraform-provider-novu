package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestWorkflowsDataSourceBasic(t *testing.T) {
	rnumber := acctest.RandIntRange(0, 1000)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				// search before the workflow exists, the environment may already hold other workflows
				Config: testAccWorkflowsDataSourceSearchConfig(rnumber),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.novu_workflows.test", tfjsonpath.New("items"), knownvalue.ListSizeExact(0)),
				},
			},
			{
				Config: testAccFullWorkflowsDataSourceValidSearchConfigWithAWorkflow(rnumber),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.novu_workflows.test", tfjsonpath.New("items"), knownvalue.ListSizeExact(1)),
				},
			},
			{
				Config: testAccFullWorkflowsDataSourceInvalidSearchConfigWithAWorkflow(rnumber),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.novu_workflows.test", tfjsonpath.New("items"), knownvalue.ListSizeExact(0)),
				},
			},
		},
	})
}

func testAccWorkflowsDataSourceSearchConfig(rnumber int) string {
	return fmt.Sprintf(`
data "novu_workflows" "test" {
	search = "test-%d"
}
`, rnumber)
}

func testAccGenerateWorkflowConfig(rnumber int) string {
	return fmt.Sprintf(`
resource "novu_workflow" "test" {
	name = "test-%d"
	workflow_id = "test-%d"
}
`, rnumber, rnumber)
}

func testAccFullWorkflowsDataSourceValidSearchConfigWithAWorkflow(rnumber int) string {
	return fmt.Sprintf(`
%s
data "novu_workflows" "test" {
	depends_on = [novu_workflow.test]
	search = "test-%d"
}
`, testAccGenerateWorkflowConfig(rnumber), rnumber)
}

func testAccFullWorkflowsDataSourceInvalidSearchConfigWithAWorkflow(rnumber int) string {
	return fmt.Sprintf(`
%s
data "novu_workflows" "test" {
	depends_on = [novu_workflow.test]
	search = "not-ok-search"
}
`, testAccGenerateWorkflowConfig(rnumber))
}
