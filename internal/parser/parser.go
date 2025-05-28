package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Kafka Connect config key constants
const (
	KeyConverter                = "key.converter"
	ValueConverter              = "value.converter"
	KeyConverterSchemaRegistry  = "key.converter.schema.registry.url"
	ValueConverterSchemaRegistry = "value.converter.schema.registry.url"
	SchemaRegistryURL           = "schema.registry.url"
)

// Parser handles parsing of Kafka Connect configuration files
type Parser struct{}

// ConnectorConfig represents a parsed Kafka Connect connector configuration
type ConnectorConfig struct {
	Name            string                 `json:"name"`
	Class           string                 `json:"connector.class"`
	TasksMax        int                    `json:"tasks.max,omitempty"`
	Topics          []string               `json:"topics,omitempty"`
	TopicsRegex     string                 `json:"topics.regex,omitempty"`
	KeyConverter    string                 `json:"key.converter,omitempty"`
	ValueConverter  string                 `json:"value.converter,omitempty"`
	HeaderConverter string                 `json:"header.converter,omitempty"`
	Transforms      []TransformConfig      `json:"transforms,omitempty"`
	Config          map[string]interface{} `json:"config,omitempty"`
	RawConfig       map[string]interface{} `json:"-"` // Store all original config
}

// TransformConfig represents a Single Message Transform (SMT) configuration
type TransformConfig struct {
	Name      string                 `json:"name"`
	Class     string                 `json:"type"`
	Config    map[string]interface{} `json:"config,omitempty"`
	Predicate string                 `json:"predicate,omitempty"`
	Negate    bool                   `json:"negate,omitempty"`
}

// WorkerConfig represents Kafka Connect worker configuration
type WorkerConfig struct {
	BootstrapServers             string `json:"bootstrap.servers"`
	GroupId                      string `json:"group.id,omitempty"`
	KeyConverter                 string `json:"key.converter,omitempty"`
	ValueConverter               string `json:"value.converter,omitempty"`
	KeyConverterSchemaRegistry   string `json:"key.converter.schema.registry.url,omitempty"`
	ValueConverterSchemaRegistry string `json:"value.converter.schema.registry.url,omitempty"`
	SchemaRegistry               string `json:"schema.registry.url,omitempty"`

	// Security settings
	SecurityProtocol      string `json:"security.protocol,omitempty"`
	SaslMechanism         string `json:"sasl.mechanism,omitempty"`
	SaslJaasConfig        string `json:"sasl.jaas.config,omitempty"`
	SslTruststoreLocation string `json:"ssl.truststore.location,omitempty"`
	SslKeystoreLocation   string `json:"ssl.keystore.location,omitempty"`

	// Connect specific settings
	ConfigStorageTopic             string `json:"config.storage.topic,omitempty"`
	OffsetStorageTopic             string `json:"offset.storage.topic,omitempty"`
	StatusStorageTopic             string `json:"status.storage.topic,omitempty"`
	ConfigStorageReplicationFactor int    `json:"config.storage.replication.factor,omitempty"`
	OffsetStorageReplicationFactor int    `json:"offset.storage.replication.factor,omitempty"`
	StatusStorageReplicationFactor int    `json:"status.storage.replication.factor,omitempty"`

	// Consumer/Producer overrides
	ConsumerOverrides map[string]interface{} `json:"consumer,omitempty"`
	ProducerOverrides map[string]interface{} `json:"producer,omitempty"`
	AdminOverrides    map[string]interface{} `json:"admin,omitempty"`

	// Plugin settings
	PluginPath []string `json:"plugin.path,omitempty"`

	// All other settings
	RawConfig map[string]interface{} `json:"-"`
}

func New() *Parser {
	return &Parser{}
}

// ParseConnectorConfig parses a connector configuration file (JSON or properties)
func (p *Parser) ParseConnectorConfig(filePath string) (*ConnectorConfig, error) {
	if strings.HasSuffix(strings.ToLower(filePath), ".json") {
		return p.parseConnectorJSON(filePath)
	} else if strings.HasSuffix(strings.ToLower(filePath), ".properties") {
		return p.parseConnectorProperties(filePath)
	}

	return nil, fmt.Errorf("unsupported file format: %s", filePath)
}

// ParseWorkerConfig parses a worker configuration file (typically .properties)
func (p *Parser) ParseWorkerConfig(filePath string) (*WorkerConfig, error) {
	props, err := p.parsePropertiesFile(filePath)
	if err != nil {
		return nil, err
	}

	config := &WorkerConfig{
		RawConfig: make(map[string]interface{}),
	}

	// Map known properties to struct fields
	for key, value := range props {
		switch key {
		case "bootstrap.servers":
			config.BootstrapServers = value
		case "group.id":
			config.GroupId = value
		case KeyConverter:
			config.KeyConverter = value
		case ValueConverter:
			config.ValueConverter = value
		case KeyConverterSchemaRegistry:
			config.KeyConverterSchemaRegistry = value
		case ValueConverterSchemaRegistry:
			config.ValueConverterSchemaRegistry = value
		case SchemaRegistryURL:
			config.SchemaRegistry = value
		case "security.protocol":
			config.SecurityProtocol = value
		case "sasl.mechanism":
			config.SaslMechanism = value
		case "sasl.jaas.config":
			config.SaslJaasConfig = value
		case "ssl.truststore.location":
			config.SslTruststoreLocation = value
		case "ssl.keystore.location":
			config.SslKeystoreLocation = value
		case "config.storage.topic":
			config.ConfigStorageTopic = value
		case "offset.storage.topic":
			config.OffsetStorageTopic = value
		case "status.storage.topic":
			config.StatusStorageTopic = value
		case "plugin.path":
			// Handle comma-separated plugin paths
			config.PluginPath = strings.Split(value, ",")
			for i := range config.PluginPath {
				config.PluginPath[i] = strings.TrimSpace(config.PluginPath[i])
			}
		default:
			// Handle prefixed configs (consumer., producer., admin.)
			switch {
			case strings.HasPrefix(key, "consumer."):
				if config.ConsumerOverrides == nil {
					config.ConsumerOverrides = make(map[string]interface{})
				}
				config.ConsumerOverrides[strings.TrimPrefix(key, "consumer.")] = value
			case strings.HasPrefix(key, "producer."):
				if config.ProducerOverrides == nil {
					config.ProducerOverrides = make(map[string]interface{})
				}
				config.ProducerOverrides[strings.TrimPrefix(key, "producer.")] = value
			case strings.HasPrefix(key, "admin."):
				if config.AdminOverrides == nil {
					config.AdminOverrides = make(map[string]interface{})
				}
				config.AdminOverrides[strings.TrimPrefix(key, "admin.")] = value
			}
		}

		// Store all config in raw config for reference
		config.RawConfig[key] = value
	}

	return config, nil
}

func (p *Parser) parseConnectorJSON(filePath string) (*ConnectorConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Try to parse as connector config JSON
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from %s: %w", filePath, err)
	}

	config := &ConnectorConfig{
		RawConfig: rawConfig,
		Config:    make(map[string]interface{}),
	}

	// Extract known fields
	if name, ok := rawConfig["name"].(string); ok {
		config.Name = name
	}

	if class, ok := rawConfig["connector.class"].(string); ok {
		config.Class = class
	} else if class, ok := rawConfig["class"].(string); ok {
		config.Class = class
	}

	if tasksMax, ok := rawConfig["tasks.max"]; ok {
		if val, err := parseIntValue(tasksMax); err == nil {
			config.TasksMax = val
		}
	}

	// Extract topics
	if topics, ok := rawConfig["topics"].(string); ok {
		config.Topics = strings.Split(topics, ",")
		for i := range config.Topics {
			config.Topics[i] = strings.TrimSpace(config.Topics[i])
		}
	}

	if topicsRegex, ok := rawConfig["topics.regex"].(string); ok {
		config.TopicsRegex = topicsRegex
	}

	// Extract converters
	if keyConverter, ok := rawConfig[KeyConverter].(string); ok {
		config.KeyConverter = keyConverter
	}

	if valueConverter, ok := rawConfig[ValueConverter].(string); ok {
		config.ValueConverter = valueConverter
	}

	if headerConverter, ok := rawConfig["header.converter"].(string); ok {
		config.HeaderConverter = headerConverter
	}

	// Extract transforms
	config.Transforms = p.extractTransforms(rawConfig)

	// Store remaining config
	for key, value := range rawConfig {
		if !isWellKnownConnectorField(key) {
			config.Config[key] = value
		}
	}

	return config, nil
}

func (p *Parser) parseConnectorProperties(filePath string) (*ConnectorConfig, error) {
	props, err := p.parsePropertiesFile(filePath)
	if err != nil {
		return nil, err
	}

	config := &ConnectorConfig{
		RawConfig: make(map[string]interface{}),
		Config:    make(map[string]interface{}),
	}

	// Convert properties to our config format
	for key, value := range props {
		config.RawConfig[key] = value

		switch key {
		case "name":
			config.Name = value
		case "connector.class", "class":
			config.Class = value
		case "tasks.max":
			if val, err := strconv.Atoi(value); err == nil {
				config.TasksMax = val
			}
		case "topics":
			config.Topics = strings.Split(value, ",")
			for i := range config.Topics {
				config.Topics[i] = strings.TrimSpace(config.Topics[i])
			}
		case "topics.regex":
			config.TopicsRegex = value
		case KeyConverter:
			config.KeyConverter = value
		case ValueConverter:
			config.ValueConverter = value
		case "header.converter":
			config.HeaderConverter = value
		default:
			if !isWellKnownConnectorField(key) {
				config.Config[key] = value
			}
		}
	}

	// Extract transforms from properties
	config.Transforms = p.extractTransformsFromProperties(props)

	return config, nil
}

func (p *Parser) parsePropertiesFile(filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	props := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		// Handle line continuations
		for strings.HasSuffix(line, "\\") && scanner.Scan() {
			line = strings.TrimSuffix(line, "\\")
			nextLine := strings.TrimSpace(scanner.Text())
			line += nextLine
		}

		// Parse key=value
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			// Remove quotes if present
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}

			props[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	return props, nil
}

func (p *Parser) extractTransforms(config map[string]interface{}) []TransformConfig {
	var transforms []TransformConfig

	// Look for transforms property
	transformsValue, ok := config["transforms"]
	if !ok {
		return transforms
	}

	var transformNames []string
	switch v := transformsValue.(type) {
	case string:
		transformNames = strings.Split(v, ",")
	case []interface{}:
		for _, name := range v {
			if nameStr, ok := name.(string); ok {
				transformNames = append(transformNames, nameStr)
			}
		}
	}

	// Extract configuration for each transform
	for _, name := range transformNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		transform := TransformConfig{
			Name:   name,
			Config: make(map[string]interface{}),
		}

		// Look for transform-specific config
		typeKey := fmt.Sprintf("transforms.%s.type", name)
		if transformType, ok := config[typeKey].(string); ok {
			transform.Class = transformType
		}

		predicateKey := fmt.Sprintf("transforms.%s.predicate", name)
		if predicate, ok := config[predicateKey].(string); ok {
			transform.Predicate = predicate
		}

		negateKey := fmt.Sprintf("transforms.%s.negate", name)
		if negate, ok := config[negateKey]; ok {
			if negateVal, err := parseBoolValue(negate); err == nil {
				transform.Negate = negateVal
			}
		}

		// Collect all other transform-specific config
		prefix := fmt.Sprintf("transforms.%s.", name)
		for key, value := range config {
			if strings.HasPrefix(key, prefix) && !strings.HasSuffix(key, ".type") &&
				!strings.HasSuffix(key, ".predicate") && !strings.HasSuffix(key, ".negate") {
				configKey := strings.TrimPrefix(key, prefix)
				transform.Config[configKey] = value
			}
		}

		transforms = append(transforms, transform)
	}

	return transforms
}

func (p *Parser) extractTransformsFromProperties(props map[string]string) []TransformConfig {
	var transforms []TransformConfig

	// Look for transforms property
	transformsStr, ok := props["transforms"]
	if !ok {
		return transforms
	}

	transformNames := strings.Split(transformsStr, ",")

	// Extract configuration for each transform
	for _, name := range transformNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		transform := TransformConfig{
			Name:   name,
			Config: make(map[string]interface{}),
		}

		// Look for transform-specific config
		typeKey := fmt.Sprintf("transforms.%s.type", name)
		if transformType, ok := props[typeKey]; ok {
			transform.Class = transformType
		}

		predicateKey := fmt.Sprintf("transforms.%s.predicate", name)
		if predicate, ok := props[predicateKey]; ok {
			transform.Predicate = predicate
		}

		negateKey := fmt.Sprintf("transforms.%s.negate", name)
		if negate, ok := props[negateKey]; ok {
			if negateVal, err := strconv.ParseBool(negate); err == nil {
				transform.Negate = negateVal
			}
		}

		// Collect all other transform-specific config
		prefix := fmt.Sprintf("transforms.%s.", name)
		for key, value := range props {
			if strings.HasPrefix(key, prefix) && !strings.HasSuffix(key, ".type") &&
				!strings.HasSuffix(key, ".predicate") && !strings.HasSuffix(key, ".negate") {
				configKey := strings.TrimPrefix(key, prefix)
				transform.Config[configKey] = value
			}
		}

		transforms = append(transforms, transform)
	}

	return transforms
}

// Helper functions
func isWellKnownConnectorField(key string) bool {
	wellKnownFields := []string{
		"name", "connector.class", "class", "tasks.max", "topics", "topics.regex",
		"key.converter", "value.converter", "header.converter", "transforms",
	}

	for _, field := range wellKnownFields {
		if key == field {
			return true
		}
	}

	// Also exclude transform-specific fields
	return strings.HasPrefix(key, "transforms.")
}

func parseIntValue(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot parse %v as int", value)
	}
}

func parseBoolValue(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	default:
		return false, fmt.Errorf("cannot parse %v as bool", value)
	}
}

// ExtractSchemaRegistryConfig extracts schema registry configuration from various sources
func (p *Parser) ExtractSchemaRegistryConfig(workerConfig *WorkerConfig, connectorConfig *ConnectorConfig) *SchemaRegistryConfig {
	config := &SchemaRegistryConfig{
		AdditionalConfig: make(map[string]string),
	}

	// First try worker config
	if workerConfig != nil {
		if workerConfig.SchemaRegistry != "" {
			config.URL = workerConfig.SchemaRegistry
		} else if workerConfig.KeyConverterSchemaRegistry != "" {
			config.URL = workerConfig.KeyConverterSchemaRegistry
		} else if workerConfig.ValueConverterSchemaRegistry != "" {
			config.URL = workerConfig.ValueConverterSchemaRegistry
		}

		// Look for schema registry specific settings in raw config
		for key, value := range workerConfig.RawConfig {
			if strings.Contains(key, "schema.registry") && key != SchemaRegistryURL {
				if strVal, ok := value.(string); ok {
					config.AdditionalConfig[key] = strVal
				}
			}
		}
	}

	// Then check connector config for converter-specific schema registry settings
	if connectorConfig != nil {
		for key, value := range connectorConfig.RawConfig {
			if strings.Contains(key, "schema.registry") {
				if strVal, ok := value.(string); ok {
					if key == KeyConverterSchemaRegistry || key == ValueConverterSchemaRegistry {
						if config.URL == "" {
							config.URL = strVal
						}
					} else {
						config.AdditionalConfig[key] = strVal
					}
				}
			}
		}
	}

	return config
}

// SchemaRegistryConfig represents schema registry specific settings
type SchemaRegistryConfig struct {
	URL                        string            `json:"url"`
	BasicAuthUserInfo          string            `json:"basic.auth.user.info,omitempty"`
	BasicAuthCredentialsSource string            `json:"basic.auth.credentials.source,omitempty"`
	SslTruststoreLocation      string            `json:"ssl.truststore.location,omitempty"`
	SslTruststorePassword      string            `json:"ssl.truststore.password,omitempty"`
	SslKeystoreLocation        string            `json:"ssl.keystore.location,omitempty"`
	SslKeystorePassword        string            `json:"ssl.keystore.password,omitempty"`
	AdditionalConfig           map[string]string `json:"additional_config,omitempty"`
}
