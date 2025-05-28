package cmd

import (
	"bufio"
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
	// Load registry with improved error handling
	registryInstance, err := registry.NewImproved(customConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load connector registry: %w", err)
	}

	// Validate registry is properly loaded
	allConnectors := registryInstance.GetAll()
	if len(allConnectors) == 0 {
		if customConfigPath != "" {
			return fmt.Errorf("no connectors found in configuration file '%s' - check file format and content", customConfigPath)
		}
		return fmt.Errorf("no connectors found in default configuration - this may indicate a build or installation issue")
	}

	// Create styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	categoryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#50FA7B"))

	// Show header
	config := registryInstance.GetConfig()
	if config == nil {
		return fmt.Errorf("registry configuration is nil - configuration failed to load properly")
	}

	fmt.Println(headerStyle.Render("🔗 Conduit Connector Registry"))
	fmt.Printf("Version: %s | Updated: %s\n", config.Version, config.Updated)
	fmt.Printf("Description: %s\n", config.Description)
	fmt.Println()

	if category != "" {
		// Validate category exists
		categories := registryInstance.GetCategories()
		if _, exists := categories[category]; !exists {
			availableCategories := make([]string, 0, len(categories))
			for catKey := range categories {
				availableCategories = append(availableCategories, catKey)
			}
			return fmt.Errorf("category '%s' not found\n\nAvailable categories: %s",
				category, strings.Join(availableCategories, ", "))
		}

		// Show specific category
		connectors := registryInstance.GetConnectorsByCategory(category)
		if len(connectors) == 0 {
			return fmt.Errorf("no connectors found in category: %s", category)
		}

		if cat, exists := categories[category]; exists {
			fmt.Println(categoryStyle.Render(fmt.Sprintf("📂 %s", cat.Name)))
			fmt.Printf("   %s\n\n", cat.Description)
		}

		displayConnectors(connectors, showDetails)
	} else {
		// Show all categories
		categories := registryInstance.GetCategories()
		if len(categories) == 0 {
			fmt.Println("⚠️  No categories defined in configuration")
		}

		for catKey, cat := range categories {
			connectors := registryInstance.GetConnectorsByCategory(catKey)
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

	// Show summary with error handling
	supported := len(registryInstance.GetByStatus(registry.StatusSupported))
	partial := len(registryInstance.GetByStatus(registry.StatusPartial))
	manual := len(registryInstance.GetByStatus(registry.StatusManual))
	unsupported := len(registryInstance.GetByStatus(registry.StatusUnsupported))

	fmt.Printf("📊 Summary: %d total connectors\n", len(allConnectors))
	fmt.Printf("   ✅ %d supported | ⚠️ %d partial | 🔧 %d manual | ❌ %d unsupported\n",
		supported, partial, manual, unsupported)

	return nil
}

func displayConnectors(connectors []registry.ConnectorInfo, showDetails bool) {
	if len(connectors) == 0 {
		fmt.Println("  No connectors in this category")
		return
	}

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

	// Load existing registry with error handling
	registryInstance, err := registry.NewImproved(customConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load existing registry: %w", err)
	}

	// Interactive connector addition with input validation
	connector := registry.ConnectorMapping{}
	reader := bufio.NewReader(os.Stdin)

	// Helper function for validated input
	getInput := func(prompt string, required bool) (string, error) {
		for {
			fmt.Print(prompt)
			input, err := reader.ReadString('\n')
			if err != nil {
				return "", fmt.Errorf("failed to read input: %w", err)
			}
			input = strings.TrimSpace(input)

			if required && input == "" {
				fmt.Println("❌ This field is required. Please enter a value.")
				continue
			}
			return input, nil
		}
	}

	// Collect connector information with validation
	var input string

	input, err = getInput("Kafka Connect Class (e.g., io.confluent.connect.jdbc.JdbcSourceConnector): ", true)
	if err != nil {
		return err
	}
	connector.KafkaConnectClass = input

	input, err = getInput("Connector Name (e.g., JDBC Source): ", true)
	if err != nil {
		return err
	}
	connector.Name = input

	// Validate category
	categories := registryInstance.GetCategories()
	var validCategories []string
	for catKey := range categories {
		validCategories = append(validCategories, catKey)
	}

	for {
		input, err = getInput(fmt.Sprintf("Category (%s): ", strings.Join(validCategories, "/")), true)
		if err != nil {
			return err
		}

		// Validate category exists
		if _, exists := categories[input]; !exists {
			fmt.Printf("❌ Invalid category '%s'. Valid options: %s\n", input, strings.Join(validCategories, ", "))
			continue
		}
		connector.Category = input
		break
	}

	input, err = getInput("Conduit Equivalent (e.g., postgres): ", true)
	if err != nil {
		return err
	}
	connector.ConduitEquivalent = input

	// Validate conduit type
	for {
		input, err = getInput("Conduit Type (source/destination): ", true)
		if err != nil {
			return err
		}
		if input == "source" || input == "destination" {
			connector.ConduitType = input
			break
		}
		fmt.Println("❌ Invalid type. Must be 'source' or 'destination'")
	}

	// Validate status
	validStatuses := []string{"supported", "partial", "manual", "unsupported"}
	for {
		input, err = getInput("Status (supported/partial/manual/unsupported): ", true)
		if err != nil {
			return err
		}

		valid := false
		for _, status := range validStatuses {
			if input == status {
				valid = true
				break
			}
		}

		if valid {
			connector.Status = input
			break
		}
		fmt.Printf("❌ Invalid status '%s'. Valid options: %s\n", input, strings.Join(validStatuses, ", "))
	}

	input, err = getInput("Estimated Effort (e.g., 30 minutes): ", false)
	if err != nil {
		return err
	}
	if input == "" {
		input = "Unknown"
	}
	connector.EstimatedEffort = input

	input, err = getInput("Notes: ", false)
	if err != nil {
		return err
	}
	connector.Notes = input

	// Add connector to registry
	connectorInfo := connector.ToConnectorInfo()
	registryInstance.AddCustomConnector(connectorInfo)

	// Save updated configuration with error handling
	savePath := customConfigPath
	if savePath == "" {
		savePath = "connectors.yaml"
	}
	if outputPath != "" {
		savePath = outputPath
	}

	if err := registryInstance.SaveConfiguration(savePath); err != nil {
		return fmt.Errorf("failed to save configuration to '%s': %w", savePath, err)
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

	// Load and validate registry with comprehensive error handling
	registryInstance, err := registry.NewImproved(customConfigPath)
	if err != nil {
		fmt.Printf("❌ Configuration failed to load: %v\n", err)

		// Provide helpful debugging information
		if customConfigPath != "" {
			if _, statErr := os.Stat(customConfigPath); os.IsNotExist(statErr) {
				fmt.Printf("   File '%s' does not exist\n", customConfigPath)
			} else {
				fmt.Printf("   Check YAML syntax in '%s'\n", customConfigPath)
			}
		} else {
			fmt.Println("   Using embedded configuration - may indicate build issue")
		}
		return err
	}

	config := registryInstance.GetConfig()
	if config == nil {
		return fmt.Errorf("configuration is nil after successful load - this indicates an internal error")
	}

	allConnectors := registryInstance.GetAll()

	fmt.Printf("✅ Configuration loaded successfully\n")
	fmt.Printf("   Version: %s\n", config.Version)
	fmt.Printf("   Connectors: %d\n", len(allConnectors))
	fmt.Printf("   Categories: %d\n", len(config.Categories))
	fmt.Printf("   Transforms: %d\n", len(config.Transforms))
	fmt.Println()

	// Validate connector mappings with detailed reporting
	issues := 0
	warnings := 0

	for class, connector := range allConnectors {
		if connector.Name == "" {
			fmt.Printf("❌ Connector %s missing name\n", class)
			issues++
		}
		if connector.ConduitEquivalent == "" {
			fmt.Printf("❌ Connector %s missing conduit equivalent\n", class)
			issues++
		}
		if connector.EstimatedEffort == "" {
			fmt.Printf("⚠️  Connector %s missing effort estimate\n", class)
			warnings++
		}
		if connector.KafkaConnectClass == "" {
			fmt.Printf("❌ Connector entry missing kafka_connect_class\n")
			issues++
		}
	}

	// Validate categories
	for catKey, category := range config.Categories {
		if category.Name == "" {
			fmt.Printf("⚠️  Category %s missing name\n", catKey)
			warnings++
		}
		if category.Description == "" {
			fmt.Printf("⚠️  Category %s missing description\n", catKey)
			warnings++
		}
	}

	// Summary
	if issues > 0 {
		fmt.Printf("\n❌ Found %d critical validation errors\n", issues)
	}
	if warnings > 0 {
		fmt.Printf("⚠️  Found %d warnings\n", warnings)
	}

	if issues == 0 && warnings == 0 {
		fmt.Println("✅ Registry validation passed - no issues found!")
	} else if issues == 0 {
		fmt.Println("✅ Registry validation passed with warnings")
	}

	return nil
}
