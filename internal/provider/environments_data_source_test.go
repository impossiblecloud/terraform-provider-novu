package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestEnvironmentsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: `
				data "novu_environments" "test" {
				}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckTypeSetElemNestedAttrs("data.novu_environments.test", "items.*", map[string]string{"name": "Development"}),
					resource.TestCheckTypeSetElemNestedAttrs("data.novu_environments.test", "items.*", map[string]string{"name": "Production"}),
				),
			},
		},
	})
}
