package provision

import (
	"errors"
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/pagination"
)

func awsGetLocalHostname() (string, error) {
	// curl http://169.254.169.254/latest/meta-data/local-hostname
	return "", nil
}

func openstackGetServerName(nodeName string) (string, error) {
	/*
		authOpts := gophercloud.AuthOptions{
			IdentityEndpoint: "http://129.254.188.235/identity",
			Username:         "admin",
			Password:         "secret00secret",
			DomainName:       "Default",
			TenantName:       "demo",
		}
	*/
	ao, err := openstack.AuthOptionsFromEnv()
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to find auth options for openstack (node=%s, err=%v)", nodeName, err))
	}

	provider, err := openstack.AuthenticatedClient(ao)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to authenticate openstack (node=%s, err=%v)", nodeName, err))
	}

	epOpts := gophercloud.EndpointOpts{Region: "RegionOne"}

	client, err := openstack.NewComputeV2(provider, epOpts)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to get the openstack client (node=%s, err=%v)", nodeName, err))
	}
	server, err := getServerByName(client, nodeName)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Failed to get the server by node's name (node=%s, err=%v)", nodeName, err))
	}

	return server.Name, nil
}

// from cloud-provider-openstack
func getServerByName(client *gophercloud.ServiceClient, name string) (*servers.Server, error) {
	opts := servers.ListOpts{
		Name: fmt.Sprintf("%s", name),
		//		Marker: fmt.Sprintf("%s", regexp.QuoteMeta(systemId)),
	}

	var s []servers.Server
	serverList := make([]servers.Server, 0, 1)

	pager := servers.List(client, opts)

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		if err := servers.ExtractServersInto(page, &s); err != nil {
			return false, err
		} else {
			serverList = append(serverList, s...)
			return true, nil
		}
	})

	if err != nil {
		return nil, err
	}

	if len(serverList) == 0 {
		return nil, errors.New(fmt.Sprintf("No server in the cloud"))
	}

	return &serverList[0], nil
}
