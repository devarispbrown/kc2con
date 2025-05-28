// internal/registry/registry.go
package registry

import (
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
)

// ConnectorInfo contains information about how a Kafka Connect connector maps to Conduit
type ConnectorInfo struct {
	Name                string              `json:"name"`
	KafkaConnectClass   string              `json:"kafka_connect_class"`
	ConduitEquivalent   string              `json:"conduit_equivalent"`
	Status              CompatibilityStatus `json:"status"`
	RequiredFields      []string            `json:"required_fields,omitempty"`
	UnsupportedFeatures []string            `json:"unsupported_features,omitempty"`
	ConfigMapper        ConfigMapperFunc    `json:"-"`
	Notes               string              `json:"notes,omitempty"`
	EstimatedEffort     string              `json:"estimated_effort"`
}

// CompatibilityStatus represents the migration compatibility level
type CompatibilityStatus string

const (
	StatusSupported   CompatibilityStatus = "supported"   // Direct 1:1 mapping available
	StatusPartial     CompatibilityStatus = "partial"     // Most features work, some manual config needed
	StatusManual      CompatibilityStatus = "manual"      // Requires custom implementation/manual work
	StatusUnsupported CompatibilityStatus = "unsupported" // No Conduit equivalent exists
)

// ConfigMapperFunc transforms Kafka Connect config to Conduit format
type ConfigMapperFunc func(*parser.ConnectorConfig) (*ConduitPipeline, []Issue)

// ConduitPipeline represents a Conduit pipeline configuration
type ConduitPipeline struct {
	Version   string     `yaml:"version"`
	Pipelines []Pipeline `yaml:"pipelines"`
}

// Pipeline represents a single Conduit pipeline
type Pipeline struct {
	ID          string                 `yaml:"id"`
	Status      string                 `yaml:"status,omitempty"`
	ConnectorID string                 `yaml:"connectorId,omitempty"`
	Source      Source                 `yaml:"source"`
	Destination Destination            `yaml:"destination,omitempty"`
	Processors  []Processor            `yaml:"processors,omitempty"`
	DLQ         map[string]interface{} `yaml:"dlq,omitempty"`
}

// Source represents a Conduit source connector
type Source struct {
	Type     string                 `yaml:"type"`
	Plugin   string                 `yaml:"plugin,omitempty"`
	Settings map[string]interface{} `yaml:"settings"`
}

// Destination represents a Conduit destination connector
type Destination struct {
	Type     string                 `yaml:"type"`
	Plugin   string                 `yaml:"plugin,omitempty"`
	Settings map[string]interface{} `yaml:"settings"`
}

// Processor represents a Conduit processor
type Processor struct {
	ID       string                 `yaml:"id"`
	Plugin   string                 `yaml:"plugin"`
	Settings map[string]interface{} `yaml:"settings,omitempty"`
}

// Issue represents a migration issue or warning
type Issue struct {
	Type        string `json:"type"`         // "error", "warning", "info"
	Field       string `json:"field"`        // The config field that has an issue
	Message     string `json:"message"`      // Human-readable description
	Suggestion  string `json:"suggestion"`   // How to fix it
	AutoFixable bool   `json:"auto_fixable"` // Can this be automatically fixed
}

// Registry holds the connector compatibility information
type Registry struct {
	connectors map[string]ConnectorInfo
}

// New creates a new connector registry with built-in connector mappings
func New() *Registry {
	registry := &Registry{
		connectors: make(map[string]ConnectorInfo),
	}

	// Populate with built-in connector mappings
	registry.registerBuiltinConnectors()

	return registry
}

// Lookup finds connector information by Kafka Connect class name
func (r *Registry) Lookup(kafkaConnectClass string) (ConnectorInfo, bool) {
	// Direct lookup first
	if info, exists := r.connectors[kafkaConnectClass]; exists {
		return info, true
	}

	// Fuzzy matching for slight variations
	for class, info := range r.connectors {
		if strings.Contains(kafkaConnectClass, class) || strings.Contains(class, kafkaConnectClass) {
			return info, true
		}
	}

	// No match found
	return ConnectorInfo{}, false
}

// GetAll returns all registered connector information
func (r *Registry) GetAll() map[string]ConnectorInfo {
	return r.connectors
}

// GetByStatus returns connectors filtered by compatibility status
func (r *Registry) GetByStatus(status CompatibilityStatus) []ConnectorInfo {
	var result []ConnectorInfo

	for _, info := range r.connectors {
		if info.Status == status {
			result = append(result, info)
		}
	}

	return result
}

// Register adds a new connector to the registry
func (r *Registry) Register(info ConnectorInfo) {
	r.connectors[info.KafkaConnectClass] = info
}

// registerBuiltinConnectors registers all the built-in connector mappings
func (r *Registry) registerBuiltinConnectors() {
	// Debezium MySQL CDC
	r.Register(ConnectorInfo{
		Name:              "MySQL CDC (Debezium)",
		KafkaConnectClass: "io.debezium.connector.mysql.MySqlConnector",
		ConduitEquivalent: "mysql-cdc",
		Status:            StatusSupported,
		RequiredFields: []string{
			"database.hostname", "database.port", "database.user",
			"database.password", "database.server.name",
		},
		UnsupportedFeatures: []string{
			"database.ssl.mode=VERIFY_IDENTITY",
			"signal.data.collection",
		},
		Notes:           "Full CDC support with snapshot and incremental sync. SSL configuration syntax differs.",
		EstimatedEffort: "30 minutes",
	})

	// Debezium PostgreSQL CDC
	r.Register(ConnectorInfo{
		Name:              "PostgreSQL CDC (Debezium)",
		KafkaConnectClass: "io.debezium.connector.postgresql.PostgresConnector",
		ConduitEquivalent: "postgres-cdc",
		Status:            StatusSupported,
		RequiredFields: []string{
			"database.hostname", "database.port", "database.user",
			"database.password", "database.dbname", "database.server.name",
		},
		UnsupportedFeatures: []string{
			"slot.drop.on.stop=false",
		},
		Notes:           "Logical replication support. Replication slot management differs from Kafka Connect.",
		EstimatedEffort: "30 minutes",
	})

	// Debezium SQL Server CDC
	r.Register(ConnectorInfo{
		Name:              "SQL Server CDC (Debezium)",
		KafkaConnectClass: "io.debezium.connector.sqlserver.SqlServerConnector",
		ConduitEquivalent: "sqlserver-cdc",
		Status:            StatusPartial,
		RequiredFields: []string{
			"database.hostname", "database.port", "database.user",
			"database.password", "database.names", "database.server.name",
		},
		UnsupportedFeatures: []string{
			"database.encrypt=true",
		},
		Notes:           "CDC support available. Some advanced SQL Server features may require manual configuration.",
		EstimatedEffort: "1-2 hours",
	})

	// JDBC Source
	r.Register(ConnectorInfo{
		Name:              "JDBC Source",
		KafkaConnectClass: "io.confluent.connect.jdbc.JdbcSourceConnector",
		ConduitEquivalent: "postgres/mysql/sqlite-source",
		Status:            StatusSupported,
		RequiredFields: []string{
			"connection.url", "mode",
		},
		UnsupportedFeatures: []string{
			"validate.non.null=false",
		},
		Notes:           "Supports most JDBC databases. Database-specific optimizations available.",
		EstimatedEffort: "45 minutes",
	})

	// JDBC Sink
	r.Register(ConnectorInfo{
		Name:              "JDBC Sink",
		KafkaConnectClass: "io.confluent.connect.jdbc.JdbcSinkConnector",
		ConduitEquivalent: "postgres/mysql/sqlite-destination",
		Status:            StatusSupported,
		RequiredFields: []string{
			"connection.url",
		},
		Notes:           "Full insert/upsert/delete support. Schema evolution supported.",
		EstimatedEffort: "30 minutes",
	})

	// S3 Sink
	r.Register(ConnectorInfo{
		Name:              "S3 Sink",
		KafkaConnectClass: "io.confluent.connect.s3.S3SinkConnector",
		ConduitEquivalent: "s3-destination",
		Status:            StatusSupported,
		RequiredFields: []string{
			"s3.bucket.name",
		},
		UnsupportedFeatures: []string{
			"storage.class=io.confluent.connect.s3.storage.S3Storage",
		},
		Notes:           "Supports JSON, Avro, and Parquet formats. Time-based partitioning available.",
		EstimatedEffort: "30 minutes",
	})

	// Elasticsearch Sink
	r.Register(ConnectorInfo{
		Name:              "Elasticsearch Sink",
		KafkaConnectClass: "io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
		ConduitEquivalent: "elasticsearch-destination",
		Status:            StatusSupported,
		RequiredFields: []string{
			"connection.url",
		},
		Notes:           "Full indexing support with dynamic mapping. Bulk operations supported.",
		EstimatedEffort: "30 minutes",
	})

	// MongoDB Source
	r.Register(ConnectorInfo{
		Name:              "MongoDB Source",
		KafkaConnectClass: "com.mongodb.kafka.connect.MongoSourceConnector",
		ConduitEquivalent: "mongo-source",
		Status:            StatusPartial,
		RequiredFields: []string{
			"connection.uri",
		},
		UnsupportedFeatures: []string{
			"copy.existing=true",
		},
		Notes:           "Change stream support available. Initial sync configuration differs.",
		EstimatedEffort: "1-2 hours",
	})

	// MongoDB Sink
	r.Register(ConnectorInfo{
		Name:              "MongoDB Sink",
		KafkaConnectClass: "com.mongodb.kafka.connect.MongoSinkConnector",
		ConduitEquivalent: "mongo-destination",
		Status:            StatusSupported,
		RequiredFields: []string{
			"connection.uri",
		},
		Notes:           "Document insert/update/delete operations supported.",
		EstimatedEffort: "30 minutes",
	})

	// HTTP Sink
	r.Register(ConnectorInfo{
		Name:              "HTTP Sink",
		KafkaConnectClass: "io.confluent.connect.http.HttpSinkConnector",
		ConduitEquivalent: "http-destination",
		Status:            StatusSupported,
		RequiredFields: []string{
			"http.api.url",
		},
		Notes:           "REST API integration with configurable headers and authentication.",
		EstimatedEffort: "30 minutes",
	})

	// File Stream Source
	r.Register(ConnectorInfo{
		Name:              "File Stream Source",
		KafkaConnectClass: "org.apache.kafka.connect.file.FileStreamSourceConnector",
		ConduitEquivalent: "file-source",
		Status:            StatusSupported,
		RequiredFields: []string{
			"file",
		},
		Notes:           "File tailing and batch processing supported.",
		EstimatedEffort: "15 minutes",
	})

	// File Stream Sink
	r.Register(ConnectorInfo{
		Name:              "File Stream Sink",
		KafkaConnectClass: "org.apache.kafka.connect.file.FileStreamSinkConnector",
		ConduitEquivalent: "file-destination",
		Status:            StatusSupported,
		RequiredFields: []string{
			"file",
		},
		Notes:           "File output with rotation and compression options.",
		EstimatedEffort: "15 minutes",
	})

	// Kafka Source (Mirror Maker)
	r.Register(ConnectorInfo{
		Name:              "Kafka Source (MirrorMaker)",
		KafkaConnectClass: "org.apache.kafka.connect.mirror.MirrorSourceConnector",
		ConduitEquivalent: "kafka-source",
		Status:            StatusPartial,
		RequiredFields: []string{
			"source.cluster.bootstrap.servers",
		},
		UnsupportedFeatures: []string{
			"sync.group.offsets.enabled=true",
		},
		Notes:           "Cross-cluster replication. Offset management and topic filtering differs.",
		EstimatedEffort: "2-3 hours",
	})

	// Redis Sink
	r.Register(ConnectorInfo{
		Name:              "Redis Sink",
		KafkaConnectClass: "com.github.jcustenborder.kafka.connect.redis.RedisSinkConnector",
		ConduitEquivalent: "redis-destination",
		Status:            StatusSupported,
		RequiredFields: []string{
			"redis.hosts",
		},
		Notes:           "Key-value operations with TTL support.",
		EstimatedEffort: "30 minutes",
	})

	// Splunk Sink
	r.Register(ConnectorInfo{
		Name:              "Splunk Sink",
		KafkaConnectClass: "com.splunk.kafka.connect.SplunkSinkConnector",
		ConduitEquivalent: "http-destination",
		Status:            StatusPartial,
		RequiredFields: []string{
			"splunk.hec.uri", "splunk.hec.token",
		},
		Notes:           "Use HTTP destination with Splunk HEC endpoint. Some Splunk-specific features need manual config.",
		EstimatedEffort: "1-2 hours",
	})
}

// AnalyzeConnector analyzes a Kafka Connect connector configuration
func (r *Registry) AnalyzeConnector(config *parser.ConnectorConfig) ConnectorAnalysis {
	analysis := ConnectorAnalysis{
		ConnectorName:  config.Name,
		ConnectorClass: config.Class,
		Issues:         []Issue{},
	}

	// Look up connector in registry
	connectorInfo, found := r.Lookup(config.Class)
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

	// Analyze transforms
	for _, transform := range config.Transforms {
		transformIssues := r.analyzeTransform(transform)
		analysis.Issues = append(analysis.Issues, transformIssues...)
	}

	return analysis
}

// ConnectorAnalysis represents the analysis result for a connector
type ConnectorAnalysis struct {
	ConnectorName  string        `json:"connector_name"`
	ConnectorClass string        `json:"connector_class"`
	ConnectorInfo  ConnectorInfo `json:"connector_info"`
	Issues         []Issue       `json:"issues"`
}

// analyzeTransform analyzes a Single Message Transform
func (r *Registry) analyzeTransform(transform parser.TransformConfig) []Issue {
	var issues []Issue

	// Known transform mappings
	transformMappings := map[string]string{
		"org.apache.kafka.connect.transforms.RegexRouter":        "supported",
		"org.apache.kafka.connect.transforms.TimestampConverter": "supported",
		"org.apache.kafka.connect.transforms.InsertField":        "supported",
		"org.apache.kafka.connect.transforms.ReplaceField":       "supported",
		"org.apache.kafka.connect.transforms.MaskField":          "supported",
		"org.apache.kafka.connect.transforms.Filter":             "supported",
		"org.apache.kafka.connect.transforms.Cast":               "supported",
		"org.apache.kafka.connect.transforms.ExtractField":       "supported",
		"org.apache.kafka.connect.transforms.Flatten":            "supported",
		"io.debezium.transforms.ExtractNewRecordState":           "supported",
		"io.debezium.transforms.ByLogicalTableRouter":            "partial",
	}

	status, known := transformMappings[transform.Class]
	if !known {
		issues = append(issues, Issue{
			Type:        "warning",
			Field:       "transforms." + transform.Name,
			Message:     "Unknown transform: " + transform.Class,
			Suggestion:  "This transform may need to be reimplemented as a Conduit processor",
			AutoFixable: false,
		})
	} else if status == "partial" {
		issues = append(issues, Issue{
			Type:        "warning",
			Field:       "transforms." + transform.Name,
			Message:     "Transform partially supported: " + transform.Class,
			Suggestion:  "Some configuration may need manual adjustment",
			AutoFixable: false,
		})
	}

	return issues
}

// Helper functions
func hasField(config *parser.ConnectorConfig, field string) bool {
	// Check in top-level fields
	switch field {
	case "name":
		return config.Name != ""
	case "connector.class", "class":
		return config.Class != ""
	case "tasks.max":
		return config.TasksMax > 0
	case "topics":
		return len(config.Topics) > 0
	case "topics.regex":
		return config.TopicsRegex != ""
	}

	// Check in config map
	if _, exists := config.Config[field]; exists {
		return true
	}

	// Check in raw config
	if _, exists := config.RawConfig[field]; exists {
		return true
	}

	return false
}

func hasUnsupportedFeature(config *parser.ConnectorConfig, feature string) bool {
	// Parse feature string like "database.ssl.mode=VERIFY_IDENTITY"
	parts := strings.Split(feature, "=")
	if len(parts) != 2 {
		return false
	}

	field, expectedValue := parts[0], parts[1]

	if value, exists := config.Config[field]; exists {
		if strValue, ok := value.(string); ok {
			return strValue == expectedValue
		}
	}

	if value, exists := config.RawConfig[field]; exists {
		if strValue, ok := value.(string); ok {
			return strValue == expectedValue
		}
	}

	return false
}

func extractFieldFromFeature(feature string) string {
	parts := strings.Split(feature, "=")
	if len(parts) > 0 {
		return parts[0]
	}
	return feature
}
