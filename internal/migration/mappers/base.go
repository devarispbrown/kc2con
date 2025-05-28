package mappers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// ConnectorMapper defines the interface for connector-specific mapping logic
type ConnectorMapper interface {
	Map(config *parser.ConnectorConfig, connectorInfo registry.ConnectorInfo) (*ConduitPipeline, error)
}

// BaseMapper provides common functionality for all mappers
type BaseMapper struct {
	settingsBuilder *SettingsBuilder
}

// NewBaseMapper creates a new base mapper
func NewBaseMapper() *BaseMapper {
	return &BaseMapper{
		settingsBuilder: NewSettingsBuilder(),
	}
}

// CreateBasePipeline creates a basic pipeline structure
func (m *BaseMapper) CreateBasePipeline(config *parser.ConnectorConfig, connectorType, plugin string) (*ConduitPipeline, error) {
	if config == nil {
		return nil, fmt.Errorf("connector config is nil")
	}

	if config.Name == "" {
		return nil, fmt.Errorf("connector name is required")
	}

	if config.Class == "" {
		return nil, fmt.Errorf("connector class is required")
	}

	pipelineID := GeneratePipelineID(config.Name)
	connectorID := fmt.Sprintf("%s-%s", pipelineID, connectorType)

	// Ensure plugin has proper format
	if !strings.Contains(plugin, "@") && strings.HasPrefix(plugin, "builtin:") {
		plugin = plugin + "@latest"
	}

	pipeline := &ConduitPipeline{
		Version: "2.2",
		Pipelines: []Pipeline{{
			ID:          pipelineID,
			Status:      "running",
			Name:        config.Name,
			Description: fmt.Sprintf("Migrated from Kafka Connect %s", config.Class),
			Connectors:  []string{connectorID},
		}},
		Connectors: []Connector{{
			ID:       connectorID,
			Type:     connectorType,
			Plugin:   plugin,
			Name:     config.Name,
			Settings: make(map[string]interface{}),
		}},
	}

	return pipeline, nil
}

// GetConnectorSettings safely returns the settings map
func (m *BaseMapper) GetConnectorSettings(pipeline *ConduitPipeline) (map[string]interface{}, error) {
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline is nil")
	}

	if len(pipeline.Connectors) == 0 {
		return nil, fmt.Errorf("no connectors in pipeline")
	}

	settings := pipeline.Connectors[0].Settings
	if settings == nil {
		settings = make(map[string]interface{})
		pipeline.Connectors[0].Settings = settings
	}

	return settings, nil
}

// SettingsBuilder helps build and validate connector settings
type SettingsBuilder struct {
	settings map[string]interface{}
	errors   []error
}

// NewSettingsBuilder creates a new settings builder
func NewSettingsBuilder() *SettingsBuilder {
	return &SettingsBuilder{
		settings: make(map[string]interface{}),
		errors:   []error{},
	}
}

// Required adds a required setting
func (sb *SettingsBuilder) Required(key, value, fieldName string) *SettingsBuilder {
	if value == "" {
		sb.errors = append(sb.errors, fmt.Errorf("%s is required", fieldName))
	} else {
		sb.settings[key] = value
	}
	return sb
}

// Optional adds an optional setting
func (sb *SettingsBuilder) Optional(key, value string) *SettingsBuilder {
	if value != "" {
		sb.settings[key] = value
	}
	return sb
}

// OptionalInt adds an optional integer setting
func (sb *SettingsBuilder) OptionalInt(key string, value int) *SettingsBuilder {
	if value > 0 {
		sb.settings[key] = value
	}
	return sb
}

// OptionalBool adds an optional boolean setting
func (sb *SettingsBuilder) OptionalBool(key string, value bool) *SettingsBuilder {
	sb.settings[key] = value
	return sb
}

// WithDefault adds a setting with a default value
func (sb *SettingsBuilder) WithDefault(key, value, defaultValue string) *SettingsBuilder {
	if value != "" {
		sb.settings[key] = value
	} else {
		sb.settings[key] = defaultValue
	}
	return sb
}

// Sensitive adds a sensitive setting with masking
func (sb *SettingsBuilder) Sensitive(key, value, fieldName string) *SettingsBuilder {
	if value == "" && fieldName != "" {
		sb.errors = append(sb.errors, fmt.Errorf("%s is required", fieldName))
	} else if value != "" {
		sb.settings[key] = MaskSensitiveValue(key, value)
	}
	return sb
}

// Build returns the built settings or an error
func (sb *SettingsBuilder) Build() (map[string]interface{}, error) {
	if len(sb.errors) > 0 {
		return nil, fmt.Errorf("validation errors: %v", sb.errors)
	}
	return sb.settings, nil
}

// Reset clears the builder for reuse
func (sb *SettingsBuilder) Reset() {
	sb.settings = make(map[string]interface{})
	sb.errors = []error{}
}

// URLBuilder helps build connection URLs
type URLBuilder struct {
	protocol string
	user     string
	password string
	host     string
	port     string
	database string
	params   map[string]string
}

// NewURLBuilder creates a new URL builder
func NewURLBuilder(protocol string) *URLBuilder {
	return &URLBuilder{
		protocol: protocol,
		params:   make(map[string]string),
	}
}

// WithAuth sets authentication
func (ub *URLBuilder) WithAuth(user, password string) *URLBuilder {
	ub.user = user
	ub.password = password
	return ub
}

// WithHost sets host and port
func (ub *URLBuilder) WithHost(host, port string) *URLBuilder {
	ub.host = host
	ub.port = port
	return ub
}

// WithDatabase sets the database
func (ub *URLBuilder) WithDatabase(database string) *URLBuilder {
	ub.database = database
	return ub
}

// WithParam adds a URL parameter
func (ub *URLBuilder) WithParam(key, value string) *URLBuilder {
	if value != "" {
		ub.params[key] = value
	}
	return ub
}

// Build constructs the URL
func (ub *URLBuilder) Build() string {
	var userInfo string
	if ub.user != "" {
		if ub.password != "" {
			userInfo = fmt.Sprintf("%s:%s@",
				url.QueryEscape(ub.user),
				url.QueryEscape(ub.password))
		} else {
			userInfo = fmt.Sprintf("%s@", url.QueryEscape(ub.user))
		}
	}

	baseURL := fmt.Sprintf("%s://%s%s", ub.protocol, userInfo, ub.host)

	if ub.port != "" {
		baseURL += ":" + ub.port
	}

	if ub.database != "" {
		baseURL += "/" + ub.database
	}

	if len(ub.params) > 0 {
		params := url.Values{}
		for k, v := range ub.params {
			params.Add(k, v)
		}
		baseURL += "?" + params.Encode()
	}

	return baseURL
}

// Common helper functions

// GetConfigValue safely retrieves a config value
func GetConfigValue(config *parser.ConnectorConfig, key string) string {
	if config == nil {
		return ""
	}

	// Check in main config first
	if val, ok := config.Config[key]; ok {
		return convertToString(val)
	}

	// Check in raw config
	if val, ok := config.RawConfig[key]; ok {
		return convertToString(val)
	}

	return ""
}

// convertToString converts various types to string
func convertToString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case int, int64, int32:
		return fmt.Sprintf("%d", v)
	case float64, float32:
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// MaskSensitiveValue masks sensitive configuration values
func MaskSensitiveValue(fieldName string, value interface{}) interface{} {
	if strVal, ok := value.(string); ok {
		// Preserve environment variable references
		if strings.HasPrefix(strVal, "${") && strings.HasSuffix(strVal, "}") {
			return strVal
		}
		// Preserve file references
		if strings.HasPrefix(strVal, "${file:") && strings.HasSuffix(strVal, "}") {
			return strVal
		}
	}

	// For actual sensitive values, return a placeholder
	return fmt.Sprintf("${MASKED_%s}", strings.ToUpper(fieldName))
}

// IsSensitiveField checks if a field name indicates sensitive data
func IsSensitiveField(fieldName string) bool {
	fieldLower := strings.ToLower(fieldName)
	sensitivePatterns := []string{
		"password", "passwd", "pwd",
		"secret", "key", "token",
		"credential", "auth",
		"private", "cert",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(fieldLower, pattern) {
			return true
		}
	}

	return false
}

// GeneratePipelineID creates a valid pipeline ID from a name
func GeneratePipelineID(connectorName string) string {
	// Clean the name for use as ID
	id := strings.ToLower(connectorName)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "_", "-")
	id = strings.ReplaceAll(id, ".", "-")

	// Remove any non-alphanumeric characters except hyphens
	cleaned := ""
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned += string(r)
		}
	}

	// Ensure it doesn't start or end with hyphen
	cleaned = strings.Trim(cleaned, "-")

	if cleaned == "" {
		cleaned = "pipeline"
	}

	return cleaned
}

// ParseJDBCURL properly parses a JDBC URL
func ParseJDBCURL(jdbcURL string) (map[string]interface{}, error) {
	if !strings.HasPrefix(jdbcURL, "jdbc:") {
		return nil, fmt.Errorf("invalid JDBC URL: must start with 'jdbc:'")
	}

	// Remove jdbc: prefix and extract protocol
	urlStr := strings.TrimPrefix(jdbcURL, "jdbc:")

	// Handle different JDBC formats
	// jdbc:postgresql://host:port/database
	// jdbc:mysql://host:port/database
	// jdbc:sqlserver://host:port;databaseName=database

	settings := make(map[string]interface{})

	// Extract protocol
	protocolEnd := strings.Index(urlStr, "://")
	if protocolEnd > 0 {
		settings["protocol"] = urlStr[:protocolEnd]
		urlStr = urlStr[protocolEnd+3:] // Skip ://
	} else {
		return nil, fmt.Errorf("invalid JDBC URL format: missing protocol")
	}

	// Parse the rest as a standard URL
	// First, handle SQL Server format
	if strings.Contains(urlStr, ";") {
		parts := strings.Split(urlStr, ";")
		if len(parts) > 0 {
			// Parse host:port
			hostPort := parts[0]
			if idx := strings.LastIndex(hostPort, ":"); idx > 0 {
				settings["host"] = hostPort[:idx]
				settings["port"] = hostPort[idx+1:]
			} else {
				settings["host"] = hostPort
			}

			// Parse additional parameters
			for i := 1; i < len(parts); i++ {
				if kv := strings.Split(parts[i], "="); len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					value := strings.TrimSpace(kv[1])
					if key == "databaseName" {
						settings["database"] = value
					} else {
						settings[key] = value
					}
				}
			}
		}
	} else {
		// Standard URL format
		// Reconstruct URL for parsing
		parseURL := "http://" + urlStr // Use http as dummy protocol
		u, err := url.Parse(parseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid JDBC URL format: %w", err)
		}

		// Extract user info
		if u.User != nil {
			settings["user"] = u.User.Username()
			if pass, ok := u.User.Password(); ok {
				settings["password"] = pass
			}
		}

		// Extract host and port
		settings["host"] = u.Hostname()
		if port := u.Port(); port != "" {
			settings["port"] = port
		}

		// Extract database name
		if path := strings.TrimPrefix(u.Path, "/"); path != "" {
			settings["database"] = path
		}

		// Extract query parameters
		for k, v := range u.Query() {
			if len(v) > 0 {
				settings[k] = v[0]
			}
		}
	}

	return settings, nil
}
