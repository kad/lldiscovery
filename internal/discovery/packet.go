package discovery

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type NeighborInfo struct {
	MachineID string `json:"machine_id"`
	Hostname  string `json:"hostname"`
	// Local side (sender's interface to this neighbor)
	LocalInterface    string `json:"local_interface"`
	LocalAddress      string `json:"local_address"`
	LocalRDMADevice   string `json:"local_rdma_device,omitempty"`
	LocalNodeGUID     string `json:"local_node_guid,omitempty"`
	LocalSysImageGUID string `json:"local_sys_image_guid,omitempty"`
	LocalSpeed        int    `json:"local_speed,omitempty"` // Link speed in Mbps
	// Remote side (neighbor's interface)
	RemoteInterface    string `json:"remote_interface"`
	RemoteAddress      string `json:"remote_address"`
	RemoteRDMADevice   string `json:"remote_rdma_device,omitempty"`
	RemoteNodeGUID     string `json:"remote_node_guid,omitempty"`
	RemoteSysImageGUID string `json:"remote_sys_image_guid,omitempty"`
	RemoteSpeed        int    `json:"remote_speed,omitempty"` // Link speed in Mbps
}

type Packet struct {
	Hostname     string         `json:"hostname"`
	MachineID    string         `json:"machine_id"`
	Timestamp    int64          `json:"timestamp"`
	Interface    string         `json:"interface"`
	SourceIP     string         `json:"source_ip"`
	RDMADevice   string         `json:"rdma_device,omitempty"`
	NodeGUID     string         `json:"node_guid,omitempty"`
	SysImageGUID string         `json:"sys_image_guid,omitempty"`
	Speed        int            `json:"speed,omitempty"` // Link speed in Mbps
	Neighbors    []NeighborInfo `json:"neighbors,omitempty"`
}

func NewPacket(iface, sourceIP string) (*Packet, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	machineID, err := readMachineID()
	if err != nil {
		return nil, err
	}

	return &Packet{
		Hostname:  hostname,
		MachineID: machineID,
		Timestamp: time.Now().Unix(),
		Interface: iface,
		SourceIP:  sourceIP,
	}, nil
}

func (p *Packet) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func UnmarshalPacket(data []byte) (*Packet, error) {
	var p Packet
	err := json.Unmarshal(data, &p)
	return &p, err
}

func readMachineID() (string, error) {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
