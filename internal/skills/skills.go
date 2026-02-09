package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Skill struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
}

// DiscoverLocal finds skills under <dir>/*/SKILL.md.
func DiscoverLocal(dir string) ([]Skill, error) {
	var out []Skill
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), "SKILL.md")
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		out = append(out, Skill{
			Name:   e.Name(),
			Path:   p,
			SHA256: shaHex(b),
		})
	}
	return out, nil
}

// ReadSkill reads a SKILL.md file.
func ReadSkill(path string) (Skill, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	return Skill{
		Name:    filepath.Base(filepath.Dir(path)),
		Path:    path,
		Content: string(b),
		SHA256:  shaHex(b),
	}, nil
}

// FetchHTTP downloads a SKILL.md (or skills bundle) over HTTP.
// This is meant for published read-only skills (no secrets).
func FetchHTTP(url string, timeout time.Duration) (Skill, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	c := &http.Client{Timeout: timeout}
	resp, err := c.Get(url)
	if err != nil {
		return Skill{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Skill{}, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return Skill{}, err
	}
	name := url
	if strings.Contains(url, "/") {
		name = url[strings.LastIndex(url, "/")+1:]
	}
	return Skill{
		Name:    name,
		Path:    url,
		Content: string(b),
		SHA256:  shaHex(b),
	}, nil
}

func shaHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

