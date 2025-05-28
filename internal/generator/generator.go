package generator

import (
	"fmt"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// Generator creates Conduit pipeline configurations
type Generator struct {
	registry *registry.ImprovedRegistry
}

// New creates a new pipeline generator
func New(reg *registry.ImprovedRegistry) *Generator {
	return &Generator{
		registry: reg,
	}
}

// GeneratePipeline generates a Conduit pipeline from a Kafka Connect configuration
func (g *Generator) GeneratePipeline(config *parser.ConnectorConfig, analysis registry.ConnectorAnalysis) (*registry.ConduitPipeline, error) {
	// Create base pipeline structure
	pipeline := &registry.ConduitPipeline{
		Version: "2.2",
		Pipelines: []registry.Pipeline{{
			ID:          sanitizeID(config.Name),
			Status:      "stopped",
			ConnectorID: config.Name,
		}},
	}

	// Determine if this is a source or sink
	isSource := g.isSourceConnector(config.Class)

	if isSource {
		// Configure source
		pipeline.Pipelines[0].Source = g.generateSource(config, analysis)

		// Add placeholder destination
		pipeline.Pipelines[0].Destination = registry.Destination{
			Type:   "destination",
			Plugin: "builtin:log",
			Settings: map[string]interface{}{
				"level": "info",
				"TODO":  "Replace with your actual destination",
			},
		}
	} else {
		// Configure destination
		pipeline.Pipelines[0].Destination = g.generateDestination(config, analysis)

		// Add placeholder source
		pipeline.Pipelines[0].Source = registry.Source{
			Type:   "source",
			Plugin: "builtin:generator",
			Settings: map[string]interface{}{
				"format.type":            "structured",
				"format.options.id.type": "int",
				"TODO":                   "Replace with your actual source",
			},
		}
	}

	// Handle transforms/processors
	if len(config.Transforms) > 0 {
		processors := g.generateProcessors(config.Transforms)
		if len(processors) > 0 {
			pipeline.Pipelines[0].Processors = processors
		}
	}

	// Use ConfigMapper if available
	if analysis.ConnectorInfo.ConfigMapper != nil {
		// Call the custom mapper function
		mappedPipeline, _ := analysis.ConnectorInfo.ConfigMapper(config)
		if mappedPipeline != nil {
			// Merge with our generated pipeline
			*pipeline = *mappedPipeline
		}
		// Issues are already in the analysis
	}

	return pipeline, nil
}

// generateSource creates a source connector configuration
func (g *Generator) generateSource(config *parser.ConnectorConfig, analysis registry.ConnectorAnalysis) registry.Source {
	source := registry.Source{
		Type:     "source",
		Plugin:   g.mapPlugin(analysis.ConnectorInfo),
		Settings: make(map[string]interface{}),
	}

	// Apply connector-specific mappings
	switch {
	case strings.Contains(config.Class, "Debezium"):
		g.mapDebeziumSettings(config, source.Settings)
	case strings.Contains(config.Class, "JdbcSourceConnector"):
		g.mapJdbcSourceSettings(config, source.Settings)
	case strings.Contains(config.Class, "FileStreamSourceConnector"):
		g.mapFileSourceSettings(config, source.Settings)
	case strings.Contains(config.Class, "MirrorSourceConnector"):
		g.mapKafkaSourceSettings(config, source.Settings)
	default:
		// Generic mapping
		g.mapGenericSettings(config, source.Settings)
	}

	return source
}

// generateDestination creates a destination connector configuration
func (g *Generator) generateDestination(config *parser.ConnectorConfig, analysis registry.ConnectorAnalysis) registry.Destination {
	dest := registry.Destination{
		Type:     "destination",
		Plugin:   g.mapPlugin(analysis.ConnectorInfo),
		Settings: make(map[string]interface{}),
	}

	// Apply connector-specific mappings
	switch {
	case strings.Contains(config.Class, "JdbcSinkConnector"):
		g.mapJdbcSinkSettings(config, dest.Settings)
	case strings.Contains(config.Class, "S3SinkConnector"):
		g.mapS3Settings(config, dest.Settings)
	case strings.Contains(config.Class, "ElasticsearchSinkConnector"):
		g.mapElasticsearchSettings(config, dest.Settings)
	case strings.Contains(config.Class, "HttpSinkConnector"):
		g.mapHttpSettings(config, dest.Settings)
	case strings.Contains(config.Class, "MongoSinkConnector"):
		g.mapMongoSettings(config, dest.Settings)
	default:
		// Generic mapping
		g.mapGenericSettings(config, dest.Settings)
	}

	return dest
}

// generateProcessors creates processor configurations from transforms
func (g *Generator) generateProcessors(transforms []parser.TransformConfig) []registry.Processor {
	var processors []registry.Processor

	for _, transform := range transforms {
		processor := registry.Processor{
			ID:       sanitizeID(transform.Name),
			Settings: make(map[string]interface{}),
		}

		// Check if we have a mapping in the registry
		if transformInfo, exists := g.registry.GetTransformInfo(transform.Class); exists {
			processor.Plugin = transformInfo.ConduitEquivalent

			// Copy transform configuration
			for k, v := range transform.Config {
				processor.Settings[k] = v
			}
		} else {
			// Unknown transform - add as custom
			processor.Plugin = "builtin:custom"
			processor.Settings["type"] = transform.Class
			processor.Settings["TODO"] = "Implement custom processor logic"
		}

		processors = append(processors, processor)
	}

	return processors
}

// Mapping helper functions

func (g *Generator) mapDebeziumSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	// Database connection settings
	if v := getConfigValue(config, "database.hostname"); v != "" {
		settings["host"] = v
	}
	if v := getConfigValue(config, "database.port"); v != "" {
		settings["port"] = v
	}
	if v := getConfigValue(config, "database.user"); v != "" {
		settings["user"] = v
	}
	if v := getConfigValue(config, "database.password"); v != "" {
		settings["password"] = v
	}
	if v := getConfigValue(config, "database.dbname", "database.name"); v != "" {
		settings["database"] = v
	}

	// Table selection
	if v := getConfigValue(config, "table.include.list"); v != "" {
		settings["tables"] = v
	}

	// Server ID for MySQL
	if v := getConfigValue(config, "database.server.id"); v != "" {
		settings["server.id"] = v
	}

	// Snapshot mode
	if v := getConfigValue(config, "snapshot.mode"); v != "" {
		settings["snapshot.mode"] = v
	}
}

func (g *Generator) mapJdbcSourceSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	// Connection URL parsing would go here
	if v := getConfigValue(config, "connection.url"); v != "" {
		settings["url"] = v
		settings["TODO_parse_url"] = "Parse JDBC URL to extract host, port, database"
	}

	// Credentials
	if v := getConfigValue(config, "connection.user"); v != "" {
		settings["user"] = v
	}
	if v := getConfigValue(config, "connection.password"); v != "" {
		settings["password"] = v
	}

	// Mode
	if v := getConfigValue(config, "mode"); v != "" {
		settings["mode"] = v
	}

	// Tables/Query
	if v := getConfigValue(config, "table.whitelist", "table.include.list"); v != "" {
		settings["tables"] = v
	}
	if v := getConfigValue(config, "query"); v != "" {
		settings["query"] = v
	}
}

func (g *Generator) mapJdbcSinkSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	// Start with source settings (connection is similar)
	g.mapJdbcSourceSettings(config, settings)

	// Sink-specific settings
	if v := getConfigValue(config, "insert.mode"); v != "" {
		settings["insert.mode"] = v
	}
	if v := getConfigValue(config, "pk.mode"); v != "" {
		settings["pk.mode"] = v
	}
	if v := getConfigValue(config, "pk.fields"); v != "" {
		settings["pk.fields"] = v
	}
}

func (g *Generator) mapS3Settings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	if v := getConfigValue(config, "s3.bucket.name"); v != "" {
		settings["aws.bucket"] = v
	}
	if v := getConfigValue(config, "s3.region", "region"); v != "" {
		settings["aws.region"] = v
	}
	if v := getConfigValue(config, "aws.access.key.id"); v != "" {
		settings["aws.accessKeyId"] = v
	}
	if v := getConfigValue(config, "aws.secret.access.key"); v != "" {
		settings["aws.secretAccessKey"] = v
	}
}

func (g *Generator) mapElasticsearchSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	if v := getConfigValue(config, "connection.url"); v != "" {
		settings["url"] = v
	}
	if v := getConfigValue(config, "index.prefix"); v != "" {
		settings["index"] = v
	}
	if v := getConfigValue(config, "connection.username"); v != "" {
		settings["username"] = v
	}
	if v := getConfigValue(config, "connection.password"); v != "" {
		settings["password"] = v
	}
}

func (g *Generator) mapHttpSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	if v := getConfigValue(config, "http.api.url"); v != "" {
		settings["url"] = v
	}
	if v := getConfigValue(config, "request.method"); v != "" {
		settings["method"] = v
	}
	if v := getConfigValue(config, "headers"); v != "" {
		settings["headers"] = v
	}
}

func (g *Generator) mapMongoSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	if v := getConfigValue(config, "connection.uri"); v != "" {
		settings["uri"] = v
	}
	if v := getConfigValue(config, "database"); v != "" {
		settings["db"] = v
	}
	if v := getConfigValue(config, "collection"); v != "" {
		settings["collection"] = v
	}
}

func (g *Generator) mapFileSourceSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	if v := getConfigValue(config, "file"); v != "" {
		settings["path"] = v
	}
	if v := getConfigValue(config, "topic"); v != "" {
		settings["TODO_topic"] = fmt.Sprintf("Original topic: %s", v)
	}
}

func (g *Generator) mapKafkaSourceSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	if v := getConfigValue(config, "source.cluster.bootstrap.servers"); v != "" {
		settings["servers"] = v
	}
	if v := getConfigValue(config, "topics", "topics.regex"); v != "" {
		settings["topics"] = v
	}
	if v := getConfigValue(config, "group.id"); v != "" {
		settings["groupID"] = v
	}
}

func (g *Generator) mapGenericSettings(config *parser.ConnectorConfig, settings map[string]interface{}) {
	// Copy all non-Kafka Connect specific settings
	for key, value := range config.Config {
		if !g.isKafkaConnectSpecificKey(key) {
			settings[key] = value
		}
	}

	// Add note about generic mapping
	settings["TODO_review"] = "Generic mapping applied - review all settings"
}

// Helper functions

func (g *Generator) isSourceConnector(class string) bool {
	class = strings.ToLower(class)
	return strings.Contains(class, "source") ||
		strings.Contains(class, "debezium") ||
		!strings.Contains(class, "sink")
}

func (g *Generator) mapPlugin(info registry.ConnectorInfo) string {
	// Check if it's a builtin plugin
	builtins := []string{"postgres", "mysql", "kafka", "s3", "http", "file", "generator", "log", "elasticsearch", "mongodb"}
	plugin := info.ConduitEquivalent

	for _, builtin := range builtins {
		if strings.HasPrefix(plugin, builtin) {
			return "builtin:" + builtin
		}
	}

	// Otherwise it's a standalone plugin
	return plugin
}

func (g *Generator) isKafkaConnectSpecificKey(key string) bool {
	prefixes := []string{
		"connector.",
		"tasks.",
		"transforms.",
		"key.converter",
		"value.converter",
		"header.converter",
		"config.action.reload",
		"errors.",
		"topic.creation.",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return key == "name" || key == "class"
}

func getConfigValue(config *parser.ConnectorConfig, keys ...string) string {
	for _, key := range keys {
		// Check in Config
		if val, ok := config.Config[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		// Check in RawConfig
		if val, ok := config.RawConfig[key]; ok {
			return fmt.Sprintf("%v", val)
		}
	}
	return ""
}

func sanitizeID(name string) string {
	replacer := strings.NewReplacer(
		" ", "-",
		".", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	sanitized := strings.ToLower(replacer.Replace(name))

	// Ensure it starts with a letter
	if len(sanitized) > 0 && (sanitized[0] < 'a' || sanitized[0] > 'z') {
		sanitized = "p" + sanitized
	}

	return sanitized
}
