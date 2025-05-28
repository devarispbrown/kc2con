package cmd

import (
	"fmt"

	"github.com/devarispbrown/kc2con/internal/compatibility"
	"github.com/spf13/cobra"
)

var compatibilityCmd = &cobra.Command{
	Use:   "compatibility",
	Short: "Show Kafka Connect to Conduit compatibility matrix",
	Long: `Display the compatibility matrix showing which Kafka Connect
connectors and features are supported in Conduit.

This helps you understand what can be automatically migrated
and what requires manual intervention or alternative approaches.`,
	Example: `  # Show full compatibility matrix
  kc2con compatibility

  # Show compatibility for specific connector
  kc2con compatibility --connector mysql`,
	RunE: runCompatibility,
}

var connectorType string

func init() {
	rootCmd.AddCommand(compatibilityCmd)

	compatibilityCmd.Flags().StringVar(&connectorType, "connector", "", "Show compatibility for specific connector type")
}

func runCompatibility(cmd *cobra.Command, args []string) error {
	// Use improved registry with proper error handling
	matrix, err := compatibility.GetMatrix(customConfigPath)
	if err != nil {
		return fmt.Errorf("failed to initialize compatibility matrix: %w", err)
	}

	if connectorType != "" {
		return matrix.ShowConnector(connectorType)
	}

	return matrix.ShowAll()
}