package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/devarispbrown/kc2con/internal/migration"
	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/spf13/cobra"
)

var (
	outputDir      string
	dryRun         bool
	validate       bool
	connectors     []string
	registryConfig string
	force          bool
	concurrent     int
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate Kafka Connect configurations to Conduit",
	Long: `Migrate your Kafka Connect configurations to Conduit pipeline configurations.

This command will:
- Convert connector configurations to Conduit pipeline YAML
- Map connector settings to Conduit equivalents
- Transform SMTs to Conduit processors
- Generate deployment-ready configurations

The migration process preserves your data flow while adapting to Conduit's architecture.`,
	Example: `  # Basic migration
  kc2con migrate --config-dir ./kafka-connect --output ./conduit-configs

  # Dry run to preview what would be generated
  kc2con migrate --config-dir ./kafka-connect --dry-run

  # Migrate specific connectors only
  kc2con migrate --config-dir ./kafka-connect --output ./conduit-configs --connectors mysql-source,s3-sink

  # Validate generated configurations
  kc2con migrate --config-dir ./kafka-connect --output ./conduit-configs --validate`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.Flags().StringVarP(&configDir, "config-dir", "d", ".", "Directory containing Kafka Connect configurations")
	migrateCmd.Flags().StringVarP(&outputDir, "output", "o", "./conduit-configs", "Output directory for Conduit configurations")
	migrateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be migrated without creating files")
	migrateCmd.Flags().BoolVar(&validate, "validate", false, "Validate generated configurations")
	migrateCmd.Flags().StringSliceVar(&connectors, "connectors", []string{}, "Specific connectors to migrate (comma-separated)")
	migrateCmd.Flags().StringVar(&registryConfig, "registry", "", "Path to custom connector registry configuration")
	migrateCmd.Flags().BoolVar(&force, "force", false, "Overwrite existing output files")
	migrateCmd.Flags().IntVar(&concurrent, "concurrent", 3, "Number of concurrent migrations (1-10)")

	migrateCmd.MarkFlagRequired("config-dir")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Create styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#50FA7B"))

	warningStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFB86C"))

	errorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF5F87"))

	fmt.Println(headerStyle.Render("🚀 Kafka Connect to Conduit Migration"))
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println()

	// Validate input directory
	if err := validateConfigDirectory(configDir); err != nil {
		return err
	}

	// Validate concurrent value
	if concurrent < 1 {
		concurrent = 1
	} else if concurrent > 10 {
		concurrent = 10
	}

	// Use orchestrator for complex migrations
	orchestrator, err := migration.NewOrchestrator(registryConfig)
	if err != nil {
		return fmt.Errorf("failed to create migration orchestrator: %w", err)
	}

	// Create migration plan
	fmt.Println("📋 Creating migration plan...")
	plan, err := orchestrator.CreateMigrationPlan(configDir)
	if err != nil {
		return fmt.Errorf("failed to create migration plan: %w", err)
	}

	if len(plan.Connectors) == 0 {
		fmt.Println(warningStyle.Render("⚠️  No connector configurations found"))
		return nil
	}

	fmt.Printf("📁 Found %d connector configuration(s)\n", len(plan.Connectors))

	// Display migration plan
	fmt.Println()
	fmt.Println("📊 Migration Plan:")
	fmt.Printf("   Total connectors: %d\n", len(plan.Connectors))
	fmt.Printf("   Estimated effort: %s\n", plan.EstimatedEffort)
	if len(plan.Dependencies) > 0 {
		fmt.Printf("   Dependencies detected: %d\n", len(plan.Dependencies))
	}
	if concurrent > 1 {
		fmt.Printf("   Concurrent migrations: %d\n", concurrent)
	}

	// Filter connectors if specified
	if len(connectors) > 0 {
		filteredConnectors := []*migration.ConnectorMigration{}
		connectorMap := make(map[string]bool)
		for _, name := range connectors {
			connectorMap[strings.ToLower(name)] = true
		}

		for _, conn := range plan.Connectors {
			if connectorMap[strings.ToLower(conn.Config.Name)] {
				filteredConnectors = append(filteredConnectors, conn)
			}
		}

		plan.Connectors = filteredConnectors
		fmt.Printf("🔍 Filtered to %d connector(s) based on --connectors flag\n", len(plan.Connectors))
	}

	if len(plan.Connectors) == 0 {
		fmt.Println(warningStyle.Render("⚠️  No connectors to migrate after filtering"))
		return nil
	}

	if !dryRun {
		// Create output directory
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		fmt.Printf("📂 Output directory: %s\n", outputDir)
	}

	fmt.Println()

	// Perform batch migration
	fmt.Println("🔄 Starting migration...")
	batchResult, err := orchestrator.MigrateBatch(plan, outputDir, dryRun, concurrent)
	if err != nil {
		return fmt.Errorf("batch migration failed: %w", err)
	}

	// Process results
	successCount := len(batchResult.Successful)
	errorCount := len(batchResult.Failed)

	// Save successful migrations
	for _, result := range batchResult.Successful {
		if len(result.Pipeline.Pipelines) == 0 {
			continue
		}

		// Generate output filename
		pipelineName := result.Pipeline.Pipelines[0].Name
		baseName := migration.GeneratePipelineID(pipelineName)
		outputPath := filepath.Join(outputDir, baseName+"-pipeline.yaml")

		if !dryRun {
			// Check if file exists
			if !force {
				if _, err := os.Stat(outputPath); err == nil {
					fmt.Printf("   %s Output file already exists: %s (use --force to overwrite)\n",
						warningStyle.Render("⚠️"), outputPath)
					continue
				}
			}

			// Save pipeline configuration
			if err := migration.SavePipeline(result.Pipeline, outputPath); err != nil {
				fmt.Printf("   %s Failed to save %s: %v\n", errorStyle.Render("❌"), pipelineName, err)
				errorCount++
				successCount--
				continue
			}

			fmt.Printf("   %s Saved: %s\n", successStyle.Render("✅"), outputPath)
		} else {
			fmt.Printf("   %s Would save to: %s\n", successStyle.Render("✅"), outputPath)
		}

		// Display warnings
		for _, warning := range result.Warnings {
			fmt.Printf("      %s %s\n", warningStyle.Render("⚠️"), warning)
		}
	}

	// Display failed migrations
	if len(batchResult.Failed) > 0 {
		fmt.Println()
		fmt.Println(errorStyle.Render("Failed migrations:"))
		for _, failed := range batchResult.Failed {
			fmt.Printf("  • %s: %v\n", failed.ConnectorName, failed.Error)
		}
	}

	// Summary
	fmt.Println()
	fmt.Println(headerStyle.Render("📊 Migration Summary"))
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("✅ Successful: %d\n", successCount)
	if len(batchResult.Warnings) > 0 {
		fmt.Printf("⚠️  Warnings: %d\n", len(batchResult.Warnings))
	}
	if errorCount > 0 {
		fmt.Printf("❌ Failed: %d\n", errorCount)
	}

	// Validate if requested
	if validate && !dryRun && successCount > 0 {
		fmt.Println()
		fmt.Println("🔍 Validating generated configurations...")

		validator := migration.NewValidator(false)
		validationErrors := 0

		for _, result := range batchResult.Successful {
			if len(result.Pipeline.Pipelines) == 0 {
				continue
			}

			valResult := validator.ValidatePipeline(result.Pipeline)
			if !valResult.Valid {
				validationErrors++
				pipelineName := result.Pipeline.Pipelines[0].Name
				fmt.Printf("   %s %s validation failed\n", errorStyle.Render("❌"), pipelineName)
				for _, err := range valResult.Errors {
					fmt.Printf("      - %s\n", err.Message)
				}
			}
		}

		if validationErrors == 0 {
			fmt.Println("   " + successStyle.Render("✅ All configurations are valid"))
		}
	}

	// Next steps
	if !dryRun && successCount > 0 {
		fmt.Println()
		fmt.Println("📋 Next Steps:")
		fmt.Println("  1. Review the generated pipeline configurations")
		fmt.Println("  2. Update any placeholder values (${MASKED_*})")
		fmt.Println("  3. Test pipelines in a development environment")

		if len(batchResult.DeploymentScripts) > 0 {
			fmt.Println("  4. Deploy using the generated scripts:")
			for _, script := range batchResult.DeploymentScripts {
				fmt.Printf("     • %s\n", filepath.Join(outputDir, script.Name))
			}
		} else {
			fmt.Println("  4. Deploy to Conduit using: conduit pipelines import <file>")
		}
	}

	// Return error if there were failures
	if errorCount > 0 {
		return fmt.Errorf("migration completed with %d error(s)", errorCount)
	}

	return nil
}

func findConnectclorConfigs(dir string) ([]string, error) {
	var configs []string
	parser := parser.New()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		name := strings.ToLower(info.Name())

		// Skip worker configurations
		if strings.Contains(name, "connect-") || strings.Contains(name, "worker") {
			log.Debug("Skipping worker config", "file", path)
			return nil
		}

		// Check if it's a potential connector config
		if ext == ".json" || ext == ".properties" {
			// Try to parse to verify it's a connector config
			config, err := parser.ParseConnectorConfig(path)
			if err != nil {
				log.Debug("Not a valid connector config", "file", path, "error", err)
				return nil
			}

			// Verify it has a connector class
			if config.Class != "" {
				configs = append(configs, path)
				log.Debug("Found connector config", "file", path, "class", config.Class)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return configs, nil
}

func filterConfigs(configs []string, connectorNames []string) []string {
	var filtered []string
	parser := parser.New()

	// Create a map for faster lookup
	nameMap := make(map[string]bool)
	for _, name := range connectorNames {
		nameMap[strings.ToLower(name)] = true
	}

	for _, configPath := range configs {
		config, err := parser.ParseConnectorConfig(configPath)
		if err != nil {
			continue
		}

		// Check if connector name matches
		if nameMap[strings.ToLower(config.Name)] {
			filtered = append(filtered, configPath)
		}
	}

	return filtered
}

