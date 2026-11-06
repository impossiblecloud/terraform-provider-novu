resource "novu_layout" "example" {
  layout_id = "example-layout"
  name      = "Example layout"
  email = {
    body = <<-EOT
      <html>
        <body>
          {{content}}
        </body>
      </html>
    EOT
  }
}

resource "novu_workflow" "example" {
  workflow_id = "example-workflow-id"
  name        = "example workflow"
  steps = [
    {
      email_step = {
        name = "email step"
        control_values = {
          subject   = "example subject"
          body      = "<p>example body</p>"
          layout_id = novu_layout.example.layout_id
        }
      }
    }
  ]
}
