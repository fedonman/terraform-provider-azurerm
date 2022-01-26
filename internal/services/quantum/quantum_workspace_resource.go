package quantum

import (
	"fmt"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/preview/quantum/mgmt/2019-04-11-preview/quantum"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/azure"
	"github.com/hashicorp/terraform-provider-azurerm/helpers/tf"
	"github.com/hashicorp/terraform-provider-azurerm/internal/clients"
	"github.com/hashicorp/terraform-provider-azurerm/internal/location"
	"github.com/hashicorp/terraform-provider-azurerm/internal/services/quantum/parse"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tags"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/pluginsdk"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tf/validation"
	"github.com/hashicorp/terraform-provider-azurerm/internal/timeouts"
	"github.com/hashicorp/terraform-provider-azurerm/utils"
)

func resourceQuantumWorkspace() *pluginsdk.Resource {
	return &pluginsdk.Resource{
		Create: resourceQuantumWorkspaceCreateUpdate,
		Read:   resourceQuantumWorkspaceRead,
		Update: resourceQuantumWorkspaceCreateUpdate,
		Delete: resourceQuantumWorkspaceDelete,

		Importer: pluginsdk.ImporterValidatingResourceId(func(id string) error {
			_, err := parse.WorkspaceID(id)
			return err
		}),

		Timeouts: &pluginsdk.ResourceTimeout{
			Create: pluginsdk.DefaultTimeout(30 * time.Minute),
			Read:   pluginsdk.DefaultTimeout(5 * time.Minute),
			Update: pluginsdk.DefaultTimeout(30 * time.Minute),
			Delete: pluginsdk.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*pluginsdk.Schema{
			"name": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringMatch(
					regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]{0,52}[a-zA-Z0-9]$`),
					"The Quantum workspace name must be between 2 and 54 characters long, it can contain only letters, numbers and hyphens, and the first and last characters must be a letter or number."),
			},

			"location": azure.SchemaLocation(),

			"resource_group_name": azure.SchemaResourceGroupName(),

			"storage_account_id": {
				Type:     pluginsdk.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: azure.ValidateResourceID			
			},

			"tags": tags.Schema(),
		},
	}
}

func resourceQuantumWorkspaceCreateUpdate(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Quantum.WorkspaceClient
	ctx, cancel := timeouts.ForCreateUpdate(meta.(*clients.Client).StopContext, d)
	defer cancel()

	name := d.Get("name").(string)
	resGroup := d.Get("resource_group_name").(string)

	existing, err := client.Get(ctx, resGroup, name)
	if err != nil {
		if !utils.ResponseWasNotFound(existing.Response) {
			return fmt.Errorf("checking for existing Quantum Workspace %q (Resource Group %q): %s", name, resGroup, err)
		}
	}
	if existing.ID != nil && *existing.ID != "" {
		return tf.ImportAsExistsError("azurerm_quantum_workspace", *existing.ID)
	}

	workspace := quantum.Workspace{
		Name:     utils.String(name),
		Location: utils.String(azure.NormalizeLocation(d.Get("location").(string))),
		Tags:     tags.Expand(d.Get("tags").(map[string]interface{})),
		WorkspaceResourceProperties: &quantum.WorkspaceResourceProperties{
			StorageAccount:                  utils.String(d.Get("storage_account_id").(string)),
		},
	}

	future, err := client.CreateOrUpdate(ctx, resGroup, name, workspace)
	if err != nil {
		return fmt.Errorf("creating Quantum Workspace %q (Resource Group %q): %+v", name, resGroup, err)
	}

	if err = future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for creation of Quantum Workspace %q (Resource Group %q): %+v", name, resGroup, err)
	}

	subscriptionId := meta.(*clients.Client).Account.SubscriptionId
	id := parse.NewWorkspaceID(subscriptionId, resGroup, name)
	d.SetId(id.ID())

	return resourceQuantumWorkspaceRead(d, meta)
}

func resourceQuantumWorkspaceRead(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Quantum.WorkspacesClient
	ctx, cancel := timeouts.ForRead(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.WorkspaceID(d.Id())
	if err != nil {
		return fmt.Errorf("parsing Quantum Workspace ID `%q`: %+v", d.Id(), err)
	}

	resp, err := client.Get(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("making Read request on Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	d.Set("name", id.Name)
	d.Set("resource_group_name", id.ResourceGroup)

	if location := resp.Location; location != nil {
		d.Set("location", azure.NormalizeLocation(*location))
	}

	if props := resp.WorkspaceResourceProperties; props != nil {
		d.Set("storage_account_id", props.StorageAccount)
	}

	if err := d.Set("identity", flattenQuantumWorkspaceIdentity(resp.Identity)); err != nil {
		return fmt.Errorf("flattening identity on Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	return tags.FlattenAndSet(d, resp.Tags)
}

func resourceQuantumWorkspaceDelete(d *pluginsdk.ResourceData, meta interface{}) error {
	client := meta.(*clients.Client).Quantum.WorkspacesClient
	ctx, cancel := timeouts.ForDelete(meta.(*clients.Client).StopContext, d)
	defer cancel()

	id, err := parse.WorkspaceID(d.Id())
	if err != nil {
		return fmt.Errorf("parsing Quantum Workspace ID `%q`: %+v", d.Id(), err)
	}

	future, err := client.Delete(ctx, id.ResourceGroup, id.Name)
	if err != nil {
		return fmt.Errorf("deleting Quantum Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	if err := future.WaitForCompletionRef(ctx, client.Client); err != nil {
		return fmt.Errorf("waiting for deletion of Quantum Workspace %q (Resource Group %q): %+v", id.Name, id.ResourceGroup, err)
	}

	return nil
}

func flattenQuantumWorkspaceIdentity(identity *quantum.Identity) []interface{} {
	if identity == nil {
		return []interface{}{}
	}

	principalID := ""
	if identity.PrincipalID != nil {
		principalID = *identity.PrincipalID
	}

	tenantID := ""
	if identity.TenantID != nil {
		tenantID = *identity.TenantID
	}

	return []interface{}{
		map[string]interface{}{
			"type":         string(identity.Type),
			"principal_id": principalID,
			"tenant_id":    tenantID,
		},
	}
}
