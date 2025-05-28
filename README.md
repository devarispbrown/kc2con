# kc2con - Kafka Connect to Conduit Migration Tool

🚀 **Easily migrate your Kafka Connect configurations to Conduit pipelines.**

[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Status](https://img.shields.io/badge/status-production--ready-brightgreen)](https://github.com/devarispbrown/kc2con)

kc2con is a comprehensive migration tool that helps you transition from Kafka Connect to [Conduit](https://conduit.io), a modern data integration platform that doesn't require JVM and offers superior performance.

## Features

✅ **Comprehensive Analysis**
- Parse JSON and Properties configuration files
- 25+ built-in connector compatibility mappings
- Cross-connector validation and conflict detection
- Performance analysis and resource estimation
- Security configuration validation
- Schema registry consistency checks

✅ **Automated Migration**
- Generate Conduit pipeline configurations from Kafka Connect configs
- Intelligent configuration mapping for common connectors
- Transform Kafka Connect SMTs to Conduit processors
- Support for Kafka Connect wrapper for unsupported connectors
- Deployment scripts and migration guides

✅ **Migration Planning**
- Dependency-aware migration ordering
- Migration readiness scoring (0-100%)
- Effort estimation and timeline planning
- Bottleneck identification and mitigation
- Actionable recommendations

✅ **Beautiful CLI**
- Colorful, easy-to-read output
- Multiple output formats (table/JSON/YAML)
- Detailed issue categorization
- Progress tracking and validation

## Quick Start

```bash
# Install
git clone https://github.com/devarispbrown/kc2con
cd kc2con
go mod tidy
go build -o kc2con

# Analyze your Kafka Connect configurations
./kc2con analyze --config-dir ./kafka-connect-configs

# Migrate to Conduit
./kc2con migrate --config-dir ./kafka-connect-configs --output ./conduit-configs

# Check connector compatibility
./kc2con connectors list
```

## Commands

### `analyze` - Analyze Kafka Connect configurations
Provides detailed analysis of your Kafka Connect setup before migration.

```bash
./kc2con analyze --config-dir ./kafka-connect-configs

# Output in different formats
./kc2con analyze --config-dir ./configs --format json
./kc2con analyze --config-dir ./configs --format yaml
```

### `migrate` - Generate Conduit configurations
Converts Kafka Connect configurations to Conduit pipeline YAML files.

```bash
# Basic migration
./kc2con migrate --config-dir ./kafka-connect --output ./conduit-configs

# Dry run to preview changes
./kc2con migrate --config-dir ./kafka-connect --dry-run

# Migrate specific connectors only
./kc2con migrate --config-dir ./kafka-connect \
  --connectors mysql-source,postgres-sink

# Handle unsupported connectors with wrapper
./kc2con migrate --config-dir ./kafka-connect --generate-wrapper
```

Options:
- `--dry-run`: Preview what would be migrated without creating files
- `--skip-unsupported`: Skip unsupported connectors instead of failing
- `--generate-wrapper`: Use Kafka Connect wrapper for unsupported connectors
- `--connectors`: Comma-separated list of specific connectors to migrate

### `connectors` - Manage connector registry
View and manage the connector compatibility registry.

```bash
# List all connectors
./kc2con connectors list

# List by category
./kc2con connectors list --category databases

# Show detailed information
./kc2con connectors list --details

# Add a new connector mapping
./kc2con connectors add

# Validate registry
./kc2con connectors validate
```

### `compatibility` - Show compatibility matrix
Display which Kafka Connect connectors are supported in Conduit.

```bash
# Show full compatibility matrix
./kc2con compatibility

# Check specific connector
./kc2con compatibility --connector mysql
```

## Supported Connectors

### ✅ Fully Supported (Direct Migration)
- **Databases**: MySQL CDC, PostgreSQL CDC, JDBC Source/Sink
- **Cloud Storage**: S3, Google Cloud Storage, Azure Blob
- **Messaging**: Kafka MirrorMaker, HTTP/REST
- **Search**: Elasticsearch, OpenSearch
- **Files**: FileStream Source/Sink

### ⚠️ Partially Supported (Some Manual Config)
- **Databases**: SQL Server CDC, MongoDB, Cassandra
- **Analytics**: BigQuery, Redshift, Snowflake
- **Monitoring**: Splunk, Datadog

### 🔧 Wrapper Support
For unsupported connectors, kc2con can generate configurations using the [Kafka Connect wrapper](https://conduit.io/docs/using/connectors/kafka-connect-connector/).

## Migration Output

Running migration creates the following structure:

```
conduit-configs/
├── pipelines/              # Conduit pipeline YAML files
│   ├── mysql-source.yaml
│   ├── postgres-sink.yaml
│   └── s3-backup.yaml
├── deploy-pipelines.sh     # Deployment script
├── MIGRATION_GUIDE.md      # Detailed migration guide
└── README.md              # Quick start guide
```

### Example: MySQL CDC Migration

**Input** (Kafka Connect):
```json
{
  "name": "mysql-source",
  "connector.class": "io.debezium.connector.mysql.MySqlConnector",
  "database.hostname": "mysql.example.com",
  "database.port": "3306",
  "database.user": "debezium",
  "database.password": "dbz123",
  "table.include.list": "inventory.customers"
}
```

**Output** (Conduit):
```yaml
version: "2.2"
pipelines:
  - id: mysql-source
    status: stopped
    connectors:
      - id: mysql-source-source
        type: source
        plugin: builtin:mysql
        settings:
          host: mysql.example.com
          port: "3306"
          user: debezium
          password: dbz123
          tables: inventory.customers
```

## Configuration Mapping

kc2con intelligently maps Kafka Connect settings to Conduit equivalents:

| Kafka Connect | Conduit | Notes |
|--------------|---------|-------|
| `database.hostname` | `host` | Common database connectors |
| `connection.url` | Parsed to `host`, `port`, `database` | JDBC connectors |
| `s3.bucket.name` | `aws.bucket` | S3 connector |
| `transforms.*` | `processors` | SMT to processor mapping |

## Architecture

```
kc2con/
├── cmd/                    # CLI commands
│   ├── analyze.go         # Analysis command
│   ├── migrate.go         # Migration command
│   ├── connectors.go      # Registry management
│   └── compatibility.go   # Compatibility matrix
├── internal/
│   ├── analyzer/          # Configuration analysis
│   ├── generator/         # Pipeline generation
│   ├── parser/            # Config file parsing
│   ├── registry/          # Connector registry
│   └── compatibility/     # Compatibility checking
└── connectors.yaml        # Connector mappings
```

## Development

### Prerequisites
- Go 1.21+
- Make (optional)

### Building
```bash
go mod tidy
go build -o kc2con
```

### Testing
```bash
go test ./...
```

### Adding Custom Connectors

1. **Via YAML Configuration**:
   Edit `internal/registry/connectors.yaml`:
   ```yaml
   connectors:
     - kafka_connect_class: "com.example.CustomConnector"
       name: "Custom Connector"
       conduit_equivalent: "custom-plugin"
       status: "supported"
       required_fields:
         - "connection.url"
         - "api.key"
   ```

2. **Via CLI**:
   ```bash
   ./kc2con connectors add
   ```

3. **Programmatically**:
   Implement a `ConfigMapperFunc` in your registry for custom mapping logic.

## Troubleshooting

### Common Issues

1. **"Unsupported connector" errors**
   - Use `--skip-unsupported` to skip them
   - Use `--generate-wrapper` for Kafka Connect wrapper
   - Add custom mapping to the registry

2. **Configuration parsing errors**
   - Ensure JSON files are valid
   - Check properties files format
   - Verify file permissions

3. **Missing required fields**
   - Review the analysis output for missing fields
   - Check connector documentation for requirements
   - Add missing fields to source configuration

### Debug Mode
```bash
# Enable verbose logging
./kc2con analyze --config-dir ./configs --verbose
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Areas for Contribution
- Additional connector mappings
- Enhanced configuration transformations
- Validation rules
- Documentation improvements
- Test coverage

## Roadmap

- [x] Configuration analysis
- [x] Basic migration functionality
- [x] Connector registry with 25+ mappings
- [x] Transform/SMT support
- [ ] Advanced validation
- [ ] Schema registry migration
- [ ] Performance optimization recommendations
- [ ] Cloud-specific configurations
- [ ] Integration tests

## Resources

- [Conduit Documentation](https://conduit.io/docs)
- [Connector List](https://conduit.io/docs/connectors/list)
- [Kafka Connect Documentation](https://docs.confluent.io/platform/current/connect/)
- [Migration Best Practices](./docs/MIGRATION.md)

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Support

- 🐛 [Report Issues](https://github.com/devarispbrown/kc2con/issues)
- 💬 [Discussions](https://github.com/devarispbrown/kc2con/discussions)
- 📧 Contact: [your-email@example.com]

---

Made with ❤️ by the kc2con team. Happy migrating! 🚀