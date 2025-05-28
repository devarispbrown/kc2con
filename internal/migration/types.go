package migration

import (
	"time"

	"github.com/devarispbrown/kc2con/internal/migration/mappers"
)

// Re-export types from mappers package for backward compatibility
type (
	ConduitPipeline      = mappers.ConduitPipeline
	Pipeline             = mappers.Pipeline
	Connector            = mappers.Connector
	Processor            = mappers.Processor
	DLQ                  = mappers.DLQ
	MigrationMetrics     = mappers.MigrationMetrics
	SchemaRegistryConfig = mappers.SchemaRegistryConfig
)

// MigrationContext holds context information for a migration session
type MigrationContext struct {
	StartTime    time.Time
	EndTime      time.Time
	Metrics      *MigrationMetrics
	DryRun       bool
	Concurrent   int
	OutputDir    string
	RegistryPath string
}

// MigrationOptions configures migration behavior
type MigrationOptions struct {
	DryRun          bool
	Validate        bool
	Force           bool
	Concurrent      int
	OutputDir       string
	RegistryPath    string
	ConnectorFilter []string
}

// BatchMigrationResult contains results from batch migration
type BatchMigrationResult struct {
	Successful        []*MigrationResult
	Failed            []*FailedMigration
	Warnings          []string
	DeploymentScripts []DeploymentScript
	Metrics           *MigrationMetrics
}

// FailedMigration represents a failed connector migration
type FailedMigration struct {
	ConnectorName string
	SourcePath    string
	Error         error
	Timestamp     time.Time
}

// DeploymentScript represents a generated deployment script
type DeploymentScript struct {
	Name        string
	Type        string // bash, powershell, etc.
	Content     string
	Executable  bool
}

// MigrationReport generates a comprehensive migration report
type MigrationReport struct {
	GeneratedAt      time.Time                  `json:"generatedAt"`
	Summary          MigrationSummary           `json:"summary"`
	ConnectorDetails []ConnectorMigrationDetail `json:"connectorDetails"`
	Issues           []MigrationIssue           `json:"issues"`
	Recommendations  []string                   `json:"recommendations"`
}

// MigrationSummary provides high-level migration statistics
type MigrationSummary struct {
	TotalConnectors     int           `json:"totalConnectors"`
	Successful          int           `json:"successful"`
	Failed              int           `json:"failed"`
	WithWarnings        int           `json:"withWarnings"`
	Duration            time.Duration `json:"duration"`
	EstimatedEffort     string        `json:"estimatedEffort"`
	MigrationReadiness  float64       `json:"migrationReadiness"`
}

// ConnectorMigrationDetail provides detailed information about a connector migration
type ConnectorMigrationDetail struct {
	Name              string                 `json:"name"`
	Class             string                 `json:"class"`
	Type              string                 `json:"type"`
	Status            string                 `json:"status"`
	ConduitPlugin     string                 `json:"conduitPlugin"`
	Issues            []MigrationIssue       `json:"issues,omitempty"`
	Warnings          []string               `json:"warnings,omitempty"`
	Settings          map[string]interface{} `json:"settings,omitempty"`
	TransformCount    int                    `json:"transformCount"`
	EstimatedEffort   string                 `json:"estimatedEffort"`
}

// MigrationIssue represents an issue encountered during migration
type MigrationIssue struct {
	Severity    string    `json:"severity"` // error, warning, info
	Category    string    `json:"category"` // configuration, compatibility, security
	Field       string    `json:"field,omitempty"`
	Message     string    `json:"message"`
	Suggestion  string    `json:"suggestion,omitempty"`
	AutoFixable bool      `json:"autoFixable"`
	Timestamp   time.Time `json:"timestamp"`
}