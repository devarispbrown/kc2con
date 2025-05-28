package compatibility

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devarispbrown/kc2con/internal/registry"
)

type Matrix struct {
	registry *registry.ImprovedRegistry
}

// GetMatrix creates a new matrix with improved registry and error handling
func GetMatrix(configPath string) (*Matrix, error) {
	improvedRegistry, err := registry.NewImproved(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}

	return &Matrix{
		registry: improvedRegistry,
	}, nil
}

func (m *Matrix) ShowAll() error {
	if m.registry == nil {
		return fmt.Errorf("registry not initialized")
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	fmt.Println(titleStyle.Render("🔗 Kafka Connect to Conduit Compatibility Matrix"))
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	connectors := m.registry.GetAll()
	if len(connectors) == 0 {
		fmt.Println("⚠️  No connectors found in registry. Check your configuration.")
		return nil
	}

	fmt.Println("┌─────────────────────────────────────┬─────────────────────┬─────────────┐")
	fmt.Println("│ Connector Type                      │ Conduit Equivalent  │ Status      │")
	fmt.Println("├─────────────────────────────────────┼─────────────────────┼─────────────┤")

	for i := range connectors {
		connectorInfo := &connectors[i]
		status := "✅ Supported"
		switch connectorInfo.Status {
		case registry.StatusPartial:
			status = "⚠️  Partial"
		case registry.StatusManual:
			status = "🔧 Manual"
		case registry.StatusUnsupported:
			status = "❌ Unsupported"
		}

		fmt.Printf("│ %-35s │ %-19s │ %-11s │\n",
			truncateString(connectorInfo.Name, 35),
			truncateString(connectorInfo.ConduitEquivalent, 19),
			status)
	}

	fmt.Println("└─────────────────────────────────────┴─────────────────────┴─────────────┘")
	fmt.Println()

	// Show statistics with error handling
	supported := len(m.registry.GetByStatus(registry.StatusSupported))
	partial := len(m.registry.GetByStatus(registry.StatusPartial))
	manual := len(m.registry.GetByStatus(registry.StatusManual))
	unsupported := len(m.registry.GetByStatus(registry.StatusUnsupported))

	fmt.Printf("📊 Summary: %d supported, %d partial, %d manual, %d unsupported\n",
		supported, partial, manual, unsupported)
	fmt.Println()

	fmt.Println("Legend:")
	fmt.Println("  ✅ Supported - Direct migration available")
	fmt.Println("  ⚠️  Partial  - Most features work, some manual config needed")
	fmt.Println("  🔧 Manual   - Requires manual implementation")
	fmt.Println("  ❌ Unsupported - No Conduit equivalent available")

	return nil
}

func (m *Matrix) ShowConnector(connectorType string) error {
	if m.registry == nil {
		return fmt.Errorf("registry not initialized")
	}

	if strings.TrimSpace(connectorType) == "" {
		return fmt.Errorf("connector type cannot be empty")
	}

	connectors := m.registry.GetAll()
	if len(connectors) == 0 {
		return fmt.Errorf("no connectors found in registry")
	}

	var found *registry.ConnectorInfo

	// Look for exact match first
	for i := range connectors {
		info := &connectors[i]
		if strings.EqualFold(info.Name, connectorType) ||
			strings.Contains(strings.ToLower(info.Name), strings.ToLower(connectorType)) ||
			strings.Contains(strings.ToLower(info.KafkaConnectClass), strings.ToLower(connectorType)) {
			found = info
			break
		}
	}

	if found == nil {
		// Provide helpful suggestions
		var suggestions []string
		for i := range connectors {
			info := &connectors[i]
			if strings.Contains(strings.ToLower(info.Name), strings.ToLower(connectorType)) ||
				strings.Contains(strings.ToLower(connectorType), strings.ToLower(info.Name)) {
				suggestions = append(suggestions, info.Name)
			}
		}

		errorMsg := fmt.Sprintf("connector type '%s' not found in registry", connectorType)
		if len(suggestions) > 0 {
			errorMsg += fmt.Sprintf("\n\nDid you mean one of these?\n")
			for _, suggestion := range suggestions {
				errorMsg += fmt.Sprintf("  • %s\n", suggestion)
			}
		}
		return fmt.Errorf(errorMsg)
	}

	fmt.Printf("🔗 %s\n", found.Name)
	fmt.Println(strings.Repeat("-", len(found.Name)+3))
	fmt.Printf("Kafka Connect Class: %s\n", found.KafkaConnectClass)
	fmt.Printf("Conduit Equivalent: %s\n", found.ConduitEquivalent)
	fmt.Printf("Status: %s\n", found.Status)
	fmt.Printf("Estimated Effort: %s\n", found.EstimatedEffort)

	if len(found.RequiredFields) > 0 {
		fmt.Printf("Required Fields: %s\n", strings.Join(found.RequiredFields, ", "))
	}

	if len(found.UnsupportedFeatures) > 0 {
		fmt.Println("Unsupported Features:")
		for _, feature := range found.UnsupportedFeatures {
			fmt.Printf("  • %s\n", feature)
		}
	}

	if found.Notes != "" {
		fmt.Printf("Notes: %s\n", found.Notes)
	}

	return nil
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}
