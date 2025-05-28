package mappers

import (
	"fmt"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// JDBC related constants
const (
	// JDBC insert modes
	InsertModeUpsert = "upsert"
	InsertModeUpdate = "update"
	
	// Database types
	DatabaseMySQL      = "mysql"
	DatabaseSQLServer  = "sqlserver"
	DatabaseOracle     = "oracle"
	DatabaseSQLite     = "sqlite"
	
	// JDBC capture modes
	CaptureModeSnapshot = "snapshot"
	CaptureModeTimestamp = "timestamp"
)


// JDBCSourceMapper handles JDBC source connector mapping
type JDBCSourceMapper struct {
	BaseMapper
}

// NewJDBCSourceMapper creates a new JDBC source mapper
func NewJDBCSourceMapper() *JDBCSourceMapper {
	return &JDBCSourceMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts JDBC source configuration
func (m *JDBCSourceMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	// Parse JDBC URL to determine database type
	connURL := GetConfigValue(config, "connection.url")
	if connURL == "" {
		return nil, fmt.Errorf("connection.url is required")
	}

	dbType, settings, err := m.parseJDBCConnection(connURL, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JDBC URL: %w", err)
	}

	// Create pipeline with appropriate plugin
	plugin := fmt.Sprintf("builtin:%s", dbType)
	pipeline, err := m.CreateBasePipeline(config, "source", plugin)
	if err != nil {
		return nil, err
	}

	// Add query/table configuration
	if query := GetConfigValue(config, "query"); query != "" {
		settings["query"] = query
	} else if table := GetConfigValue(config, "table.name"); table != "" {
		settings["table"] = table
	} else {
		// Try whitelist/blacklist
		if whitelist := GetConfigValue(config, "table.whitelist"); whitelist != "" {
			settings["tables"] = whitelist
		} else if blacklist := GetConfigValue(config, "table.blacklist"); blacklist != "" {
			settings["tables.exclude"] = blacklist
		}
	}

	// Map mode and related settings
	mode := GetConfigValue(config, "mode")
	switch mode {
	case "incrementing":
		settings["cdc.mode"] = "auto_incrementing"
		if col := GetConfigValue(config, "incrementing.column.name"); col != "" {
			settings["incrementing.column"] = col
		}
	case CaptureModeTimestamp:
		settings["cdc.mode"] = "timestamp"
		if col := GetConfigValue(config, "timestamp.column.name"); col != "" {
			settings["timestamp.column"] = col
		}
	case "timestamp+incrementing":
		settings["cdc.mode"] = "timestamp_incrementing"
		if col := GetConfigValue(config, "incrementing.column.name"); col != "" {
			settings["incrementing.column"] = col
		}
		if col := GetConfigValue(config, "timestamp.column.name"); col != "" {
			settings["timestamp.column"] = col
		}
	case "bulk":
	settings["cdc.mode"] = CaptureModeSnapshot
		settings["snapshot.mode"] = "initial_only"
	default:
		settings["cdc.mode"] = "snapshot"
	}

	// Poll interval
	if pollInterval := GetConfigValue(config, "poll.interval.ms"); pollInterval != "" {
		settings["poll.interval"] = pollInterval + "ms"
	}

	// Batch settings
	if batchSize := GetConfigValue(config, "batch.max.rows"); batchSize != "" {
		settings["batch.size"] = batchSize
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// JDBCSinkMapper handles JDBC sink connector mapping
type JDBCSinkMapper struct {
	BaseMapper
}

// NewJDBCSinkMapper creates a new JDBC sink mapper
func NewJDBCSinkMapper() *JDBCSinkMapper {
	return &JDBCSinkMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts JDBC sink configuration
func (m *JDBCSinkMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	// Parse JDBC URL
	connURL := GetConfigValue(config, "connection.url")
	if connURL == "" {
		return nil, fmt.Errorf("connection.url is required")
	}

	dbType, settings, err := m.parseJDBCConnection(connURL, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JDBC URL: %w", err)
	}

	// Create pipeline
	plugin := fmt.Sprintf("builtin:%s", dbType)
	pipeline, err := m.CreateBasePipeline(config, "destination", plugin)
	if err != nil {
		return nil, err
	}

	// Table configuration
	if tableFormat := GetConfigValue(config, "table.name.format"); tableFormat != "" {
		settings["table.format"] = tableFormat
	} else if topics := GetConfigValue(config, "topics"); topics != "" {
		// Use first topic as table name if no format specified
		topicList := strings.Split(topics, ",")
		if len(topicList) > 0 {
			settings["table"] = strings.TrimSpace(topicList[0])
		}
	}

	// Insert mode mapping
	insertMode := GetConfigValue(config, "insert.mode")
	switch insertMode {
	case "insert":
		settings["sdk.record.operation"] = "create"
	case InsertModeUpsert:
		settings["sdk.record.operation"] = "upsert"
	case InsertModeUpdate:
		settings["sdk.record.operation"] = "update"
	default:
		settings["sdk.record.operation"] = "upsert" // Safe default
	}

	// Primary key configuration
	if pkMode := GetConfigValue(config, "pk.mode"); pkMode != "" {
		settings["primary.key.mode"] = pkMode

		if pkFields := GetConfigValue(config, "pk.fields"); pkFields != "" {
			settings["primary.key.fields"] = strings.Split(pkFields, ",")
		}
	}

	// Batch configuration
	if batchSize := GetConfigValue(config, "batch.size"); batchSize != "" {
		settings["batch.size"] = batchSize
	}

	// Auto-create and auto-evolve
	if autoCreate := GetConfigValue(config, "auto.create"); autoCreate == "true" {
		settings["table.auto.create"] = true
	}

	if autoEvolve := GetConfigValue(config, "auto.evolve"); autoEvolve == "true" {
		settings["table.auto.evolve"] = true
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// parseJDBCConnection extracts database type and connection settings from JDBC URL
func (m *BaseMapper) parseJDBCConnection(jdbcURL string, config *parser.ConnectorConfig) (string, map[string]interface{}, error) {
	parsed, err := ParseJDBCURL(jdbcURL)
	if err != nil {
		return "", nil, err
	}

	// Determine database type
	protocol, _ := parsed["protocol"].(string)
	var dbType string
	switch protocol {
	case "postgresql":
		dbType = "postgres"
	case DatabaseMySQL, "mariadb":
		dbType = "mysql"
	case DatabaseSQLServer, "microsoft:sqlserver":
		dbType = "sqlserver"
	case DatabaseOracle:
		dbType = "oracle"
	case DatabaseSQLite:
		dbType = "sqlite"
	default:
		return "", nil, fmt.Errorf("unsupported JDBC database type: %s", protocol)
	}

	// Build settings
	sb := NewSettingsBuilder()

	// Add host/port/database
	if host, ok := parsed["host"].(string); ok {
		sb.Required("host", host, "host")
	}

	if port, ok := parsed["port"].(string); ok {
		sb.Optional("port", port)
	}

	if database, ok := parsed["database"].(string); ok {
		sb.Required("database", database, "database")
	}

	// Add authentication
	if user, ok := parsed["user"].(string); ok {
		sb.Required("user", user, "user")
	} else if user := GetConfigValue(config, "connection.user"); user != "" {
		sb.Required("user", user, "user")
	}

	if password, ok := parsed["password"].(string); ok {
		sb.Sensitive("password", password, "")
	} else if password := GetConfigValue(config, "connection.password"); password != "" {
		sb.Sensitive("password", password, "")
	}

	// Add SSL/TLS settings if present
	if sslMode, ok := parsed["sslmode"].(string); ok {
		sb.Optional("sslmode", sslMode)
	} else if sslMode, ok := parsed["ssl"].(string); ok {
		sb.Optional("ssl", sslMode)
	}

	settings, err := sb.Build()
	if err != nil {
		return "", nil, err
	}

	return dbType, settings, nil
}
