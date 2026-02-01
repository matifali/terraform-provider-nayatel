package client

import "time"

// InstanceStatus represents the status of an instance.
type InstanceStatus string

const (
	InstanceStatusBuild   InstanceStatus = "BUILD"
	InstanceStatusActive  InstanceStatus = "ACTIVE"
	InstanceStatusStopped InstanceStatus = "SHUTOFF"
	InstanceStatusError   InstanceStatus = "ERROR"
	InstanceStatusDeleted InstanceStatus = "DELETED"
	InstanceStatusReboot  InstanceStatus = "REBOOT"
)

// InstanceAction represents an action that can be performed on an instance.
type InstanceAction string

const (
	InstanceActionStart  InstanceAction = "START"
	InstanceActionStop   InstanceAction = "STOP"
	InstanceActionReboot InstanceAction = "REBOOT"
)

// Project represents a Nayatel Cloud project.
type Project struct {
	ID          string     `json:"id,omitempty"`
	ProjectID   string     `json:"project_id,omitempty"`
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	Instances   []Instance `json:"instances,omitempty"`
}

// GetID returns the project ID (handles both id and project_id fields).
func (p *Project) GetID() string {
	if p.ID != "" {
		return p.ID
	}
	return p.ProjectID
}

// Instance represents a Nayatel Cloud instance (VM).
type Instance struct {
	ID             string                 `json:"id,omitempty"`
	InstanceID     string                 `json:"Instance ID,omitempty"`
	Name           string                 `json:"Name,omitempty"`
	NameLower      string                 `json:"name,omitempty"`
	Status         InstanceStatus         `json:"Status,omitempty"`
	StatusLower    InstanceStatus         `json:"status,omitempty"`
	PowerState     string                 `json:"Power State,omitempty"`
	Flavor         *Flavor                `json:"flavor,omitempty"`
	Image          *Image                 `json:"image,omitempty"`
	KeyName        string                 `json:"key_name,omitempty"`
	Addresses      map[string][]Address   `json:"addresses,omitempty"`
	IPAddresses    map[string][]string    `json:"IP Addresses,omitempty"`
	FloatingIPs    []string               `json:"Floating IPs,omitempty"`
	SecurityGroups []SecurityGroupRef     `json:"security_groups,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Created        time.Time              `json:"created,omitempty"`
	CreatedAt      string                 `json:"Created At,omitempty"`
	Updated        time.Time              `json:"updated,omitempty"`
	TenantID       string                 `json:"tenant_id,omitempty"`
	HostID         string                 `json:"host_id,omitempty"`
	CPU            int                    `json:"CPU,omitempty"`
	RAM            int                    `json:"RAM,omitempty"`
	Zone           string                 `json:"Availability Zone,omitempty"`
}

// GetID returns the instance ID (handles both field names).
func (i *Instance) GetID() string {
	if i.ID != "" {
		return i.ID
	}
	return i.InstanceID
}

// GetName returns the instance name (handles both field names).
func (i *Instance) GetName() string {
	if i.Name != "" {
		return i.Name
	}
	return i.NameLower
}

// GetStatus returns the instance status (handles both field names).
func (i *Instance) GetStatus() InstanceStatus {
	if i.Status != "" {
		return i.Status
	}
	return i.StatusLower
}

// GetPublicIP returns the public/floating IP if attached.
func (i *Instance) GetPublicIP() string {
	// Check Floating IPs array first (new API format)
	if len(i.FloatingIPs) > 0 {
		return i.FloatingIPs[0]
	}
	// Fallback to old addresses format
	for _, addrs := range i.Addresses {
		for _, addr := range addrs {
			if addr.Type == "floating" {
				return addr.Addr
			}
		}
	}
	return ""
}

// GetPrivateIP returns the private/fixed IP.
func (i *Instance) GetPrivateIP() string {
	// Check IP Addresses map first (new API format)
	for _, ips := range i.IPAddresses {
		if len(ips) > 0 {
			return ips[0]
		}
	}
	// Fallback to old addresses format
	for _, addrs := range i.Addresses {
		for _, addr := range addrs {
			if addr.Type == "fixed" {
				return addr.Addr
			}
		}
	}
	return ""
}

// Address represents an IP address.
type Address struct {
	Addr    string `json:"addr"`
	Version int    `json:"version"`
	Type    string `json:"OS-EXT-IPS:type,omitempty"`
	MacAddr string `json:"OS-EXT-IPS-MAC:mac_addr,omitempty"`
}

// SecurityGroupRef is a reference to a security group.
type SecurityGroupRef struct {
	Name string `json:"name"`
}

// InstanceCreateRequest represents a request to create an instance.
type InstanceCreateRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	ImageID        string `json:"image_id"`
	CPU            int    `json:"cpu"`
	RAM            int    `json:"ram"`  // in GB
	Disk           int    `json:"disk"` // in GB
	NetworkID      string `json:"network_id"`
	SSHFingerprint string `json:"ssh_fingerprint"`
	InstanceCount  int    `json:"instance_count,omitempty"`
}

// ToAPIPayload converts the request to the API payload format.
func (r *InstanceCreateRequest) ToAPIPayload() map[string]interface{} {
	initialization := map[string]interface{}{
		"name":        r.Name,
		"description": r.Description,
		"image":       map[string]string{"id": r.ImageID},
		"auth": map[string]string{
			"method":      "ssh",
			"fingerprint": r.SSHFingerprint,
		},
		"network_ids": r.NetworkID,
	}

	if r.Description == "" {
		initialization["description"] = "Nayatel Cloud VPS"
	}

	instanceCount := r.InstanceCount
	if instanceCount == 0 {
		instanceCount = 1
	}

	return map[string]interface{}{
		"conf": map[string]interface{}{
			"STORAGE":        r.Disk,
			"INSTANCE_COUNT": instanceCount,
			"CPU":            r.CPU,
			"RAM":            r.RAM,
			"INITIALIZATION": initialization,
		},
	}
}

// Network represents a Nayatel Cloud network.
type Network struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status,omitempty"`
	Subnets    string `json:"subnets,omitempty"`
	SubnetID   string `json:"last_subnet_id,omitempty"`
	SubnetCIDR string `json:"last_subnet_cidr,omitempty"`
	Bandwidth  string `json:"bandwidth,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Usage      int    `json:"usage,omitempty"`
	TenantID   string `json:"tenant_id,omitempty"`
	Shared     bool   `json:"shared,omitempty"`
	External   bool   `json:"router:external,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
}

// NetworkCreateRequest represents a request to create a network.
type NetworkCreateRequest struct {
	BandwidthLimit int `json:"bandwidth_limit,omitempty"` // 25-250 Mbps increments
}

// ToAPIPayload converts the request to the API payload format.
func (r *NetworkCreateRequest) ToAPIPayload() map[string]interface{} {
	limit := r.BandwidthLimit
	if limit == 0 {
		limit = 1
	}
	return map[string]interface{}{
		"25-250_LIMIT": limit,
	}
}

// NetworkCreateResponse represents the response from creating a network.
type NetworkCreateResponse struct {
	Network *Network  `json:"network,omitempty"`
	Subnets []*Subnet `json:"subnets,omitempty"`
}

// Subnet represents a Nayatel Cloud subnet.
type Subnet struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	NetworkID      string   `json:"network_id"`
	CIDR           string   `json:"cidr"`
	GatewayIP      string   `json:"gateway_ip,omitempty"`
	DNSNameservers []string `json:"dns_nameservers,omitempty"`
	EnableDHCP     bool     `json:"enable_dhcp,omitempty"`
}

// Router represents a Nayatel Cloud router.
type Router struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Status              string                 `json:"status,omitempty"`
	ExternalGatewayInfo map[string]interface{} `json:"external_gateway_info,omitempty"`
	TenantID            string                 `json:"tenant_id,omitempty"`
}

// RouterCreateRequest represents a request to create a router.
type RouterCreateRequest struct {
	NetworkID  string `json:"network_id"`
	RouterName string `json:"router_name,omitempty"`
}

// ToAPIPayload converts the request to the API payload format.
func (r *RouterCreateRequest) ToAPIPayload() map[string]interface{} {
	name := r.RouterName
	if name == "" {
		name = "default"
	}
	return map[string]interface{}{
		"network_id":  r.NetworkID,
		"router_name": name,
	}
}

// RouterCreateResponse represents the response from creating a router.
type RouterCreateResponse struct {
	Router *Router `json:"router,omitempty"`
}

// FloatingIPPortDetails contains port details for a floating IP.
type FloatingIPPortDetails struct {
	DeviceID    string `json:"device_id,omitempty"`
	DeviceOwner string `json:"device_owner,omitempty"`
	NetworkID   string `json:"network_id,omitempty"`
	MACAddress  string `json:"mac_address,omitempty"`
	Status      string `json:"status,omitempty"`
}

// FloatingIP represents a Nayatel Cloud floating IP.
type FloatingIP struct {
	ID             string                `json:"id"`
	IPAddress      string                `json:"floating_ip_address,omitempty"`
	IP             string                `json:"ip,omitempty"` // Alternative field name
	Status         string                `json:"status,omitempty"`
	PortID         string                `json:"port_id,omitempty"`
	InstanceID     string                `json:"instance_id,omitempty"`
	FixedIPAddress string                `json:"fixed_ip_address,omitempty"`
	PortDetails    FloatingIPPortDetails `json:"port_details,omitempty"`
}

// GetIPAddress returns the IP address (handles both field names).
func (f *FloatingIP) GetIPAddress() string {
	if f.IPAddress != "" {
		return f.IPAddress
	}
	return f.IP
}

// SecurityGroup represents a Nayatel Cloud security group.
type SecurityGroup struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Rules       []SecurityGroupRule `json:"rules,omitempty"`
	TenantID    string              `json:"tenant_id,omitempty"`
}

// SecurityGroupRule represents a rule in a security group.
type SecurityGroupRule struct {
	ID             string `json:"id"`
	Index          int    `json:"index,omitempty"`
	Direction      string `json:"direction"`           // ingress or egress
	Ethertype      string `json:"ethertype,omitempty"` // IPv4 or IPv6
	Protocol       string `json:"protocol,omitempty"`  // tcp, udp, icmp, or Any
	PortRange      string `json:"port_range,omitempty"`
	PortRangeMin   int    `json:"port_range_min,omitempty"`
	PortRangeMax   int    `json:"port_range_max,omitempty"`
	RemoteIPPrefix string `json:"remote_ip_prefix,omitempty"`
	RemoteGroupID  string `json:"remote_group_id,omitempty"`
	Description    string `json:"description,omitempty"`
}

// SecurityGroupCreateRequest represents a request to create a security group.
type SecurityGroupCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ToAPIPayload converts the request to the API payload format.
func (r *SecurityGroupCreateRequest) ToAPIPayload() map[string]interface{} {
	payload := map[string]interface{}{
		"name": r.Name,
	}
	if r.Description != "" {
		payload["description"] = r.Description
	}
	return payload
}

// SecurityGroupRuleCreateRequest represents a request to create a security group rule.
type SecurityGroupRuleCreateRequest struct {
	RuleName   string `json:"rule_name,omitempty"` // e.g., "HTTP", "SSH"
	Direction  string `json:"direction"`           // Ingress or Egress
	Ethertype  string `json:"ether_type"`          // IPv4 or IPv6
	Protocol   string `json:"protocol,omitempty"`  // tcp, udp, icmp
	PortNumber string `json:"port_number,omitempty"`
	CIDR       string `json:"cidr,omitempty"` // e.g., "0.0.0.0/0"
}

// ToAPIPayload converts the request to the API payload format.
func (r *SecurityGroupRuleCreateRequest) ToAPIPayload() map[string]interface{} {
	payload := map[string]interface{}{
		"direction": r.Direction,
		"etherType": r.Ethertype,
		"cidrSg":    "CIDR",
	}

	// Determine ruleName based on port/protocol or use provided name
	ruleName := r.RuleName
	if ruleName == "" && r.PortNumber != "" {
		// Map common ports to preset rule names
		switch r.PortNumber {
		case "22":
			ruleName = "SSH"
		case "80":
			ruleName = "HTTP"
		case "443":
			ruleName = "HTTPS"
		case "3389":
			ruleName = "RDP"
		default:
			// Use "Custom TCP Rule" or "Custom UDP Rule" for other ports
			switch r.Protocol {
			case "tcp", "TCP":
				ruleName = "Custom TCP Rule"
			case "udp", "UDP":
				ruleName = "Custom UDP Rule"
			case "icmp", "ICMP":
				ruleName = "ICMP"
			default:
				ruleName = "Custom TCP Rule" // Default to TCP
			}
		}
	}
	if ruleName != "" {
		payload["ruleName"] = ruleName
	}

	// When we have a port number, open the specific port
	if r.PortNumber != "" {
		payload["openPort"] = true
		payload["portNumber"] = r.PortNumber
	} else {
		payload["openPort"] = false
	}
	if r.CIDR != "" {
		payload["cidr"] = r.CIDR
	}
	return payload
}

// Image represents a Nayatel Cloud OS image.
type Image struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Status    string                 `json:"status,omitempty"`
	Size      float64                `json:"size,omitempty"`
	MinDisk   int                    `json:"min_disk,omitempty"`
	MinRAM    int                    `json:"min_ram,omitempty"`
	OSDistro  string                 `json:"os_distro,omitempty"`
	OSVersion string                 `json:"os_version,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Flavor represents a Nayatel Cloud flavor (instance size).
type Flavor struct {
	ID           string  `json:"id,omitempty"`
	Name         string  `json:"name,omitempty"`
	VCPUs        int     `json:"vcpus,omitempty"`
	CPU          int     `json:"CPU,omitempty"` // Alternative field name
	RAM          int     `json:"ram,omitempty"` // in MB or GB depending on source
	RAMAlt       int     `json:"RAM,omitempty"` // Alternative field name
	Disk         int     `json:"disk,omitempty"`
	Storage      int     `json:"STORAGE,omitempty"` // Alternative field name
	PriceHourly  float64 `json:"price_hourly,omitempty"`
	PriceMonthly float64 `json:"price_monthly,omitempty"`
}

// GetVCPUs returns the number of vCPUs (handles both field names).
func (f *Flavor) GetVCPUs() int {
	if f.VCPUs > 0 {
		return f.VCPUs
	}
	return f.CPU
}

// GetRAM returns the RAM (handles both field names).
func (f *Flavor) GetRAM() int {
	if f.RAM > 0 {
		return f.RAM
	}
	return f.RAMAlt
}

// GetDisk returns the disk size (handles both field names).
func (f *Flavor) GetDisk() int {
	if f.Disk > 0 {
		return f.Disk
	}
	return f.Storage
}

// SSHKey represents a Nayatel Cloud SSH key.
type SSHKey struct {
	Name        string    `json:"name"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	PublicKey   string    `json:"public_key,omitempty"`
	KeyContent  string    `json:"key_content,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}

// GetFingerprint returns the fingerprint or key content (handles both field names).
func (k *SSHKey) GetFingerprint() string {
	if k.Fingerprint != "" {
		return k.Fingerprint
	}
	return k.KeyContent
}

// Volume represents a Nayatel Cloud volume.
type Volume struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name,omitempty"`
	Description      string                 `json:"description,omitempty"`
	Size             int                    `json:"size,omitempty"`        // in GB
	Status           string                 `json:"status,omitempty"`      // available, in-use, creating, deleting, error
	VolumeType       string                 `json:"volume_type,omitempty"` // e.g., "ssd", "hdd"
	Bootable         string                 `json:"bootable,omitempty"`    // "true" or "false"
	Encrypted        bool                   `json:"encrypted,omitempty"`
	AvailabilityZone string                 `json:"availability_zone,omitempty"`
	SourceVolumeID   string                 `json:"source_volid,omitempty"`
	SnapshotID       string                 `json:"snapshot_id,omitempty"`
	ImageID          string                 `json:"image_id,omitempty"` // For bootable volumes
	Attachments      []VolumeAttachment     `json:"attachments,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        string                 `json:"created_at,omitempty"`
	UpdatedAt        string                 `json:"updated_at,omitempty"`
}

// VolumeAttachment represents an attachment of a volume to an instance.
type VolumeAttachment struct {
	ID           string `json:"id,omitempty"`
	VolumeID     string `json:"volume_id,omitempty"`
	ServerID     string `json:"server_id,omitempty"`
	InstanceID   string `json:"instance_id,omitempty"` // Alternative field name
	AttachmentID string `json:"attachment_id,omitempty"`
	Device       string `json:"device,omitempty"` // e.g., "/dev/vdb"
	AttachedAt   string `json:"attached_at,omitempty"`
}

// GetInstanceID returns the instance ID (handles both field names).
func (a *VolumeAttachment) GetInstanceID() string {
	if a.ServerID != "" {
		return a.ServerID
	}
	return a.InstanceID
}

// IsBootable returns true if the volume is bootable.
func (v *Volume) IsBootable() bool {
	return v.Bootable == "true"
}

// IsAttached returns true if the volume is attached to an instance.
func (v *Volume) IsAttached() bool {
	return len(v.Attachments) > 0
}

// GetAttachedInstanceID returns the ID of the instance this volume is attached to.
func (v *Volume) GetAttachedInstanceID() string {
	if len(v.Attachments) > 0 {
		return v.Attachments[0].GetInstanceID()
	}
	return ""
}

// VolumeCreateRequest represents a request to create a volume.
type VolumeCreateRequest struct {
	Name             string `json:"name,omitempty"`
	Description      string `json:"description,omitempty"`
	Size             int    `json:"size"`                        // in GB
	VolumeType       string `json:"volume_type,omitempty"`       // optional
	AvailabilityZone string `json:"availability_zone,omitempty"` // optional
	SnapshotID       string `json:"snapshot_id,omitempty"`       // create from snapshot
	SourceVolumeID   string `json:"source_volid,omitempty"`      // create from volume
	ImageID          string `json:"image_id,omitempty"`          // create bootable volume from image
}

// VolumeAttachRequest represents a request to attach a volume to an instance.
type VolumeAttachRequest struct {
	VolumeID   string `json:"volume_id"`
	InstanceID string `json:"instance_id"`
	Device     string `json:"device,omitempty"` // optional, e.g., "/dev/vdb"
}

// APIResponse is a generic API response.
type APIResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message,omitempty"`
}
