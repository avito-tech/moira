package netbox

// list is plain JSON list having structure nobody cares about
type list []interface{}

// Object is plain JSON object having structure nobody cares about
type object map[string]interface{}

// ContainerBrief is DTO definition of brief container's data, part of DeviceBrief
type ContainerBrief struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

// ContainerBriefList is DTO definition of result structure returned by method /api/virtualization/containers/
type ContainerBriefList struct {
	Count    int              `json:"count"`
	List     []ContainerBrief `json:"results,omitempty"`
	Next     *string          `json:"next,omitempty"`
	Previous *string          `json:"previous,omitempty"`
}

// Device is DTO definition of single device, part of DeviceList
type Device struct {
	Id   int         `json:"id"`
	Name string      `json:"name"`
	Role *DeviceRole `json:"device_role,omitempty"`

	// some simple data we are not (yet) interested in
	AssetTag     *string `json:"asset_tag,omitempty"`
	Comments     *string `json:"comments,omitempty"`
	Created      *string `json:"created,omitempty"`
	DisplayName  *string `json:"display_name,omitempty"`
	LastUpdated  *string `json:"last_updated,omitempty"`
	Position     *int    `json:"position,omitempty"`
	Serial       *string `json:"serial,omitempty"`
	VcPosition   *int    `json:"vc_position,omitempty"`
	VcPriority   *int    `json:"vc_priority,omitempty"`
	WarrantyDesc *string `json:"warranty_description,omitempty"`
	WarrantyEnd  *string `json:"warranty_end,omitempty"`

	// some complex data we are not (yet) interested in
	Cluster        *object `json:"cluster,omitempty"`
	CustomFields   *object `json:"custom_fields,omitempty"`
	DeviceType     *object `json:"device_type,omitempty"`
	Face           *object `json:"face,omitempty"`
	ParentDevice   *object `json:"parent_device,omitempty"`
	Platform       *object `json:"platform,omitempty"`
	PrimaryIp      *object `json:"primary_ip,omitempty"`
	PrimaryIp4     *object `json:"primary_ip4,omitempty"`
	PrimaryIp6     *object `json:"primary_ip6,omitempty"`
	Rack           *object `json:"rack,omitempty"`
	Site           *object `json:"site,omitempty"`
	Status         *object `json:"status,omitempty"`
	Tags           *list   `json:"tags,omitempty"`
	Tenant         *object `json:"tenant,omitempty"`
	VirtualChassis *object `json:"virtual_chassis,omitempty"`
}

// DeviceBrief is DTO definition of brief device's data
type DeviceBrief struct {
	Id           int               `json:"id"`
	Name         string            `json:"name"`
	NamePrevious string            `json:"name_previous"`
	Containers   []ContainerBrief  `json:"containers"`
	Status       DeviceStatusBrief `json:"status"`
}

// DeviceBriefList is DTO definition of result structure returned by method /api/dcim/devices-inactive/
type DeviceBriefList struct {
	Count    int           `json:"count"`
	List     []DeviceBrief `json:"results,omitempty"`
	Next     *string       `json:"next,omitempty"`
	Previous *string       `json:"previous,omitempty"`
}

// DeviceList is DTO definition of result structure returned by method /api/dcim/devices/
type DeviceList struct {
	Count    int      `json:"count"`
	List     []Device `json:"results,omitempty"`
	Next     *string  `json:"next,omitempty"`
	Previous *string  `json:"previous,omitempty"`
}

// DeviceRole is DTO definition of the role of single device, part of Device
type DeviceRole struct {
	Id   int     `json:"id"`
	Name string  `json:"name"`
	Slug string  `json:"slug"`
	Url  *string `json:"url,omitempty"`
}

// DeviceStatusBrief is DTO definition of brief status of the device
type DeviceStatusBrief struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// Rack is DTO definition of single rack, part of RackList
type Rack struct {
	Id   int    `json:"id"`
	Name string `json:"name"`

	// some simple data we are not (yet) interested in
	Comments    *string `json:"comments,omitempty"`
	Created     *string `json:"created,omitempty"`
	DescUnits   *bool   `json:"desc_units,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	FacilityId  *string `json:"facility_id,omitempty"`
	LastUpdated *string `json:"last_updated,omitempty"`
	Serial      *string `json:"serial,omitempty"`
	UHeight     *int    `json:"u_height,omitempty"`

	// some complex data we are not (yet) interested in
	CustomFields *object `json:"custom_fields,omitempty"`
	Group        *object `json:"group,omitempty"`
	Role         *object `json:"role,omitempty"`
	Site         *object `json:"site,omitempty"`
	Tags         *list   `json:"tags,omitempty"`
	Tenant       *object `json:"tenant,omitempty"`
	Type         *object `json:"type,omitempty"`
	Width        *object `json:"width,omitempty"`
}

// RackList is DTO definition of result structure returned by method /api/dcim/racks/
type RackList struct {
	Count    int     `json:"count"`
	Next     *string `json:"next,omitempty"`
	Previous *string `json:"previous,omitempty"`
	List     []*Rack `json:"results,omitempty"`
}

// RackGroup is DTO definition of single rack group, part of RackGroupList
type RackGroup struct {
	Id   int     `json:"id"`
	Name string  `json:"name"`
	Slug string  `json:"slug"`
	Site *object `json:"site,omitempty"`
}

// RackGroupList is DTO definition of result structure returned by method /api/dcim/rack-groups/
type RackGroupList struct {
	Count    int          `json:"count"`
	Next     *string      `json:"next,omitempty"`
	Previous *string      `json:"previous,omitempty"`
	List     []*RackGroup `json:"results,omitempty"`
}
