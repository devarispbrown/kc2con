package mappers

import (
	"fmt"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// MongoDB operation constants
const (
	MongoOpReplaceOne  = "replace_one"
	MongoTypeBSONObjID = "bson_objectid"
)

// MongoDBMapper handles MongoDB connector mapping
type MongoDBMapper struct {
	BaseMapper
}

// NewMongoDBMapper creates a new MongoDB mapper
func NewMongoDBMapper() *MongoDBMapper {
	return &MongoDBMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts MongoDB configuration
func (m *MongoDBMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:mongo")
	if err != nil {
		return nil, err
	}

	// MongoDB connection
	connString := GetConfigValue(config, "connection.uri")
	if connString == "" {
		connString = GetConfigValue(config, "mongodb.connection.string")
	}

	if connString == "" {
		return nil, fmt.Errorf("connection.uri or mongodb.connection.string is required")
	}

	settings := map[string]interface{}{
		"uri": connString,
	}

	// Database and collection
	if database := GetConfigValue(config, "database"); database != "" {
		settings["database"] = database
	}

	if collection := GetConfigValue(config, "collection"); collection != "" {
		settings["collection"] = collection
	}

	// Source-specific settings
	if connectorType == "source" {
		// Pipeline for filtering
		if pipeline := GetConfigValue(config, "pipeline"); pipeline != "" {
			settings["pipeline"] = pipeline
		}

		// Copy existing data
		if copyExisting := GetConfigValue(config, "copy.existing"); copyExisting == "true" {
			settings["snapshot"] = true
		}

		// Batch size
		if batchSize := GetConfigValue(config, "batch.size"); batchSize != "" {
			settings["batch.size"] = batchSize
		}

		// Full document for updates
		if fullDoc := GetConfigValue(config, "full.document"); fullDoc != "" {
			settings["full.document"] = fullDoc
		}
	}

	// Destination-specific settings
	if connectorType == "destination" {
		// Document ID strategy
		if docIDStrategy := GetConfigValue(config, "document.id.strategy"); docIDStrategy != "" {
			settings["document.id.strategy"] = mapDocumentIDStrategy(docIDStrategy)
		}

		// Write model strategy
		if writeModel := GetConfigValue(config, "writemodel.strategy"); writeModel != "" {
			settings["write.model"] = mapWriteModelStrategy(writeModel)
		}

		// Max batch size
		if maxBatchSize := GetConfigValue(config, "max.batch.size"); maxBatchSize != "" {
			settings["batch.size"] = maxBatchSize
		}

		// Upsert behavior
		if upsert := GetConfigValue(config, "mongodb.upsert"); upsert == "true" {
			settings["upsert"] = true
		}
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// ElasticsearchMapper handles Elasticsearch connector mapping
type ElasticsearchMapper struct {
	BaseMapper
}

// NewElasticsearchMapper creates a new Elasticsearch mapper
func NewElasticsearchMapper() *ElasticsearchMapper {
	return &ElasticsearchMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Elasticsearch configuration
func (m *ElasticsearchMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo) (*ConduitPipeline, error) {
	if info.Status == registry.StatusUnsupported {
		return nil, fmt.Errorf("elasticsearch connector is not supported for migration")
	}

	// Elasticsearch is typically only a destination
	pipeline, err := m.CreateBasePipeline(config, "destination", "builtin:elasticsearch")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Connection settings
	urls := GetConfigValue(config, "connection.url")
	if urls == "" {
		urls = GetConfigValue(config, "elasticsearch.hosts")
	}
	sb.Required("urls", urls, "connection.url")

	// Authentication
	username := GetConfigValue(config, "connection.username")
	if username == "" {
		username = GetConfigValue(config, "elasticsearch.username")
	}
	if username != "" {
		sb.Optional("username", username)

		password := GetConfigValue(config, "connection.password")
		if password == "" {
			password = GetConfigValue(config, "elasticsearch.password")
		}
		sb.Sensitive("password", password, "")
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Index configuration
	if indexName := GetConfigValue(config, "index"); indexName != "" {
		settings["index"] = indexName
	} else if topics := GetConfigValue(config, "topics"); topics != "" {
		// Use topic name as index name
		topicList := strings.Split(topics, ",")
		if len(topicList) > 0 {
			settings["index"] = strings.TrimSpace(topicList[0])
		}
	}

	// Type name (for older ES versions)
	if typeName := GetConfigValue(config, "type.name"); typeName != "" {
		settings["type"] = typeName
	}

	// Document ID
	if keyIgnore := GetConfigValue(config, "key.ignore"); keyIgnore == "false" {
		settings["use.document.id"] = true
	}

	// Behavior settings
	if behavior := GetConfigValue(config, "behavior.on.null.values"); behavior != "" {
		settings["behavior.on.null"] = behavior
	}

	if behavior := GetConfigValue(config, "behavior.on.malformed.documents"); behavior != "" {
		settings["behavior.on.malformed"] = behavior
	}

	// Bulk settings
	if batchSize := GetConfigValue(config, "batch.size"); batchSize != "" {
		settings["bulk.size"] = batchSize
	}

	if maxInFlight := GetConfigValue(config, "max.in.flight.requests"); maxInFlight != "" {
		settings["max.in.flight.requests"] = maxInFlight
	}

	// Flush settings
	if flushTimeout := GetConfigValue(config, "flush.timeout.ms"); flushTimeout != "" {
		settings["flush.timeout"] = flushTimeout + "ms"
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// CassandraMapper handles Cassandra connector mapping
type CassandraMapper struct {
	BaseMapper
}

// NewCassandraMapper creates a new Cassandra mapper
func NewCassandraMapper() *CassandraMapper {
	return &CassandraMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Cassandra configuration
func (m *CassandraMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:cassandra")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Contact points
	contactPoints := GetConfigValue(config, "cassandra.contact.points")
	if contactPoints == "" {
		contactPoints = GetConfigValue(config, "contact.points")
	}
	sb.Required("contact.points", contactPoints, "cassandra.contact.points")

	// Port
	sb.WithDefault("port", GetConfigValue(config, "cassandra.port"), "9042")

	// Keyspace
	keyspace := GetConfigValue(config, "cassandra.keyspace")
	if keyspace == "" {
		keyspace = GetConfigValue(config, "keyspace")
	}
	sb.Required("keyspace", keyspace, "cassandra.keyspace")

	// Authentication
	if username := GetConfigValue(config, "cassandra.username"); username != "" {
		sb.Optional("username", username)
		sb.Sensitive("password", GetConfigValue(config, "cassandra.password"), "")
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Table configuration
	if table := GetConfigValue(config, "cassandra.table"); table != "" {
		settings["table"] = table
	} else if table := GetConfigValue(config, "table"); table != "" {
		settings["table"] = table
	}

	// Consistency level
	if consistency := GetConfigValue(config, "cassandra.consistency.level"); consistency != "" {
		settings["consistency.level"] = consistency
	}

	// SSL/TLS
	if sslEnabled := GetConfigValue(config, "cassandra.ssl.enabled"); sslEnabled == "true" {
		settings["ssl.enabled"] = true

		if certPath := GetConfigValue(config, "cassandra.ssl.cert.path"); certPath != "" {
			settings["ssl.cert.path"] = certPath
		}
	}

	// Datacenter awareness
	if localDC := GetConfigValue(config, "cassandra.local.datacenter"); localDC != "" {
		settings["local.datacenter"] = localDC
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// DynamoDBMapper handles DynamoDB connector mapping
type DynamoDBMapper struct {
	BaseMapper
}

// NewDynamoDBMapper creates a new DynamoDB mapper
func NewDynamoDBMapper() *DynamoDBMapper {
	return &DynamoDBMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts DynamoDB configuration
func (m *DynamoDBMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:dynamodb")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// AWS region
	sb.Required("aws.region", GetConfigValue(config, "aws.region"), "aws.region")

	// Table name
	tableName := GetConfigValue(config, "dynamodb.table.name")
	if tableName == "" {
		tableName = GetConfigValue(config, "table.name")
	}
	sb.Required("table", tableName, "dynamodb.table.name")

	// AWS credentials
	if accessKey := GetConfigValue(config, "aws.access.key.id"); accessKey != "" {
		sb.Optional("aws.accessKeyId", accessKey)
	}
	if secretKey := GetConfigValue(config, "aws.secret.access.key"); secretKey != "" {
		sb.Sensitive("aws.secretAccessKey", secretKey, "")
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Endpoint override (for local development)
	if endpoint := GetConfigValue(config, "dynamodb.endpoint"); endpoint != "" {
		settings["endpoint"] = endpoint
	}

	// Read/Write capacity
	if readCapacity := GetConfigValue(config, "dynamodb.read.capacity.units"); readCapacity != "" {
		settings["read.capacity"] = readCapacity
	}
	if writeCapacity := GetConfigValue(config, "dynamodb.write.capacity.units"); writeCapacity != "" {
		settings["write.capacity"] = writeCapacity
	}

	// Batch settings
	if batchSize := GetConfigValue(config, "batch.size"); batchSize != "" {
		settings["batch.size"] = batchSize
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// Helper functions

func mapDocumentIDStrategy(strategy string) string {
	switch strategy {
	case "com.mongodb.kafka.connect.sink.processor.id.strategy.BsonOidStrategy":
		return MongoTypeBSONObjID
	case "com.mongodb.kafka.connect.sink.processor.id.strategy.UuidStrategy":
		return "uuid"
	case "com.mongodb.kafka.connect.sink.processor.id.strategy.ProvidedInKeyStrategy":
		return "provided_in_key"
	case "com.mongodb.kafka.connect.sink.processor.id.strategy.ProvidedInValueStrategy":
		return "provided_in_value"
	case "com.mongodb.kafka.connect.sink.processor.id.strategy.PartialKeyStrategy":
		return "partial_key"
	case "com.mongodb.kafka.connect.sink.processor.id.strategy.PartialValueStrategy":
		return "partial_value"
	default:
		return MongoTypeBSONObjID // Safe default
	}
}

func mapWriteModelStrategy(strategy string) string {
	switch {
	case strings.Contains(strategy, "ReplaceOneDefaultStrategy"):
		return MongoOpReplaceOne
	case strings.Contains(strategy, "ReplaceOneBusinessKeyStrategy"):
		return "replace_one_business_key"
	case strings.Contains(strategy, "DeleteOneDefaultStrategy"):
		return "delete_one"
	case strings.Contains(strategy, "UpdateOneTimestampsStrategy"):
		return "update_one_timestamps"
	case strings.Contains(strategy, "InsertOneDefaultStrategy"):
		return "insert_one"
	default:
		return MongoOpReplaceOne // Safe default
	}
}
