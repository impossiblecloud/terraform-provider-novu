package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	novugo "github.com/novuhq/novu-go"
	"github.com/novuhq/novu-go/models/apierrors"
	"github.com/novuhq/novu-go/models/components"
)

var (
	_ resource.Resource                = &LayoutResource{}
	_ resource.ResourceWithConfigure   = &LayoutResource{}
	_ resource.ResourceWithImportState = &LayoutResource{}
)

type LayoutResource struct {
	client *novugo.Novu
}

type LayoutResourceModel struct {
	ID        types.String      `tfsdk:"id"`
	LayoutID  types.String      `tfsdk:"layout_id"`
	Name      types.String      `tfsdk:"name"`
	IsDefault types.Bool        `tfsdk:"is_default"`
	Email     *LayoutEmailModel `tfsdk:"email"`
}

type LayoutEmailModel struct {
	Body       types.String `tfsdk:"body"`
	EditorType types.String `tfsdk:"editor_type"`
}

func NewLayoutResource() resource.Resource {
	return &LayoutResource{}
}

func (r *LayoutResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_layout"
}

func (r *LayoutResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	if clients, ok := req.ProviderData.(*ProviderClients); ok && clients.Novu != nil {
		r.client = clients.Novu
	} else {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *ProviderClients, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
}

func (r *LayoutResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Layout resource. Manage an email layout within the environment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Database identifier of the layout",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"layout_id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier of the layout, used by email steps to reference it",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`), "Layout ID must contain only letters, numbers, underscores and hyphens"),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the layout",
				Required:            true,
			},
			"is_default": schema.BoolAttribute{
				MarkdownDescription: "Whether the layout is the default layout of the environment",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.SingleNestedAttribute{
				MarkdownDescription: "Email content of the layout",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"body": schema.StringAttribute{
						MarkdownDescription: "Body of the layout. Must contain the `{{content}}` variable where the email step content is rendered, and for the `html` editor a full document with `<html>` and `<body>` tags.",
						Required:            true,
					},
					"editor_type": schema.StringAttribute{
						MarkdownDescription: "Type of editor used for the body. Allowed values: `block`, `html`. Defaults to `html`.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString(string(components.EditorTypeHTML)),
						Validators: []validator.String{
							stringvalidator.OneOf(string(components.EditorTypeBlock), string(components.EditorTypeHTML)),
						},
					},
				},
			},
		},
	}
}

func (r *LayoutResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LayoutResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	layoutID := plan.LayoutID.ValueString()

	createReq := components.CreateLayoutDto{
		LayoutID: layoutID,
		Name:     plan.Name.ValueString(),
	}
	if _, err := r.client.Layouts.Create(ctx, createReq, nil); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create layout: %s", err))
		return
	}

	// The create endpoint does not accept the email content, so it is set with an update
	layoutRes, err := r.client.Layouts.Update(ctx, layoutID, plan.toUpdateDto(), nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set the content of the created layout: %s", err))
		return
	}
	if layoutRes == nil || layoutRes.LayoutResponseDto == nil {
		resp.Diagnostics.AddError("Client Error", "Unable to confirm creation of layout: Response is nil")
		return
	}

	plan.setFromResponseDTO(layoutRes.LayoutResponseDto)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LayoutResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LayoutResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	layoutRes, err := r.client.Layouts.Retrieve(ctx, state.LayoutID.ValueString(), nil)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Could not retrieve layout: %s", err))
		return
	}
	if layoutRes == nil || layoutRes.LayoutResponseDto == nil {
		resp.Diagnostics.AddError("Client Error", "Could not retrieve layout: Response is nil")
		return
	}

	state.setFromResponseDTO(layoutRes.LayoutResponseDto)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LayoutResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LayoutResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	layoutRes, err := r.client.Layouts.Update(ctx, plan.LayoutID.ValueString(), plan.toUpdateDto(), nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update layout: %s", err))
		return
	}
	if layoutRes == nil || layoutRes.LayoutResponseDto == nil {
		resp.Diagnostics.AddError("Client Error", "Unable to confirm update of layout: Response is nil")
		return
	}

	plan.setFromResponseDTO(layoutRes.LayoutResponseDto)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LayoutResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state LayoutResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Layouts.Delete(ctx, state.LayoutID.ValueString(), nil); err != nil {
		if isNotFound(err) {
			resp.Diagnostics.AddWarning("Client warning", fmt.Sprintf("Unable to find layout with id %s, so it will be considered as already deleted", state.LayoutID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete layout: %s", err))
	}
}

func (r *LayoutResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("layout_id"), req, resp)
}

func (data *LayoutResourceModel) toUpdateDto() components.UpdateLayoutDto {
	return components.UpdateLayoutDto{
		Name: data.Name.ValueString(),
		ControlValues: &components.ControlValues{
			Email: &components.EmailControlsDto{
				Body:       data.Email.Body.ValueString(),
				EditorType: components.EditorType(data.Email.EditorType.ValueString()),
			},
		},
	}
}

func (data *LayoutResourceModel) setFromResponseDTO(layout *components.LayoutResponseDto) {
	data.ID = types.StringValue(layout.ID)
	data.LayoutID = types.StringValue(layout.LayoutID)
	data.IsDefault = types.BoolValue(layout.IsDefault)
	data.Name = types.StringValue(layout.Name)

	data.Email = nil
	if email := layout.Controls.Values.Email; email != nil {
		data.Email = &LayoutEmailModel{
			Body:       types.StringValue(email.Body),
			EditorType: types.StringValue(string(email.EditorType)),
		}
	}
}

func isNotFound(err error) bool {
	var e *apierrors.ErrorDto
	return errors.As(err, &e) && e.StatusCode == 404
}
