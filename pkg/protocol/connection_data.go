package protocol

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
)

type ServiceDefinition struct {
	ServiceType       string
	Identifier        string
	DisplayName       string
	RelativeToSetting int
	RelativePath      string
	Description       string
	ServiceOwner      string
	ResourceVersion   int
}

type LocationServiceData struct {
	ServiceDefinitions []ServiceDefinition
}

type ConnectionData struct {
	LocationServiceData LocationServiceData
}

func (vssConnection *VssConnection) GetConnectionData() *ConnectionData {
	url, _ := url.Parse(vssConnection.TenantUrl)
	url.Path = path.Join(url.Path, "_apis/connectionData")
	q := url.Query()
	q.Add("connectOptions", "1")
	q.Add("lastChangeId", "-1")
	q.Add("lastChangeId64", "-1")
	url.RawQuery = q.Encode()
	connectionDataReq, _ := http.NewRequest("GET", url.String(), nil)
	connectionDataResp, err := vssConnection.Client.Do(connectionDataReq)
	connectionData := &ConnectionData{}
	if err != nil {
		fmt.Println("fatal:" + err.Error())
		return nil
	}
	defer connectionDataResp.Body.Close()
	dec2 := json.NewDecoder(connectionDataResp.Body)
	dec2.Decode(connectionData)
	return connectionData
}

func (connectionData *ConnectionData) GetServiceDefinition(id string) *ServiceDefinition {
	for i := 0; i < len(connectionData.LocationServiceData.ServiceDefinitions); i++ {
		if connectionData.LocationServiceData.ServiceDefinitions[i].Identifier == id {
			return &connectionData.LocationServiceData.ServiceDefinitions[i]
		}
	}
	return nil
}
