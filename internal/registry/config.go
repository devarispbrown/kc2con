package registry

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Embed the default connector configuration
//
//go:embed connectors.yaml
var defaultConfig embed.FS

// ConnectorConfig represents the YAML configuration structure
type ConnectorConfig struct {
	Version     string              `yaml:"version"`
	Description string              `yaml:"description"`
	Updated     string              `yaml:"updated"`
	Categories  map[string]Category `yaml:"categories"`
	Connectors  []ConnectorMapping  `yaml:"connectors"`
	Transforms  []TransformMapping  `yaml:"transforms"`
}

// Category represents a connector category
type Category struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// ConnectorMapping represents a single connector mapping from YAML
type ConnectorMapping struct {
	KafkaConnectClass   string   `yaml:"kafka_connect_class"`
	Name                string   `yaml:"name"`
	Category            string   `yaml:"category"`
	ConduitEquivalent   string   `yaml:"conduit_equivalent"`
	ConduitType         string   `yaml:"conduit_type"`
	Status              string   `yaml:"status"`
	Confidence          string   `yaml:"confidence"`
	RequiredFields      []string `yaml:"required_fields"`
	UnsupportedFeatures []string `yaml:"unsupported_features"`
	EstimatedEffort     string   `yaml:"estimated_effort"`
	Notes               string   `yaml:"notes"`
	DocumentationURL    string   `yaml:"documentation_url"`
}

// TransformMapping represents SMT mapping from YAML
type TransformMapping struct {
	KafkaConnectClass string `yaml:"kafka_connect_class"`
	ConduitEquivalent string `yaml:"conduit_equivalent"`
	Status            string `yaml:"status"`
	Notes             string `yaml:"notes"`
}

// LoadConnectorConfig loads connector configuration from YAML
func LoadConnectorConfig(configPath string) (*ConnectorConfig, error) {
	var data []byte
	var err error

	// Try to load from custom path first
	if configPath != "" {
		data, err = os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read custom config file %s: %w", configPath, err)
		}
	} else {
		// Load embedded default configuration
		data, err = defaultConfig.ReadFile("connectors.yaml")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded config: %w", err)
		}
	}

	var config ConnectorConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return &config, nil
}

// Convert YAML mapping to internal ConnectorInfo
func (cm ConnectorMapping) ToConnectorInfo() ConnectorInfo {
	status := StatusSupported
	switch strings.ToLower(cm.Status) {
	case "partial":
		status = StatusPartial
	case "manual":
		status = StatusManual
	case "unsupported":
		status = StatusUnsupported
	}

	return ConnectorInfo{
		Name:                cm.Name,
		KafkaConnectClass:   cm.KafkaConnectClass,
		ConduitEquivalent:   cm.ConduitEquivalent,
		Status:              status,
		RequiredFields:      cm.RequiredFields,
		UnsupportedFeatures: cm.UnsupportedFeatures,
		Notes:               cm.Notes,
		EstimatedEffort:     cm.EstimatedEffort,
		ConfigMapper:        nil, // Will be set separately if needed
	}
}
