package mappers

import (
	"fmt"
	"strings"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/devarispbrown/kc2con/internal/registry"
)

// S3Mapper handles S3 connector mapping
type S3Mapper struct {
	BaseMapper
}

// NewS3Mapper creates a new S3 mapper
func NewS3Mapper() *S3Mapper {
	return &S3Mapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts S3 configuration
func (m *S3Mapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:s3")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// S3 settings
	sb.Required("aws.bucket", GetConfigValue(config, "s3.bucket.name"), "s3.bucket.name")
	sb.WithDefault("aws.region", GetConfigValue(config, "s3.region"), "us-east-1")

	// AWS credentials
	if accessKey := GetConfigValue(config, "aws.access.key.id"); accessKey != "" {
		sb.Optional("aws.accessKeyId", accessKey)
	}
	if secretKey := GetConfigValue(config, "aws.secret.access.key"); secretKey != "" {
		sb.Sensitive("aws.secretAccessKey", secretKey, "")
	}

	// Path configuration
	if pathPrefix := GetConfigValue(config, "topics.dir"); pathPrefix != "" {
		sb.Optional("path.prefix", pathPrefix)
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	if connectorType == "destination" {
		// Additional sink-specific settings

		// File format
		format := "json" // default
		if formatClass := GetConfigValue(config, "format.class"); formatClass != "" {
			if strings.Contains(strings.ToLower(formatClass), "json") {
				format = "json"
			} else if strings.Contains(strings.ToLower(formatClass), "avro") {
				format = "avro"
			} else if strings.Contains(strings.ToLower(formatClass), "parquet") {
				format = "parquet"
			}
		}
		settings["format"] = format

		// Partitioning
		if partitioner := GetConfigValue(config, "partitioner.class"); partitioner != "" {
			if strings.Contains(partitioner, "TimeBasedPartitioner") {
				settings["partition.format"] = "time"
				if pathFormat := GetConfigValue(config, "path.format"); pathFormat != "" {
					settings["path.format"] = pathFormat
				}
			} else if strings.Contains(partitioner, "FieldPartitioner") {
				settings["partition.format"] = "field"
				if fieldName := GetConfigValue(config, "partition.field.name"); fieldName != "" {
					settings["partition.field"] = fieldName
				}
			}
		}

		// Flush settings
		if flushSize := GetConfigValue(config, "flush.size"); flushSize != "" {
			settings["buffer.size"] = flushSize
		}
		if rotateInterval := GetConfigValue(config, "rotate.interval.ms"); rotateInterval != "" {
			settings["rotate.interval"] = rotateInterval + "ms"
		}
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// FileMapper handles file connector mapping
type FileMapper struct {
	BaseMapper
}

// NewFileMapper creates a new file mapper
func NewFileMapper() *FileMapper {
	return &FileMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts file configuration
func (m *FileMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:file")
	if err != nil {
		return nil, err
	}

	filePath := GetConfigValue(config, "file")
	if filePath == "" {
		return nil, fmt.Errorf("file path is required")
	}

	settings := map[string]interface{}{
		"path": filePath,
	}

	// Format detection
	if format := GetConfigValue(config, "format"); format != "" {
		settings["format"] = format
	}

	// For file source
	if connectorType == "source" {
		// Tail mode
		if tail := GetConfigValue(config, "tail"); tail == "true" {
			settings["tail"] = true
		}

		// Start position
		if position := GetConfigValue(config, "start.position"); position != "" {
			settings["start.position"] = position
		}
	}

	// For file sink
	if connectorType == "destination" {
		// Rotation settings
		if rotateInterval := GetConfigValue(config, "rotate.interval.ms"); rotateInterval != "" {
			settings["rotate.interval"] = rotateInterval + "ms"
		}

		if rotateSize := GetConfigValue(config, "rotate.size"); rotateSize != "" {
			settings["rotate.size"] = rotateSize
		}

		// Compression
		if compression := GetConfigValue(config, "compression"); compression != "" {
			settings["compression"] = compression
		}
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// GCSMapper handles Google Cloud Storage connector mapping
type GCSMapper struct {
	BaseMapper
}

// NewGCSMapper creates a new GCS mapper
func NewGCSMapper() *GCSMapper {
	return &GCSMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts GCS configuration
func (m *GCSMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:gcs")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// GCS settings
	sb.Required("bucket", GetConfigValue(config, "gcs.bucket.name"), "gcs.bucket.name")
	sb.Optional("project.id", GetConfigValue(config, "gcs.project.id"))

	// Credentials
	if credPath := GetConfigValue(config, "gcs.credentials.path"); credPath != "" {
		sb.Optional("credentials.path", credPath)
	} else if credJSON := GetConfigValue(config, "gcs.credentials.json"); credJSON != "" {
		sb.Sensitive("credentials.json", credJSON, "")
	}

	// Path configuration
	if pathPrefix := GetConfigValue(config, "topics.dir"); pathPrefix != "" {
		sb.Optional("path.prefix", pathPrefix)
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	// Format and partitioning (similar to S3)
	if connectorType == "destination" {
		format := "json"
		if formatClass := GetConfigValue(config, "format.class"); formatClass != "" {
			if strings.Contains(strings.ToLower(formatClass), "avro") {
				format = "avro"
			} else if strings.Contains(strings.ToLower(formatClass), "parquet") {
				format = "parquet"
			}
		}
		settings["format"] = format
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}

// AzureBlobMapper handles Azure Blob Storage connector mapping
type AzureBlobMapper struct {
	BaseMapper
}

// NewAzureBlobMapper creates a new Azure Blob mapper
func NewAzureBlobMapper() *AzureBlobMapper {
	return &AzureBlobMapper{
		BaseMapper: *NewBaseMapper(),
	}
}

// Map converts Azure Blob configuration
func (m *AzureBlobMapper) Map(config *parser.ConnectorConfig, info registry.ConnectorInfo, connectorType string) (*ConduitPipeline, error) {
	pipeline, err := m.CreateBasePipeline(config, connectorType, "builtin:azure-blob")
	if err != nil {
		return nil, err
	}

	sb := NewSettingsBuilder()

	// Azure settings
	sb.Required("container", GetConfigValue(config, "azblob.container.name"), "azblob.container.name")
	sb.Required("account", GetConfigValue(config, "azblob.account.name"), "azblob.account.name")

	// Authentication
	if accountKey := GetConfigValue(config, "azblob.account.key"); accountKey != "" {
		sb.Sensitive("account.key", accountKey, "")
	} else if sasToken := GetConfigValue(config, "azblob.sas.token"); sasToken != "" {
		sb.Sensitive("sas.token", sasToken, "")
	}

	// Path configuration
	if pathPrefix := GetConfigValue(config, "topics.dir"); pathPrefix != "" {
		sb.Optional("path.prefix", pathPrefix)
	}

	settings, err := sb.Build()
	if err != nil {
		return nil, err
	}

	pipeline.Connectors[0].Settings = settings
	return pipeline, nil
}