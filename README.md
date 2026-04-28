# Terraform Provider for Novu

A minimal Terraform provider for managing [Novu](https://novu.co) resources. Novu is an open-source notification infrastructure that provides a unified API for managing multi-channel notifications across Email, SMS, Push, Chat, and In-App channels.

This provider is built using the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) and allows you to manage Novu resources such as workflows, integrations, environments, and API keys using Infrastructure as Code.

**Please note that this provider only implements a minimal set of resources and data sources for our needs.**

## Features

- **Workflow Management**: Create and manage notification workflows with support for push notifications and emails
- **Integration Management**: Configure notification provider integrations (currently supports FCM only)
- **Environment & API Key Discovery**: Query Novu environments and API keys
- **Provider Validation**: Verify provider type existence (through the Novu Go SDK)
- **Multi-Region Support**: Support for both US and EU Novu regions, as well as custom API URLs


## Installation

### Terraform Registry

The provider will be available on the Terraform Registry at:

```hcl
terraform {
  required_providers {
    novu = {
      source  = "flex-o-sas/novu"
      version = "~> 0.1"
    }
  }
}
```

### Local Development

This will build the provider and install it to your `$GOPATH/bin` directory. You can then use a provider development override in your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "flex-o-sas/novu" = "/path/to/your/go/bin"
  }
  # For all other providers, install them directly from their origin provider registry
  direct {}
}
```

## Provider Configuration

### Configuration Options

| Argument | Description | Required | Default | Environment Variable |
|----------|-------------|----------|---------|---------------------|
| `api_key` | Novu API key for authentication | Yes | - | `NOVU_API_KEY` |
| `eu_region` | Use EU region API endpoint | No | `false` | `NOVU_EU_REGION` |
| `api_url` | Custom API URL (overrides `eu_region`) | No | SDK default (US) | `NOVU_API_URL` |



**Notes**: 
- `eu_region` and `api_url` cannot be set simultaneously.
- Required arguments means either the provider configuration or the environment variable must be set.

## Development

### Installing Locally

Build the provider and install it to your `$GOPATH/bin` directory:
```shell
go install -v .
```

Set up the provider development override in your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "flex-o-sas/novu" = "/path/to/your/go/bin"
  }
  # For all other providers, install them directly from their origin provider registry
  direct {}
}
```

### Running Tests

Testing requires setting the `NOVU_API_KEY` environment variable, pointing to an empty Novu environment.
If the environment already contains data, the tests may fail.

```shell
# Acceptance tests
make provider-testacc

# Acceptance tests with OpenTofu
make provider-testacc-tofu
```

**Warning**: Acceptance tests create real resources that might not be deleted if a test crashes.

### Generating Documentation

Documentation is auto-generated from schema definitions and examples:

```shell
make generate
```

### Code Quality

```shell
# Format code
make fmt

# Run linter
make lint
```

## Important Notes

### Novu SDK Compatibility

This provider uses the [Novu Go SDK](https://github.com/novuhq/novu-go). Due to limitations and bugs in the SDK, this provider implements a hybrid approach:

- Uses the SDK for some operations
- Makes direct API calls for others
- Reuses SDK structures and models where possible

**SDK Version Caution**: Exercise caution when upgrading the Novu Go SDK:
1. Breaking changes may occur even in minor versions
2. New versions may introduce critical bugs
3. Documentation links are generated based on SDK version

The current SDK version is pinned to a tested version. Check `go.mod` for the specific version in use.

### Current Limitations

1. **Workflow Steps**: Only push notification steps are currently supported.
2. **Integrations**: Only FCM integration is currently supported.
3. **State Management**: Some Novu API responses may not include all fields, which can cause state drift.


## Contributing

Contributions are welcome! Please ensure:

1. Code follows Go best practices and passes `make lint`
2. New resources/data sources are added with tests, documentation and examples
3. All tests pass: `make provider-testacc`
4. Documentation is updated: `make generate`



## Related Resources

- [Novu Documentation](https://docs.novu.co)
- [Novu API Reference](https://docs.novu.co/api-reference)
- [Novu Go SDK](https://github.com/novuhq/novu-go)
- [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework)
- [Terraform Provider Scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding)
