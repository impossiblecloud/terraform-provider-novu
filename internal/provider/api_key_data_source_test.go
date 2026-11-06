package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestApiKeyDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: `
				data "novu_api_key" "test" {
					environment_id = "non-existing-id"
				}
				`,
				ExpectError: regexp.MustCompile("No api key found for environment non-existing-id"),
			},
			{
				Config: testApiKeyConfigWithEnvironmentConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.novu_api_key.test", "value"),
					resource.TestCheckResourceAttrSet("data.novu_api_key.test", "owner_id"),
					resource.TestCheckResourceAttrSet("data.novu_api_key.test", "hash"),
				),
			},
		},
	})
}

// The API key only sees the keys of its own environment, so look up that one
var testApiKeyConfigWithEnvironmentConfig = `
data "novu_api_keys" "all" {
}
data "novu_api_key" "test" {
	environment_id = data.novu_api_keys.all.items[0].environment_id
}
`
