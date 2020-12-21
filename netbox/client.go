package netbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.avito.ru/DO/moira"
)

const (
	defaultTimeout = 10 * time.Second
	idsPerQuery    = 40

	apiMethodContainers      = "/api/virtualization/containers/"
	apiMethodDevices         = "/api/dcim/devices/"
	apiMethodDevicesInactive = "/api/dcim/devices-inactive/"
	apiMethodRackGroups      = "/api/dcim/rack-groups/"
	apiMethodRacks           = "/api/dcim/racks/"
)

type Client struct {
	config     Config
	httpClient *http.Client
}

type QueryParams map[string]string

func CreateClient(config *Config) *Client {
	if config != nil {
		return &Client{
			config:     *config,
			httpClient: &http.Client{Timeout: defaultTimeout},
		}
	} else {
		return nil
	}
}

func (client *Client) ContainerList(deviceIds []int) ContainerBriefList {
	devicesQty := len(deviceIds)
	pageLimit := 50

	result := ContainerBriefList{}
	result.List = make([]ContainerBrief, 0, 5*devicesQty)

	// send only `idsPerQuery` device ids per chunk
	// so that query string isn't too large (it can be truncated if it is)
	for i := 0; i < devicesQty; i += idsPerQuery {
		deviceIdsChunk := deviceIds[i:moira.MinI(i+idsPerQuery, devicesQty)]
		deviceIdsParamValue := client.makeIdsParamValue(deviceIdsChunk)
		pageOffset := 0

		// fetch pages one by one until there is no next page
		for {
			resultPage := ContainerBriefList{}
			resultPage.List = make([]ContainerBrief, 0, pageLimit+1)
			queryParams := QueryParams{
				"device_id__in": deviceIdsParamValue,
				"limit":         strconv.Itoa(pageLimit),
				"offset":        strconv.Itoa(pageOffset * pageLimit),
			}

			client.decodeJson(client.doApiQuery(apiMethodContainers, queryParams), &resultPage)
			result.Count += len(resultPage.List)
			result.List = append(result.List, resultPage.List...)

			if resultPage.Next == nil {
				break
			} else {
				pageOffset++
			}
		}
	}

	return result
}

func (client *Client) DeviceList(rackGroupId *int, rackId *int) DeviceList {
	queryParams := QueryParams{"limit": "0"} // limit = 0 means that maximum page size will be specified
	if rackGroupId != nil {
		queryParams["rack_group_id"] = strconv.Itoa(*rackGroupId)
	}
	if rackId != nil {
		queryParams["rack_id"] = strconv.Itoa(*rackId)
	}

	result := DeviceList{}
	result.List = make([]Device, 0, 1024)

	client.decodeJson(client.doApiQuery(apiMethodDevices, queryParams), &result)
	return result
}

func (client *Client) InactiveDeviceList() DeviceBriefList {
	queryParams := QueryParams{"limit": "0"} // limit = 0 means that maximum page size will be specified
	singleAllocateQty := 1024

	result := DeviceBriefList{}
	result.List = make([]DeviceBrief, 0, singleAllocateQty)

	// fetch pages one by one until there is no next page
	for {
		resultPage := DeviceBriefList{}
		resultPage.List = make([]DeviceBrief, 0, singleAllocateQty)

		queryParams["offset"] = strconv.Itoa(len(result.List))
		client.decodeJson(client.doApiQuery(apiMethodDevicesInactive, queryParams), &resultPage)

		result.Count += len(resultPage.List)
		result.List = append(result.List, resultPage.List...)

		if resultPage.Next == nil {
			break
		}
	}

	return result
}

func (client *Client) RackGroupList(rackGroupName *string) RackGroupList {
	params := QueryParams{}
	if rackGroupName != nil {
		params["name"] = *rackGroupName
	}

	result := RackGroupList{}
	client.decodeJson(client.doApiQuery(apiMethodRackGroups, params), &result)
	return result
}

func (client *Client) RackList(rackName *string) RackList {
	params := QueryParams{}
	if rackName != nil {
		params["name"] = *rackName
	}

	result := RackList{}
	client.decodeJson(client.doApiQuery(apiMethodRacks, params), &result)
	return result
}

func (client *Client) SetTimeout(timeout time.Duration) {
	client.httpClient.Timeout = timeout
}

func (client *Client) createRequest(apiMethod string) (*http.Request, error) {
	request, err := http.NewRequest("GET", client.config.URL+apiMethod, nil)
	if request != nil && client.config.Token != "" {
		request.Header.Set("Authorization", fmt.Sprintf("Token %s", client.config.Token))
	}

	return request, err
}

func (client *Client) doApiQuery(apiMethod string, params QueryParams) string {
	request, err := client.createRequest(apiMethod)
	if err != nil {
		panic(errors.New(`Failed to create request: ` + err.Error()))
	}

	query := url.Values{}
	for key, value := range params {
		query.Add(key, value)
	}
	request.URL.RawQuery = query.Encode()

	response, err := client.httpClient.Do(request)
	if err != nil {
		panic(errors.New(`Failed to do request: ` + err.Error()))
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	result := string(responseBody)
	if err != nil {
		panic(errors.New(`Failed to obtain response body: ` + err.Error()))
	}

	return result
}

func (client *Client) decodeJson(jsonString string, target interface{}) {
	if err := json.NewDecoder(strings.NewReader(jsonString)).Decode(target); err != nil {
		panic(errors.New(`Could not decode api response as JSON: ` + err.Error()))
	}
}

func (client *Client) makeIdsParamValue(ids []int) string {
	builder := strings.Builder{}
	qty := len(ids)

	builder.Grow(qty * 5)
	for i, id := range ids {
		builder.WriteString(strconv.Itoa(id))
		if i != qty-1 {
			builder.WriteString(",")
		}
	}

	return builder.String()
}
