package registry

import (
	"github.com/devarispbrown/kc2con/internal/parser"
)

// TransformAnalyzerFunc is a function type for analyzing transforms
type TransformAnalyzerFunc func(transform parser.TransformConfig) []Issue

// analyzeConnectorCommon is a helper function that contains the common code for analyzing connectors
// It takes a transform analyzer function to handle the only part that varies between implementations
func analyzeConnectorCommon(config *parser.ConnectorConfig, lookup func(string) (ConnectorInfo, bool), transformAnalyzer TransformAnalyzerFunc) ConnectorAnalysis {
	analysis := ConnectorAnalysis{
		ConnectorName:  config.Name,
		ConnectorClass: config.Class,
		Issues:         []Issue{},
	}

	// Look up connector in registry
	connectorInfo, found := lookup(config.Class)
	if !found {
		// Unknown connector - treat as custom
		connectorInfo = ConnectorInfo{
			Name:              "Unknown Connector",
			KafkaConnectClass: config.Class,
			ConduitEquivalent: "manual-implementation-required",
			Status:            StatusManual,
			Notes:             "Connector not found in registry. Manual implementation required.",
			EstimatedEffort:   "2-5 days",
		}

		analysis.Issues = append(analysis.Issues, Issue{
			Type:       "warning",
			Field:      "connector.class",
			Message:    "Unknown connector class - not in compatibility registry",
			Suggestion: "Check if this is a custom connector that needs manual implementation",
		})
	}

	analysis.ConnectorInfo = connectorInfo

	// Validate required fields
	for _, requiredField := range connectorInfo.RequiredFields {
		if !hasField(config, requiredField) {
			analysis.Issues = append(analysis.Issues, Issue{
				Type:        "error",
				Field:       requiredField,
				Message:     "Required field missing for migration",
				Suggestion:  "Add this field to your connector configuration",
				AutoFixable: false,
			})
		}
	}

	// Check for unsupported features
	for _, unsupportedFeature := range connectorInfo.UnsupportedFeatures {
		if hasUnsupportedFeature(config, unsupportedFeature) {
			analysis.Issues = append(analysis.Issues, Issue{
				Type:        "warning",
				Field:       extractFieldFromFeature(unsupportedFeature),
				Message:     "Unsupported feature detected: " + unsupportedFeature,
				Suggestion:  "This feature may need manual configuration in Conduit",
				AutoFixable: false,
			})
		}
	}

	// Analyze transforms using the provided analyzer function
	for _, transform := range config.Transforms {
		transformIssues := transformAnalyzer(transform)
		analysis.Issues = append(analysis.Issues, transformIssues...)
	}

	return analysis
}
