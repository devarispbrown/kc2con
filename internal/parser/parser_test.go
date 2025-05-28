package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConnectorJSON(t *testing.T) {
	parser := New()

	// Create temporary JSON file
	jsonConfig := `{
		"name": "test-connector",
		"connector.class": "io.debezium.connector.mysql.MySqlConnector",
		"tasks.max": 1,
		"database.hostname": "localhost",
		"database.port": 3306,
		"transforms": "unwrap",
		"transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState"
	}`

	tmpFile := createTempFile(t, "connector.json", jsonConfig)
	defer os.Remove(tmpFile)

	config, err := parser.ParseConnectorConfig(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "test-connector", config.Name)
	assert.Equal(t, "io.debezium.connector.mysql.MySqlConnector", config.Class)
	assert.Equal(t, 1, config.TasksMax)
	assert.Len(t, config.Transforms, 1)
	assert.Equal(t, "unwrap", config.Transforms[0].Name)
	assert.Equal(t, "io.debezium.transforms.ExtractNewRecordState", config.Transforms[0].Class)
}

func TestParseConnectorProperties(t *testing.T) {
	parser := New()

	propertiesConfig := `name=test-connector
connector.class=io.confluent.connect.jdbc.JdbcSourceConnector
tasks.max=2
connection.url=jdbc:postgresql://localhost:5432/test
mode=incrementing
incrementing.column.name=id
`

	tmpFile := createTempFile(t, "connector.properties", propertiesConfig)
	defer os.Remove(tmpFile)

	config, err := parser.ParseConnectorConfig(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "test-connector", config.Name)
	assert.Equal(t, "io.confluent.connect.jdbc.JdbcSourceConnector", config.Class)
	assert.Equal(t, 2, config.TasksMax)
	assert.Equal(t, "jdbc:postgresql://localhost:5432/test", config.Config["connection.url"])
	assert.Equal(t, "incrementing", config.Config["mode"])
}

func TestParseWorkerConfig(t *testing.T) {
	parser := New()

	workerConfig := `bootstrap.servers=localhost:9092
group.id=connect-cluster
key.converter=org.apache.kafka.connect.json.JsonConverter
value.converter=org.apache.kafka.connect.json.JsonConverter
schema.registry.url=http://localhost:8081
security.protocol=SSL
consumer.max.poll.records=500
producer.batch.size=16384
`

	tmpFile := createTempFile(t, "worker.properties", workerConfig)
	defer os.Remove(tmpFile)

	config, err := parser.ParseWorkerConfig(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "localhost:9092", config.BootstrapServers)
	assert.Equal(t, "connect-cluster", config.GroupID)
	assert.Equal(t, "http://localhost:8081", config.SchemaRegistry)
	assert.Equal(t, "SSL", config.SecurityProtocol)
	assert.Equal(t, "500", config.ConsumerOverrides["max.poll.records"])
	assert.Equal(t, "16384", config.ProducerOverrides["batch.size"])
}

func TestUnsupportedFileFormat(t *testing.T) {
	parser := New()

	tmpFile := createTempFile(t, "config.txt", "invalid format")
	defer os.Remove(tmpFile)

	_, err := parser.ParseConnectorConfig(tmpFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported file format")
}

func TestExtractSchemaRegistryConfig(t *testing.T) {
	parser := New()

	workerConfig := &WorkerConfig{
		SchemaRegistry: "http://localhost:8081",
		RawConfig: map[string]interface{}{
			"schema.registry.basic.auth.user.info": "user:pass",
		},
	}

	connectorConfig := &ConnectorConfig{
		RawConfig: map[string]interface{}{
			"key.converter.schema.registry.url": "http://backup:8081",
		},
	}

	schemaConfig := parser.ExtractSchemaRegistryConfig(workerConfig, connectorConfig)

	assert.Equal(t, "http://localhost:8081", schemaConfig.URL)
	assert.Equal(t, "user:pass", schemaConfig.AdditionalConfig["schema.registry.basic.auth.user.info"])
}

func createTempFile(t *testing.T, name, content string) string {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, name)
	err := os.WriteFile(filePath, []byte(content), 0o644)
	require.NoError(t, err)
	return filePath
}
