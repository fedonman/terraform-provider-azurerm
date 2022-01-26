package client

import (
	"github.com/Azure/azure-sdk-for-go/services/preview/quantum/mgmt/2019-04-11-preview/quantum"
	"github.com/hashicorp/terraform-provider-azurerm/internal/common"
)

type Client struct {
	WorkspaceClient *quantum.WorkspaceClient
}

func NewClient(o *common.ClientOptions) *Client {
	workspaceClient := quantum.NewWorkspaceClientWithBaseURI(o.ResourceManagerEndpoint, o.SubscriptionId)
	o.ConfigureClient(&workspaceClient.Client, o.ResourceManagerAuthorizer)

	return &Client{
		WorkspaceClient: &workspaceClient,
	}
}
