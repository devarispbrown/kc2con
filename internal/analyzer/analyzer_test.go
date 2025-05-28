package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnalyzer(t *testing.T) {
	tmpDir := t.TempDir()

	analyzer, err := NewAnalyzer(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, tmpDir, analyzer.configDir)

	// Test with empty string
	_, err = NewAnalyzer("")
	assert.Error(t, err)

	// Test with non-existent directory
	_, err = NewAnalyzer("/nonexistent/path")
	assert.Error(t, err)
}

func TestAnalyze(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	createTestConnectorConfig(t, tmpDir)
	createTestWorkerConfig(t, tmpDir)

	analyzer, err := NewAnalyzer(tmpDir)
	require.NoError(t, err)

	result, err := analyzer.Analyze()
	require.NoError(t, err)

	assert.Greater(t, len(result.Connectors), 0)
	assert.NotNil(t, result.WorkerConfig)
	assert.Greater(t, result.MigrationPlan.TotalConnectors, 0)
	assert.NotEmpty(t, result.EstimatedEffort)
}

func TestResultOutputJSON(t *testing.T) {
	result := &Result{
		Connectors: []ConnectorInfo{
			{
				Name:           "test-connector",
				Type:           "source",
				Class:          "com.test.TestConnector",
				ConduitMapping: "test",
				Status:         "supported",
			},
		},
		MigrationPlan: MigrationPlan{
			TotalConnectors: 1,
			DirectMigration: 1,
		},
		EstimatedEffort: "30 minutes",
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := result.OutputJSON()
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(output), &parsed)
	assert.NoError(t, err)
	assert.Contains(t, parsed, "connectors")
}

func TestDetermineConnectorType(t *testing.T) {
	assert.Equal(t, "source", determineConnectorType("io.debezium.connector.mysql.MySqlConnector"))
	assert.Equal(t, "source", determineConnectorType("com.example.SourceConnector"))
	assert.Equal(t, "sink", determineConnectorType("io.confluent.connect.s3.S3SinkConnector"))
	assert.Equal(t, "unknown", determineConnectorType("com.example.UnknownConnector"))
}

func TestIsKnownTransform(t *testing.T) {
	assert.True(t, isKnownTransform("org.apache.kafka.connect.transforms.RegexRouter"))
	assert.True(t, isKnownTransform("io.debezium.transforms.ExtractNewRecordState"))
	assert.False(t, isKnownTransform("com.example.UnknownTransform"))
}

func createTestConnectorConfig(t *testing.T, dir string) {
	config := map[string]interface{}{
		"name":                 "test-connector",
		"connector.class":      "io.debezium.connector.mysql.MySqlConnector",
		"tasks.max":            1,
		"database.hostname":    "localhost",
		"database.server.name": "test",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	path := filepath.Join(dir, "test-connector.json")
	err = os.WriteFile(path, data, 0644)
	require.NoError(t, err)
}

func createTestWorkerConfig(t *testing.T, dir string) {
	config := `bootstrap.servers=localhost:9092
group.id=connect-cluster
key.converter=org.apache.kafka.connect.json.JsonConverter
value.converter=org.apache.kafka.connect.json.JsonConverter
`

	path := filepath.Join(dir, "worker.properties")
	err := os.WriteFile(path, []byte(config), 0644)
	require.NoError(t, err)
}
