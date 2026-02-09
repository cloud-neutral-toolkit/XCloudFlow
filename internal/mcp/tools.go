package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"xcloudflow/internal/stackflow"
)

func (s *Server) callTool(ctx context.Context, name string, args json.RawMessage) (any, error) {
	switch name {
	case "stackflow.validate":
		var in struct {
			ConfigYAML string `json:"config_yaml"`
			Env        string `json:"env"`
		}
		if err := json.Unmarshal(args, &in); err != nil || in.ConfigYAML == "" {
			return nil, fmt.Errorf("missing config_yaml")
		}
		cfg, err := stackflow.LoadYAML([]byte(in.ConfigYAML))
		if err != nil {
			return nil, err
		}
		if in.Env != "" {
			cfg = stackflow.ApplyEnvOverrides(cfg, in.Env)
		}
		out, err := stackflow.Validate(cfg)
		if err != nil {
			return nil, err
		}
		return out, nil

	case "stackflow.plan.dns":
		var in struct {
			ConfigYAML string `json:"config_yaml"`
			Env        string `json:"env"`
		}
		if err := json.Unmarshal(args, &in); err != nil || in.ConfigYAML == "" {
			return nil, fmt.Errorf("missing config_yaml")
		}
		cfg, err := stackflow.LoadYAML([]byte(in.ConfigYAML))
		if err != nil {
			return nil, err
		}
		out, err := stackflow.DNSPlan(cfg, in.Env)
		if err != nil {
			return nil, err
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

