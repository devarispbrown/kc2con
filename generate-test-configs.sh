#!/bin/bash

# Generate Test Kafka Connect Configurations
echo "🔧 Creating test Kafka Connect configurations..."

# Create the directory
mkdir -p test-configs

# 1. MySQL CDC Source (JSON) - Fully supported
cat > test-configs/mysql-cdc-source.json << 'EOFINNER'
{
  "name": "mysql-cdc-orders",
  "connector.class": "io.debezium.connector.mysql.MySqlConnector",
  "tasks.max": 1,
  "database.hostname": "mysql-server",
  "database.port": 3306,
  "database.user": "debezium",
  "database.password": "secret123",
  "database.server.id": 184054,
  "database.server.name": "mysql-prod",
  "database.include.list": "inventory",
  "table.include.list": "inventory.orders,inventory.customers",
  "key.converter": "org.apache.kafka.connect.json.JsonConverter",
  "value.converter": "org.apache.kafka.connect.json.JsonConverter",
  "transforms": "unwrap",
  "transforms.unwrap.type": "io.debezium.transforms.ExtractNewRecordState"
}
EOFINNER

# 2. PostgreSQL JDBC Source (Properties) - Supported
cat > test-configs/postgres-source.properties << 'EOFINNER'
name=postgres-users-source
connector.class=io.confluent.connect.jdbc.JdbcSourceConnector
tasks.max=2
connection.url=jdbc:postgresql://postgres:5432/userdb
connection.user=connect_user
connection.password=secret123
mode=incrementing
incrementing.column.name=id
topic.prefix=postgres-
table.whitelist=users,profiles
poll.interval.ms=5000
key.converter=org.apache.kafka.connect.storage.StringConverter
value.converter=org.apache.kafka.connect.json.JsonConverter
EOFINNER

# 3. Elasticsearch Sink (JSON) - Supported
cat > test-configs/elasticsearch-sink.json << 'EOFINNER'
{
  "name": "elasticsearch-orders-sink",
  "connector.class": "io.confluent.connect.elasticsearch.ElasticsearchSinkConnector",
  "tasks.max": 2,
  "topics": "mysql-prod.inventory.orders,postgres-users",
  "connection.url": "http://elasticsearch:9200",
  "type.name": "_doc",
  "key.ignore": false,
  "schema.ignore": true,
  "key.converter": "org.apache.kafka.connect.storage.StringConverter",
  "value.converter": "org.apache.kafka.connect.json.JsonConverter",
  "transforms": "routeRecords",
  "transforms.routeRecords.type": "org.apache.kafka.connect.transforms.RegexRouter",
  "transforms.routeRecords.regex": "([^.]+)\\.(.+)",
  "transforms.routeRecords.replacement": "$2"
}
EOFINNER

# 4. S3 Sink (Properties) - Supported
cat > test-configs/s3-sink.properties << 'EOFINNER'
name=s3-data-lake-sink
connector.class=io.confluent.connect.s3.S3SinkConnector
tasks.max=3
topics.regex=mysql-prod.*
s3.bucket.name=data-lake-raw
s3.region=us-west-2
flush.size=1000
rotate.interval.ms=60000
timezone=UTC
key.converter=org.apache.kafka.connect.storage.StringConverter
value.converter=org.apache.kafka.connect.json.JsonConverter
transforms=flatten
transforms.flatten.type=org.apache.kafka.connect.transforms.Flatten$Value
transforms.flatten.delimiter=_
EOFINNER

# 5. MongoDB Source (JSON) - Partial support
cat > test-configs/mongodb-source.json << 'EOFINNER'
{
  "name": "mongodb-products-source",
  "connector.class": "com.mongodb.kafka.connect.MongoSourceConnector",
  "tasks.max": 1,
  "connection.uri": "mongodb://mongo-user:secret123@mongodb:27017/products?authSource=admin",
  "database": "products", 
  "collection": "catalog",
  "copy.existing": true,
  "poll.max.batch.size": 1000,
  "key.converter": "org.apache.kafka.connect.json.JsonConverter",
  "value.converter": "org.apache.kafka.connect.json.JsonConverter"
}
EOFINNER

# 6. Custom/Unknown Connector - Manual migration needed
cat > test-configs/custom-connector.properties << 'EOFINNER'
name=custom-analytics-connector
connector.class=com.mycompany.analytics.CustomAnalyticsConnector
tasks.max=1
analytics.endpoint=https://analytics.mycompany.com/api
api.key=secret-key-123
batch.size=500
flush.interval=30000
key.converter=org.apache.kafka.connect.storage.StringConverter
value.converter=org.apache.kafka.connect.json.JsonConverter
EOFINNER

# 7. File Source (JSON) - Supported
cat > test-configs/file-source.json << 'EOFINNER'
{
  "name": "log-file-source",
  "connector.class": "org.apache.kafka.connect.file.FileStreamSourceConnector",
  "tasks.max": 1,
  "file": "/var/log/application.log",
  "topic": "application-logs",
  "key.converter": "org.apache.kafka.connect.storage.StringConverter",
  "value.converter": "org.apache.kafka.connect.storage.StringConverter"
}
EOFINNER

# 8. Worker Configuration
cat > test-configs/connect-distributed.properties << 'EOFINNER'
bootstrap.servers=kafka1:9092,kafka2:9092,kafka3:9092
group.id=connect-cluster
config.storage.topic=connect-configs
config.storage.replication.factor=3
offset.storage.topic=connect-offsets
offset.storage.replication.factor=3
status.storage.topic=connect-status
status.storage.replication.factor=3
key.converter=org.apache.kafka.connect.json.JsonConverter
value.converter=org.apache.kafka.connect.json.JsonConverter
schema.registry.url=http://schema-registry:8081
rest.port=8083
plugin.path=/usr/share/java,/usr/share/confluent-hub-components
EOFINNER

echo "✅ Test configurations created successfully!"
echo ""
echo "📁 Created $(ls test-configs/ | wc -l) configuration files:"
ls -la test-configs/
echo ""
echo "🚀 Now run:"
echo "   ./kc2con analyze --config-dir ./test-configs"
