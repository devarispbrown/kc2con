package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/devarispbrown/kc2con/internal/registry"
)

var (
	customConfigPath string
	category         string
	showDetails      bool
	outputPath       string
)

var connectorsCmd = &cobra.Command{
	Use:   "connectors",
	Short: "Manage connector registry and compatibility information",
	Long: `Manage the connector registry with subcommands to list, add, and update
connector compatibility information. The registry can be customized with external
YAML configuration files for easy maintenance.`,
	Example: `  # List all connectors
  kc2con connectors list

  # List connectors by category
  kc2con connectors list --category databases

  # Show detailed information
  kc2con connectors list --details

  # Use custom configuration
  kc2con connectors list --config ./my-connectors.yaml`,
}

var listConnectorsCmd = &cobra.Command{
	Use:   "list",
	Short: "List available connectors in the registry",
	Long: `List all connectors available in the compatibility registry.
Can be filtered by category and shown with detailed information.`,
	RunE: runListConnectors,
}

var addConnectorCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new connector to the registry",
	Long: `Add a new connector mapping to the registry. This will create or update
the connector configuration file with the new mapping.`,
	Example: `  # Add a new connector interactively
  kc2con connectors add

  # Add connector with custom config file
  kc2con connectors add --config ./my-connectors.yaml`,
	RunE: runAddConnector,
}

var updateRegistryCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the connector registry from Conduit documentation",
	Long: `Update the connector registry by fetching the latest connector information
from Conduit documentation. This helps keep the registry up-to-date with new
connector releases.`,
	RunE: runUpdateRegistry,
}

var validateRegistryCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the connector registry configuration",
	Long: `Validate the connector registry YAML configuration for syntax errors,
missing required fields, and consistency issues.`,
	RunE: runValidateRegistry,
}

func init() {
	rootCmd.AddCommand(connectorsCmd)

	// Add subcommands
	connectorsCmd.AddCommand(listConnectorsCmd)
	connectorsCmd.AddCommand(addConnectorCmd)
	connectorsCmd.AddCommand(updateRegistryCmd)
	connectorsCmd.AddCommand(validateRegistryCmd)

	// Flags for connectors command
	connectorsCmd.PersistentFlags().StringVar(&customConfigPath, "config", "", "Path to custom connector configuration YAML file")

	// Flags for list command
	listConnectorsCmd.Flags().StringVar(&category, "category", "", "Filter by connector category (databases, storage, messaging, search, monitoring)")
	listConnectorsCmd.Flags().BoolVar(&showDetails, "details", false, "Show detailed connector information")

	// Flags for add command
	addConnectorCmd.Flags().StringVar(&outputPath, "output", "", "Output path for updated configuration (default: update source file)")
}

func runListConnectors(cmd *cobra.Command, args []string) error {
	// Load registry with optional custom config
	registry, err := registry.NewImproved(customConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load connector registry: %w", err)
	}

	// Create styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	categoryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#50FA7B"))

	// Show header
	config := registry.GetConfig()
	fmt.Println(headerStyle.Render("🔗 Conduit Connector Registry"))
	fmt.Printf("Version: %s | Updated: %s\n", config.Version, config.Updated)
	fmt.Printf("Description: %s\n", config.Description)
	fmt.Println()

	if category != "" {
		// Show specific category
		connectors := registry.GetConnectorsByCategory(category)
		if len(connectors) == 0 {
			return fmt.Errorf("no connectors found in category: %s", category)
		}

		categories := registry.GetCategories()
		if cat, exists := categories[category]; exists {
			fmt.Println(categoryStyle.Render(fmt.Sprintf("📂 %s", cat.Name)))
			fmt.Printf("   %s\n\n", cat.Description)
		}

		displayConnectors(connectors, showDetails)
	} else {
		// Show all categories
		categories := registry.GetCategories()
		for catKey, cat := range categories {
			connectors := registry.GetConnectorsByCategory(catKey)
			if len(connectors) == 0 {
				continue
			}

			fmt.Println(categoryStyle.Render(fmt.Sprintf("📂 %s (%d connectors)", cat.Name, len(connectors))))
			fmt.Printf("   %s\n", cat.Description)
			fmt.Println()

			displayConnectors(connectors, showDetails)
			fmt.Println()
		}
	}

	// Show summary
	allConnectors := registry.GetAll()
	supported := len(registry.GetByStatus("supported"))
	partial := len(registry.GetByStatus("partial"))
	manual := len(registry.GetByStatus("manual"))

	fmt.Printf("📊 Summary: %d total connectors\n", len(allConnectors))
	fmt.Printf("   ✅ %d supported | ⚠️ %d partial | 🔧 %d manual\n", supported, partial, manual)

	return nil
}

func displayConnectors(connectors []registry.ConnectorInfo, showDetails bool) {
	for _, connector := range connectors {
		// Status icon
		statusIcon := "✅"
		switch connector.Status {
		case registry.StatusPartial:
			statusIcon = "⚠️"
		case registry.StatusManual:
			statusIcon = "🔧"
		case registry.StatusUnsupported:
			statusIcon = "❌"
		}

		fmt.Printf("  %s %s → %s\n", statusIcon, connector.Name, connector.ConduitEquivalent)

		if showDetails {
			fmt.Printf("     Class: %s\n", connector.KafkaConnectClass)
			fmt.Printf("     Status: %s | Effort: %s\n", connector.Status, connector.EstimatedEffort)

			if len(connector.RequiredFields) > 0 {
				fmt.Printf("     Required: %s\n", strings.Join(connector.RequiredFields, ", "))
			}

			if connector.Notes != "" {
				fmt.Printf("     Notes: %s\n", connector.Notes)
			}
			fmt.Println()
		}
	}
}

func runAddConnector(cmd *cobra.Command, args []string) error {
	fmt.Println("🔧 Add New Connector Mapping")
	fmt.Println("============================")
	fmt.Println()

	// Interactive connector addition
	connector := registry.ConnectorMapping{}

	fmt.Print("Kafka Connect Class (e.g., io.confluent.connect.jdbc.JdbcSourceConnector): ")
	fmt.Scanln(&connector.KafkaConnectClass)

	fmt.Print("Connector Name (e.g., JDBC Source): ")
	fmt.Scanln(&connector.Name)

	fmt.Print("Category (databases/storage/messaging/search/monitoring): ")
	fmt.Scanln(&connector.Category)

	fmt.Print("Conduit Equivalent (e.g., postgres): ")
	fmt.Scanln(&connector.ConduitEquivalent)

	fmt.Print("Conduit Type (source/destination): ")
	fmt.Scanln(&connector.ConduitType)

	fmt.Print("Status (supported/partial/manual/unsupported): ")
	fmt.Scanln(&connector.Status)

	fmt.Print("Estimated Effort (e.g., 30 minutes): ")
	fmt.Scanln(&connector.EstimatedEffort)

	fmt.Print("Notes: ")
	fmt.Scanln(&connector.Notes)

	// Load existing registry
	registry, err := registry.NewImproved(customConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Add connector to registry
	connectorInfo := connector.ToConnectorInfo()
	registry.AddCustomConnector(connectorInfo)

	// Save updated configuration
	savePath := customConfigPath
	if savePath == "" {
		savePath = "connectors.yaml"
	}
	if outputPath != "" {
		savePath = outputPath
	}

	if err := registry.SaveConfiguration(savePath); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Connector added successfully to %s\n", savePath)
	return nil
}

func runUpdateRegistry(cmd *cobra.Command, args []string) error {
	fmt.Println("🔄 Updating Connector Registry")
	fmt.Println("=============================")
	fmt.Println()

	fmt.Println("This feature would:")
	fmt.Println("• Fetch latest connector list from Conduit documentation")
	fmt.Println("• Update connector mappings and compatibility info")
	fmt.Println("• Preserve custom configurations")
	fmt.Println()

	fmt.Println("🚧 This feature is planned for future release")
	fmt.Println("   For now, manually update the connectors.yaml file")
	fmt.Println("   or use 'kc2con connectors add' to add individual connectors")

	return nil
}

func runValidateRegistry(cmd *cobra.Command, args []string) error {
	fmt.Println("✅ Validating Connector Registry")
	fmt.Println("===============================")
	fmt.Println()

	// Load and validate registry
	registry, err := registry.NewImproved(customConfigPath)
	if err != nil {
		fmt.Printf("❌ Configuration failed to load: %v\n", err)
		return err
	}

	config := registry.GetConfig()
	allConnectors := registry.GetAll()

	fmt.Printf("✅ Configuration loaded successfully\n")
	fmt.Printf("   Version: %s\n", config.Version)
	fmt.Printf("   Connectors: %d\n", len(allConnectors))
	fmt.Printf("   Categories: %d\n", len(config.Categories))
	fmt.Printf("   Transforms: %d\n", len(config.Transforms))
	fmt.Println()

	// Validate connector mappings
	issues := 0
	for class, connector := range allConnectors {
		if connector.Name == "" {
			fmt.Printf("⚠️  Connector %s missing name\n", class)
			issues++
		}
		if connector.ConduitEquivalent == "" {
			fmt.Printf("⚠️  Connector %s missing conduit equivalent\n", class)
			issues++
		}
		if connector.EstimatedEffort == "" {
			fmt.Printf("⚠️  Connector %s missing effort estimate\n", class)
			issues++
		}
	}

	if issues > 0 {
		fmt.Printf("\n⚠️  Found %d validation issues\n", issues)
	} else {
		fmt.Println("✅ Registry validation passed - no issues found!")
	}

	return nil
}

// Helper function to create default connectors.yaml if it doesn't exist
func createDefaultConnectorsConfig() error {
	if _, err := os.Stat("connectors.yaml"); os.IsNotExist(err) {
		// Create default config from embedded resource
		registry, err := registry.NewImproved("")
		if err != nil {
			return err
		}

		return registry.SaveConfiguration("connectors.yaml")
	}
	return nil
}

// Usage examples and documentation
var exampleConnectorYAML = `
# Example of adding a new connector to connectors.yaml:

connectors:
  - kafka_connect_class: "com.example.MyCustomConnector"
    name: "My Custom Connector"
    category: "databases"
    conduit_equivalent: "custom-db"
    conduit_type: "source"
    status: "supported"
    confidence: "high"
    required_fields:
      - "database.url"
      - "username"
      - "password"
    estimated_effort: "45 minutes"
    notes: "Custom database connector with CDC support"
    documentation_url: "https://conduit.io/docs/using/connectors/custom-db"
`
