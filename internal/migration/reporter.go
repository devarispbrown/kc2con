package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// Reporter generates migration reports
type Reporter struct {
	outputFormat string // json, yaml, markdown
}

// NewReporter creates a new migration reporter
func NewReporter(format string) *Reporter {
	return &Reporter{
		outputFormat: format,
	}
}

// GenerateReport creates a comprehensive migration report
func (r *Reporter) GenerateReport(result *BatchMigrationResult, options Options) (*Report, error) {
	report := &Report{
		GeneratedAt: time.Now(),
		Summary: Summary{
			TotalConnectors:    result.Metrics.TotalConfigs,
			Successful:         result.Metrics.Successful,
			Failed:             result.Metrics.Failed,
			WithWarnings:       result.Metrics.WithWarnings,
			Duration:           time.Duration(result.Metrics.Duration) * time.Second,
			MigrationReadiness: r.calculateReadiness(result.Metrics),
		},
		ConnectorDetails: []ConnectorMigrationDetail{},
		Issues:           []Issue{},
		Recommendations:  []string{},
	}

	// Add connector details
	for _, success := range result.Successful {
		detail := ConnectorMigrationDetail{
			Name:           success.Pipeline.Pipelines[0].Name,
			Status:         "successful",
			TransformCount: len(success.Pipeline.Processors),
			Warnings:       success.Warnings,
		}

		if len(success.Pipeline.Connectors) > 0 {
			detail.ConduitPlugin = success.Pipeline.Connectors[0].Plugin
			detail.Type = success.Pipeline.Connectors[0].Type
		}

		report.ConnectorDetails = append(report.ConnectorDetails, detail)

		// Add issues
		for _, issue := range success.Issues {
			report.Issues = append(report.Issues, Issue{
				Severity:   issue.Type,
				Category:   "configuration",
				Field:      issue.Field,
				Message:    issue.Message,
				Suggestion: issue.Suggestion,
				Timestamp:  time.Now(),
			})
		}
	}

	// Add failed connectors
	for _, failed := range result.Failed {
		detail := ConnectorMigrationDetail{
			Name:   failed.ConnectorName,
			Status: "failed",
		}
		report.ConnectorDetails = append(report.ConnectorDetails, detail)

		report.Issues = append(report.Issues, Issue{
			Severity:  "error",
			Category:  "migration",
			Message:   failed.Error.Error(),
			Timestamp: failed.Timestamp,
		})
	}

	// Generate recommendations
	report.Recommendations = r.generateRecommendations(result)

	return report, nil
}

// SaveReport saves the report to a file
func (r *Reporter) SaveReport(report *Report, outputPath string) error {
	var data []byte
	var err error

	switch r.outputFormat {
	case "json":
		data, err = json.MarshalIndent(report, "", "  ")
	case "yaml":
		data, err = yaml.Marshal(report)
	case "markdown":
		data = []byte(r.formatMarkdown(report))
	default:
		return fmt.Errorf("unsupported output format: %s", r.outputFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// PrintSummary prints a colorful summary to the console
func (r *Reporter) PrintSummary(report *Report) {
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

	fmt.Println()
	fmt.Println(headerStyle.Render("📊 Migration Report Summary"))
	fmt.Println("=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=" + "=")

	fmt.Printf("Generated: %s\n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Duration: %s\n", report.Summary.Duration)
	fmt.Println()

	// Results
	fmt.Printf("Total Connectors: %d\n", report.Summary.TotalConnectors)
	fmt.Printf("%s Successful: %d\n", successStyle.Render("✅"), report.Summary.Successful)
	if report.Summary.WithWarnings > 0 {
		fmt.Printf("%s With Warnings: %d\n", warningStyle.Render("⚠️"), report.Summary.WithWarnings)
	}
	if report.Summary.Failed > 0 {
		fmt.Printf("%s Failed: %d\n", errorStyle.Render("❌"), report.Summary.Failed)
	}
	fmt.Printf("Migration Readiness: %.1f%%\n", report.Summary.MigrationReadiness)

	// Issues summary
	if len(report.Issues) > 0 {
		fmt.Println()
		fmt.Println("Issues by Severity:")

		severityCounts := make(map[string]int)
		for _, issue := range report.Issues {
			severityCounts[issue.Severity]++
		}

		if errors := severityCounts["error"]; errors > 0 {
			fmt.Printf("  %s Errors: %d\n", errorStyle.Render("❌"), errors)
		}
		if warnings := severityCounts["warning"]; warnings > 0 {
			fmt.Printf("  %s Warnings: %d\n", warningStyle.Render("⚠️"), warnings)
		}
		if info := severityCounts["info"]; info > 0 {
			fmt.Printf("  ℹ️  Info: %d\n", info)
		}
	}

	// Recommendations
	if len(report.Recommendations) > 0 {
		fmt.Println()
		fmt.Println("Top Recommendations:")
		for i, rec := range report.Recommendations {
			if i >= 3 { // Show top 3
				break
			}
			fmt.Printf("  • %s\n", rec)
		}
	}
}

// Helper methods

func (r *Reporter) calculateReadiness(metrics *Metrics) float64 {
	if metrics.TotalConfigs == 0 {
		return 0
	}

	readiness := float64(metrics.Successful) / float64(metrics.TotalConfigs) * 100

	// Penalize for warnings
	if metrics.WithWarnings > 0 {
		warningPenalty := float64(metrics.WithWarnings) / float64(metrics.TotalConfigs) * 10
		readiness -= warningPenalty
	}

	if readiness < 0 {
		readiness = 0
	}

	return readiness
}

func (r *Reporter) generateRecommendations(result *BatchMigrationResult) []string {
	var recommendations []string

	// Check for common issues
	failureRate := float64(len(result.Failed)) / float64(result.Metrics.TotalConfigs)
	if failureRate > 0.2 {
		recommendations = append(recommendations,
			"High failure rate detected. Review connector compatibility and configuration.")
	}

	// Check for security issues
	hasCredentials := false
	for _, success := range result.Successful {
		for _, conn := range success.Pipeline.Connectors {
			for key := range conn.Settings {
				if strings.Contains(strings.ToLower(key), "password") ||
					strings.Contains(strings.ToLower(key), "secret") {
					hasCredentials = true
					break
				}
			}
		}
	}

	if hasCredentials {
		recommendations = append(recommendations,
			"Review and update masked credentials (${MASKED_*}) before deployment.")
	}

	// Check for manual work
	manualCount := 0
	for _, success := range result.Successful {
		for _, warning := range success.Warnings {
			if strings.Contains(warning, "manual") || strings.Contains(warning, "Manual") {
				manualCount++
				break
			}
		}
	}

	if manualCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("%d connectors require manual configuration. Review generated configs carefully.", manualCount))
	}

	// Testing recommendation
	if result.Metrics.Successful > 0 {
		recommendations = append(recommendations,
			"Test migrated pipelines in a development environment before production deployment.")
	}

	// Schema registry
	recommendations = append(recommendations,
		"If using Schema Registry, configure it in Conduit after migration.")

	return recommendations
}

func (r *Reporter) formatMarkdown(report *Report) string {
	var md strings.Builder

	md.WriteString("# Migration Report\n\n")
	md.WriteString(fmt.Sprintf("Generated: %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))

	md.WriteString("## Summary\n\n")
	md.WriteString(fmt.Sprintf("- **Total Connectors**: %d\n", report.Summary.TotalConnectors))
	md.WriteString(fmt.Sprintf("- **Successful**: %d\n", report.Summary.Successful))
	md.WriteString(fmt.Sprintf("- **Failed**: %d\n", report.Summary.Failed))
	md.WriteString(fmt.Sprintf("- **With Warnings**: %d\n", report.Summary.WithWarnings))
	md.WriteString(fmt.Sprintf("- **Duration**: %s\n", report.Summary.Duration))
	md.WriteString(fmt.Sprintf("- **Migration Readiness**: %.1f%%\n\n", report.Summary.MigrationReadiness))

	if len(report.ConnectorDetails) > 0 {
		md.WriteString("## Connector Details\n\n")
		md.WriteString("| Name | Type | Plugin | Status | Warnings |\n")
		md.WriteString("|------|------|--------|--------|----------|\n")

		for _, detail := range report.ConnectorDetails {
			warningCount := len(detail.Warnings)
			md.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d |\n",
				detail.Name, detail.Type, detail.ConduitPlugin, detail.Status, warningCount))
		}
		md.WriteString("\n")
	}

	if len(report.Issues) > 0 {
		md.WriteString("## Issues\n\n")

		// Group by severity
		bySeverity := make(map[string][]Issue)
		for _, issue := range report.Issues {
			bySeverity[issue.Severity] = append(bySeverity[issue.Severity], issue)
		}

		for _, severity := range []string{"error", "warning", "info"} {
			if issues, ok := bySeverity[severity]; ok && len(issues) > 0 {
				md.WriteString(fmt.Sprintf("### %s\n\n", strings.Title(severity)))
				for _, issue := range issues {
					md.WriteString(fmt.Sprintf("- %s", issue.Message))
					if issue.Suggestion != "" {
						md.WriteString(fmt.Sprintf(" (%s)", issue.Suggestion))
					}
					md.WriteString("\n")
				}
				md.WriteString("\n")
			}
		}
	}

	if len(report.Recommendations) > 0 {
		md.WriteString("## Recommendations\n\n")
		for _, rec := range report.Recommendations {
			md.WriteString(fmt.Sprintf("- %s\n", rec))
		}
	}

	return md.String()
}
