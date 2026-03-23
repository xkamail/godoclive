package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/xkamail/godoclive/internal/model"
	"gopkg.in/yaml.v3"
)

// Config represents the optional .godoclive.yaml configuration file.
type Config struct {
	Title           string        `yaml:"title"`
	Version         string        `yaml:"version"`
	BaseURL         string        `yaml:"base_url"`
	Theme           string        `yaml:"theme"`
	Overrides       []Override    `yaml:"overrides"`
	Exclude         []string      `yaml:"exclude"`
	Mounts          []MountConfig `yaml:"mounts"`
	Auth            AuthConfig    `yaml:"auth"`
	ResponseHelpers []string      `yaml:"response_helpers"`
	OpenAPI         OpenAPIConfig `yaml:"openapi"`
}

// MountConfig maps a package to a path prefix. Endpoints whose handler
// package contains the given string get the prefix prepended to their path.
//
// Example:
//
//	mounts:
//	  - package: backoffice
//	    prefix: /backoffice
type MountConfig struct {
	Package string `yaml:"package"` // matched against handler package path (contains)
	Prefix  string `yaml:"prefix"`  // prepended to endpoint path
}

// OpenAPIConfig holds OpenAPI-specific settings from the config file.
type OpenAPIConfig struct {
	Description string         `yaml:"description"`
	Contact     ContactConfig  `yaml:"contact"`
	License     LicenseConfig  `yaml:"license"`
	Servers     []ServerConfig `yaml:"servers"`
}

// ContactConfig holds contact information for the OpenAPI spec.
type ContactConfig struct {
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	Email string `yaml:"email"`
}

// LicenseConfig holds license information for the OpenAPI spec.
type LicenseConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// ServerConfig holds a server entry for the OpenAPI spec.
type ServerConfig struct {
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

// Override allows users to supplement or override static analysis results.
type Override struct {
	Path      string             `yaml:"path"`      // "POST /users"
	Summary   string             `yaml:"summary"`   // Override inferred summary
	Tags      []string           `yaml:"tags"`      // Override inferred tags
	Responses []ResponseOverride `yaml:"responses"` // Additional responses
}

// ResponseOverride adds a response that static analysis may have missed.
type ResponseOverride struct {
	Status      int    `yaml:"status"`
	Description string `yaml:"description"`
}

// AuthConfig overrides auth detection settings.
type AuthConfig struct {
	Header string `yaml:"header"` // Default: "Authorization"
	Scheme string `yaml:"scheme"` // "bearer", "apikey", "basic"
}

// LoadConfig loads a .godoclive.yaml file from the given directory.
// Returns nil config (no error) if the file does not exist.
func LoadConfig(dir string) (*Config, error) {
	path := filepath.Join(dir, ".godoclive.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ApplyAuth overrides auth detection for endpoints that have auth detected.
// If Scheme is set, it replaces the detected scheme(s). If Header is set
// (non-default), it updates the auth source label.
func ApplyAuth(endpoints []model.EndpointDef, auth AuthConfig) []model.EndpointDef {
	if auth.Scheme == "" && auth.Header == "" {
		return endpoints
	}
	for i := range endpoints {
		if !endpoints[i].Auth.Required {
			continue
		}
		if auth.Scheme != "" {
			endpoints[i].Auth.Schemes = []model.AuthScheme{model.AuthScheme(auth.Scheme)}
		}
	}
	return endpoints
}

// ApplyMounts prepends a path prefix to endpoints whose handler package
// matches a mount config entry. Matching uses strings.Contains so short
// names like "backoffice" match "github.com/org/repo/backoffice/game".
func ApplyMounts(endpoints []model.EndpointDef, mounts []MountConfig) []model.EndpointDef {
	if len(mounts) == 0 {
		return endpoints
	}
	for i := range endpoints {
		for _, m := range mounts {
			if m.Package == "" || m.Prefix == "" {
				continue
			}
			if strings.Contains(endpoints[i].Package, m.Package) {
				endpoints[i].Path = strings.TrimSuffix(m.Prefix, "/") + endpoints[i].Path
				break
			}
		}
	}
	return endpoints
}

// ApplyExclusions removes endpoints matching any of the given glob patterns.
// Patterns are matched against "METHOD /path" (e.g., "GET /internal/*").
func ApplyExclusions(endpoints []model.EndpointDef, patterns []string) []model.EndpointDef {
	if len(patterns) == 0 {
		return endpoints
	}

	var result []model.EndpointDef
	for _, ep := range endpoints {
		key := ep.Method + " " + ep.Path
		excluded := false
		for _, pattern := range patterns {
			if matched, _ := filepath.Match(pattern, key); matched {
				excluded = true
				break
			}
		}
		if !excluded {
			result = append(result, ep)
		}
	}
	return result
}

// ApplyOverrides merges user-defined overrides into the endpoint list.
// Overrides match on "METHOD /path" and can set Summary, Tags, and add Responses.
func ApplyOverrides(endpoints []model.EndpointDef, overrides []Override) []model.EndpointDef {
	if len(overrides) == 0 {
		return endpoints
	}

	// Build a map for quick lookup.
	overrideMap := make(map[string]*Override)
	for i := range overrides {
		overrideMap[strings.ToUpper(overrides[i].Path)] = &overrides[i]
	}

	for i := range endpoints {
		key := endpoints[i].Method + " " + endpoints[i].Path
		ov, ok := overrideMap[strings.ToUpper(key)]
		if !ok {
			continue
		}

		if ov.Summary != "" {
			endpoints[i].Summary = ov.Summary
		}
		if len(ov.Tags) > 0 {
			endpoints[i].Tags = ov.Tags
		}
		for _, r := range ov.Responses {
			endpoints[i].Responses = append(endpoints[i].Responses, model.ResponseDef{
				StatusCode:  r.Status,
				Description: r.Description,
				Source:      "override",
			})
		}
	}

	return endpoints
}
