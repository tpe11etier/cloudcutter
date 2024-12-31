FROM docker.elastic.co/elasticsearch/elasticsearch:6.8.23

ENV discovery.type=single-node
ENV ES_JAVA_OPTS="-Xms512m -Xmx512m"

COPY ./data/sample-data.json /tmp/sample-data.json
COPY ./init.sh /usr/local/bin/init.sh

RUN chmod +x /usr/local/bin/init.sh

CMD ["/usr/local/bin/init.sh"]