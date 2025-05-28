# Migration Guide: Kafka Connect to Conduit

This guide explains how to use kc2con's migration features to convert your Kafka Connect configurations to Conduit pipelines.

## Table of Contents

- [Overview](#overview)
- [Migration Process](#migration-process)
- [Connector Mapping](#connector-mapping)
- [Transform Mapping](#transform-mapping)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Overview

The migration feature in kc2con automates the conversion of Kafka Connect configurations to Conduit pipeline configurations. It handles:

- ✅ Connector configuration mapping
- ✅ Transform (SMT) conversion to processors
- ✅ Security and authentication settings
- ✅ Schema registry configuration
- ✅ Batch migration of multiple connectors
- ✅ Validation of generated configurations

## Migration Process

### 1. Basic Migration

Convert a single connector configuration:

```bash
# Migrate a specific connector
kc2con migrate --config-dir ./kafka-connect-configs --output ./conduit-configs

# Dry run to preview changes
kc2con migrate --config-dir ./kafka-connect-configs --dry-run

# Migrate specific connectors only
kc2con migrate --config-dir ./kafka-connect-configs \
  --output ./conduit-configs \
  --connectors mysql-source,s3-sink
```

### 2. Validate Generated Configurations

After migration, validate the generated Conduit configurations:

```bash
# Validate all generated configs
kc2con validate --config-dir ./conduit-configs

# Strict validation mode
kc2con validate --config-dir ./conduit-configs --strict
```

### 3. Deploy to Conduit

The migration tool generates deployment scripts for easy deployment:

```bash
# Import pipelines to Conduit
./conduit-configs/import-pipelines.sh

# Validate running pipelines
./conduit-configs/validate-pipelines.sh
```

## Connector Mapping

### Supported Connectors

The following connectors have direct mappings to Conduit:

| Kafka Connect Connector | Conduit Plugin | Status | Notes |
|------------------------|----------------|---------|-------|
| Debezium MySQL | `builtin:mysql` | ✅ Supported | Full CDC support |
| Debezium PostgreSQL | `builtin:postgres` | ✅ Supported | Logical replication |
| JDBC Source | `builtin:postgres/mysql` | ✅ Supported | Database-specific |
| JDBC Sink | `builtin:postgres/mysql` | ✅ Supported | Insert/upsert/delete |
| S3 Sink | `builtin:s3` | ✅ Supported | JSON/Avro/Parquet |
| Elasticsearch Sink | `builtin:elasticsearch` | ✅ Supported | Bulk operations |
| MongoDB Source/Sink | `builtin:mongo` | ⚠️ Partial | Change streams |
| HTTP Sink | `builtin:http` | ✅ Supported | REST API integration |
| File Source/Sink | `builtin:file` | ✅ Supported | File I/O |

### Configuration Mapping

#### Debezium MySQL Example

**Kafka Connect Configuration:**
```json
{
  "connector.class": "io.debezium.connector.mysql.MySqlConnector",
  "database.hostname": "mysql.example.com",
  "database.port": "3306",
  "database.user": "debezium",
  "database.password": "secret",
  "database.server.name": "mysql-server",
  "table.include.list": "inventory.products"
}
```

**Migrated Conduit Configuration:**
```yaml
connectors:
  - id: mysql-source
    type: source
    plugin: builtin:mysql
    settings:
      host: mysql.example.com
      port: "3306"
      user: debezium
      password: secret
      tables: inventory.products
      cdc.enabled: true
```

#### JDBC Source Example

**Kafka Connect Configuration:**
```json
{
  "connector.class": "io.confluent.connect.jdbc.JdbcSourceConnector",
  "connection.url": "jdbc:postgresql://localhost:5432/mydb",
  "mode": "incrementing",
  "incrementing.column.name": "id",
  "topic.prefix": "postgres-"
}
```

**Migrated Conduit Configuration:**
```yaml
connectors:
  - id: jdbc-source
    type: source
    plugin: builtin:postgres
    settings:
      host: localhost
      port: "5432"
      database: mydb
      cdc.mode: auto_incrementing
      incrementing.column: id
```

## Transform Mapping

Kafka Connect Single Message Transforms (SMTs) are mapped to Conduit processors:

| Kafka Connect SMT | Conduit Processor | Notes |
|-------------------|-------------------|-------|
| RegexRouter | `builtin:field.rename` | Pattern-based routing |
| TimestampConverter | `builtin:field.convert` | Type conversion |
| InsertField | `builtin:field.set` | Add fields |
| MaskField | `builtin:field.mask` | Data masking |
| Filter | `builtin:filter.field` | Record filtering |
| ExtractNewRecordState | `builtin:unwrap.debezium` | CDC unwrapping |

### Transform Example

**Kafka Connect Configuration:**
```json
{
  "transforms": "unwrap,route",
  "transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState",
  "transforms.route.type": "org.apache.kafka.connect.transforms.RegexRouter",
  "transforms.route.regex": "([^.]+)\\.([^.]+)\\.([^.]+)",
  "transforms.route.replacement": "$3"
}
```

**Migrated Conduit Configuration:**
```yaml
processors:
  - id: processor-0
    plugin: builtin:unwrap.debezium
  - id: processor-1
    plugin: builtin:field.rename
    settings:
      regex: "([^.]+)\\.([^.]+)\\.([^.]+)"
      replacement: "$3"
```

## Examples

### Example 1: MySQL CDC to S3

```bash
# Original Kafka Connect setup:
# - MySQL CDC source (Debezium)
# - S3 sink

# Migrate both connectors
kc2con migrate --config-dir ./kafka-connect \
  --output ./conduit-pipelines \
  --connectors mysql-cdc,s3-sink

# Result: Two Conduit pipelines
# - mysql-cdc-pipeline.yaml
# - s3-sink-pipeline.yaml
```

### Example 2: PostgreSQL to Elasticsearch

```bash
# Migrate with validation
kc2con migrate --config-dir ./kafka-connect \
  --output ./conduit-pipelines \
  --validate

# The tool will:
# 1. Convert PostgreSQL source config
# 2. Convert Elasticsearch sink config
# 3. Map any transforms to processors
# 4. Validate the generated configs
# 5. Generate deployment scripts
```

### Example 3: Batch Migration

```bash
# Migrate all connectors in a directory
kc2con migrate --config-dir ./production-connectors \
  --output ./migrated-pipelines

# Check the migration summary
cat ./migrated-pipelines/migration-summary.txt
```

## Troubleshooting

### Common Issues

1. **Unsupported Connector**
   ```
   Error: connector class not found in registry: com.custom.MyConnector
   ```
   **Solution:** Add custom mapping or use generic mapper with manual adjustments.

2. **Missing Required Fields**
   ```
   Error: Required field missing for migration: database.hostname
   ```
   **Solution:** Ensure all required fields are present in the source configuration.

3. **Transform Not Supported**
   ```
   Warning: Unknown transform: com.custom.MyTransform
   ```
   **Solution:** Implement custom processor or adjust manually after migration.

### Manual Adjustments

Some configurations may require manual adjustments after migration:

1. **Security Settings**: Review and update authentication methods
2. **Custom Transforms**: Implement as custom Conduit processors
3. **Advanced Features**: Some connector-specific features may need adaptation

### Getting Help

- Run `kc2con migrate --help` for command options
- Check connector compatibility: `kc2con compatibility --connector <name>`
- Analyze before migrating: `kc2con analyze --config-dir <dir>`

## Best Practices

1. **Test First**: Always test migrations in a development environment
2. **Validate**: Use the validation command to check generated configs
3. **Review**: Manually review security settings and credentials
4. **Incremental**: Migrate and test one connector at a time for complex setups
5. **Backup**: Keep backups of original Kafka Connect configurations

## Next Steps

After successful migration:

1. Review generated pipeline configurations
2. Update placeholder values and credentials
3. Test pipelines with sample data
4. Monitor pipeline performance
5. Gradually transition from Kafka Connect to Conduit

For more information, see:
- [Conduit Documentation](https://conduit.io/docs)
- [Pipeline Configuration Guide](https://conduit.io/docs/using/pipelines)
- [Connector Reference](https://conduit.io/docs/using/connectors)