package model

import "time"

const (
	JobStatus_Created = "created"
	JobStatus_Pending = "pending"
	JobStatus_Running = "running"
	JobStatus_Done    = "done"
	JobStatus_Failed  = "failed"
)

type Job struct {
	ID        int       `json:"id,omitempty"`
	Command   []string  `json:"command,omitempty"`
	Logs      []string  `json:"logs,omitempty"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type Error struct {
	Message string `json:"message,omitempty"`
}
