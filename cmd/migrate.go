package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/devarispbrown/kc2con/internal/analyzer"
	"github.com/devarispbrown/kc2con/internal/generator"
	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	outputDir       string
	dryRun          bool
	validate        bool
	connectors      []string
	skipUnsupported bool
	generateWrapper bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate Kafka Connect configurations to Conduit",
	Long: `Migrate your Kafka Connect configurations to Conduit pipeline configurations.

This command will:
- Convert connector configurations to Conduit pipeline YAML
- Transform configuration settings appropriately
- Generate processor configurations from transforms
- Create deployment scripts and guides`,
	Example: `  # Basic migration
  kc2con migrate --config-dir ./kafka-connect --output ./conduit-configs

  # Dry run to see what would be generated
  kc2con migrate --config-dir ./kafka-connect --dry-run

  # Migrate specific connectors only
  kc2con migrate --config-dir ./kafka-connect --connectors mysql-source,postgres-sink

  # Generate with Kafka Connect wrapper for unsupported connectors
  kc2con migrate --config-dir ./kafka-connect --generate-wrapper`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.Flags().StringVarP(&configDir, "config-dir", "d", ".", "Directory containing Kafka Connect configurations")
	migrateCmd.Flags().StringVarP(&outputDir, "output", "o", "./conduit-configs", "Output directory for Conduit configurations")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be migrated without creating files")
	migrateCmd.Flags().BoolVar(&validate, "validate", false, "Validate generated configurations")
	migrateCmd.Flags().StringSliceVar(&connectors, "connectors", []string{}, "Specific connectors to migrate (comma-separated)")
	migrateCmd.Flags().BoolVar(&skipUnsupported, "skip-unsupported", false, "Skip unsupported connectors instead of failing")
	migrateCmd.Flags().BoolVar(&generateWrapper, "generate-wrapper", false, "Generate Kafka Connect wrapper config for unsupported connectors")

	if err := migrateCmd.MarkFlagRequired("config-dir"); err != nil {
		log.Fatalf("Failed to mark config-dir flag as required: %v", err)
	}
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Validate input directory
	if err := validateDirectory(configDir); err != nil {
		return fmt.Errorf("invalid config directory: %w", err)
	}

	// Create output directory if not in dry-run mode
	if !dryRun {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Show migration header
	fmt.Println(titleStyle.Render("🚀 Kafka Connect to Conduit Migration"))
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// Create analyzer to scan configurations
	log.Info("Analyzing Kafka Connect configurations...")
	analyzer := analyzer.New(configDir)
	analysisResult, err := analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Show analysis summary
	fmt.Printf("📊 Found %d connectors to migrate\n", len(analysisResult.Connectors))
	supported := 0
	partial := 0
	manual := 0
	unsupported := 0

	for _, conn := range analysisResult.Connectors {
		switch conn.Status {
		case "supported":
			supported++
		case "partial":
			partial++
		case "manual":
			manual++
		case "unsupported":
			unsupported++
		}
	}

	fmt.Printf("  ✅ %d can be migrated directly\n", supported)
	fmt.Printf("  ⚠️  %d need manual configuration\n", partial)
	fmt.Printf("  🔧 %d require manual implementation\n", manual)

	if unsupported > 0 {
		fmt.Printf("  ❌ %d unsupported", unsupported)
		if generateWrapper {
			fmt.Printf(" (will use Kafka Connect wrapper)")
		}
		fmt.Println()
	}
	fmt.Println()

	// Check if we should proceed
	if len(analysisResult.Connectors) == 0 {
		return fmt.Errorf("no connectors found to migrate")
	}

	if unsupported > 0 && !skipUnsupported && !generateWrapper {
		return fmt.Errorf("found %d unsupported connectors. Use --skip-unsupported to skip them or --generate-wrapper to use Kafka Connect wrapper", unsupported)
	}

	// Load parser and registry
	configParser := parser.New()
	registryInstance, err := registry.NewImproved("")
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Create pipeline generator
	gen := generator.New(registryInstance)

	// Create pipelines directory
	pipelinesDir := filepath.Join(outputDir, "pipelines")
	if !dryRun {
		if mkdirErr := os.MkdirAll(pipelinesDir, 0o755); mkdirErr != nil {
			return fmt.Errorf("failed to create pipelines directory: %w", mkdirErr)
		}
	}

	// Process each connector
	log.Info("Starting migration...")
	var results []MigrationResult

	err = filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !isConfigFile(path) {
			return nil
		}

		// Parse connector config
		connectorConfig, err := configParser.ParseConnectorConfig(path)
		if err != nil {
			log.Debug("Skipping non-connector file", "file", path)
			return nil
		}

		// Skip if specific connectors requested and this isn't one
		if len(connectors) > 0 && !contains(connectors, connectorConfig.Name) {
			return nil
		}

		// Migrate the connector
		result := migrateConnector(connectorConfig, gen, registryInstance, pipelinesDir)
		results = append(results, result)

		return nil
	})

	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Display results
	fmt.Println("\n📋 Migration Results:")
	fmt.Println(strings.Repeat("-", 50))

	successCount := 0
	for _, result := range results {
		status := "✅"
		if result.Error != nil {
			status = "❌"
		} else if result.HasWarnings {
			status = "⚠️"
		} else {
			successCount++
		}

		fmt.Printf("%s %s\n", status, result.ConnectorName)

		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		} else {
			fmt.Printf("   Pipeline: %s\n", result.PipelineFile)
			if len(result.Warnings) > 0 {
				fmt.Println("   Warnings:")
				for _, warning := range result.Warnings {
					fmt.Printf("   - %s\n", warning)
				}
			}
		}
	}

	// Generate additional files
	if !dryRun && successCount > 0 {
		// Create deployment script
		if err := generateDeploymentScript(outputDir, results); err != nil {
			log.Warn("Failed to generate deployment script", "error", err)
		}

		// Create migration guide
		if err := generateMigrationGuide(outputDir, results); err != nil {
			log.Warn("Failed to generate migration guide", "error", err)
		}

		// Create README
		if err := generateReadme(outputDir); err != nil {
			log.Warn("Failed to generate README", "error", err)
		}
	}

	// Show next steps
	fmt.Println("\n📚 Next Steps:")
	fmt.Println("1. Review generated pipeline configurations in:", outputDir)
	fmt.Println("2. Update any connector-specific settings marked with TODO")
	fmt.Println("3. Configure Conduit with your infrastructure details")

	if validate {
		fmt.Println("4. Validate configurations with: conduit pipelines validate")
	}

	fmt.Println("5. Deploy pipelines using the generated deployment script")

	if !dryRun {
		fmt.Printf("\n✨ Migration complete! Generated %d pipeline configurations.\n", successCount)
	} else {
		fmt.Println("\n🔍 Dry run complete. No files were created.")
	}

	return nil
}

// MigrationResult represents the result of migrating a single connector
type MigrationResult struct {
	ConnectorName string
	PipelineFile  string
	HasWarnings   bool
	Warnings      []string
	Error         error
}

func migrateConnector(config *parser.ConnectorConfig, gen *generator.Generator, reg *registry.ImprovedRegistry, outputDir string) MigrationResult {
	result := MigrationResult{
		ConnectorName: config.Name,
	}

	// Analyze the connector
	analysis := reg.AnalyzeConnector(config)

	// Check if unsupported
	if analysis.ConnectorInfo.Status == registry.StatusUnsupported {
		if skipUnsupported {
			result.Error = fmt.Errorf("unsupported connector (skipped)")
			return result
		}
		if !generateWrapper {
			result.Error = fmt.Errorf("unsupported connector: %s", config.Class)
			return result
		}
		// Generate wrapper configuration
		return generateWrapperConfig(config, outputDir)
	}

	// Generate pipeline
	pipeline, err := gen.GeneratePipeline(config, analysis)
	if err != nil {
		result.Error = fmt.Errorf("failed to generate pipeline: %w", err)
		return result
	}

	// Collect warnings from analysis
	for _, issue := range analysis.Issues {
		if issue.Type == "warning" {
			result.Warnings = append(result.Warnings, issue.Message)
			result.HasWarnings = true
		}
	}

	// Generate filename
	filename := fmt.Sprintf("%s.yaml", sanitizeFilename(config.Name))
	result.PipelineFile = filepath.Join(outputDir, filename)

	// Write pipeline file
	if !dryRun {
		data, err := yaml.Marshal(pipeline)
		if err != nil {
			result.Error = fmt.Errorf("failed to marshal pipeline: %w", err)
			return result
		}

		if err := os.WriteFile(result.PipelineFile, data, 0o600); err != nil {
			result.Error = fmt.Errorf("failed to write pipeline file: %w", err)
			return result
		}
	}

	return result
}

func generateWrapperConfig(config *parser.ConnectorConfig, outputDir string) MigrationResult {
	result := MigrationResult{
		ConnectorName: config.Name,
		HasWarnings:   true,
		Warnings: []string{
			"Using Kafka Connect wrapper - requires Java runtime and connector JARs",
			"Consider migrating to native Conduit connector when available",
		},
	}

	// Determine connector type
	connectorType := "source"
	if strings.Contains(strings.ToLower(config.Class), "sink") {
		connectorType = "destination"
	}

	// Create wrapper pipeline
	pipeline := registry.ConduitPipeline{
		Version: "2.2",
		Pipelines: []registry.Pipeline{{
			ID:          sanitizeID(config.Name),
			Status:      "stopped",
			ConnectorID: config.Name,
			Source: registry.Source{
				Type:   connectorType,
				Plugin: "standalone:conduit-kafka-connect-wrapper",
				Settings: map[string]interface{}{
					"wrapper.connector.class": config.Class,
				},
			},
		}},
	}

	// Copy all original settings
	settings := pipeline.Pipelines[0].Source.Settings
	for key, value := range config.Config {
		settings[key] = value
	}
	for key, value := range config.RawConfig {
		if _, exists := settings[key]; !exists {
			settings[key] = value
		}
	}

	// Generate filename
	filename := fmt.Sprintf("%s-wrapper.yaml", sanitizeFilename(config.Name))
	result.PipelineFile = filepath.Join(outputDir, filename)

	// Write file
	if !dryRun {
		data, err := yaml.Marshal(pipeline)
		if err != nil {
			result.Error = fmt.Errorf("failed to marshal pipeline: %w", err)
			return result
		}

		if err := os.WriteFile(result.PipelineFile, data, 0o600); err != nil {
			result.Error = fmt.Errorf("failed to write pipeline file: %w", err)
			return result
		}
	}

	return result
}

func validateDirectory(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}
	return nil
}

func isConfigFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".json" || ext == ".properties"
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
		".", "-",
	)
	return strings.ToLower(replacer.Replace(name))
}

func sanitizeID(name string) string {
	sanitized := sanitizeFilename(name)
	// Ensure it starts with a letter
	if sanitized != "" && sanitized[0] >= '0' && sanitized[0] <= '9' {
		sanitized = "p" + sanitized
	}
	return sanitized
}

func generateDeploymentScript(outputDir string, results []MigrationResult) error {
	scriptPath := filepath.Join(outputDir, "deploy-pipelines.sh")

	content := `#!/bin/bash
# Conduit Pipeline Deployment Script
# Generated by kc2con

set -e

CONDUIT_URL="${CONDUIT_URL:-http://localhost:8080}"

echo "🚀 Deploying Conduit pipelines..."
echo "   Target: $CONDUIT_URL"
echo ""

# Function to create pipeline
create_pipeline() {
    local pipeline_file=$1
    local pipeline_name=$(basename "$pipeline_file" .yaml)
    
    echo "Creating pipeline: $pipeline_name"
    
    # Using Conduit CLI
    # conduit pipeline create --file "$pipeline_file"
    
    # Or using API
    curl -X POST \
        -H "Content-Type: application/yaml" \
        -d "@$pipeline_file" \
        "$CONDUIT_URL/v1/pipelines" || {
        echo "❌ Failed to create pipeline: $pipeline_name"
        return 1
    }
    
    echo "✅ Created pipeline: $pipeline_name"
}

# Deploy pipelines
`

	// Add each pipeline to the script
	for _, result := range results {
		if result.Error == nil && result.PipelineFile != "" {
			relativePath := filepath.Join("pipelines", filepath.Base(result.PipelineFile))
			content += fmt.Sprintf("create_pipeline \"%s\"\n", relativePath)
		}
	}

	content += `
echo ""
echo "✨ Deployment complete!"
echo ""
echo "To start pipelines:"
echo "  conduit pipeline start --all"
echo ""
echo "To check status:"
echo "  conduit pipeline list"
`

	return os.WriteFile(scriptPath, []byte(content), 0o600)
}

func generateMigrationGuide(outputDir string, results []MigrationResult) error {
	guidePath := filepath.Join(outputDir, "MIGRATION_GUIDE.md")

	content := `# Kafka Connect to Conduit Migration Guide

## Migration Summary

This directory contains the migrated Conduit pipeline configurations.

## Connector Details

`

	for _, result := range results {
		if result.Error != nil {
			content += fmt.Sprintf("### ❌ %s\n\nFailed to migrate: %v\n\n", result.ConnectorName, result.Error)
			continue
		}

		status := "✅ Migrated"
		if result.HasWarnings {
			status = "⚠️ Migrated with warnings"
		}

		content += fmt.Sprintf("### %s %s\n\n", status, result.ConnectorName)
		content += fmt.Sprintf("- Pipeline File: `%s`\n\n", filepath.Base(result.PipelineFile))

		if len(result.Warnings) > 0 {
			content += "**Warnings:**\n"
			for _, warning := range result.Warnings {
				content += fmt.Sprintf("- %s\n", warning)
			}
			content += "\n"
		}
	}

	content += `## Next Steps

1. Review each pipeline configuration
2. Update TODO items and placeholder values
3. Test pipelines with small data sets
4. Deploy using the provided script
5. Monitor pipeline health and performance

## Resources

- [Conduit Documentation](https://conduit.io/docs)
- [Connector Reference](https://conduit.io/docs/connectors)
`

	return os.WriteFile(guidePath, []byte(content), 0o600)
}

func generateReadme(outputDir string) error {
	readmePath := filepath.Join(outputDir, "README.md")

	content := `# Conduit Pipeline Configurations

This directory contains Conduit pipeline configurations migrated from Kafka Connect.

## Structure

- ` + "`pipelines/`" + ` - Pipeline YAML files
- ` + "`deploy-pipelines.sh`" + ` - Deployment script
- ` + "`MIGRATION_GUIDE.md`" + ` - Detailed migration guide

## Quick Start

1. Review and update pipeline configurations
2. Deploy pipelines:
   ` + "```bash" + `
   ./deploy-pipelines.sh
   ` + "```" + `

## Configuration

Set the Conduit URL if not using default:
` + "```bash" + `
export CONDUIT_URL=http://your-conduit-host:8080
./deploy-pipelines.sh
` + "```" + `
`

	return os.WriteFile(readmePath, []byte(content), 0o600)
}
