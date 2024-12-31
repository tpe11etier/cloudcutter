#!/bin/bash

# Start Elasticsearch in the background
/usr/local/bin/docker-entrypoint.sh eswrapper &

# Wait for Elasticsearch to start
until curl -s http://localhost:9200 >/dev/null; do
    sleep 1
done

# Create index and load sample data
curl -XPUT "http://localhost:9200/sample-index" -H 'Content-Type: application/json' -d'{
    "settings": {
        "number_of_shards": 1,
        "number_of_replicas": 0
    }
}'

curl -XPOST "http://localhost:9200/sample-index/_bulk" -H 'Content-Type: application/json' --data-binary "@/tmp/sample-data.json"

# Keep container running
wait