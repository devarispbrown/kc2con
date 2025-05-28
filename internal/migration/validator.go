package migration

import (
	"fmt"
	"os"
	"strings"

	"github.com/devarispbrown/kc2con/internal/migration/mappers"
	"gopkg.in/yaml.v3"
)

// Validator validates Conduit pipeline configurations
type Validator struct {
	strictMode bool
}

// ValidationResult contains the results of validation
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError represents a validation error
type ValidationError struct {
	Path    string
	Message string
	Field   string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Path    string
	Message string
	Field   string
}

// NewValidator creates a new pipeline validator
func NewValidator(strictMode bool) *Validator {
	return &Validator{
		strictMode: strictMode,
	}
}

// ValidatePipeline validates a Conduit pipeline configuration
func (v *Validator) ValidatePipeline(pipeline *mappers.ConduitPipeline) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	// Check for nil pipeline
	if pipeline == nil {
		result.addError("", "Pipeline configuration is nil", "")
		return result
	}

	// Validate version
	if pipeline.Version == "" {
		result.addError("", "Pipeline version is required", "version")
	} else if !v.isValidVersion(pipeline.Version) {
		result.addError("", fmt.Sprintf("Invalid pipeline version: %s", pipeline.Version), "version")
	}

	// Validate pipelines
	if len(pipeline.Pipelines) == 0 {
		result.addError("", "At least one pipeline is required", "pipelines")
	}

	for i, p := range pipeline.Pipelines {
		v.validatePipeline(p, fmt.Sprintf("pipelines[%d]", i), result)
	}

	// Validate connectors
	connectorIDs := make(map[string]bool)
	for i, c := range pipeline.Connectors {
		v.validateConnector(c, fmt.Sprintf("connectors[%d]", i), result)

		// Check for duplicate IDs
		if connectorIDs[c.ID] {
			result.addError(fmt.Sprintf("connectors[%d]", i),
				fmt.Sprintf("Duplicate connector ID: %s", c.ID), "id")
		}
		connectorIDs[c.ID] = true
	}

	// Validate processors
	processorIDs := make(map[string]bool)
	for i, p := range pipeline.Processors {
		v.validateProcessor(p, fmt.Sprintf("processors[%d]", i), result)

		// Check for duplicate IDs
		if processorIDs[p.ID] {
			result.addError(fmt.Sprintf("processors[%d]", i),
				fmt.Sprintf("Duplicate processor ID: %s", p.ID), "id")
		}
		processorIDs[p.ID] = true
	}

	// Cross-validate references
	v.validateReferences(pipeline, result)

	return result
}

// ValidateFile validates a Conduit pipeline configuration file
func (v *Validator) ValidateFile(filePath string) (*ValidationResult, error) {
	// Validate file path
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check for empty file
	if len(data) == 0 {
		return nil, fmt.Errorf("file is empty: %s", filePath)
	}

	var pipeline mappers.ConduitPipeline
	if err := yaml.Unmarshal(data, &pipeline); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return v.ValidatePipeline(&pipeline), nil
}

// Validation helper methods

func (v *Validator) validatePipeline(p mappers.Pipeline, path string, result *ValidationResult) {
	// Required fields
	if p.ID == "" {
		result.addError(path, "Pipeline ID is required", "id")
	}

	if p.Name == "" {
		result.addError(path, "Pipeline name is required", "name")
	}

	// Validate status
	validStatuses := []string{"running", "paused", "stopped"}
	if p.Status != "" && !contains(validStatuses, p.Status) {
		result.addError(path, fmt.Sprintf("Invalid pipeline status: %s", p.Status), "status")
	}

	// Validate connector references
	if len(p.Connectors) == 0 {
		result.addError(path, "Pipeline must have at least one connector", "connectors")
	}

	// In strict mode, check description
	if v.strictMode && p.Description == "" {
		result.addWarning(path, "Pipeline description is recommended", "description")
	}

	// Validate DLQ if present
	if p.DLQ != nil {
		v.validateDLQ(p.DLQ, path+".dlq", result)
	}
}

func (v *Validator) validateConnector(c mappers.Connector, path string, result *ValidationResult) {
	// Required fields
	if c.ID == "" {
		result.addError(path, "Connector ID is required", "id")
	}

	if c.Type == "" {
		result.addError(path, "Connector type is required", "type")
	} else if c.Type != "source" && c.Type != "destination" {
		result.addError(path, fmt.Sprintf("Invalid connector type: %s", c.Type), "type")
	}

	if c.Plugin == "" {
		result.addError(path, "Connector plugin is required", "plugin")
	} else {
		// Validate plugin format
		if !v.isValidPlugin(c.Plugin) {
			result.addError(path, fmt.Sprintf("Invalid plugin format: %s", c.Plugin), "plugin")
		}
	}

	if c.Name == "" {
		result.addError(path, "Connector name is required", "name")
	}

	// Validate settings based on plugin type
	v.validateConnectorSettings(c, path, result)
}

func (v *Validator) validateProcessor(p mappers.Processor, path string, result *ValidationResult) {
	// Required fields
	if p.ID == "" {
		result.addError(path, "Processor ID is required", "id")
	}

	if p.Plugin == "" {
		result.addError(path, "Processor plugin is required", "plugin")
	} else {
		// Validate plugin format
		if !v.isValidPlugin(p.Plugin) {
			result.addError(path, fmt.Sprintf("Invalid plugin format: %s", p.Plugin), "plugin")
		}
	}

	// Validate condition syntax if present
	if p.Condition != "" {
		if !v.isValidCondition(p.Condition) {
			result.addError(path, fmt.Sprintf("Invalid condition syntax: %s", p.Condition), "condition")
		}
	}
}

func (v *Validator) validateDLQ(dlq *mappers.DLQ, path string, result *ValidationResult) {
	if dlq.Plugin == "" {
		result.addError(path, "DLQ plugin is required", "plugin")
	}

	if dlq.WindowSize > 0 && dlq.WindowNackThreshold > dlq.WindowSize {
		result.addError(path, "WindowNackThreshold cannot be greater than WindowSize", "windowNackThreshold")
	}
}

func (v *Validator) validateConnectorSettings(c mappers.Connector, path string, result *ValidationResult) {
	// Use the known capabilities from mappers package
	capabilities, ok := mappers.KnownConnectorCapabilities[c.Plugin]
	if ok {
		// Validate required settings
		for _, required := range capabilities.RequiredSettings {
			if _, exists := c.Settings[required]; !exists {
				result.addError(path+".settings",
					fmt.Sprintf("Required setting missing: %s", required), required)
			}
		}

		// In strict mode, warn about unknown settings
		if v.strictMode {
			knownSettings := make(map[string]bool)
			for _, s := range capabilities.RequiredSettings {
				knownSettings[s] = true
			}
			for _, s := range capabilities.OptionalSettings {
				knownSettings[s] = true
			}

			for key := range c.Settings {
				if !knownSettings[key] && !strings.HasPrefix(key, "_") {
					result.addWarning(path+".settings",
						fmt.Sprintf("Unknown setting for %s: %s", c.Plugin, key), key)
				}
			}
		}
	} else {
		// Unknown plugin type - basic validation only
		if len(c.Settings) == 0 && v.strictMode {
			result.addWarning(path, "Connector has no settings configured", "settings")
		}
	}
}

func (v *Validator) validateReferences(pipeline *mappers.ConduitPipeline, result *ValidationResult) {
	// Build maps of available IDs
	connectorIDs := make(map[string]bool)
	for _, c := range pipeline.Connectors {
		connectorIDs[c.ID] = true
	}

	processorIDs := make(map[string]bool)
	for _, p := range pipeline.Processors {
		processorIDs[p.ID] = true
	}

	// Validate pipeline references
	for i, p := range pipeline.Pipelines {
		// Check connector references
		for j, connID := range p.Connectors {
			if !connectorIDs[connID] {
				result.addError(fmt.Sprintf("pipelines[%d].connectors[%d]", i, j),
					fmt.Sprintf("Referenced connector not found: %s", connID), "")
			}
		}

		// Check processor references
		for j, procID := range p.Processors {
			if !processorIDs[procID] {
				result.addError(fmt.Sprintf("pipelines[%d].processors[%d]", i, j),
					fmt.Sprintf("Referenced processor not found: %s", procID), "")
			}
		}
	}
}

// Utility validation methods

func (v *Validator) isValidVersion(version string) bool {
	// Accept common version formats
	parts := strings.Split(version, ".")
	return len(parts) >= 1 && len(parts) <= 3
}

func (v *Validator) isValidPlugin(plugin string) bool {
	// Plugin format: type:name or type:name@version
	parts := strings.Split(plugin, ":")
	if len(parts) != 2 {
		return false
	}

	validTypes := []string{"builtin", "standalone", "custom"}
	if !contains(validTypes, parts[0]) {
		return false
	}

	// Check name part (may include version)
	namePart := parts[1]
	if namePart == "" {
		return false
	}

	// If version is included, validate format
	if strings.Contains(namePart, "@") {
		nameParts := strings.Split(namePart, "@")
		if len(nameParts) != 2 || nameParts[0] == "" || nameParts[1] == "" {
			return false
		}
	}

	return true
}

func (v *Validator) isValidCondition(condition string) bool {
	// Basic condition validation
	// Conditions can start with ! for negation
	cond := strings.TrimPrefix(condition, "!")

	// Should have a prefix like "predicate:"
	if !strings.Contains(cond, ":") {
		return false
	}

	return true
}

// Result helper methods

func (r *ValidationResult) addError(path, message, field string) {
	r.Errors = append(r.Errors, ValidationError{
		Path:    path,
		Message: message,
		Field:   field,
	})
	r.Valid = false
}

func (r *ValidationResult) addWarning(path, message, field string) {
	r.Warnings = append(r.Warnings, ValidationWarning{
		Path:    path,
		Message: message,
		Field:   field,
	})
}

// Utility functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
