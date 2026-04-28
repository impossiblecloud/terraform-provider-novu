// Partial implementation of the workflow resource. Only push steps are supported for now.
// TODO : ça vaudrait pas le coup de le split en plusieurs fichiers ? Si oui comment ?
package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	api_client "terraform-provider-novu/internal/api-client"
	"terraform-provider-novu/internal/customplanmodifiers"
	"terraform-provider-novu/internal/helpers"
	customvalidators "terraform-provider-novu/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	novugo "github.com/novuhq/novu-go"
	"github.com/novuhq/novu-go/models/apierrors"
	"github.com/novuhq/novu-go/models/components"
)

var (
	_ resource.Resource                = &WorkflowResource{}
	_ resource.ResourceWithConfigure   = &WorkflowResource{}
	_ resource.ResourceWithImportState = &WorkflowResource{}
)

type WorkflowResource struct {
	apiClient *api_client.ApiClient
	client    *novugo.Novu
}

type WorkflowResourceModel struct {
	Id              types.String                `tfsdk:"id"`
	WorkflowId      types.String                `tfsdk:"workflow_id"`
	Name            types.String                `tfsdk:"name"`
	Description     types.String                `tfsdk:"description"`
	Active          types.Bool                  `tfsdk:"active"`
	Origin          types.String                `tfsdk:"origin"`
	Status          types.String                `tfsdk:"status"`
	ValidatePayload types.Bool                  `tfsdk:"validate_payload"`
	CreatedAt       types.String                `tfsdk:"created_at"`
	UpdatedAt       types.String                `tfsdk:"updated_at"`
	LastTriggeredAt types.String                `tfsdk:"last_triggered_at"`
	Tags            types.List                  `tfsdk:"tags"`
	Slug            types.String                `tfsdk:"slug"`
	Steps           []WorkflowStepResourceModel `tfsdk:"steps"` // Should probably be a list type
	//Steps types.List `tfsdk:"steps"` // list of WorkflowStepResourceModel

	// PayloadSchema   types.Dynamic `tfsdk:"payload_schema"` // Skipped : not sure how to implement and if it's even useful
	// PayloadExample  types.Dynamic `tfsdk:"payload_example"` // Skipped : not sure how to implement and if it's even useful
}

// only push and email steps are supported for now.
type WorkflowStepResourceModel struct {
	//Type     types.String                 `tfsdk:"type"` // -> ignored redundant : to delete ?
	PushStep  *WorkflowPushStepResourceModel  `tfsdk:"push_step"`
	EmailStep *WorkflowEmailStepResourceModel `tfsdk:"email_step"`
	//SmsStep *types.Object 				`tfsdk:"sms_step"`
	//ChatStep *types.Object 				`tfsdk:"chat_step"`
	//DelayStep *types.Object 				`tfsdk:"delay_step"`
	//DigestStep *types.Object 				`tfsdk:"digest_step"`
	//CustomStep *types.Object 				`tfsdk:"custom_step"`
}

type WorkflowPushStepResourceModel struct {
	//Type 			   types.String 						  `tfsdk:"type"` // -> ignored redundant: to delete ?
	Id                 types.String                            `tfsdk:"id"`
	Name               types.String                            `tfsdk:"name"`
	StepId             types.String                            `tfsdk:"step_id"`
	Slug               types.String                            `tfsdk:"slug"`
	Origin             types.String                            `tfsdk:"origin"`
	WorkflowId         types.String                            `tfsdk:"workflow_id"`
	WorkflowDatabaseId types.String                            `tfsdk:"workflow_database_id"`
	Issues             types.Object                            `tfsdk:"issues"`
	ControlValues      *WorkflowStepControlValuesResourceModel `tfsdk:"control_values"`
}

type WorkflowStepIssuesResourceModel struct {
	Controls    []WorkflowStepIssuesControlsResourceModel    `tfsdk:"controls"`
	Integration []WorkflowStepIssuesIntegrationResourceModel `tfsdk:"integration"`
}

type WorkflowStepIssuesControlsResourceModel struct {
	Key          types.String `tfsdk:"key"`
	IssueType    types.String `tfsdk:"issue_type"`
	VariableName types.String `tfsdk:"variable_name"`
	Message      types.String `tfsdk:"message"`
}

type WorkflowStepIssuesIntegrationResourceModel struct {
	Key          types.String `tfsdk:"key"`
	IssueType    types.String `tfsdk:"issue_type"`
	VariableName types.String `tfsdk:"variable_name"`
	Message      types.String `tfsdk:"message"`
}

type WorkflowStepControlValuesResourceModel struct {
	Subject types.String `tfsdk:"subject"`
	Body    types.String `tfsdk:"body"`
}

type WorkflowEmailStepResourceModel struct {
	Id                 types.String                             `tfsdk:"id"`
	Name               types.String                             `tfsdk:"name"`
	StepId             types.String                             `tfsdk:"step_id"`
	Slug               types.String                             `tfsdk:"slug"`
	Origin             types.String                             `tfsdk:"origin"`
	WorkflowId         types.String                             `tfsdk:"workflow_id"`
	WorkflowDatabaseId types.String                             `tfsdk:"workflow_database_id"`
	Issues             types.Object                             `tfsdk:"issues"`
	ControlValues      *WorkflowEmailControlValuesResourceModel `tfsdk:"control_values"`
}

type WorkflowEmailControlValuesResourceModel struct {
	Subject    types.String `tfsdk:"subject"`
	Body       types.String `tfsdk:"body"`
	EditorType types.String `tfsdk:"editor_type"`
}

func NewWorkflowResource() resource.Resource {
	return &WorkflowResource{}
}

func (r *WorkflowResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow"
}

func (r *WorkflowResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	// multiple clients, use the api client
	if clients, ok := req.ProviderData.(*ProviderClients); ok && clients.Api != nil && clients.Novu != nil {
		r.apiClient = clients.Api
		r.client = clients.Novu
	} else {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("*ProviderClients with both a novuClient and apiClient set, got: %T. \nPlease report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
}

func (r *WorkflowResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Workflow resource. Manage any workflow within the organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "database identifier of the workflow",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workflow_id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier of the workflow",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					// Can be removed once the SDK actually returns errors.....
					stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`), "Workflow ID must contain only letters, numbers, underscores and hyphens"),
				},
			},
			"slug": schema.StringAttribute{ // not returned by GET
				MarkdownDescription: "Slug of the workflow",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the workflow",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "description of the workflow",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""), // API default
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the workflow is active",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false), // API default
			},
			"origin": schema.StringAttribute{
				MarkdownDescription: "Origin of the workflow",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Status of the workflow",
				Computed:            true,
			},
			"validate_payload": schema.BoolAttribute{ // do I make it optional ?
				MarkdownDescription: "Whether the payload schema validation is enabled",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true), // API default
			},

			"created_at": schema.StringAttribute{ //TODO : do I add this ?
				MarkdownDescription: "Last updated timestamp",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{ //TODO : do I add this ?
				MarkdownDescription: "Last updated timestamp",
				Computed:            true,
			},
			"last_triggered_at": schema.StringAttribute{ //TODO : do I add this ?
				MarkdownDescription: "Timestamp of the last workflow trigger",
				Computed:            true,
			},

			"tags": schema.ListAttribute{ // TODO : Verify Optional true with empty list
				ElementType:         types.StringType,
				MarkdownDescription: "Tags associated with the workflow",
				Optional:            true,
				Computed:            true, // For the default value to work
				PlanModifiers: []planmodifier.List{
					customplanmodifiers.EmptyListIfNull(),
				},
			},

			// "payload_schema": schema.DynamicAttribute{ // Skipped : not sure how to implement and if it's even useful
			// 	MarkdownDescription: "Payload schema of the workflow.",
			// 	Optional:            true,
			// },

			// "payload_example": schema.DynamicAttribute{ // Skipped : not sure how to implement and if it's even useful
			// 	MarkdownDescription: "Generated payload example of the workflow based on the payload schema",
			// 	Computed:            true,
			// },

			"steps": schema.ListNestedAttribute{
				MarkdownDescription: "Ordered list of steps of the workflow.",
				Optional:            true,
				Computed:            true, // For the default value to work
				PlanModifiers: []planmodifier.List{
					customplanmodifiers.EmptyListIfNull(),
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					// The validator that only one property is set is inside push_step attribute.
					// The validator will run even if push_step is not set, so we don't have to copy it on each attribute.
					Attributes: map[string]schema.Attribute{
						"push_step": schema.SingleNestedAttribute{
							MarkdownDescription: "Push step",
							Optional:            true,
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},

							// Validator : Validate that exactly one type of step is present in each step of the list.
							// NB : Because terraform is very logical, we have to set the object validator on one of its attributes and not the nested object...
							// The validator will run even if push_step is not set, so we don't have to copy it on each attribute.
							Validators: []validator.Object{
								objectvalidator.ExactlyOneOf(
									path.MatchRelative().AtParent().AtName("push_step"),
									path.MatchRelative().AtParent().AtName("email_step"),
									// future proof for future step kinds, uncomment associated path when attribute is implemented
									// path.MatchRelative().AtParent().AtName("sms_step"),
									// path.MatchRelative().AtParent().AtName("chat_step"),
									// path.MatchRelative().AtParent().AtName("delay_step"),
									// path.MatchRelative().AtParent().AtName("digest_step"),
									// path.MatchRelative().AtParent().AtName("custom_step"),
								),
							},
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									// TODO: verify it's this one used both in GET and POST
									MarkdownDescription: "ID of the push step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"name": schema.StringAttribute{
									MarkdownDescription: "Name of the push step",
									Required:            true,
								},
								// "type": schema.StringAttribute{ // superfluous
								// 	MarkdownDescription: "Type of the push step",
								// 	Required:            true,
								// },
								"control_values": schema.SingleNestedAttribute{
									MarkdownDescription: "Control values of the push step",
									Optional:            true,
									Attributes: map[string]schema.Attribute{
										"subject": schema.StringAttribute{
											MarkdownDescription: "Subject/title of the push notification. The push step will have an error if it is not set.",
											Optional:            true,
										},
										"body": schema.StringAttribute{
											MarkdownDescription: "Body content of the push notification. The push step will have an error if it is not set.",
											Optional:            true,
										},
									},
								},
								"step_id": schema.StringAttribute{
									MarkdownDescription: "ID of the push step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"slug": schema.StringAttribute{
									MarkdownDescription: "Slug of the push step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
										customvalidators.StringUnknownIfSiblingChanged("name"),
									},
								},
								"origin": schema.StringAttribute{
									MarkdownDescription: "Origin of the push step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"workflow_id": schema.StringAttribute{
									MarkdownDescription: "Workflow ID of the push steps",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"workflow_database_id": schema.StringAttribute{
									MarkdownDescription: "Workflow database ID of the push step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"issues": schema.SingleNestedAttribute{
									MarkdownDescription: "Issues associated with the push step",
									Computed:            true,
									PlanModifiers: []planmodifier.Object{
										customvalidators.ObjectUnkownOnSiblingNotKnownVal("step_id"),
									},
									Attributes: map[string]schema.Attribute{
										"controls": schema.ListNestedAttribute{
											MarkdownDescription: "Controls-related issues",
											Computed:            true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"key": schema.StringAttribute{
														MarkdownDescription: "Key of the control issue",
														Computed:            true,
													},
													"issue_type": schema.StringAttribute{
														MarkdownDescription: "Type of step content issue",
														Computed:            true,
													},
													"variable_name": schema.StringAttribute{
														MarkdownDescription: "Name of the variable that caused the issue",
														Computed:            true,
													},
													"message": schema.StringAttribute{
														MarkdownDescription: "Detailed message describing the issue",
														Computed:            true,
													},
												},
											},
										},
										"integration": schema.ListNestedAttribute{
											MarkdownDescription: "Integration-related issues",
											Computed:            true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"key": schema.StringAttribute{
														MarkdownDescription: "Key of the integration issue",
														Computed:            true,
													},
													"issue_type": schema.StringAttribute{
														MarkdownDescription: "Type of integration issue",
														Computed:            true,
													},
													"variable_name": schema.StringAttribute{
														MarkdownDescription: "Name of the variable that caused the issue",
														Computed:            true,
													},
													"message": schema.StringAttribute{
														MarkdownDescription: "Detailed message describing the issue",
														Computed:            true,
													},
												},
											},
										},
									},
								},
							},
						},
						"email_step": schema.SingleNestedAttribute{
							MarkdownDescription: "Email step",
							Optional:            true,
							PlanModifiers: []planmodifier.Object{
								objectplanmodifier.UseStateForUnknown(),
							},
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									MarkdownDescription: "ID of the email step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"name": schema.StringAttribute{
									MarkdownDescription: "Name of the email step",
									Required:            true,
								},
								"control_values": schema.SingleNestedAttribute{
									MarkdownDescription: "Control values of the email step",
									Optional:            true,
									Attributes: map[string]schema.Attribute{
										"subject": schema.StringAttribute{
											MarkdownDescription: "Subject of the email.",
											Required:            true,
										},
										"body": schema.StringAttribute{
											MarkdownDescription: "Body content of the email (HTML string or Maily JSON).",
											Optional:            true,
										},
										"editor_type": schema.StringAttribute{
											MarkdownDescription: "Type of editor used for the body. Allowed values: `block`, `html`. Defaults to `block`.",
											Optional:            true,
											Computed:            true,
											Default:             stringdefault.StaticString("block"),
										},
									},
								},
								"step_id": schema.StringAttribute{
									MarkdownDescription: "Step ID of the email step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"slug": schema.StringAttribute{
									MarkdownDescription: "Slug of the email step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
										customvalidators.StringUnknownIfSiblingChanged("name"),
									},
								},
								"origin": schema.StringAttribute{
									MarkdownDescription: "Origin of the email step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"workflow_id": schema.StringAttribute{
									MarkdownDescription: "Workflow ID of the email step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"workflow_database_id": schema.StringAttribute{
									MarkdownDescription: "Workflow database ID of the email step",
									Computed:            true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseStateForUnknown(),
										customvalidators.StringUnkownOnSiblingNotKnownVal("step_id"),
									},
								},
								"issues": schema.SingleNestedAttribute{
									MarkdownDescription: "Issues associated with the email step",
									Computed:            true,
									PlanModifiers: []planmodifier.Object{
										customvalidators.ObjectUnkownOnSiblingNotKnownVal("step_id"),
									},
									Attributes: map[string]schema.Attribute{
										"controls": schema.ListNestedAttribute{
											MarkdownDescription: "Controls-related issues",
											Computed:            true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"key": schema.StringAttribute{
														MarkdownDescription: "Key of the control issue",
														Computed:            true,
													},
													"issue_type": schema.StringAttribute{
														MarkdownDescription: "Type of step content issue",
														Computed:            true,
													},
													"variable_name": schema.StringAttribute{
														MarkdownDescription: "Name of the variable that caused the issue",
														Computed:            true,
													},
													"message": schema.StringAttribute{
														MarkdownDescription: "Detailed message describing the issue",
														Computed:            true,
													},
												},
											},
										},
										"integration": schema.ListNestedAttribute{
											MarkdownDescription: "Integration-related issues",
											Computed:            true,
											NestedObject: schema.NestedAttributeObject{
												Attributes: map[string]schema.Attribute{
													"key": schema.StringAttribute{
														MarkdownDescription: "Key of the integration issue",
														Computed:            true,
													},
													"issue_type": schema.StringAttribute{
														MarkdownDescription: "Type of integration issue",
														Computed:            true,
													},
													"variable_name": schema.StringAttribute{
														MarkdownDescription: "Name of the variable that caused the issue",
														Computed:            true,
													},
													"message": schema.StringAttribute{
														MarkdownDescription: "Detailed message describing the issue",
														Computed:            true,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *WorkflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkflowResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.WorkflowId.IsNull() || data.WorkflowId.IsUnknown() || data.WorkflowId.ValueString() == "" {
		resp.Diagnostics.AddError("Client Error", "workflow_id is required but was not set")
	}
	if data.Name.IsNull() || data.Name.IsUnknown() || data.Name.ValueString() == "" {
		resp.Diagnostics.AddError("Client Error", "name is required but was not set")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// The API will continue the creation even if the workflow ID is already in use, so we need to check if it already exists
	alreadyExists := false
	_, apiResponse, err := r.apiClient.GetWorkflow(ctx, data.WorkflowId.ValueString())

	if apiResponse != nil && apiResponse.StatusCode == 200 {
		alreadyExists = true
	} else if err != nil {
		if apiResponse != nil && apiResponse.StatusCode == 404 {
			alreadyExists = false
		} else {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to check if the workflow already exists: %s", err))
			return
		}
	}

	if alreadyExists {
		resp.Diagnostics.AddError("Client Error", "Workflow already exists : A workflow with this workflow_id already exists")
		return
	}

	createReq, diags := createWorklowRequest(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if createReq == nil {
		resp.Diagnostics.AddError("Client Error", "Unable to create workflow: Request is nil")
		return
	}

	workflowRes, err := r.client.Workflows.Create(ctx, *createReq, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create workflow: %s", err))
		return
	}
	if workflowRes == nil {
		resp.Diagnostics.AddError("Client Error", "Unable to confirm creation of workflow: Response is nil")
		return
	}

	configSteps := data.Steps

	resp.Diagnostics.Append(data.setFromResponseDTO(ctx, workflowRes.WorkflowResponseDto)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Steps = restoreEmailControlValueBodies(configSteps, data.Steps)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkflowResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.WorkflowId.IsNull() || data.WorkflowId.IsUnknown() || data.WorkflowId.ValueString() == "" {
		resp.Diagnostics.AddError("Client Error", "workflow_id is required but was not set")
		return
	}

	workflowRes, err := r.client.Workflows.Get(ctx, data.WorkflowId.ValueString(), nil, nil)
	statusCode := 0

	if workflowRes == nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Could not retrieve workflow: %s", err))
		return
	}

	if err != nil {
		if workflowRes.GetHTTPMeta().Response != nil {
			statusCode = workflowRes.GetHTTPMeta().Response.StatusCode
		}
		if statusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		} else {
			// if it's a 404, we don't throw but consider the resource as deleted (below)
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Could not retrieve workflow: %s", err))
			return
		}
	}
	priorSteps := data.Steps

	resp.Diagnostics.Append(data.setFromResponseDTO(ctx, workflowRes.WorkflowResponseDto)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Steps = restoreEmailControlValueBodies(priorSteps, data.Steps)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WorkflowResourceModel

	var state WorkflowResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if plan.WorkflowId.IsNull() || plan.WorkflowId.IsUnknown() {
		resp.Diagnostics.AddError("Client error", "Cannot update workflow : unable to retrieve workflow_id")
		return
	}

	stepsChanged, updateSteps, err := buildStepsUpdate(plan.Steps, state.Steps)
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to build steps update: %s", err))
		return
	}

	// return if no changes
	if plan.Name.Equal(state.Name) && plan.Origin.Equal(state.Origin) &&
		plan.Tags.Equal(state.Tags) &&
		plan.Description.Equal(state.Description) &&
		plan.Active.Equal(state.Active) &&
		plan.ValidatePayload.Equal(state.ValidatePayload) &&
		!stepsChanged {
		return
	}

	var updateReq components.UpdateWorkflowDto
	updateReq.Name = plan.Name.ValueString()                                    // must always be set
	updateReq.Origin = components.ResourceOriginEnum(plan.Origin.ValueString()) // must always be set
	updateReq.Steps = updateSteps                                               // must always be set
	updateReq.Active = helpers.FromTfBool(plan.Active)                          // will default to false for prod if not set
	tags := make([]string, 0)
	resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
	updateReq.Tags = tags // must always be set

	// helpers.SetStringPtrIfChanged(plan.WorkflowId, state.WorkflowId, &updateReq.WorkflowID) // it's a trap, you cannot update the workflow_id through this endpoint of the API
	helpers.SetStringPtrIfChanged(plan.Description, state.Description, &updateReq.Description)
	helpers.SetBoolPtrIfChanged(plan.ValidatePayload, state.ValidatePayload, &updateReq.ValidatePayload)

	workflowRes, err := r.client.Workflows.Update(ctx, plan.WorkflowId.ValueString(), updateReq, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to update workflow: %s", err))
		return
	}

	if workflowRes == nil {
		resp.Diagnostics.AddError("Client Error", "Unable to confirm creation of workflow: Response is nil")
		return
	}

	// Read the workflow response into the state data model
	resp.Diagnostics.Append(state.setFromResponseDTO(ctx, workflowRes.WorkflowResponseDto)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Steps = restoreEmailControlValueBodies(plan.Steps, state.Steps)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WorkflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkflowResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.WorkflowId.IsNull() || data.WorkflowId.IsUnknown() {
		resp.Diagnostics.AddError("Client error", "Cannot delete workflow : unable to retrieve workflow_id")
	}

	_, err := r.client.Workflows.Delete(ctx, data.WorkflowId.ValueString(), nil)
	if err != nil {
		var e *apierrors.ErrorDto
		if errors.As(err, &e) && e.StatusCode == 404 {
			// workflow not found => we continue with the deletion of the resource
			resp.Diagnostics.AddWarning("Client warning", fmt.Sprintf("Unable to find workflow with id %s, so it will be considered as already deleted", data.WorkflowId.ValueString()))
		} else {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to delete workflow: %s", err))
			return
		}
	}
}

// restoreEmailControlValueBodies preserves email step control_values.body from srcSteps
// into destSteps when the Novu API response omits or clears the body field.
func restoreEmailControlValueBodies(srcSteps, destSteps []WorkflowStepResourceModel) []WorkflowStepResourceModel {
	for i := range destSteps {
		if i >= len(srcSteps) {
			break
		}
		dest := &destSteps[i]
		src := srcSteps[i]
		if dest.EmailStep == nil || dest.EmailStep.ControlValues == nil {
			continue
		}
		if src.EmailStep == nil || src.EmailStep.ControlValues == nil {
			continue
		}
		destBody := dest.EmailStep.ControlValues.Body
		srcBody := src.EmailStep.ControlValues.Body
		if (destBody.IsNull() || destBody.ValueString() == "") &&
			!srcBody.IsNull() && !srcBody.IsUnknown() && srcBody.ValueString() != "" {
			dest.EmailStep.ControlValues.Body = srcBody
		}
	}
	return destSteps
}

func (r *WorkflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("workflow_id"), req, resp)
}

func (data *WorkflowResourceModel) setFromResponseDTO(ctx context.Context, workflow *components.WorkflowResponseDto) diag.Diagnostics {
	var rdiags diag.Diagnostics
	if workflow == nil {
		rdiags.AddError("Client Error", "Workflow is nil")
		return rdiags
	}

	data.Id = types.StringValue(workflow.ID)
	data.WorkflowId = types.StringValue(workflow.WorkflowID)
	data.Name = types.StringValue(workflow.Name)
	data.Origin = types.StringValue(string(workflow.Origin))
	data.Status = types.StringValue(string(workflow.Status))
	data.CreatedAt = types.StringValue(workflow.CreatedAt)
	data.UpdatedAt = types.StringValue(workflow.UpdatedAt)
	data.Slug = types.StringValue(workflow.Slug)

	data.Description = helpers.TfString(workflow.Description)
	data.LastTriggeredAt = helpers.TfString(workflow.LastTriggeredAt)
	data.Active = helpers.TfBool(workflow.Active)
	data.ValidatePayload = helpers.TfBool(workflow.ValidatePayload)

	tags := make([]attr.Value, 0)
	for _, tag := range workflow.Tags {
		tags = append(tags, types.StringValue(tag))
	}
	tagsList, diags := types.ListValue(types.StringType, tags)
	if diags.HasError() {
		rdiags.Append(diags...)
	}
	data.Tags = tagsList

	stepsList, err := buildStepsList(ctx, workflow)
	if err != nil {
		rdiags.AddError("Client Error", fmt.Sprintf("Unable to build steps list: %s", err))
	}
	data.Steps = stepsList

	return rdiags
}

func (w *WorkflowStepResourceModel) convertToNovuStep() (*components.Steps, error) {
	if w == nil {
		return nil, fmt.Errorf("step is nil")
	}
	stepType, err := w.getType()
	if err != nil {
		return nil, err
	}
	switch stepType {
	case components.StepsTypePush:
		if w.PushStep == nil {
			return nil, fmt.Errorf("push step is nil")
		}
		pushStep := w.PushStep

		novuStepPush := components.PushStepUpsertDto{
			ID:   helpers.FromTfString(pushStep.Id),
			Name: pushStep.Name.ValueString(),
			Type: components.StepTypeEnumPush,
		}
		if pushStep.ControlValues != nil {
			controlValues := components.CreatePushStepUpsertDtoControlValuesPushControlDto(
				components.PushControlDto{
					Subject: helpers.FromTfString(pushStep.ControlValues.Subject),
					Body:    helpers.FromTfString(pushStep.ControlValues.Body),
				})
			novuStepPush.ControlValues = &controlValues
		}

		steps := components.CreateStepsPush(novuStepPush)
		return &steps, nil

	case components.StepsTypeEmail:
		if w.EmailStep == nil {
			return nil, fmt.Errorf("email step is nil")
		}
		emailStep := w.EmailStep

		novuStepEmail := components.EmailStepUpsertDto{
			ID:   helpers.FromTfString(emailStep.Id),
			Name: emailStep.Name.ValueString(),
			Type: components.StepTypeEnumEmail,
		}
		if emailStep.ControlValues != nil {
			cv := emailStep.ControlValues
			emailControlDto := components.EmailControlDto{
				Subject: cv.Subject.ValueString(),
				Body:    helpers.FromTfString(cv.Body),
			}
			if !cv.EditorType.IsNull() && !cv.EditorType.IsUnknown() && cv.EditorType.ValueString() != "" {
				editorType := components.EmailControlDtoEditorType(cv.EditorType.ValueString())
				emailControlDto.EditorType = &editorType
			}
			controlValues := components.CreateEmailStepUpsertDtoControlValuesEmailControlDto(emailControlDto)
			novuStepEmail.ControlValues = &controlValues
		}

		steps := components.CreateStepsEmail(novuStepEmail)
		return &steps, nil

	default:
		return nil, fmt.Errorf("unsupported step type: %s", stepType)
	}
}

func (w *WorkflowStepResourceModel) getType() (components.StepsType, error) {
	if w == nil {
		return "", fmt.Errorf("step is nil")
	}
	if w.PushStep != nil {
		return components.StepsTypePush, nil
	}
	if w.EmailStep != nil {
		return components.StepsTypeEmail, nil
	}
	return "", fmt.Errorf("no step type found or unsupported step type")
}

func (w *WorkflowStepResourceModel) sameAs(other *WorkflowStepResourceModel, skipComputed bool) bool {
	if w == nil || other == nil {
		return false
	}
	planStepType, err1 := w.getType()
	otherStepType, err2 := other.getType()
	if err1 != nil || err2 != nil {
		return false
	}
	if planStepType != otherStepType {
		return false
	}
	switch planStepType {
	case components.StepsTypePush:
		return w.PushStep.sameAs(other.PushStep, skipComputed)
	case components.StepsTypeEmail:
		return w.EmailStep.sameAs(other.EmailStep, skipComputed)
	default:
		return false
	}
}

func (w *WorkflowPushStepResourceModel) sameAs(other *WorkflowPushStepResourceModel, skipComputed bool) bool {
	if w == nil || other == nil {
		return false
	}

	same := true
	if !skipComputed {
		same = same &&
			w.Id.Equal(other.Id) &&
			w.StepId.Equal(other.StepId) &&
			w.Slug.Equal(other.Slug) &&
			w.Origin.Equal(other.Origin) &&
			w.WorkflowId.Equal(other.WorkflowId) &&
			w.WorkflowDatabaseId.Equal(other.WorkflowDatabaseId)
	}

	same = same &&
		w.Name.Equal(other.Name) &&
		w.ControlValues.sameAs(other.ControlValues, skipComputed)

	return same
}

func (cv *WorkflowStepControlValuesResourceModel) sameAs(other *WorkflowStepControlValuesResourceModel, _ bool) bool {
	if cv == nil || other == nil {
		return false
	}
	return cv.Subject.Equal(other.Subject) && cv.Body.Equal(other.Body)
}

func (w *WorkflowEmailStepResourceModel) sameAs(other *WorkflowEmailStepResourceModel, skipComputed bool) bool {
	if w == nil || other == nil {
		return false
	}

	same := true
	if !skipComputed {
		same = same &&
			w.Id.Equal(other.Id) &&
			w.StepId.Equal(other.StepId) &&
			w.Slug.Equal(other.Slug) &&
			w.Origin.Equal(other.Origin) &&
			w.WorkflowId.Equal(other.WorkflowId) &&
			w.WorkflowDatabaseId.Equal(other.WorkflowDatabaseId)
	}

	same = same &&
		w.Name.Equal(other.Name) &&
		w.ControlValues.sameAs(other.ControlValues, skipComputed)

	return same
}

func (cv *WorkflowEmailControlValuesResourceModel) sameAs(other *WorkflowEmailControlValuesResourceModel, _ bool) bool {
	if cv == nil || other == nil {
		return false
	}
	return cv.Subject.Equal(other.Subject) && cv.Body.Equal(other.Body) && cv.EditorType.Equal(other.EditorType)
}

func buildStepsList(ctx context.Context, workflow *components.WorkflowResponseDto) ([]WorkflowStepResourceModel, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow is nil")
	}

	stepsSlice, err := readStepsSlice(ctx, workflow)
	if err != nil {
		return nil, err
	}

	return stepsSlice, nil
}

func readStepsSlice(ctx context.Context, workflow *components.WorkflowResponseDto) ([]WorkflowStepResourceModel, error) {
	if workflow == nil {
		return nil, fmt.Errorf("workflow is nil")
	}
	out := make([]WorkflowStepResourceModel, 0)
	// Process each step from the workflow
	for _, step := range workflow.Steps {
		switch step.Type {
		case components.WorkflowResponseDtoStepsTypePush:
			pushStep, err := readPushStepAsModel(ctx, &step)
			if err != nil {
				return nil, err
			}
			out = append(out, WorkflowStepResourceModel{PushStep: pushStep})
		case components.WorkflowResponseDtoStepsTypeEmail:
			emailStep, err := readEmailStepAsModel(ctx, &step)
			if err != nil {
				return nil, err
			}
			out = append(out, WorkflowStepResourceModel{EmailStep: emailStep})
		default:
			return nil, fmt.Errorf("unsupported step type in the workflow resource: %s", step.Type)
		}
	}
	return out, nil
}

func readPushStepAsModel(ctx context.Context, step *components.WorkflowResponseDtoSteps) (*WorkflowPushStepResourceModel, error) {
	if step.PushStepResponseDto == nil {
		return nil, fmt.Errorf("push step is empty")
	}

	// Handle push steps (only supported type for now)
	pushStep := step.PushStepResponseDto
	pushStepModel := WorkflowPushStepResourceModel{
		Id:                 types.StringValue(pushStep.ID),
		Name:               types.StringValue(pushStep.Name),
		StepId:             types.StringValue(pushStep.StepID),
		Slug:               types.StringValue(pushStep.Slug),
		Origin:             types.StringValue(string(pushStep.Origin)),
		WorkflowId:         types.StringValue(pushStep.WorkflowID),
		WorkflowDatabaseId: types.StringValue(pushStep.WorkflowDatabaseID),
	}

	if cv := pushStep.ControlValues; cv != nil && (cv.Subject != nil || cv.Body != nil) {
		pushStepModel.ControlValues = &WorkflowStepControlValuesResourceModel{
			Subject: helpers.TfString(cv.Subject),
			Body:    helpers.TfString(cv.Body),
		}
	}

	issuesObject, err := buildStepIssuesObject(ctx, pushStep.Issues)
	if err != nil {
		return nil, err
	}
	pushStepModel.Issues = issuesObject

	return &pushStepModel, nil
}

func readEmailStepAsModel(ctx context.Context, step *components.WorkflowResponseDtoSteps) (*WorkflowEmailStepResourceModel, error) {
	if step.EmailStepResponseDto == nil {
		return nil, fmt.Errorf("email step is empty")
	}

	emailStep := step.EmailStepResponseDto
	emailStepModel := WorkflowEmailStepResourceModel{
		Id:                 types.StringValue(emailStep.ID),
		Name:               types.StringValue(emailStep.Name),
		StepId:             types.StringValue(emailStep.StepID),
		Slug:               types.StringValue(emailStep.Slug),
		Origin:             types.StringValue(string(emailStep.Origin)),
		WorkflowId:         types.StringValue(emailStep.WorkflowID),
		WorkflowDatabaseId: types.StringValue(emailStep.WorkflowDatabaseID),
	}

	if cv := emailStep.ControlValues; cv != nil {
		emailStepModel.ControlValues = &WorkflowEmailControlValuesResourceModel{
			Subject: types.StringValue(cv.Subject),
			Body:    helpers.TfString(cv.Body),
		}
		if cv.EditorType != nil {
			emailStepModel.ControlValues.EditorType = types.StringValue(string(*cv.EditorType))
		} else {
			emailStepModel.ControlValues.EditorType = types.StringValue("block")
		}
	}

	issuesObject, err := buildStepIssuesObject(ctx, emailStep.Issues)
	if err != nil {
		return nil, err
	}
	emailStepModel.Issues = issuesObject

	return &emailStepModel, nil
}

func buildStepIssuesObject(ctx context.Context, issues *components.StepIssuesDto) (types.Object, error) {
	issuesType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"controls": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
				"key":           types.StringType,
				"issue_type":    types.StringType,
				"variable_name": types.StringType,
				"message":       types.StringType,
			}}},
			"integration": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
				"key":           types.StringType,
				"issue_type":    types.StringType,
				"variable_name": types.StringType,
				"message":       types.StringType,
			}}},
		},
	}

	if issues == nil {
		return types.ObjectNull(issuesType.AttrTypes), nil
	}

	controlsModel := make([]WorkflowStepIssuesControlsResourceModel, 0)
	for key, controls := range issues.Controls {
		for _, control := range controls {
			controlsModel = append(controlsModel, WorkflowStepIssuesControlsResourceModel{
				Key:          types.StringValue(key),
				IssueType:    types.StringValue(string(control.IssueType)),
				Message:      types.StringValue(control.Message),
				VariableName: helpers.TfString(control.VariableName),
			})
		}
	}
	integrationModel := make([]WorkflowStepIssuesIntegrationResourceModel, 0)
	for key, integrations := range issues.Integration {
		for _, integration := range integrations {
			integrationModel = append(integrationModel, WorkflowStepIssuesIntegrationResourceModel{
				Key:          types.StringValue(key),
				IssueType:    types.StringValue(string(integration.IssueType)),
				Message:      types.StringValue(integration.Message),
				VariableName: helpers.TfString(integration.VariableName),
			})
		}
	}

	issuesModel := WorkflowStepIssuesResourceModel{
		Controls:    controlsModel,
		Integration: integrationModel,
	}
	issuesObject, diags := types.ObjectValueFrom(ctx, issuesType.AttrTypes, issuesModel)
	if diags.HasError() {
		return types.ObjectNull(issuesType.AttrTypes), fmt.Errorf("unable to convert issues to object: %s", diags.Errors())
	}
	return issuesObject, nil
}

func createWorklowRequest(ctx context.Context, data *WorkflowResourceModel) (*components.CreateWorkflowDto, diag.Diagnostics) {
	var diags diag.Diagnostics
	createReq := components.CreateWorkflowDto{
		WorkflowID:    data.WorkflowId.ValueString(),
		Name:          data.Name.ValueString(),
		Source:        nil, // unhandled
		Preferences:   nil, // unhandled
		PayloadSchema: nil, // unhandled
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		createReq.Description = novugo.String(data.Description.ValueString())
	}
	if !data.Active.IsNull() && !data.Active.IsUnknown() {
		createReq.Active = helpers.FromTfBool(data.Active)
	}
	if !data.ValidatePayload.IsNull() && !data.ValidatePayload.IsUnknown() {
		createReq.ValidatePayload = helpers.FromTfBool(data.ValidatePayload)
	}
	tags := make([]string, 0)
	if !data.Tags.IsNull() && !data.Tags.IsUnknown() {
		diags.Append(data.Tags.ElementsAs(ctx, &tags, false)...)
	}
	createReq.Tags = tags
	createSteps := make([]components.Steps, 0)
	if len(data.Steps) > 0 {
		steps := data.Steps // let's pray this works. We data.Steps is a list of WorkflowStepResourceModel, not components.Steps
		for _, step := range steps {
			novuStep, err := step.convertToNovuStep()
			if err != nil {
				diags.AddError("Client Error", fmt.Sprintf("Unable to convert step to novu step: %s", err))
				return nil, diags
			}
			createSteps = append(createSteps, *novuStep)
		}
	}
	createReq.Steps = createSteps

	return &createReq, diags
}

func buildStepsUpdate(planSteps []WorkflowStepResourceModel, stateSteps []WorkflowStepResourceModel) (bool, []components.UpdateWorkflowDtoSteps, error) {

	dtoSteps := make([]components.UpdateWorkflowDtoSteps, 0)

	hasChanges := false
	nbPlanSteps := len(planSteps)
	nbStateSteps := len(stateSteps)

	// 1. Detect if changes
	hasChanges = nbPlanSteps != nbStateSteps
	if !hasChanges {
		for i := range nbPlanSteps {
			if !planSteps[i].sameAs(&stateSteps[i], true) {
				hasChanges = true
				break
			}
		}
	}

	//2. If changes, build the update steps
	for _, planStep := range planSteps {
		novuStep, err := planStep.convertToNovuStep()
		if err != nil {
			return hasChanges, dtoSteps, fmt.Errorf("unable to convert step to novu step: %s", err)
		}
		updateStep := toUpdateSteps(novuStep)
		dtoSteps = append(dtoSteps, updateStep)
	}

	return hasChanges, dtoSteps, nil

}

func toUpdateSteps(step *components.Steps) components.UpdateWorkflowDtoSteps {
	return components.UpdateWorkflowDtoSteps{
		InAppStepUpsertDto:  step.InAppStepUpsertDto,
		EmailStepUpsertDto:  step.EmailStepUpsertDto,
		SmsStepUpsertDto:    step.SmsStepUpsertDto,
		ChatStepUpsertDto:   step.ChatStepUpsertDto,
		DelayStepUpsertDto:  step.DelayStepUpsertDto,
		DigestStepUpsertDto: step.DigestStepUpsertDto,
		CustomStepUpsertDto: step.CustomStepUpsertDto,
		PushStepUpsertDto:   step.PushStepUpsertDto,

		Type: components.UpdateWorkflowDtoStepsType(step.Type),
	}
}
