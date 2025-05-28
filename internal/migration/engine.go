package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/devarispbrown/kc2con/internal/migration/mappers"
	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
	"gopkg.in/yaml.v3"
)

// Engine handles the migration from Kafka Connect to Conduit
type Engine struct {
	registry      *registry.ImprovedRegistry
	mapperFactory *mappers.MapperFactory
	parser        *parser.Parser
}

// Result contains the result of a migration
type Result struct {
	Pipeline   *mappers.ConduitPipeline
	SourceFile string
	Issues     []registry.Issue
	Warnings   []string
}

// NewEngine creates a new migration engine
func NewEngine(registryPath string) (*Engine, error) {
	reg, err := registry.NewImproved(registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}

	engine := &Engine{
		registry:      reg,
		mapperFactory: mappers.NewMapperFactory(),
		parser:        parser.New(),
	}

	return engine, nil
}

// MigrateConnector migrates a single Kafka Connect connector to Conduit
func (e *Engine) MigrateConnector(configPath string) (*Result, error) {
	log.Debug("Migrating connector", "path", configPath)

	// Parse the connector configuration
	connectorConfig, err := e.parser.ParseConnectorConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connector config: %w", err)
	}

	// Analyze the connector
	analysis := e.registry.AnalyzeConnector(connectorConfig)

	// Look up connector info
	connectorInfo, found := e.registry.Lookup(connectorConfig.Class)
	if !found {
		return nil, fmt.Errorf("connector class not found in registry: %s", connectorConfig.Class)
	}

	// Check if migration is supported
	if connectorInfo.Status == registry.StatusUnsupported {
		return nil, fmt.Errorf("connector %s is not supported for migration", connectorConfig.Name)
	}

	// Use the mapper factory to get appropriate mapper and perform mapping
	pipeline, err := e.mapperFactory.MapConnector(connectorConfig, connectorInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to map connector: %w", err)
	}

	// Add processors for transforms
	if len(connectorConfig.Transforms) > 0 {
		processors := e.mapTransforms(connectorConfig.Transforms)
		pipeline.Processors = append(pipeline.Processors, processors...)

		// Update pipeline to reference processors
		if len(pipeline.Pipelines) > 0 {
			for i := range processors {
				pipeline.Pipelines[0].Processors = append(
					pipeline.Pipelines[0].Processors,
					processors[i].ID,
				)
			}
		}
	}

	// Validate the generated pipeline
	validationErrors := e.validatePipeline(pipeline)
	for _, err := range validationErrors {
		analysis.Issues = append(analysis.Issues, registry.Issue{
			Type:    "error",
			Message: err.Error(),
		})
	}

	result := &Result{
		Pipeline:   pipeline,
		SourceFile: configPath,
		Issues:     analysis.Issues,
		Warnings:   []string{},
	}

	// Add warnings for partial support
	if connectorInfo.Status == registry.StatusPartial {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Connector has partial support. %s", connectorInfo.Notes))
	}

	if connectorInfo.Status == registry.StatusManual {
		result.Warnings = append(result.Warnings,
			"Manual configuration required. Please review and adjust the generated configuration.")
	}

	return result, nil
}

// MigrateDirectory migrates all connectors in a directory
func (e *Engine) MigrateDirectory(inputDir string, outputDir string) ([]*Result, error) {
	var results []*Result

	// Validate input directory
	if err := validateDirectory(inputDir); err != nil {
		return nil, fmt.Errorf("invalid input directory: %w", err)
	}

	// Find all connector configurations
	files, err := findConnectorConfigs(inputDir)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no connector configurations found in %s", inputDir)
	}

	// Migrate each connector
	for _, file := range files {
		result, err := e.MigrateConnector(file)
		if err != nil {
			log.Error("Failed to migrate connector", "file", file, "error", err)
			// Continue with other connectors
			results = append(results, &Result{
				SourceFile: file,
				Issues: []registry.Issue{{
					Type:    "error",
					Message: err.Error(),
				}},
			})
			continue
		}

		// Save the pipeline
		outputFile := filepath.Join(outputDir, generateOutputFilename(file))
		if err := SavePipeline(result.Pipeline, outputFile); err != nil {
			result.Issues = append(result.Issues, registry.Issue{
				Type:    "error",
				Message: fmt.Sprintf("Failed to save pipeline: %v", err),
			})
		} else {
			log.Info("Migrated connector", "input", file, "output", outputFile)
		}

		results = append(results, result)
	}

	return results, nil
}

// mapTransforms converts Kafka Connect transforms to Conduit processors
func (e *Engine) mapTransforms(transforms []parser.TransformConfig) []mappers.Processor {
	processors := make([]mappers.Processor, 0, len(transforms))

	for i, transform := range transforms {
		processor := mappers.Processor{
			ID:       fmt.Sprintf("processor-%d", i),
			Settings: make(map[string]interface{}),
		}

		// Get transform mapping from registry
		if transformInfo, exists := e.registry.GetTransformInfo(transform.Class); exists {
			processor.Plugin = transformInfo.ConduitEquivalent

			// Copy relevant settings based on transform type
			transformCopy := transform // Create a copy to prevent memory aliasing
			mapTransformSettings(&processor, &transformCopy)
		} else {
			// For unknown transforms, create a placeholder
			processor.Plugin = "custom:transform@latest"
			processor.Settings["original_class"] = transform.Class
			processor.Settings["config"] = transform.Config
			processor.Settings["_comment"] = "Manual implementation required for this transform"
		}

		// Handle predicates
		if transform.Predicate != "" {
			processor.Condition = fmt.Sprintf("predicate:%s", transform.Predicate)
			if transform.Negate {
				processor.Condition = "!" + processor.Condition
			}
		}

		processors = append(processors, processor)
	}

	return processors
}

// mapTransformSettings maps transform-specific settings
func mapTransformSettings(processor *mappers.Processor, transform *parser.TransformConfig) {
	switch transform.Class {
	case "org.apache.kafka.connect.transforms.RegexRouter":
		if regex, ok := transform.Config["regex"].(string); ok {
			processor.Settings["regex"] = regex
		}
		if replacement, ok := transform.Config["replacement"].(string); ok {
			processor.Settings["replacement"] = replacement
		}

	case "org.apache.kafka.connect.transforms.TimestampConverter":
		if field, ok := transform.Config["field"]; ok {
			processor.Settings["field"] = field
		}
		processor.Settings["type"] = "timestamp"

	case "org.apache.kafka.connect.transforms.InsertField":
		for k, v := range transform.Config {
			if k != "type" {
				processor.Settings[k] = v
			}
		}

	case "org.apache.kafka.connect.transforms.MaskField":
		if fields, ok := transform.Config["fields"].(string); ok {
			processor.Settings["fields"] = strings.Split(fields, ",")
		}

	case "org.apache.kafka.connect.transforms.Filter":
		for k, v := range transform.Config {
			if k != "type" {
				processor.Settings[k] = v
			}
		}

	case "io.debezium.transforms.ExtractNewRecordState":
		if dropTombstones, ok := transform.Config["drop.tombstones"]; ok {
			processor.Settings["drop.tombstones"] = dropTombstones
		}

	default:
		// Copy all settings except type
		for k, v := range transform.Config {
			if k != "type" {
				processor.Settings[k] = v
			}
		}
	}
}

// validatePipeline performs basic validation on the generated pipeline
func (e *Engine) validatePipeline(pipeline *mappers.ConduitPipeline) []error {
	var errors []error

	if pipeline == nil {
		return []error{fmt.Errorf("pipeline is nil")}
	}

	// Validate version
	if pipeline.Version == "" {
		errors = append(errors, fmt.Errorf("pipeline version is required"))
	}

	// Validate pipelines
	if len(pipeline.Pipelines) == 0 {
		errors = append(errors, fmt.Errorf("at least one pipeline is required"))
	}

	// Validate connectors
	if len(pipeline.Connectors) == 0 {
		errors = append(errors, fmt.Errorf("at least one connector is required"))
	}

	// Validate connectors have required settings
	for _, connector := range pipeline.Connectors {
		if connector.Plugin == "" {
			errors = append(errors, fmt.Errorf("connector %s missing plugin", connector.ID))
		}

		// Validate settings against capabilities
		validateErrors := mappers.ValidateConnectorSettings(connector.Plugin, connector.Settings)
		errors = append(errors, validateErrors...)
	}

	// Validate processors
	for _, processor := range pipeline.Processors {
		if processor.Plugin == "" {
			errors = append(errors, fmt.Errorf("processor %s missing plugin", processor.ID))
		}
	}

	return errors
}

// SavePipeline saves the pipeline configuration to a YAML file with security checks
func SavePipeline(pipeline *mappers.ConduitPipeline, outputPath string) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline is nil")
	}

	// Sanitize output path to prevent path traversal
	cleanPath := filepath.Clean(outputPath)

	// Ensure the path doesn't escape the intended directory
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid output path: contains directory traversal")
	}

	// Validate filename
	filename := filepath.Base(cleanPath)
	if !isValidFilename(filename) {
		return fmt.Errorf("invalid filename: %s", filename)
	}

	data, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("failed to marshal pipeline: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write file with restricted permissions
	if err := os.WriteFile(cleanPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write pipeline file: %w", err)
	}

	return nil
}

// Helper functions

// validateDirectory checks if a directory exists and is accessible
func validateDirectory(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", dir)
		}
		return fmt.Errorf("cannot access directory: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}

	return nil
}

// findConnectorConfigs finds all connector configuration files in a directory
func findConnectorConfigs(dir string) ([]string, error) {
	var configs []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".json" || ext == ".properties" {
			// Skip worker configs
			name := strings.ToLower(info.Name())
			if strings.Contains(name, "worker") || strings.Contains(name, "connect-") {
				return nil
			}
			configs = append(configs, path)
		}

		return nil
	})

	return configs, err
}

// generateOutputFilename generates an output filename from input
func generateOutputFilename(inputPath string) string {
	base := filepath.Base(inputPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Clean the name
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	return fmt.Sprintf("%s-pipeline.yaml", name)
}

// isValidFilename checks if a filename is safe to use
func isValidFilename(filename string) bool {
	// Check for empty filename
	if filename == "" || filename == "." || filename == ".." {
		return false
	}

	// Check for null bytes
	if strings.Contains(filename, "\x00") {
		return false
	}

	// Ensure it has a valid extension
	ext := filepath.Ext(filename)
	if ext != ".yaml" && ext != ".yml" {
		return false
	}

	// Check for suspicious patterns
	suspicious := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range suspicious {
		if strings.Contains(filename, char) {
			return false
		}
	}

	return true
}

// GeneratePipelineID generates a unique pipeline ID from connector name
func GeneratePipelineID(connectorName string) string {
	// Clean the name for use as ID
	id := strings.ToLower(connectorName)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "_", "-")
	id = strings.ReplaceAll(id, ".", "-")

	// Remove any non-alphanumeric characters except hyphens
	cleaned := ""
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned += string(r)
		}
	}

	// Ensure it doesn't start or end with hyphen
	cleaned = strings.Trim(cleaned, "-")

	if cleaned == "" {
		cleaned = "pipeline"
	}

	return cleaned
}
