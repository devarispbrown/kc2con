package cmd

import (
	"fmt"
	"os"

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

	analyzeCmd.MarkFlagRequired("config-dir")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	log.Info("Starting Kafka Connect configuration analysis", "dir", configDir)

	// Validate config directory
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return fmt.Errorf("configuration directory does not exist: %s", configDir)
	}

	// Create analyzer
	analyzer := analyzer.New(configDir)

	// Run analysis
	result, err := analyzer.Analyze()
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Display results based on format
	switch outputFormat {
	case "json":
		return result.OutputJSON()
	case "yaml":
		return result.OutputYAML()
	default:
		return result.OutputTable()
	}
}
