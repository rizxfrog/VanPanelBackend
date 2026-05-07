package model

import "time"

type ListQuery struct {
	Name     string
	State    string
	Page     int
	PageSize int
}

type LogOptions struct {
	Tail       string
	Since      string
	Timestamps bool
	Follow     bool
}

type Container struct {
	ID        string            `json:"id"`
	ShortID   string            `json:"shortId"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	ImageID   string            `json:"imageId"`
	State     string            `json:"state"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
	Ports     []string          `json:"ports"`
	IPs       []string          `json:"ips"`
	Labels    map[string]string `json:"labels"`
	IsCompose bool              `json:"isCompose"`
}

type ListResult struct {
	Items []Container `json:"items"`
	Total int         `json:"total"`
}

type Stats struct {
	CPUPercent    float64   `json:"cpuPercent"`
	MemoryUsage   uint64    `json:"memoryUsage"`
	MemoryLimit   uint64    `json:"memoryLimit"`
	MemoryPercent float64   `json:"memoryPercent"`
	NetworkRx     uint64    `json:"networkRx"`
	NetworkTx     uint64    `json:"networkTx"`
	BlockRead     uint64    `json:"blockRead"`
	BlockWrite    uint64    `json:"blockWrite"`
	ReadAt        time.Time `json:"readAt"`
}
