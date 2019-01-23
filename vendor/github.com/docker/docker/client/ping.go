package client // import "github.com/docker/docker/client"

import (
	"context"
	"path"

	"github.com/docker/docker/api/types"
)

// Ping pings the server and returns the value of the "Docker-Experimental", "Builder-Version", "OS-Type" & "API-Version" headers
func (cli *Client) Ping(ctx context.Context) (types.Ping, error) {
	var ping types.Ping
	req, err := cli.buildRequest("GET", path.Join(cli.basePath, "/_ping"), nil, nil)
	if err != nil {
		return ping, err
	}
	serverResp, err := cli.doRequest(ctx, req)
	if err != nil {
		return ping, err
	}
	defer ensureReaderClosed(serverResp)

	if serverResp.header != nil {
		ping.APIVersion = serverResp.header.Get("API-Version")

		if serverResp.header.Get("Docker-Experimental") == "true" {
			ping.Experimental = true
		}
		ping.OSType = serverResp.header.Get("OSType")
		if bv := serverResp.header.Get("Builder-Version"); bv != "" {
			ping.BuilderVersion = types.BuilderVersion(bv)
		}
	}
	return ping, cli.checkResponseErr(serverResp)
}
