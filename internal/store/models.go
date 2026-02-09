package store

import "time"

type Run struct {
	RunID      string
	Stack      string
	Env        string
	Phase      string
	Status     string
	Actor      string
	ConfigRef  string
	StartedAt  time.Time
	FinishedAt *time.Time
	InputsJSON []byte
	PlanJSON   []byte
	ResultJSON []byte
}

type MCPServer struct {
	ServerID  string
	Name      string
	BaseURL   string
	Kind      string
	AuthType  string
	Audience  string
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SkillSource struct {
	SourceID string
	Name     string
	Type     string
	URI      string
	Ref      string
	BasePath string
	Enabled  bool
}

