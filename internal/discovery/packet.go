package discovery

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type Packet struct {
	Hostname  string `json:"hostname"`
	MachineID string `json:"machine_id"`
	Timestamp int64  `json:"timestamp"`
	Interface string `json:"interface"`
	SourceIP  string `json:"source_ip"`
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
