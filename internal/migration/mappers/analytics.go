package mappers

import (
	"fmt"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// BigQueryMapper handles Google BigQuery connector mapping
type BigQueryMapper struct {
	BaseMapper
}

// NewBigQueryMapper creates a new BigQuery mapper
func NewBigQueryMapper() *BigQueryMapper {
	return &BigQueryMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts BigQuery configuration
func (m *BigQueryMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	// BigQuery is typically only a destination
	pipeline, err := m.CreateBasePipeline(config, "destination", "builtin:bigquery")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Project ID
	sb.Required("project.id", GetConfigValue(config, "project"), "project")

	// Dataset
	dataset := GetConfigValue(config, "datasets")
	if dataset == "" {
		dataset = GetConfigValue(config, "defaultDataset")
	}
	sb.Required("dataset", dataset, "dataset")

	// Credentials
	if keyfile := GetConfigValue(config, "keyfile"); keyfile != "" {
		sb.Optional("credentials.path", keyfile)
	} else if credentials := GetConfigValue(config, "credentials"); credentials != "" {
		sb.Sensitive("credentials.json", credentials, "")
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Table configuration
	if tableFormat := GetConfigValue(config, "table"); tableFormat != "" {
		settings["table"] = tableFormat
	} else if topics := GetConfigValue(config, "topics"); topics != "" {
		// Use topic name as table name
		topicList := strings.Split(topics, ",")
		if len(topicList) > 0 {
			settings["table"] = strings.TrimSpace(topicList[0])
		}
	}

	// Auto-create tables
	if autoCreate := GetConfigValue(config, "autoCreateTables"); autoCreate == "true" {
		settings["auto.create.tables"] = true
	}

	// Partitioning
	if partitioning := GetConfigValue(config, "timePartitioning"); partitioning != "" {
		settings["time.partitioning"] = partitioning
	}

	// Clustering
	if clustering := GetConfigValue(config, "clusteredFields"); clustering != "" {
		settings["clustered.fields"] = strings.Split(clustering, ",")
	}

	// Write disposition
	if writeDisposition := GetConfigValue(config, "writeDisposition"); writeDisposition != "" {
		settings["write.disposition"] = writeDisposition
	}

	// Batch settings
	if batchSize := GetConfigValue(config, "batchSize"); batchSize != "" {
		settings["batch.size"] = batchSize
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// SnowflakeMapper handles Snowflake connector mapping
type SnowflakeMapper struct {
	BaseMapper
}

// NewSnowflakeMapper creates a new Snowflake mapper
func NewSnowflakeMapper() *SnowflakeMapper {
	return &SnowflakeMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Snowflake configuration
func (m *SnowflakeMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, "destination", "builtin:snowflake")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Connection URL or components
	if url := GetConfigValue(config, "snowflake.url.name"); url != "" {
		sb.Required("account", url, "snowflake.url.name")
	} else {
		sb.Required("account", GetConfigValue(config, "snowflake.account"), "snowflake.account")
	}

	// Authentication
	sb.Required("user", GetConfigValue(config, "snowflake.user.name"), "snowflake.user.name")
	sb.Sensitive("password", GetConfigValue(config, "snowflake.password"), "snowflake.password")

	// Database and schema
	sb.Required("database", GetConfigValue(config, "snowflake.database.name"), "snowflake.database.name")
	sb.Required("schema", GetConfigValue(config, "snowflake.schema.name"), "snowflake.schema.name")

	// Warehouse
	sb.Optional("warehouse", GetConfigValue(config, "snowflake.warehouse"))

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Table name
	if table := GetConfigValue(config, "snowflake.table.name"); table != "" {
		settings["table"] = table
	}

	// Role
	if role := GetConfigValue(config, "snowflake.role.name"); role != "" {
		settings["role"] = role
	}

	// Private key authentication
	if privateKey := GetConfigValue(config, "snowflake.private.key"); privateKey != "" {
		settings["private.key"] = MaskSensitiveValue("private.key", privateKey)
		if passphrase := GetConfigValue(config, "snowflake.private.key.passphrase"); passphrase != "" {
			settings["private.key.passphrase"] = MaskSensitiveValue("private.key.passphrase", passphrase)
		}
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// RedshiftMapper handles Amazon Redshift connector mapping
type RedshiftMapper struct {
	BaseMapper
}

// NewRedshiftMapper creates a new Redshift mapper
func NewRedshiftMapper() *RedshiftMapper {
	return &RedshiftMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Redshift configuration
func (m *RedshiftMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, "destination", "builtin:redshift")
	if err != nil {
		return nil, err
	}

	// Redshift uses JDBC-like connection
	jdbcURL := GetConfigValue(config, "connection.url")
	if jdbcURL == "" {
		return nil, fmt.Errorf("connection.url is required for Redshift")
	}

	// Parse JDBC URL
	parsed, err := ParseJDBCURL(jdbcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redshift JDBC URL: %w", err)
	}

	sb := NewSettingsBuilder()

	// Connection settings
	if host, ok := parsed["host"].(string); ok {
		sb.Required("host", host, "host")
	}
	sb.WithDefault("port", parsed["port"].(string), "5439")

	if database, ok := parsed["database"].(string); ok {
		sb.Required("database", database, "database")
	}

	// Authentication
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

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// S3 staging (required for bulk loading)
	if s3Bucket := GetConfigValue(config, "s3.bucket.name"); s3Bucket != "" {
		settings["s3.bucket"] = s3Bucket
	}
	if s3Region := GetConfigValue(config, "s3.region"); s3Region != "" {
		settings["s3.region"] = s3Region
	}

	// IAM role for S3 access
	if iamRole := GetConfigValue(config, "aws.redshift.iam.role"); iamRole != "" {
		settings["iam.role"] = iamRole
	}

	// Table configuration
	if table := GetConfigValue(config, "table.name"); table != "" {
		settings["table"] = table
	}

	// Copy options
	if copyOptions := GetConfigValue(config, "redshift.copy.options"); copyOptions != "" {
		settings["copy.options"] = copyOptions
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// ClickHouseMapper handles ClickHouse connector mapping
type ClickHouseMapper struct {
	BaseMapper
}

// NewClickHouseMapper creates a new ClickHouse mapper
func NewClickHouseMapper() *ClickHouseMapper {
	return &ClickHouseMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts ClickHouse configuration
func (m *ClickHouseMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, "destination", "builtin:clickhouse")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Connection settings
	sb.Required("host", GetConfigValue(config, "hostname"), "hostname")
	sb.WithDefault("port", GetConfigValue(config, "port"), "8123")
	sb.Required("database", GetConfigValue(config, "database"), "database")

	// Authentication
	if user := GetConfigValue(config, "username"); user != "" {
		sb.Optional("username", user)
		sb.Sensitive("password", GetConfigValue(config, "password"), "")
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Table configuration
	if table := GetConfigValue(config, "table"); table != "" {
		settings["table"] = table
	}

	// SSL/TLS
	if ssl := GetConfigValue(config, "ssl"); ssl == "true" {
		settings["ssl"] = true
		if sslMode := GetConfigValue(config, "sslmode"); sslMode != "" {
			settings["sslmode"] = sslMode
		}
	}

	// Engine configuration
	if engine := GetConfigValue(config, "clickhouse.engine"); engine != "" {
		settings["engine"] = engine
	}

	// Batch settings
	if batchSize := GetConfigValue(config, "batch.size"); batchSize != "" {
		settings["batch.size"] = batchSize
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// SplunkMapper handles Splunk connector mapping
type SplunkMapper struct {
	BaseMapper
}

// NewSplunkMapper creates a new Splunk mapper
func NewSplunkMapper() *SplunkMapper {
	return &SplunkMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Splunk configuration
func (m *SplunkMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	// Splunk typically uses HTTP Event Collector (HEC)
	// Map to HTTP destination with Splunk-specific settings
	pipeline, err := m.CreateBasePipeline(config, "destination", "builtin:http")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// HEC endpoint
	hecURI := GetConfigValue(config, "splunk.hec.uri")
	if hecURI == "" {
		return nil, fmt.Errorf("splunk.hec.uri is required")
	}
	sb.Required("url", hecURI, "splunk.hec.uri")

	// Always POST for HEC
	sb.Optional("method", "POST")

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// HEC token
	if hecToken := GetConfigValue(config, "splunk.hec.token"); hecToken != "" {
		headers := map[string]string{
			"Authorization": fmt.Sprintf("Splunk %s", MaskSensitiveValue("hec.token", hecToken)),
		}
		settings["headers"] = headers
	}

	// Index and source type
	bodyTemplate := map[string]interface{}{
		"event": "{{.Payload}}",
	}

	if index := GetConfigValue(config, "splunk.indexes"); index != "" {
		bodyTemplate["index"] = index
	}

	if sourceType := GetConfigValue(config, "splunk.sourcetype"); sourceType != "" {
		bodyTemplate["sourcetype"] = sourceType
	}

	if source := GetConfigValue(config, "splunk.source"); source != "" {
		bodyTemplate["source"] = source
	}

	settings["body.template"] = bodyTemplate

	// SSL verification
	if ackEnabled := GetConfigValue(config, "splunk.hec.ack.enabled"); ackEnabled == "false" {
		settings["tls.skip.verify"] = true
	}

	// Batch settings
	if batchSize := GetConfigValue(config, "batch.size"); batchSize != "" {
		settings["batch.size"] = batchSize
	}

	// Retry configuration
	if maxRetries := GetConfigValue(config, "splunk.hec.max.retries"); maxRetries != "" {
		settings["retry.max"] = maxRetries
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}
