package registry

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/devarispbrown/kc2con/internal/parser"
	"gopkg.in/yaml.v3"
)

// ImprovedRegistry with configuration loading support
type ImprovedRegistry struct {
	connectors map[string]ConnectorInfo
	transforms map[string]TransformInfo
	config     *ConnectorConfig
	configPath string
}

// TransformInfo represents transform/SMT information
type TransformInfo struct {
	KafkaConnectClass string `json:"kafka_connect_class"`
	ConduitEquivalent string `json:"conduit_equivalent"`
	Status            string `json:"status"`
	Notes             string `json:"notes"`
}

// NewImproved creates a new improved registry with optional custom config
func NewImproved(configPath string) (*ImprovedRegistry, error) {
	registry := &ImprovedRegistry{
		connectors: make(map[string]ConnectorInfo),
		transforms: make(map[string]TransformInfo),
		configPath: configPath,
	}

	if err := registry.loadConfiguration(); err != nil {
		return nil, fmt.Errorf("failed to load registry configuration: %w", err)
	}

	return registry, nil
}

// loadConfiguration loads connector mappings from YAML configuration
func (r *ImprovedRegistry) loadConfiguration() error {
	log.Debug("Loading connector configuration", "path", r.configPath)

	config, err := LoadConnectorConfig(r.configPath)
	if err != nil {
		return err
	}

	r.config = config

	// Load connectors
	for _, connectorMapping := range config.Connectors {
		connectorInfo := connectorMapping.ToConnectorInfo()
		r.connectors[connectorMapping.KafkaConnectClass] = connectorInfo

		log.Debug("Loaded connector",
			"class", connectorMapping.KafkaConnectClass,
			"name", connectorMapping.Name,
			"status", connectorMapping.Status)
	}

	// Load transforms
	for _, transformMapping := range config.Transforms {
		transformInfo := TransformInfo{
			KafkaConnectClass: transformMapping.KafkaConnectClass,
			ConduitEquivalent: transformMapping.ConduitEquivalent,
			Status:            transformMapping.Status,
			Notes:             transformMapping.Notes,
		}
		r.transforms[transformMapping.KafkaConnectClass] = transformInfo

		log.Debug("Loaded transform",
			"class", transformMapping.KafkaConnectClass,
			"conduit", transformMapping.ConduitEquivalent)
	}

	log.Info("Registry loaded successfully",
		"connectors", len(r.connectors),
		"transforms", len(r.transforms),
		"version", config.Version)

	return nil
}

// Reload reloads the configuration (useful for updates)
func (r *ImprovedRegistry) Reload() error {
	// Clear existing data
	r.connectors = make(map[string]ConnectorInfo)
	r.transforms = make(map[string]TransformInfo)

	// Reload configuration
	return r.loadConfiguration()
}

// GetConfig returns the loaded configuration
func (r *ImprovedRegistry) GetConfig() *ConnectorConfig {
	return r.config
}

// GetCategories returns all connector categories
func (r *ImprovedRegistry) GetCategories() map[string]Category {
	if r.config == nil {
		return make(map[string]Category)
	}
	return r.config.Categories
}

// GetConnectorsByCategory returns connectors filtered by category
func (r *ImprovedRegistry) GetConnectorsByCategory(category string) []ConnectorInfo {
	var result []ConnectorInfo

	for _, connectorMapping := range r.config.Connectors {
		if connectorMapping.Category == category {
			result = append(result, connectorMapping.ToConnectorInfo())
		}
	}

	return result
}

// GetTransformInfo looks up transform information
func (r *ImprovedRegistry) GetTransformInfo(transformClass string) (TransformInfo, bool) {
	info, exists := r.transforms[transformClass]
	return info, exists
}

// All the existing Registry methods remain the same
func (r *ImprovedRegistry) Lookup(kafkaConnectClass string) (ConnectorInfo, bool) {
	// Direct lookup first
	if info, exists := r.connectors[kafkaConnectClass]; exists {
		return info, true
	}

	// Fuzzy matching for slight variations
	for class, info := range r.connectors {
		if strings.Contains(kafkaConnectClass, class) || strings.Contains(class, kafkaConnectClass) {
			return info, true
		}
	}

	// No match found
	return ConnectorInfo{}, false
}

func (r *ImprovedRegistry) GetAll() map[string]ConnectorInfo {
	return r.connectors
}

func (r *ImprovedRegistry) GetByStatus(status CompatibilityStatus) []ConnectorInfo {
	var result []ConnectorInfo

	for _, info := range r.connectors {
		if info.Status == status {
			result = append(result, info)
		}
	}

	return result
}

func (r *ImprovedRegistry) AnalyzeConnector(config *parser.ConnectorConfig) ConnectorAnalysis {
	// Create a lookup function that uses the ImprovedRegistry's Lookup method
	lookup := func(class string) (ConnectorInfo, bool) {
		return r.Lookup(class)
	}

	// Create a transform analyzer function that uses the ImprovedRegistry's analyzeTransformImproved method
	transformAnalyzer := func(transform parser.TransformConfig) []Issue {
		return r.analyzeTransformImproved(transform)
	}

	// Use the common helper function
	return analyzeConnectorCommon(config, lookup, transformAnalyzer)
}

// analyzeTransformImproved uses the YAML-based transform registry
func (r *ImprovedRegistry) analyzeTransformImproved(transform parser.TransformConfig) []Issue {
	var issues []Issue

	transformInfo, known := r.GetTransformInfo(transform.Class)
	if !known {
		issues = append(issues, Issue{
			Type:        "warning",
			Field:       "transforms." + transform.Name,
			Message:     "Unknown transform: " + transform.Class,
			Suggestion:  "This transform may need to be reimplemented as a Conduit processor",
			AutoFixable: false,
		})
	} else if transformInfo.Status == "partial" {
		issues = append(issues, Issue{
			Type:        "warning",
			Field:       "transforms." + transform.Name,
			Message:     "Transform partially supported: " + transform.Class,
			Suggestion:  transformInfo.Notes,
			AutoFixable: false,
		})
	}

	return issues
}

// AddCustomConnector allows adding connectors at runtime
func (r *ImprovedRegistry) AddCustomConnector(info ConnectorInfo) {
	r.connectors[info.KafkaConnectClass] = info
	log.Debug("Added custom connector", "class", info.KafkaConnectClass, "name", info.Name)
}

// SaveConfiguration saves the current registry to a YAML file
func (r *ImprovedRegistry) SaveConfiguration(outputPath string) error {
	if r.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	data, err := yaml.Marshal(r.config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	log.Info("Configuration saved", "path", outputPath)
	return nil
}
