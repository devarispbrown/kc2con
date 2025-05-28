package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/devarispbrown/kc2con/internal/analyzer"
	"github.com/spf13/cobra"
)

var (
	configDir    string
	outputFormat string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze Kafka Connect configurations for migration",
	Long: `Analyze your existing Kafka Connect configurations to understand
what can be migrated to Conduit and what requires manual intervention.

This command will scan your configuration directory and provide:
- Migration compatibility report
- List of supported vs unsupported connectors
- Required manual migration steps
- Estimated migration effort
- Performance analysis and bottleneck detection
- Security configuration validation`,
	Example: `  # Analyze configs in current directory
  kc2con analyze --config-dir ./kafka-connect-configs

  # Analyze with JSON output
  kc2con analyze --config-dir ./configs --format json

  # Verbose analysis
  kc2con analyze --config-dir ./configs --verbose`,
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringVarP(&configDir, "config-dir", "d", ".", "Directory containing Kafka Connect configurations")
	analyzeCmd.Flags().StringVarP(&outputFormat, "format", "f", "table", "Output format (table, json, yaml)")

	if err := analyzeCmd.MarkFlagRequired("config-dir"); err != nil {
		log.Fatalf("Failed to mark config-dir flag as required: %v", err)
	}
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	log.Info("Starting Kafka Connect configuration analysis", "dir", configDir)

	// Enhanced directory validation
	if err := validateConfigDirectory(configDir); err != nil {
		return err
	}

	// Create analyzer with proper error handling
	analyzer, err := analyzer.NewAnalyzer(configDir)
	if err != nil {
		return fmt.Errorf("failed to create analyzer: %w", err)
	}

	// Run analysis with comprehensive error handling
	result, err := analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Validate result before output
	if result == nil {
		return fmt.Errorf("analysis returned nil result")
	}

	// Display results based on format with error handling
	switch outputFormat {
	case "json":
		if err := result.OutputJSON(); err != nil {
			return fmt.Errorf("failed to output JSON: %w", err)
		}
	case "yaml":
		if err := result.OutputYAML(); err != nil {
			return fmt.Errorf("failed to output YAML: %w", err)
		}
	case "table":
		if err := result.OutputTable(); err != nil {
			return fmt.Errorf("failed to output table: %w", err)
		}
	default:
		return fmt.Errorf("unsupported output format '%s' - supported formats: table, json, yaml", outputFormat)
	}

	return nil
}

// validateConfigDirectory performs comprehensive directory validation
func validateConfigDirectory(dir string) error {
	// Check if directory exists
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("configuration directory does not exist: %s", dir)
	}
	if err != nil {
		return fmt.Errorf("cannot access configuration directory %s: %w", dir, err)
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", dir)
	}

	// Check if directory is readable
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read configuration directory %s: %w", dir, err)
	}

	// Check if directory contains any potential configuration files
	hasConfigFiles := false
	supportedExts := []string{".json", ".properties", ".yaml", ".yml"}
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		ext := filepath.Ext(strings.ToLower(file.Name()))
		for _, supportedExt := range supportedExts {
			if ext == supportedExt {
				hasConfigFiles = true
				break
			}
		}
		
		if hasConfigFiles {
			break
		}
	}

	if !hasConfigFiles {
		log.Warn("No configuration files found in directory", 
			"dir", dir, 
			"supported_extensions", supportedExts)
		fmt.Printf("⚠️  Warning: No configuration files found in %s\n", dir)
		fmt.Printf("   Looking for files with extensions: %v\n", supportedExts)
		fmt.Println("   The analysis may return empty results.")
		fmt.Println()
	}

	return nil
}

