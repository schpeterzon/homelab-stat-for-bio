package models

import "time"

// Status is the portable snapshot committed to the profile repository.
type Status struct {
	Cluster      string    `json:"cluster"`
	Nodes        int       `json:"nodes"`
	Pods         int       `json:"pods"`
	Containers   int       `json:"containers"`
	CPU          float64   `json:"cpu"`
	Memory       float64   `json:"memory"`
	StorageUsed  float64   `json:"storage_used"`
	StorageTotal float64   `json:"storage_total"`
	Updated      time.Time `json:"updated"`
	Errors       []string  `json:"errors,omitempty"`
	History      History   `json:"history"`
}

type History struct {
	CPU     []float64 `json:"cpu"`
	Memory  []float64 `json:"memory"`
	Storage []float64 `json:"storage"`
}
