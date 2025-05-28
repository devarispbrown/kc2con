package mappers

import (
	"fmt"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// DebeziumMySQLMapper handles MySQL CDC connector mapping
type DebeziumMySQLMapper struct {
	BaseMapper
}

// NewDebeziumMySQLMapper creates a new MySQL mapper
func NewDebeziumMySQLMapper() *DebeziumMySQLMapper {
	return &DebeziumMySQLMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts MySQL CDC configuration
func (m *DebeziumMySQLMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, "source", "builtin:mysql")
	if err != nil {
		return nil, err
	}

	// Build settings using the builder pattern
	sb := NewSettingsBuilder()
	settings, err := sb.
		Required("host", GetConfigValue(config, "database.hostname"), "database.hostname").
		Required("user", GetConfigValue(config, "database.user"), "database.user").
		Required("database", GetConfigValue(config, "database.dbname"), "database.dbname").
		WithDefault("port", GetConfigValue(config, "database.port"), "3306").
		Sensitive("password", GetConfigValue(config, "database.password"), "").
		Optional("server.id", GetConfigValue(config, "database.server.id")).
		OptionalBool("cdc.enabled", true).
		Build()

	if err != nil {
		return nil, fmt.Errorf("MySQL configuration error: %w", err)
	}

	// Add table configuration
	if tableInclude := GetConfigValue(config, "table.include.list"); tableInclude != "" {
		settings["tables"] = tableInclude
	} else if tableWhitelist := GetConfigValue(config, "table.whitelist"); tableWhitelist != "" {
		settings["tables"] = tableWhitelist
	}

	// Map snapshot mode
	if snapshotMode := GetConfigValue(config, "snapshot.mode"); snapshotMode != "" {
		settings["snapshot.mode"] = mapSnapshotMode(snapshotMode)
	}

	// SSL configuration
	if sslMode := GetConfigValue(config, "database.ssl.mode"); sslMode != "" && sslMode != "disabled" {
		settings["ssl.mode"] = sslMode
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// DebeziumPostgresMapper handles PostgreSQL CDC connector mapping
type DebeziumPostgresMapper struct {
	BaseMapper
}

// NewDebeziumPostgresMapper creates a new PostgreSQL mapper
func NewDebeziumPostgresMapper() *DebeziumPostgresMapper {
	return &DebeziumPostgresMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts PostgreSQL CDC configuration
func (m *DebeziumPostgresMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, "source", "builtin:postgres")
	if err != nil {
		return nil, err
	}

	// Build settings
	sb := NewSettingsBuilder()
	settings, err := sb.
		Required("host", GetConfigValue(config, "database.hostname"), "database.hostname").
		Required("user", GetConfigValue(config, "database.user"), "database.user").
		Required("database", GetConfigValue(config, "database.dbname"), "database.dbname").
		WithDefault("port", GetConfigValue(config, "database.port"), "5432").
		Sensitive("password", GetConfigValue(config, "database.password"), "").
		WithDefault("sslmode", GetConfigValue(config, "database.sslmode"), "prefer").
		Build()

	if err != nil {
		return nil, fmt.Errorf("PostgreSQL configuration error: %w", err)
	}

	// Schema and table configuration
	if schemaInclude := GetConfigValue(config, "schema.include.list"); schemaInclude != "" {
		settings["schema"] = schemaInclude
	}
	if tableInclude := GetConfigValue(config, "table.include.list"); tableInclude != "" {
		settings["table"] = tableInclude
	}

	// CDC specific settings
	settings["cdc.mode"] = "logrepl"

	if slotName := GetConfigValue(config, "slot.name"); slotName != "" {
		settings["replication.slot.name"] = slotName
	}

	if publication := GetConfigValue(config, "publication.name"); publication != "" {
		settings["publication.name"] = publication
	}

	// Snapshot mode
	if snapshotMode := GetConfigValue(config, "snapshot.mode"); snapshotMode != "" {
		settings["snapshot.mode"] = mapSnapshotMode(snapshotMode)
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// DebeziumSQLServerMapper handles SQL Server CDC connector mapping
type DebeziumSQLServerMapper struct {
	BaseMapper
}

// NewDebeziumSQLServerMapper creates a new SQL Server mapper
func NewDebeziumSQLServerMapper() *DebeziumSQLServerMapper {
	return &DebeziumSQLServerMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts SQL Server CDC configuration
func (m *DebeziumSQLServerMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, "source", "builtin:sqlserver")
	if err != nil {
		return nil, err
	}

	// Build settings
	sb := NewSettingsBuilder()
	settings, err := sb.
		Required("host", GetConfigValue(config, "database.hostname"), "database.hostname").
		Required("user", GetConfigValue(config, "database.user"), "database.user").
		Required("database", GetConfigValue(config, "database.dbname"), "database.dbname").
		WithDefault("port", GetConfigValue(config, "database.port"), "1433").
		Sensitive("password", GetConfigValue(config, "database.password"), "").
		OptionalBool("cdc.enabled", true).
		Build()

	if err != nil {
		return nil, fmt.Errorf("SQL Server configuration error: %w", err)
	}

	// Database names (SQL Server can monitor multiple databases)
	if dbNames := GetConfigValue(config, "database.names"); dbNames != "" {
		settings["databases"] = dbNames
	}

	// Table configuration
	if tableInclude := GetConfigValue(config, "table.include.list"); tableInclude != "" {
		settings["tables"] = tableInclude
	}

	// SQL Server specific settings
	if encrypt := GetConfigValue(config, "database.encrypt"); encrypt == "true" {
		settings["encrypt"] = true
		settings["trustServerCertificate"] = GetConfigValue(config, "database.trustServerCertificate") == "true"
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// DebeziumMongoDBMapper handles MongoDB CDC connector mapping
type DebeziumMongoDBMapper struct {
	BaseMapper
}

// NewDebeziumMongoDBMapper creates a new MongoDB mapper
func NewDebeziumMongoDBMapper() *DebeziumMongoDBMapper {
	return &DebeziumMongoDBMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts MongoDB CDC configuration
func (m *DebeziumMongoDBMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, "source", "builtin:mongo")
	if err != nil {
		return nil, err
	}

	// MongoDB typically uses connection string
	connString := GetConfigValue(config, "mongodb.connection.string")
	if connString == "" {
		// Build from components
		hosts := GetConfigValue(config, "mongodb.hosts")
		if hosts == "" {
			return nil, fmt.Errorf("mongodb.connection.string or mongodb.hosts is required")
		}

		user := GetConfigValue(config, "mongodb.user")
		password := GetConfigValue(config, "mongodb.password")

		if user != "" && password != "" {
			connString = fmt.Sprintf("mongodb://%s:%s@%s", user, password, hosts)
		} else {
			connString = fmt.Sprintf("mongodb://%s", hosts)
		}

		// Add auth source if specified
		if authSource := GetConfigValue(config, "mongodb.authsource"); authSource != "" {
			connString += "/?authSource=" + authSource
		}
	}

	settings := map[string]interface{}{
		"uri": connString,
	}

	// Database and collection filters
	if dbInclude := GetConfigValue(config, "database.include.list"); dbInclude != "" {
		settings["database"] = dbInclude
	}

	if collInclude := GetConfigValue(config, "collection.include.list"); collInclude != "" {
		settings["collection"] = collInclude
	}

	// Change stream options
	settings["cdc.enabled"] = true

	if pipeline := GetConfigValue(config, "pipeline"); pipeline != "" {
		settings["pipeline"] = pipeline
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// mapSnapshotMode converts Kafka Connect snapshot modes to Conduit equivalents
func mapSnapshotMode(kafkaMode string) string {
	switch kafkaMode {
	case "initial":
		return "initial"
	case "never":
		return "never"
	case "initial_only":
		return "initial_only"
	case "when_needed":
		return "when_needed"
	case "always":
		return "always"
	default:
		return "initial" // Safe default
	}
}
