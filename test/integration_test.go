// test/integration_test.go (moved to test/ directory, not test/integration/)
//go:build integration
// +build integration

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var binaryPath string

// TestMain builds the binary before running tests
func TestMain(m *testing.M) {
	// Build the binary with coverage support
	if err := buildBinary(); err != nil {
		fmt.Printf("Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	cleanup()
	os.Exit(code)
}

func buildBinary() error {
	binaryName := "kc2con"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	// Build with coverage support
	cmd := exec.Command("go", "build", "-cover", "-o", binaryName, "..")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Get absolute path
	var err error
	binaryPath, err = filepath.Abs(binaryName)
	return err
}

func cleanup() {
	if binaryPath != "" {
		os.Remove(binaryPath)
	}
	os.RemoveAll("coverage")
}

func TestVersion(t *testing.T) {
	output, err := runCommand(t, "--version")
	require.NoError(t, err)
	assert.Contains(t, output, "kc2con version")
}

func TestHelp(t *testing.T) {
	output, err := runCommand(t, "--help")
	require.NoError(t, err)
	assert.Contains(t, output, "kc2con - Kafka Connect to Conduit Migration Tool")
	assert.Contains(t, output, "analyze")
	assert.Contains(t, output, "compatibility")
	assert.Contains(t, output, "connectors")
}

// Analyze Command Tests
func TestAnalyzeCommand(t *testing.T) {
	testDataDir := setupTestData(t)

	t.Run("BasicAnalysis", func(t *testing.T) {
		output, err := runCommand(t, "analyze", "--config-dir", testDataDir)
		require.NoError(t, err)
		
		assert.Contains(t, output, "Migration Analysis")
		assert.Contains(t, output, "connector configurations")
		assert.Contains(t, output, "Migration readiness")
	})

	t.Run("JSONOutput", func(t *testing.T) {
		output, err := runCommand(t, "analyze", "--config-dir", testDataDir, "--format", "json")
		require.NoError(t, err)
		
		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "Output should be valid JSON")
		
		assert.Contains(t, result, "connectors")
		assert.Contains(t, result, "migration_plan")
		assert.Contains(t, result, "estimated_effort")
	})

	t.Run("YAMLOutput", func(t *testing.T) {
		output, err := runCommand(t, "analyze", "--config-dir", testDataDir, "--format", "yaml")
		require.NoError(t, err)
		
		var result map[string]interface{}
		err = yaml.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "Output should be valid YAML")
		
		assert.Contains(t, result, "connectors")
		
		assert.Contains(t, result, "migrationplan")
	})
	t.Run("NonExistentDirectory", func(t *testing.T) {
		_, err := runCommand(t, "analyze", "--config-dir", "nonexistent")
		assert.Error(t, err, "Should fail with non-existent directory")
	})

	t.Run("EmptyDirectory", func(t *testing.T) {
		emptyDir := t.TempDir()

		output, err := runCommand(t, "analyze", "--config-dir", emptyDir)
		require.NoError(t, err)
		assert.Contains(t, output, "Warning: No configuration files found")
	})
}

// Compatibility Command Tests
func TestCompatibilityCommand(t *testing.T) {
	t.Run("ShowAllConnectors", func(t *testing.T) {
		output, err := runCommand(t, "compatibility")
		require.NoError(t, err)
		
		assert.Contains(t, output, "Compatibility Matrix")
		assert.Contains(t, output, "MySQL CDC")
		assert.Contains(t, output, "PostgreSQL CDC")
		assert.Contains(t, output, "Summary:")
	})

	t.Run("SpecificConnector", func(t *testing.T) {
		output, err := runCommand(t, "compatibility", "--connector", "mysql")
		require.NoError(t, err)
		
		assert.Contains(t, output, "MySQL CDC")
		assert.Contains(t, output, "Kafka Connect Class:")
		assert.Contains(t, output, "Conduit Equivalent:")
		assert.Contains(t, output, "Status:")
	})

	t.Run("NonExistentConnector", func(t *testing.T) {
		_, err := runCommand(t, "compatibility", "--connector", "nonexistent-connector")
		assert.Error(t, err, "Should fail for non-existent connector")
	})

	t.Run("CustomConfig", func(t *testing.T) {
		customConfigPath := createCustomConnectorConfig(t)

		output, err := runCommand(t, "compatibility", "--config", customConfigPath)
		require.NoError(t, err)
		assert.Contains(t, output, "Compatibility Matrix")
	})
}

// Connectors Command Tests
func TestConnectorsCommand(t *testing.T) {
	t.Run("ListAllConnectors", func(t *testing.T) {
		output, err := runCommand(t, "connectors", "list")
		require.NoError(t, err)
		
		assert.Contains(t, output, "Conduit Connector Registry")
		assert.Contains(t, output, "Database Connectors")
		assert.Contains(t, output, "Summary:")
	})

	t.Run("ListByCategory", func(t *testing.T) {
		output, err := runCommand(t, "connectors", "list", "--category", "databases")
		require.NoError(t, err)
		
		assert.Contains(t, output, "Database Connectors")
		assert.Contains(t, output, "MySQL CDC")
		assert.Contains(t, output, "PostgreSQL CDC")
	})

	t.Run("ListWithDetails", func(t *testing.T) {
		output, err := runCommand(t, "connectors", "list", "--details")
		require.NoError(t, err)
		
		assert.Contains(t, output, "Class:")
		assert.Contains(t, output, "Status:")
		assert.Contains(t, output, "Effort:")
	})

	t.Run("InvalidCategory", func(t *testing.T) {
		_, err := runCommand(t, "connectors", "list", "--category", "invalid")
		assert.Error(t, err, "Should fail for invalid category")
	})

	t.Run("ValidateRegistry", func(t *testing.T) {
		output, err := runCommand(t, "connectors", "validate")
		require.NoError(t, err)
		
		assert.Contains(t, output, "Validating Connector Registry")
		assert.Contains(t, output, "Configuration loaded successfully")
	})
}

// Error Handling Tests
func TestErrorHandling(t *testing.T) {
	t.Run("InvalidCommand", func(t *testing.T) {
		_, err := runCommand(t, "invalid-command")
		assert.Error(t, err)
	})

	t.Run("MissingRequiredFlag", func(t *testing.T) {
		_, err := runCommand(t, "analyze") // Missing --config-dir
		assert.Error(t, err)
	})

	t.Run("InvalidOutputFormat", func(t *testing.T) {
		testDataDir := setupTestData(t)
		_, err := runCommand(t, "analyze", "--config-dir", testDataDir, "--format", "invalid")
		assert.Error(t, err)
	})
}

// End-to-End Workflow Tests
func TestEndToEndWorkflow(t *testing.T) {
	testDataDir := setupTestData(t)

	t.Run("CompleteAnalysisWorkflow", func(t *testing.T) {
		// 1. Check compatibility first
		compatOutput, err := runCommand(t, "compatibility")
		require.NoError(t, err)
		assert.Contains(t, compatOutput, "Compatibility Matrix")

		// 2. List available connectors
		listOutput, err := runCommand(t, "connectors", "list")
		require.NoError(t, err)
		assert.Contains(t, listOutput, "Connector Registry")

		// 3. Analyze existing configuration
		analyzeOutput, err := runCommand(t, "analyze", "--config-dir", testDataDir)
		require.NoError(t, err)
		assert.Contains(t, analyzeOutput, "Migration Analysis")

		// 4. Get JSON output for programmatic use
		jsonOutput, err := runCommand(t, "analyze", "--config-dir", testDataDir, "--format", "json")
		require.NoError(t, err)
		
		var result map[string]interface{}
		err = json.Unmarshal([]byte(jsonOutput), &result)
		require.NoError(t, err)
		assert.Contains(t, result, "migration_plan")
	})
}

// Helper functions
func runCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	
	cmd := exec.Command(binaryPath, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	
	// Set coverage environment for CLI binary
	cmd.Env = append(os.Environ(), "GOCOVERDIR=./coverage")
	
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %v, stderr: %s", err, stderr.String())
	}
	
	return out.String(), nil
}

func setupTestData(t *testing.T) string {
	t.Helper()
	
	testDataDir := t.TempDir()

	// Create test connector configs
	createJSONTestFile(t, testDataDir)
	createPropertiesTestFile(t, testDataDir)
	createWorkerTestFile(t, testDataDir)
	
	return testDataDir
}

func createJSONTestFile(t *testing.T, dir string) {
	t.Helper()
	
	config := map[string]interface{}{
		"name":           "test-mysql-connector",
		"connector.class": "io.debezium.connector.mysql.MySqlConnector",
		"tasks.max":      1,
		"database.hostname": "localhost",
		"database.port":     3306,
		"database.user":     "debezium",
		"database.password": "dbz",
		"database.server.name": "myserver",
		"database.include.list": "inventory",
		"transforms": "unwrap",
		"transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState",
	}

	data, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "mysql-connector.json"), data, 0644)
	require.NoError(t, err)
}

func createPropertiesTestFile(t *testing.T, dir string) {
	t.Helper()
	
	properties := `name=properties-test-connector
connector.class=io.confluent.connect.jdbc.JdbcSourceConnector
tasks.max=1
connection.url=jdbc:postgresql://localhost:5432/testdb
connection.user=testuser
connection.password=testpass
mode=incrementing
incrementing.column.name=id
topic.prefix=test-
`
	err := os.WriteFile(filepath.Join(dir, "jdbc-connector.properties"), []byte(properties), 0644)
	require.NoError(t, err)
}

func createWorkerTestFile(t *testing.T, dir string) {
	t.Helper()
	
	workerConfig := `bootstrap.servers=localhost:9092
group.id=connect-cluster
key.converter=org.apache.kafka.connect.json.JsonConverter
value.converter=org.apache.kafka.connect.json.JsonConverter
key.converter.schemas.enable=false
value.converter.schemas.enable=false
offset.storage.topic=connect-offsets
config.storage.topic=connect-configs
status.storage.topic=connect-status
schema.registry.url=http://localhost:8081
`
	err := os.WriteFile(filepath.Join(dir, "connect-worker.properties"), []byte(workerConfig), 0644)
	require.NoError(t, err)
}

func createCustomConnectorConfig(t *testing.T) string {
	t.Helper()
	
	config := `
version: "1.0"
description: "Test Connector Registry"
updated: "2024-12-19"

categories:
  test:
    name: "Test Connectors"
    description: "Test connector category"

connectors:
  - kafka_connect_class: "com.test.TestConnector"
    name: "Test Custom Connector"
    category: "test"
    conduit_equivalent: "test-connector"
    conduit_type: "source"
    status: "supported"
    confidence: "high"
    estimated_effort: "30 minutes"
    notes: "Test connector for integration tests"

transforms: []
`

	tmpFile := filepath.Join(t.TempDir(), "test-connectors.yaml")
	err := os.WriteFile(tmpFile, []byte(config), 0644)
	require.NoError(t, err)
	return tmpFile
}