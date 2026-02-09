package stackflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadYAML parses a StackFlow YAML document into a generic mapping.
func LoadYAML(b []byte) (map[string]any, error) {
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("config must be a YAML mapping")
	}
	return m, nil
}

func StackName(cfg map[string]any) (string, error) {
	md, _ := cfg["metadata"].(map[string]any)
	return getStr(md, "name", "metadata.name")
}

// ApplyEnvOverrides shallow-merges global.environments.<env> into global.
func ApplyEnvOverrides(cfg map[string]any, env string) map[string]any {
	c := cloneMap(cfg)
	g, _ := c["global"].(map[string]any)
	if g == nil {
		return c
	}
	envs, _ := g["environments"].(map[string]any)
	if envs == nil {
		return c
	}
	ovAny, ok := envs[env]
	if !ok {
		return c
	}
	ov, _ := ovAny.(map[string]any)
	if ov == nil {
		return c
	}
	ng := cloneMap(g)
	for k, v := range ov {
		ng[k] = v
	}
	c["global"] = ng
	return c
}

func Validate(cfg map[string]any) (map[string]any, error) {
	kind, _ := cfg["kind"].(string)
	if kind != "StackFlow" {
		return nil, fmt.Errorf("kind must be StackFlow, got %q", kind)
	}
	md, _ := cfg["metadata"].(map[string]any)
	name, err := getStr(md, "name", "metadata.name")
	if err != nil {
		return nil, err
	}

	g, _ := cfg["global"].(map[string]any)
	if g == nil {
		return nil, fmt.Errorf("missing required field: global")
	}
	rootDomain, err := getStr(g, "domain", "global.domain")
	if err != nil {
		return nil, err
	}
	dnsProvider, err := getStr(g, "dns_provider", "global.dns_provider")
	if err != nil {
		return nil, err
	}
	cloud, err := getStr(g, "cloud", "global.cloud")
	if err != nil {
		return nil, err
	}

	targetsAny, ok := cfg["targets"]
	if !ok {
		return nil, fmt.Errorf("missing required field: targets")
	}
	targets, ok := targetsAny.([]any)
	if !ok {
		return nil, fmt.Errorf("targets must be a list")
	}

	for i, tAny := range targets {
		t, ok := tAny.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("targets[%d] must be a mapping", i)
		}
		if _, err := getStr(t, "id", fmt.Sprintf("targets[%d].id", i)); err != nil {
			return nil, err
		}
		if _, err := getStr(t, "type", fmt.Sprintf("targets[%d].type", i)); err != nil {
			return nil, err
		}

		domsAny, ok := t["domains"]
		if !ok {
			return nil, fmt.Errorf("targets[%d].domains must be a non-empty list", i)
		}
		doms, ok := domsAny.([]any)
		if !ok || len(doms) == 0 {
			return nil, fmt.Errorf("targets[%d].domains must be a non-empty list", i)
		}
		for j, dAny := range doms {
			fqdn, ok := dAny.(string)
			if !ok || strings.TrimSpace(fqdn) == "" {
				return nil, fmt.Errorf("targets[%d].domains[%d] must be a non-empty string", i, j)
			}
			if !(fqdn == rootDomain || strings.HasSuffix(fqdn, "."+rootDomain)) {
				return nil, fmt.Errorf("targets[%d].domains[%d] must be under global.domain (%s), got %s", i, j, rootDomain, fqdn)
			}
		}

		if dnsAny, ok := t["dns"]; ok {
			dns, ok := dnsAny.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("targets[%d].dns must be a mapping", i)
			}
			recsAny := dns["records"]
			if recsAny == nil {
				continue
			}
			recs, ok := recsAny.([]any)
			if !ok {
				return nil, fmt.Errorf("targets[%d].dns.records must be a list", i)
			}
			for k, rAny := range recs {
				r, ok := rAny.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("targets[%d].dns.records[%d] must be a mapping", i, k)
				}
				if _, err := normalizeRecord(r); err != nil {
					return nil, fmt.Errorf("targets[%d].dns.records[%d]: %w", i, k, err)
				}
			}
		}
	}

	return map[string]any{
		"ok":           true,
		"stack":        name,
		"domain":       rootDomain,
		"dns_provider": dnsProvider,
		"cloud":        cloud,
		"targets":      len(targets),
	}, nil
}

func DNSPlan(cfg map[string]any, env string) (map[string]any, error) {
	cfg2 := cfg
	if strings.TrimSpace(env) != "" {
		g, _ := cfg["global"].(map[string]any)
		envs, _ := g["environments"].(map[string]any)
		if envs == nil {
			return nil, fmt.Errorf("env not found in global.environments: %s", env)
		}
		if _, ok := envs[env]; !ok {
			return nil, fmt.Errorf("env not found in global.environments: %s", env)
		}
		cfg2 = ApplyEnvOverrides(cfg, env)
	}

	if _, err := Validate(cfg2); err != nil {
		return nil, err
	}

	md, _ := cfg2["metadata"].(map[string]any)
	name, err := getStr(md, "name", "metadata.name")
	if err != nil {
		return nil, err
	}
	g, _ := cfg2["global"].(map[string]any)
	rootDomain, err := getStr(g, "domain", "global.domain")
	if err != nil {
		return nil, err
	}
	dnsProvider, err := getStr(g, "dns_provider", "global.dns_provider")
	if err != nil {
		return nil, err
	}

	targetsAny := cfg2["targets"].([]any)
	var outRecs []map[string]any
	for _, tAny := range targetsAny {
		t := tAny.(map[string]any)
		tid, _ := t["id"].(string)
		dns, _ := t["dns"].(map[string]any)
		if dns == nil {
			continue
		}
		recsAny, _ := dns["records"].([]any)
		for _, rAny := range recsAny {
			nr, err := normalizeRecord(rAny.(map[string]any))
			if err != nil {
				return nil, err
			}
			nr["target"] = tid
			outRecs = append(outRecs, nr)
		}
	}

	return map[string]any{
		"stack": name,
		"env":   strings.TrimSpace(env),
		"global": map[string]any{
			"domain":       rootDomain,
			"dns_provider": dnsProvider,
		},
		"records": outRecs,
	}, nil
}

func getStr(m map[string]any, key string, ctx string) (string, error) {
	if m == nil {
		return "", fmt.Errorf("missing required field: %s", ctx)
	}
	vAny, ok := m[key]
	if !ok {
		return "", fmt.Errorf("missing required field: %s", ctx)
	}
	v, ok := vAny.(string)
	if !ok || strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", ctx)
	}
	return v, nil
}

func normalizeRecord(rec map[string]any) (map[string]any, error) {
	nameAny, ok := rec["name"]
	if !ok {
		return nil, fmt.Errorf("dns.records entries require name and type")
	}
	typeAny, ok := rec["type"]
	if !ok {
		return nil, fmt.Errorf("dns.records entries require name and type")
	}
	name, ok := nameAny.(string)
	if !ok || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("dns.records[].name must be a non-empty string")
	}
	rtype, ok := typeAny.(string)
	if !ok || strings.TrimSpace(rtype) == "" {
		return nil, fmt.Errorf("dns.records[].type must be a non-empty string")
	}
	rtype = strings.ToUpper(strings.TrimSpace(rtype))

	out := map[string]any{"name": name, "type": rtype}

	if vf, ok := rec["valueFrom"]; ok {
		s, ok := vf.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("dns.records[].valueFrom must be a non-empty string")
		}
		out["valueFrom"] = s
	} else if v, ok := rec["value"]; ok {
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("dns.records[].value must be a non-empty string")
		}
		out["value"] = s
	} else {
		return nil, fmt.Errorf("dns.records entries require either value or valueFrom")
	}

	if ttlAny, ok := rec["ttl"]; ok {
		switch t := ttlAny.(type) {
		case int:
			if t <= 0 {
				return nil, fmt.Errorf("dns.records[].ttl must be a positive int")
			}
			out["ttl"] = t
		case int64:
			if t <= 0 {
				return nil, fmt.Errorf("dns.records[].ttl must be a positive int")
			}
			out["ttl"] = int(t)
		default:
			return nil, fmt.Errorf("dns.records[].ttl must be a positive int")
		}
	}

	if pAny, ok := rec["proxied"]; ok {
		b, ok := pAny.(bool)
		if !ok {
			return nil, fmt.Errorf("dns.records[].proxied must be boolean")
		}
		out["proxied"] = b
	}

	_, _ = json.Marshal(out)
	return out, nil
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

