package provider

import (
	"context"
	"os"

	api_client "terraform-provider-novu/internal/api-client"

	novugo "github.com/novuhq/novu-go"
	"github.com/novuhq/novu-go/retry"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure NovuProvider satisfies various provider interfaces.
var _ provider.Provider = &NovuProvider{}

var markdownDescription = `A minimal Terraform provider for managing some [Novu](https://novu.co) resources.`

// NovuProvider defines the provider implementation.
type NovuProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// NovuProviderModel describes the provider data model.
type NovuProviderModel struct {
	ApiKey   types.String `tfsdk:"api_key"`
	EuRegion types.Bool   `tfsdk:"eu_region"`
	ApiUrl   types.String `tfsdk:"api_url"`
}

type ProviderClients struct {
	Novu *novugo.Novu
	Api  *api_client.ApiClient
}

func (p *NovuProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "novu"
	resp.Version = p.version
}

func (p *NovuProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: markdownDescription,
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "The API key for the Novu provider",
				// Required:            true, TODO: uncomment after testing
				Optional:  true,
				Sensitive: true,
			},
			"eu_region": schema.BoolAttribute{
				MarkdownDescription: "Whether the provider should use the EU region API (default: false)",
				Optional:            true,
			},
			"api_url": schema.StringAttribute{
				MarkdownDescription: "The API URL for the Novu provider. Overrides eu_region (default:SDK default, US: https://api.novu.co as of provider v1.0.0)",
				Optional:            true,
			},
		},
	}
}

func (p *NovuProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config NovuProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// default with env values
	apiKey := os.Getenv("NOVU_API_KEY")
	euRegion := os.Getenv("NOVU_EU_REGION") == "true" || os.Getenv("NOVU_EU_REGION") == "1"
	apiUrl := os.Getenv("NOVU_API_URL")

	//override with config values if set
	if !config.ApiKey.IsNull() && config.ApiKey.ValueString() != "" {
		apiKey = config.ApiKey.ValueString()
	}
	if !config.EuRegion.IsNull() && !config.ApiUrl.IsNull() &&
		!config.EuRegion.IsUnknown() && !config.ApiUrl.IsUnknown() {
		resp.Diagnostics.AddError("Client Error", "Both eu_region and api_url cannot be set at the same time, please set only one")
		return
	}
	if !config.EuRegion.IsNull() && !config.EuRegion.IsUnknown() {
		euRegion = config.EuRegion.ValueBool()
	}

	if euRegion {
		apiUrl = "https://eu.api.novu.co"
	}
	if !config.ApiUrl.IsNull() && !config.ApiUrl.IsUnknown() {
		apiUrl = config.ApiUrl.ValueString()
	}
	// default to US
	// if apiUrl == "" {
	// 	apiUrl = "https://api.novu.co"
	// }

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing or Empty Novu API Key",
			"the 'api_key' property must be set in the provider configuration or via the NOVU_API_KEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	sdkOpts := []novugo.SDKOption{
		novugo.WithSecurity(apiKey),
		//Override default 1 hour max interval
		novugo.WithRetryConfig(retry.Config{
			Strategy: "backoff",
			Backoff: &retry.BackoffStrategy{
				InitialInterval: 1 * 1000,
				MaxInterval:     2 * 60 * 1000,
				Exponent:        1.5,
			},
		}),
	}
	apiClientOpts := []api_client.ApiClientOption{
		api_client.WithApiKey(apiKey),
	}

	if apiUrl != "" {
		sdkOpts = append(sdkOpts, novugo.WithServerURL(apiUrl))
		apiClientOpts = append(apiClientOpts, api_client.WithServerUrl(apiUrl))
	}

	novuClient := novugo.New(sdkOpts...)
	apiClient := api_client.New(apiClientOpts...)
	clients := &ProviderClients{
		Novu: novuClient,
		Api:  apiClient,
	}
	resp.DataSourceData = clients
	resp.ResourceData = clients
}

func (p *NovuProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewWorkflowResource,
		NewFCMIntegrationResource,
		NewLayoutResource,
	}
}

func (p *NovuProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEnvironmentsDataSource,
		NewEnvironmentDataSource,
		NewApiKeysDataSource,
		NewApiKeyDataSource,
		NewProviderDataSource,
		NewWorkflowsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NovuProvider{
			version: version,
		}
	}
}
