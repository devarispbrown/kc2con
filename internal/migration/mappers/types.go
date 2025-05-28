package mappers

// ConduitPipeline represents a complete Conduit pipeline configuration
type ConduitPipeline struct {
	Version    string      `yaml:"version"`
	Pipelines  []Pipeline  `yaml:"pipelines"`
	Connectors []Connector `yaml:"connectors"`
	Processors []Processor `yaml:"processors,omitempty"`
}

// Pipeline represents a Conduit pipeline
type Pipeline struct {
	ID          string   `yaml:"id"`
	Status      string   `yaml:"status"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Connectors  []string `yaml:"connectors"`
	Processors  []string `yaml:"processors,omitempty"`
	DLQ         *DLQ     `yaml:"dlq,omitempty"`
}

// Connector represents a Conduit connector
type Connector struct {
	ID       string                 `yaml:"id"`
	Type     string                 `yaml:"type"`
	Plugin   string                 `yaml:"plugin"`
	Name     string                 `yaml:"name"`
	Settings map[string]interface{} `yaml:"settings"`
}

// Processor represents a Conduit processor
type Processor struct {
	ID        string                 `yaml:"id"`
	Plugin    string                 `yaml:"plugin"`
	Condition string                 `yaml:"condition,omitempty"`
	Settings  map[string]interface{} `yaml:"settings,omitempty"`
}

// DLQ represents Dead Letter Queue configuration
type DLQ struct {
	Plugin              string                 `yaml:"plugin"`
	Settings            map[string]interface{} `yaml:"settings"`
	WindowSize          int                    `yaml:"windowSize,omitempty"`
	WindowNackThreshold int                    `yaml:"windowNackThreshold,omitempty"`
}

// MigrationMetrics tracks migration statistics
type MigrationMetrics struct {
	StartTime      int64          `json:"startTime"`
	EndTime        int64          `json:"endTime"`
	TotalConfigs   int            `json:"totalConfigs"`
	Successful     int            `json:"successful"`
	Failed         int            `json:"failed"`
	WithWarnings   int            `json:"withWarnings"`
	Duration       int64          `json:"durationMs"`
	ConnectorTypes map[string]int `json:"connectorTypes"`
}

// SchemaRegistryConfig represents schema registry settings
type SchemaRegistryConfig struct {
	URL                        string            `yaml:"url"`
	BasicAuthUserInfo          string            `yaml:"basic.auth.user.info,omitempty"`
	BasicAuthCredentialsSource string            `yaml:"basic.auth.credentials.source,omitempty"`
	SslTruststoreLocation      string            `yaml:"ssl.truststore.location,omitempty"`
	SslTruststorePassword      string            `yaml:"ssl.truststore.password,omitempty"`
	SslKeystoreLocation        string            `yaml:"ssl.keystore.location,omitempty"`
	SslKeystorePassword        string            `yaml:"ssl.keystore.password,omitempty"`
	AdditionalConfig           map[string]string `yaml:"additional_config,omitempty"`
}

// ConnectorCapabilities defines what a Conduit connector supports
type ConnectorCapabilities struct {
	Plugin           string
	RequiredSettings []string
	OptionalSettings []string
	Features         []string
	Limitations      []string
}

// KnownConnectorCapabilities provides capability information for validation
var KnownConnectorCapabilities = map[string]ConnectorCapabilities{
	"builtin:postgres": {
		Plugin:           "builtin:postgres",
		RequiredSettings: []string{"host", "database", "user"},
		OptionalSettings: []string{"port", "password", "sslmode", "schema", "table"},
		Features:         []string{"cdc", "snapshot", "schema-evolution"},
		Limitations:      []string{"max-connections-100"},
	},
	"builtin:mysql": {
		Plugin:           "builtin:mysql",
		RequiredSettings: []string{"host", "database", "user"},
		OptionalSettings: []string{"port", "password", "ssl-mode", "server-id"},
		Features:         []string{"cdc", "snapshot", "binlog"},
		Limitations:      []string{"requires-binlog-enabled"},
	},
	"builtin:kafka": {
		Plugin:           "builtin:kafka",
		RequiredSettings: []string{"brokers", "topics"},
		OptionalSettings: []string{"consumer.group.id", "tls.enabled", "sasl.mechanism"},
		Features:         []string{"exactly-once", "consumer-groups", "regex-topics"},
		Limitations:      []string{"kafka-0.11.0-minimum"},
	},
	"builtin:s3": {
		Plugin:           "builtin:s3",
		RequiredSettings: []string{"aws.bucket"},
		OptionalSettings: []string{"aws.region", "aws.accessKeyId", "aws.secretAccessKey", "path.prefix"},
		Features:         []string{"versioning", "encryption", "multipart-upload"},
		Limitations:      []string{"max-object-size-5TB"},
	},
	"builtin:http": {
		Plugin:           "builtin:http",
		RequiredSettings: []string{"url"},
		OptionalSettings: []string{"method", "headers", "timeout", "retry.max"},
		Features:         []string{"retry", "backoff", "auth-basic", "auth-bearer"},
		Limitations:      []string{"max-payload-10MB"},
	},
}
