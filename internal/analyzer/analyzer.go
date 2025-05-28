// internal/analyzer/analyzer.go
package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
	"gopkg.in/yaml.v3"
)

// Status constants
const (
	StatusSupported   = "supported"
	StatusPartial     = "partial"
	StatusManual      = "manual"
	StatusUnsupported = "unsupported"
)

// Connector type constants
const (
	connectorTypeSource  = "source"
	connectorTypeSink    = "sink"
	connectorTypeUnknown = "unknown"
)

type Analyzer struct {
	configDir string
}

type Result struct {
	Connectors      []ConnectorInfo `json:"connectors"`
	Transforms      []TransformInfo `json:"transforms"`
	WorkerConfig    *WorkerInfo     `json:"worker_config,omitempty"`
	MigrationPlan   MigrationPlan   `json:"migration_plan"`
	EstimatedEffort string          `json:"estimated_effort"`
}

type ConnectorInfo struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Class          string   `json:"class"`
	ConduitMapping string   `json:"conduit_mapping"`
	Status         string   `json:"status"` // "supported", "partial", "manual", "unsupported"
	Issues         []string `json:"issues,omitempty"`
}

type TransformInfo struct {
	Name   string `json:"name"`
	Class  string `json:"class"`
	Status string `json:"status"`
}

type WorkerInfo struct {
	BootstrapServers string `json:"bootstrap_servers"`
	SchemaRegistry   string `json:"schema_registry,omitempty"`
	Security         string `json:"security,omitempty"`
}

type MigrationPlan struct {
	TotalConnectors   int    `json:"total_connectors"`
	DirectMigration   int    `json:"direct_migration"`
	ManualMigration   int    `json:"manual_migration"`
	UnsupportedItems  int    `json:"unsupported_items"`
	EstimatedDowntime string `json:"estimated_downtime"`
}

func New(configDir string) *Analyzer {
	return &Analyzer{
		configDir: configDir,
	}
}

func (a *Analyzer) Analyze() (*Result, error) {
	log.Debug("Scanning configuration directory", "dir", a.configDir)

	result := &Result{
		Connectors: []ConnectorInfo{},
		Transforms: []TransformInfo{},
	}

	// Create parser and registry
	configParser := parser.New()
	registry, err := registry.NewImproved("")
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	var connectorConfigs []*parser.ConnectorConfig

	// Scan for configuration files
	err = filepath.Walk(a.configDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			name := strings.ToLower(info.Name())

			switch {
			case ext == ".json":
				log.Debug("Processing JSON config file", "file", path)
				if err := a.processConnectorConfig(configParser, registry, path, result, &connectorConfigs); err != nil {
					log.Warn("Failed to process connector config", "file", path, "error", err)
				}

			case ext == ".properties":
				log.Debug("Processing properties file", "file", path)
				if strings.Contains(name, "connect-") || strings.Contains(name, "worker") {
					if err := a.processWorkerConfig(configParser, path, result); err != nil {
						log.Warn("Failed to process worker config", "file", path, "error", err)
					}
				} else {
					// Might be a connector config in properties format
					if err := a.processConnectorConfig(configParser, registry, path, result, &connectorConfigs); err != nil {
						log.Warn("Failed to process connector properties", "file", path, "error", err)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Build migration plan using registry analysis
	result.MigrationPlan = a.buildMigrationPlan(result)
	result.EstimatedEffort = a.estimateEffort(result)

	return result, nil
}

func (a *Analyzer) processConnectorConfig(parser *parser.Parser, registry *registry.ImprovedRegistry, configPath string, result *Result, connectorConfigs *[]*parser.ConnectorConfig) error {
	connectorConfig, err := parser.ParseConnectorConfig(configPath)
	if err != nil {
		return err
	}

	*connectorConfigs = append(*connectorConfigs, connectorConfig)

	// Analyze connector using registry
	analysis := registry.AnalyzeConnector(connectorConfig)

	// Convert to our result format
	connectorInfo := ConnectorInfo{
		Name:           connectorConfig.Name,
		Type:           determineConnectorType(connectorConfig.Class),
		Class:          connectorConfig.Class,
		ConduitMapping: analysis.ConnectorInfo.ConduitEquivalent,
		Status:         string(analysis.ConnectorInfo.Status),
		Issues:         []string{},
	}

	// Convert issues to string messages for the table output
	for _, issue := range analysis.Issues {
		var prefix string
		switch issue.Type {
		case "error":
			prefix = "❌ "
		case "warning":
			prefix = "⚠️ "
		default:
			prefix = "ℹ️ "
		}

		message := prefix + issue.Message
		if issue.Suggestion != "" {
			message += " (Suggestion: " + issue.Suggestion + ")"
		}
		connectorInfo.Issues = append(connectorInfo.Issues, message)
	}

	// Extract any SMTs
	for _, transform := range connectorConfig.Transforms {
		transformInfo := TransformInfo{
			Name:   transform.Name,
			Class:  transform.Class,
			Status: "unknown", // Will be analyzed by registry
		}

		// Simple transform status determination
		if isKnownTransform(transform.Class) {
			transformInfo.Status = StatusSupported
		} else if isPartialTransform(transform.Class) {
			transformInfo.Status = StatusPartial
		} else {
			transformInfo.Status = StatusManual
		}

		result.Transforms = append(result.Transforms, transformInfo)
	}

	result.Connectors = append(result.Connectors, connectorInfo)
	return nil
}

func (a *Analyzer) processWorkerConfig(parser *parser.Parser, configPath string, result *Result) error {
	workerConfig, err := parser.ParseWorkerConfig(configPath)
	if err != nil {
		return err
	}

	result.WorkerConfig = &WorkerInfo{
		BootstrapServers: workerConfig.BootstrapServers,
		SchemaRegistry:   workerConfig.SchemaRegistry,
		Security:         workerConfig.SecurityProtocol,
	}

	return nil
}

func (a *Analyzer) buildMigrationPlan(result *Result) MigrationPlan {
	plan := MigrationPlan{
		TotalConnectors:   len(result.Connectors),
		EstimatedDowntime: "< 30 seconds",
	}

	// Count by status
	for i := range result.Connectors {
		connector := &result.Connectors[i]
		switch connector.Status {
		case StatusSupported:
			plan.DirectMigration++
		case StatusPartial:
			plan.ManualMigration++
		case StatusManual, StatusUnsupported:
			plan.UnsupportedItems++
		default:
			plan.ManualMigration++
		}
	}

	// Adjust downtime estimate based on complexity
	if plan.ManualMigration > 2 || plan.UnsupportedItems > 0 {
		plan.EstimatedDowntime = "2-5 minutes"
	}

	return plan
}

func (a *Analyzer) estimateEffort(result *Result) string {
	totalMinutes := 0

	for i := range result.Connectors {
		connector := &result.Connectors[i]
		switch connector.Status {
		case StatusSupported:
			totalMinutes += 30
		case StatusPartial:
			totalMinutes += 90
		case StatusManual:
			totalMinutes += 240 // 4 hours
		case StatusUnsupported:
			totalMinutes += 480 // 8 hours
		default:
			totalMinutes += 120 // 2 hours
		}
	}

	// Add time for transforms
	for i := range result.Transforms {
		transform := &result.Transforms[i]
		switch transform.Status {
		case StatusSupported:
			totalMinutes += 15
		case StatusPartial:
			totalMinutes += 45
		default:
			totalMinutes += 120
		}
	}

	// Convert minutes to human-readable format using switch
	switch {
	case totalMinutes <= 60:
		return "< 1 hour"
	case totalMinutes <= 240:
		return fmt.Sprintf("%d-%d hours", totalMinutes/60, (totalMinutes/60)+1)
	case totalMinutes <= 480:
		return "4-8 hours"
	default:
		return "1-2 days"
	}
}

func (r *Result) OutputTable() error {
	// Create beautiful table output using lipgloss
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		Align(lipgloss.Center)

	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#50FA7B"))

	warningStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFB86C"))

	errorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF5F87"))

	fmt.Println(headerStyle.Render("📊 Comprehensive Kafka Connect Migration Analysis"))
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// Discovered Resources
	fmt.Println("🔍 Discovered Resources:")
	fmt.Printf("  • %d connector configurations\n", len(r.Connectors))
	fmt.Printf("  • %d transforms (SMTs)\n", len(r.Transforms))
	if r.WorkerConfig != nil {
		fmt.Println("  • 1 worker configuration")
		if r.WorkerConfig.SchemaRegistry != "" {
			fmt.Printf("  • Schema Registry: %s\n", r.WorkerConfig.SchemaRegistry)
		}
		if r.WorkerConfig.Security != "" {
			fmt.Printf("  • Security: %s\n", r.WorkerConfig.Security)
		}
	}
	fmt.Println()

	// Migration Plan Table
	fmt.Println("📋 Migration Compatibility Matrix:")
	fmt.Println("┌─────────────────────┬──────────────────┬─────────────┐")
	fmt.Println("│ Kafka Connect Item  │ Conduit Mapping  │ Status      │")
	fmt.Println("├─────────────────────┼──────────────────┼─────────────┤")

	for i := range r.Connectors {
		connector := &r.Connectors[i]
		status := "✅ Supported"
		statusStyle := successStyle
		switch connector.Status {
		case StatusPartial:
			status = "⚠️  Partial"
			statusStyle = warningStyle
		case StatusManual:
			status = "🔧 Manual"
			statusStyle = warningStyle
		case StatusUnsupported:
			status = "❌ Unsupported"
			statusStyle = errorStyle
		default:
			// Leave as "✅ Supported" with successStyle
		}

		fmt.Printf("│ %-19s │ %-16s │ %s │\n",
			truncate(connector.Name, 19),
			truncate(connector.ConduitMapping, 16),
			statusStyle.Render(status[:11])) // Truncate to fit column
	}

	fmt.Println("└─────────────────────┴──────────────────┴─────────────┘")
	fmt.Println()

	// Enhanced Analysis Results
	hasIssues := false
	hasErrors := false
	hasWarnings := false

	for i := range r.Connectors {
		connector := &r.Connectors[i]
		if len(connector.Issues) > 0 {
			hasIssues = true
			for _, issue := range connector.Issues {
				if strings.Contains(issue, "❌") {
					hasErrors = true
				}
				if strings.Contains(issue, "⚠️") {
					hasWarnings = true
				}
			}
		}
	}

	if hasIssues {
		fmt.Println("🔍 Analysis Results:")

		if hasErrors {
			fmt.Println()
			fmt.Println(errorStyle.Render("❌ Critical Issues Found:"))
			for i := range r.Connectors {
				connector := &r.Connectors[i]
				for _, issue := range connector.Issues {
					if strings.Contains(issue, "❌") {
						fmt.Printf("  %s: %s\n", connector.Name, strings.TrimPrefix(issue, "❌ "))
					}
				}
			}
		}

		if hasWarnings {
			fmt.Println()
			fmt.Println(warningStyle.Render("⚠️  Warnings & Recommendations:"))
			for i := range r.Connectors {
				connector := &r.Connectors[i]
				for _, issue := range connector.Issues {
					if strings.Contains(issue, "⚠️") {
						fmt.Printf("  %s: %s\n", connector.Name, strings.TrimPrefix(issue, "⚠️ "))
					}
				}
			}
		}
		fmt.Println()
	} else {
		fmt.Println(successStyle.Render("✨ No issues detected! Configuration looks good for migration."))
		fmt.Println()
	}

	// Transform Analysis
	if len(r.Transforms) > 0 {
		fmt.Println("🔄 Transform (SMT) Analysis:")
		supportedTransforms := 0
		partialTransforms := 0
		manualTransforms := 0

		for i := range r.Transforms {
			transform := &r.Transforms[i]
			switch transform.Status {
			case StatusSupported:
				supportedTransforms++
			case StatusPartial:
				partialTransforms++
			default:
				manualTransforms++
			}
		}

		fmt.Printf("  • %s %d supported transforms\n", successStyle.Render("✅"), supportedTransforms)
		if partialTransforms > 0 {
			fmt.Printf("  • %s %d partially supported transforms\n", warningStyle.Render("⚠️"), partialTransforms)
		}
		if manualTransforms > 0 {
			fmt.Printf("  • %s %d transforms need manual implementation\n", warningStyle.Render("🔧"), manualTransforms)
		}
		fmt.Println()
	}

	// Migration Summary
	fmt.Printf("🚀 Migration Summary:\n")
	fmt.Printf("  • Total connectors: %d\n", r.MigrationPlan.TotalConnectors)
	fmt.Printf("  • %s Direct migration: %d\n", successStyle.Render("✅"), r.MigrationPlan.DirectMigration)
	if r.MigrationPlan.ManualMigration > 0 {
		fmt.Printf("  • %s Manual work needed: %d\n", warningStyle.Render("⚠️"), r.MigrationPlan.ManualMigration)
	}
	if r.MigrationPlan.UnsupportedItems > 0 {
		fmt.Printf("  • %s Unsupported items: %d\n", errorStyle.Render("❌"), r.MigrationPlan.UnsupportedItems)
	}
	fmt.Printf("  • Estimated effort: %s\n", r.EstimatedEffort)
	fmt.Printf("  • Estimated downtime: %s\n", r.MigrationPlan.EstimatedDowntime)

	// Migration readiness score
	score := float64(r.MigrationPlan.DirectMigration) / float64(r.MigrationPlan.TotalConnectors) * 100
	fmt.Printf("  • Migration readiness: %.1f%%\n", score)

	// Next steps
	fmt.Println()
	fmt.Println("📋 Next Steps:")
	if hasErrors {
		fmt.Println("  1. ❌ Fix critical configuration errors before migration")
	}
	if hasWarnings {
		fmt.Println("  2. ⚠️  Review warnings and apply recommended changes")
	}
	if r.MigrationPlan.ManualMigration > 0 || r.MigrationPlan.UnsupportedItems > 0 {
		fmt.Println("  3. 🔧 Plan manual migration for unsupported connectors")
	}
	if score >= 80 {
		fmt.Println("  4. ✅ Ready to proceed with migration!")
	} else {
		fmt.Println("  4. 📝 Address issues above before migrating")
	}

	return nil
}

func (r *Result) OutputJSON() error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

func (r *Result) OutputYAML() error {
	encoder := yaml.NewEncoder(os.Stdout)
	defer encoder.Close()
	return encoder.Encode(r)
}

// Helper functions
func determineConnectorType(class string) string {
	class = strings.ToLower(class)
	if strings.Contains(class, "source") || strings.Contains(class, "debezium") {
		return connectorTypeSource
	} else if strings.Contains(class, "sink") {
		return connectorTypeSink
	}
	return connectorTypeUnknown
}

func isKnownTransform(class string) bool {
	knownTransforms := []string{
		"org.apache.kafka.connect.transforms.RegexRouter",
		"org.apache.kafka.connect.transforms.TimestampConverter",
		"org.apache.kafka.connect.transforms.InsertField",
		"org.apache.kafka.connect.transforms.ReplaceField",
		"org.apache.kafka.connect.transforms.MaskField",
		"org.apache.kafka.connect.transforms.Filter",
		"org.apache.kafka.connect.transforms.Cast",
		"org.apache.kafka.connect.transforms.ExtractField",
		"org.apache.kafka.connect.transforms.Flatten",
		"io.debezium.transforms.ExtractNewRecordState",
	}

	for _, known := range knownTransforms {
		if class == known {
			return true
		}
	}
	return false
}

func isPartialTransform(class string) bool {
	partialTransforms := []string{
		"io.debezium.transforms.ByLogicalTableRouter",
	}

	for _, partial := range partialTransforms {
		if class == partial {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func NewAnalyzer(configDir string) (*Analyzer, error) {
	if strings.TrimSpace(configDir) == "" {
		return nil, fmt.Errorf("config directory cannot be empty")
	}

	// Validate directory exists and is accessible
	if _, err := os.Stat(configDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config directory does not exist: %s", configDir)
		}
		return nil, fmt.Errorf("cannot access config directory %s: %w", configDir, err)
	}

	return &Analyzer{
		configDir: configDir,
	}, nil
}
