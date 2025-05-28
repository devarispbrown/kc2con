package mappers

import (
	"fmt"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// Connector type constants
const (
	ConnectorTypeSource      = "source"
	ConnectorTypeDestination = "destination"
)

// MapperFactory creates the appropriate mapper for a connector
type MapperFactory struct {
	mappers map[string]ConnectorMapper
}

// NewMapperFactory creates a new mapper factory
func NewMapperFactory() *MapperFactory {
	f := &MapperFactory{
		mappers: make(map[string]ConnectorMapper),
	}
	f.registerMappers()
	return f
}

// registerMappers registers all available mappers
func (f *MapperFactory) registerMappers() {
	// Debezium CDC mappers
	f.mappers["io.debezium.connector.mysql.MySqlConnector"] = NewDebeziumMySQLMapper()
	f.mappers["io.debezium.connector.postgresql.PostgresConnector"] = NewDebeziumPostgresMapper()
	f.mappers["io.debezium.connector.sqlserver.SqlServerConnector"] = NewDebeziumSQLServerMapper()
	f.mappers["io.debezium.connector.mongodb.MongoDbConnector"] = NewDebeziumMongoDBMapper()

	// JDBC mappers
	f.mappers["io.confluent.connect.jdbc.JdbcSourceConnector"] = NewJDBCSourceMapper()
	f.mappers["io.confluent.connect.jdbc.JdbcSinkConnector"] = NewJDBCSinkMapper()

	// Storage mappers
	f.mappers["io.confluent.connect.s3.S3SinkConnector"] = &s3MapperWrapper{&S3Mapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["io.confluent.connect.gcs.GcsSinkConnector"] = &gcsMapperWrapper{&GCSMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["io.confluent.connect.azure.blob.AzureBlobSinkConnector"] = &azureBlobMapperWrapper{&AzureBlobMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["org.apache.kafka.connect.file.FileStreamSourceConnector"] = &fileMapperWrapper{&FileMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["org.apache.kafka.connect.file.FileStreamSinkConnector"] = &fileMapperWrapper{&FileMapper{BaseMapper: *NewBaseMapper()}}

	// NoSQL mappers
	f.mappers["com.mongodb.kafka.connect.MongoSourceConnector"] = &mongoDBMapperWrapper{&MongoDBMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["com.mongodb.kafka.connect.MongoSinkConnector"] = &mongoDBMapperWrapper{&MongoDBMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["io.confluent.connect.elasticsearch.ElasticsearchSinkConnector"] = NewElasticsearchMapper()
	f.mappers["com.datastax.oss.kafka.sink.CassandraSinkConnector"] = &cassandraMapperWrapper{&CassandraMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["com.github.jcustenborder.kafka.connect.redis.RedisSinkConnector"] = &redisMapperWrapper{&RedisMapper{BaseMapper: *NewBaseMapper()}}

	// Messaging mappers
	f.mappers["org.apache.kafka.connect.mirror.MirrorSourceConnector"] = &kafkaMapperWrapper{&KafkaMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["org.apache.kafka.connect.mirror.MirrorCheckpointConnector"] = &kafkaMapperWrapper{&KafkaMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["org.apache.kafka.connect.mirror.MirrorHeartbeatConnector"] = &kafkaMapperWrapper{&KafkaMapper{BaseMapper: *NewBaseMapper()}}
	f.mappers["io.confluent.connect.http.HttpSinkConnector"] = &httpMapperWrapper{NewHTTPMapper()}

	// Analytics mappers
	f.mappers["com.wepay.kafka.connect.bigquery.BigQuerySinkConnector"] = NewBigQueryMapper()
	f.mappers["com.snowflake.kafka.connector.SnowflakeSinkConnector"] = NewSnowflakeMapper()
	f.mappers["io.confluent.connect.aws.redshift.RedshiftSinkConnector"] = NewRedshiftMapper()
	f.mappers["com.clickhouse.kafka.connect.ClickHouseSinkConnector"] = NewClickHouseMapper()
	f.mappers["com.splunk.kafka.connect.SplunkSinkConnector"] = NewSplunkMapper()
}

// GetMapper returns the appropriate mapper for a connector
func (f *MapperFactory) GetMapper(config *parser.ConnectorConfig, info registry.ConnectorInfo) (ConnectorMapper, error) {
	// First try exact match
	if mapper, ok := f.mappers[config.Class]; ok {
		return mapper, nil
	}

	// Try pattern matching for variants
	classLower := strings.ToLower(config.Class)

	// Check for known patterns
	patterns := map[string][]string{
		"debezium": {"mysql", "postgres", "sqlserver", "mongodb", "oracle"},
		"jdbc":     {"source", "sink"},
		"s3":       {"sink"},
		"kafka":    {"mirror", "source", "sink"},
		"mongodb":  {"source", "sink"},
		"http":     {"sink"},
		"file":     {"stream"},
	}

	for pattern, variants := range patterns {
		if strings.Contains(classLower, pattern) {
			for _, variant := range variants {
				if strings.Contains(classLower, variant) {
					// Find the appropriate mapper
					for class, mapper := range f.mappers {
						if strings.Contains(strings.ToLower(class), pattern) &&
							strings.Contains(strings.ToLower(class), variant) {
							return mapper, nil
						}
					}
				}
			}
		}
	}

	// Return base mapper as fallback
	return &baseMapperWrapper{NewBaseMapper()}, nil
}

// MapConnector performs the actual mapping
func (f *MapperFactory) MapConnector(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	mapper, err := f.GetMapper(config, info)
	if err != nil {
		return nil, err
	}

	// All mappers now implement the standard ConnectorMapper interface
	return mapper.Map(config, info)
}

// Wrapper types to adapt mappers with 3-parameter Map methods to the ConnectorMapper interface
type s3MapperWrapper struct{ *S3Mapper }

func (w *s3MapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.S3Mapper.Map(config, info, determineConnectorType(config.Class))
}

type fileMapperWrapper struct{ *FileMapper }

func (w *fileMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.FileMapper.Map(config, info, determineConnectorType(config.Class))
}

type mongoDBMapperWrapper struct{ *MongoDBMapper }

func (w *mongoDBMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.MongoDBMapper.Map(config, info, determineConnectorType(config.Class))
}

type kafkaMapperWrapper struct{ *KafkaMapper }

func (w *kafkaMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.KafkaMapper.Map(config, info, determineConnectorType(config.Class))
}

type httpMapperWrapper struct{ *HTTPMapper }

func (w *httpMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.HTTPMapper.Map(config, info, determineConnectorType(config.Class))
}

type redisMapperWrapper struct{ *RedisMapper }

func (w *redisMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.RedisMapper.Map(config, info, determineConnectorType(config.Class))
}

type cassandraMapperWrapper struct{ *CassandraMapper }

func (w *cassandraMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.CassandraMapper.Map(config, info, determineConnectorType(config.Class))
}

type gcsMapperWrapper struct{ *GCSMapper }

func (w *gcsMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.GCSMapper.Map(config, info, determineConnectorType(config.Class))
}

type azureBlobMapperWrapper struct{ *AzureBlobMapper }

func (w *azureBlobMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	return w.AzureBlobMapper.Map(config, info, determineConnectorType(config.Class))
}

type baseMapperWrapper struct{ *BaseMapper }

func (w *baseMapperWrapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	// Create a basic pipeline using the BaseMapper's functionality
	pipeline := &ConduitPipeline{
		Version: "2.11.0",
		Pipelines: []Pipeline{
			{
				ID:         config.Name,
				Status:     "running",
				Name:       config.Name,
				Connectors: []string{config.Name},
			},
		},
	}
	return pipeline, nil
}

// determineConnectorType infers whether a connector is source or destination
func determineConnectorType(class string) string {
	classLower := strings.ToLower(class)

	// Explicit indicators
	if strings.Contains(classLower, "source") {
		return "source"
	}
	if strings.Contains(classLower, "sink") {
		return ConnectorTypeDestination
	}

	// Known source patterns
	sourcePatterns := []string{
		"debezium",
		"cdc",
		"reader",
		"consumer",
		"poll",
	}

	for _, pattern := range sourcePatterns {
		if strings.Contains(classLower, pattern) {
			return "source"
		}
	}

	// Known destination patterns
	destPatterns := []string{
		"writer",
		"producer",
		"exporter",
		"publisher",
	}

	for _, pattern := range destPatterns {
		if strings.Contains(classLower, pattern) {
			return "destination"
		}
	}

	// Default to source if unclear
	return ConnectorTypeSource
}

// ValidateConnectorSettings validates settings against known capabilities
func ValidateConnectorSettings(plugin string, settings map[string]interface{}) []error {
	capabilities, ok := KnownConnectorCapabilities[plugin]
	if !ok {
		// Unknown connector, can't validate
		return nil
	}

	var errors []error

	// Check required settings
	for _, required := range capabilities.RequiredSettings {
		if _, ok := settings[required]; !ok {
			errors = append(errors, fmt.Errorf("required setting missing: %s", required))
		}
	}

	// Warn about unknown settings
	knownSettings := make(map[string]bool)
	for _, s := range capabilities.RequiredSettings {
		knownSettings[s] = true
	}
	for _, s := range capabilities.OptionalSettings {
		knownSettings[s] = true
	}

	for key := range settings {
		if !knownSettings[key] && !strings.HasPrefix(key, "_") {
			// Settings starting with _ are internal/metadata
			errors = append(errors, fmt.Errorf("unknown setting for %s: %s", plugin, key))
		}
	}

	return errors
}
