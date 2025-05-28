package mappers

import (
	"fmt"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// Data format constants
const (
	FormatAvro   = "avro"
	FormatString = "string"
	FormatBytes  = "bytes"
	FormatJSON   = "json"
)

// Connection and security related constants
const (
	bootstrapServers = "bootstrap.servers"
	securityProtocol = "security.protocol"
	saslMechanism    = "sasl.mechanism"
	saslJaasConfig   = "sasl.jaas.config"
)

// Connector type constants
const (
	typeSource      = "source"
	typeDestination = "destination"
)

// Configuration key constants
const (
	configTopics      = "topics"
	configTopicsRegex = "topics.regex"
	configGroupID     = "group.id"
)

// HTTP auth type constants
const (
	authTypeBasic  = "BASIC"
	authTypeOAuth2 = "OAUTH2"
	authTypeBearer = "BEARER"
)

// KafkaMapper handles Kafka connector mapping
type KafkaMapper struct {
	BaseMapper
}

// NewKafkaMapper creates a new Kafka mapper
func NewKafkaMapper() *KafkaMapper {
	return &KafkaMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Kafka configuration
func (m *KafkaMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:kafka")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Kafka brokers
	brokers := GetConfigValue(config, bootstrapServers)
	if brokers == "" {
		brokers = GetConfigValue(config, "source.cluster.bootstrap.servers")
	}
	if brokers == "" {
		brokers = GetConfigValue(config, "kafka.bootstrap.servers")
	}
	sb.Required("brokers", brokers, bootstrapServers)

	// Topics configuration
	topics := GetConfigValue(config, configTopics)
	topicsRegex := GetConfigValue(config, configTopicsRegex)

	if topics != "" {
		sb.Optional("topics", topics)
	} else if topicsRegex != "" {
		sb.Optional("topic.regex", topicsRegex)
	} else if connectorType == typeSource {
		return nil, fmt.Errorf("%s or %s is required for source connector", configTopics, configTopicsRegex)
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Consumer configuration (for source)
	if connectorType == typeSource {
		if groupID := GetConfigValue(config, configGroupID); groupID != "" {
			settings["consumer.group.id"] = groupID
		}

		if offset := GetConfigValue(config, "consumer.auto.offset.reset"); offset != "" {
			settings["consumer.offset.reset"] = offset
		}

		// Key/Value deserializers
		if keyDeserializer := GetConfigValue(config, "key.deserializer"); keyDeserializer != "" {
			settings["key.deserializer"] = mapKafkaDeserializer(keyDeserializer)
		}
		if valueDeserializer := GetConfigValue(config, "value.deserializer"); valueDeserializer != "" {
			settings["value.deserializer"] = mapKafkaDeserializer(valueDeserializer)
		}
	}

	// Producer configuration (for destination)
	if connectorType == typeDestination {
		if acks := GetConfigValue(config, "producer.acks"); acks != "" {
			settings["producer.acks"] = acks
		}

		if compression := GetConfigValue(config, "producer.compression.type"); compression != "" {
			settings["producer.compression"] = compression
		}

		// Key/Value serializers
		if keySerializer := GetConfigValue(config, "key.serializer"); keySerializer != "" {
			settings["key.serializer"] = mapKafkaSerializer(keySerializer)
		}
		if valueSerializer := GetConfigValue(config, "value.serializer"); valueSerializer != "" {
			settings["value.serializer"] = mapKafkaSerializer(valueSerializer)
		}
	}

	// Security configuration
	if protocol := GetConfigValue(config, securityProtocol); protocol != "" {
		settings[securityProtocol] = protocol

		// SASL configuration
		if strings.Contains(protocol, "SASL") {
			if mechanism := GetConfigValue(config, saslMechanism); mechanism != "" {
				settings[saslMechanism] = mechanism
			}

			if jaasConfig := GetConfigValue(config, saslJaasConfig); jaasConfig != "" {
				settings[saslJaasConfig] = jaasConfig
			}
		}

		// SSL configuration
		if strings.Contains(protocol, "SSL") {
			if truststore := GetConfigValue(config, "ssl.truststore.location"); truststore != "" {
				settings["ssl.truststore.location"] = truststore
			}
			if truststorePass := GetConfigValue(config, "ssl.truststore.password"); truststorePass != "" {
				settings["ssl.truststore.password"] = MaskSensitiveValue("ssl.truststore.password", truststorePass)
			}
		}
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// HTTPMapper handles HTTP connector mapping
type HTTPMapper struct {
	BaseMapper
}

// NewHTTPMapper creates a new HTTP mapper
func NewHTTPMapper() *HTTPMapper {
	return &HTTPMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts HTTP configuration
func (m *HTTPMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	if connectorType != typeDestination {
		return nil, fmt.Errorf("HTTP connector only supports %s type", typeDestination)
	}

	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:http")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// URL configuration
	url := GetConfigValue(config, "http.api.url")
	if url == "" {
		url = GetConfigValue(config, "url")
	}
	sb.Required("url", url, "http.api.url")

	// HTTP method
	sb.WithDefault("method", GetConfigValue(config, "request.method"), "POST")

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Headers
	headers := make(map[string]string)
	for key, value := range config.Config {
		if strings.HasPrefix(key, "headers.") {
			headerName := strings.TrimPrefix(key, "headers.")
			if strVal, ok := value.(string); ok {
				headers[headerName] = strVal
			}
		}
	}
	if len(headers) > 0 {
		settings["headers"] = headers
	}

	// Authentication
	authType := GetConfigValue(config, "auth.type")
	switch authType {
	case authTypeBasic:
		if userInfo := GetConfigValue(config, "auth.user.info"); userInfo != "" {
			parts := strings.SplitN(userInfo, ":", 2)
			if len(parts) == 2 {
				settings["auth.username"] = parts[0]
				settings["auth.password"] = MaskSensitiveValue("auth.password", parts[1])
			}
		}
	case authTypeOAuth2:
		settings["oauth2.enabled"] = true
		if tokenURL := GetConfigValue(config, "oauth2.token.url"); tokenURL != "" {
			settings["oauth2.token.url"] = tokenURL
		}
		if clientID := GetConfigValue(config, "oauth2.client.id"); clientID != "" {
			settings["oauth2.client.id"] = clientID
		}
		if clientSecret := GetConfigValue(config, "oauth2.client.secret"); clientSecret != "" {
			settings["oauth2.client.secret"] = MaskSensitiveValue("oauth2.client.secret", clientSecret)
		}
	case authTypeBearer:
		if token := GetConfigValue(config, "auth.bearer.token"); token != "" {
			headers["Authorization"] = "Bearer " + MaskSensitiveValue("bearer.token", token).(string)
			settings["headers"] = headers
		}
	}

	// Retry configuration
	if maxRetries := GetConfigValue(config, "max.retries"); maxRetries != "" {
		settings["retry.max"] = maxRetries
	}
	if retryBackoff := GetConfigValue(config, "retry.backoff.ms"); retryBackoff != "" {
		settings["retry.backoff"] = retryBackoff + "ms"
	}

	// Timeout
	if timeout := GetConfigValue(config, "request.timeout.ms"); timeout != "" {
		settings["timeout"] = timeout + "ms"
	}

	// Batch settings
	if batchSize := GetConfigValue(config, "batch.size"); batchSize != "" {
		settings["batch.size"] = batchSize
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// RedisMapper handles Redis connector mapping
type RedisMapper struct {
	BaseMapper
}

// NewRedisMapper creates a new Redis mapper
func NewRedisMapper() *RedisMapper {
	return &RedisMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Redis configuration
func (m *RedisMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:redis")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Redis connection
	hosts := GetConfigValue(config, "redis.hosts")
	if hosts == "" {
		hosts = GetConfigValue(config, "redis.host")
	}
	sb.Required("host", hosts, "redis.hosts")

	sb.WithDefault("port", GetConfigValue(config, "redis.port"), "6379")
	sb.Optional("database", GetConfigValue(config, "redis.database"))
	sb.Sensitive("password", GetConfigValue(config, "redis.password"), "")

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// SSL/TLS
	if sslEnabled := GetConfigValue(config, "redis.ssl.enabled"); sslEnabled == "true" {
		settings["tls.enabled"] = true
		if certPath := GetConfigValue(config, "redis.ssl.cert.path"); certPath != "" {
			settings["tls.cert.path"] = certPath
		}
	}

	// Key configuration
	if keyPrefix := GetConfigValue(config, "redis.key.prefix"); keyPrefix != "" {
		settings["key.prefix"] = keyPrefix
	}

	if keyField := GetConfigValue(config, "redis.key.field"); keyField != "" {
		settings["key.field"] = keyField
	}

	// TTL configuration
	if ttl := GetConfigValue(config, "redis.ttl"); ttl != "" {
		settings["ttl"] = ttl
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// Helper functions

func mapKafkaSerializer(serializer string) string {
	switch {
	case strings.Contains(serializer, "ByteArraySerializer"):
		return FormatBytes
	case strings.Contains(serializer, "StringSerializer"):
		return FormatString
	case strings.Contains(serializer, "JsonSerializer"):
		return FormatJSON
	case strings.Contains(serializer, "AvroSerializer"):
		return FormatAvro
	default:
		return FormatBytes
	}
}

func mapKafkaDeserializer(deserializer string) string {
	switch {
	case strings.Contains(deserializer, "ByteArrayDeserializer"):
		return FormatBytes
	case strings.Contains(deserializer, "StringDeserializer"):
		return FormatString
	case strings.Contains(deserializer, "JsonDeserializer"):
		return FormatJSON
	case strings.Contains(deserializer, "AvroDeserializer"):
		return FormatAvro
	default:
		return FormatBytes
	}
}
