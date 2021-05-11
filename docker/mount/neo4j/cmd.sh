#!/bin/sh
set -e

echo "Initialising neo4j..."

neo4j-admin set-initial-password neo4j

config_backup_path=/tmp
if [ ! -f $config_backup_path/neo4j.conf ] ; then
    cat $NEO4J_HOME/conf/neo4j.conf > $config_backup_path/neo4j.conf
fi

sed --in-place "/^dbms.connector.bolt.listen_address=.*/d" $NEO4J_HOME/conf/neo4j.conf
sed --in-place "/^dbms.connector.bolt.advertised_address=.*/d" $NEO4J_HOME/conf/neo4j.conf
sed --in-place "/^dbms.connector.http.enabled=.*/d" $NEO4J_HOME/conf/neo4j.conf
echo "dbms.connector.bolt.listen_address=localhost:17687" >> $NEO4J_HOME/conf/neo4j.conf
echo "dbms.connector.bolt.advertised_address=localhost:17687" >> $NEO4J_HOME/conf/neo4j.conf
echo "dbms.connector.http.enabled=false" >> $NEO4J_HOME/conf/neo4j.conf

neo4j start

while true ; do
    echo "Waiting for neo4j to start..."
    if [ "$(cat /logs/neo4j.log | tail -n 1 | grep Started.$ | wc -l)" = "1" ] ; then
        echo "Neo4j started."
        break
    fi
    sleep 1
done

cypher-shell -a localhost:17687 -u neo4j -p neo4j "CREATE CONSTRAINT service_name_unique IF NOT EXISTS ON (s:Service) ASSERT s.name IS UNIQUE"

neo4j stop

cat $config_backup_path/neo4j.conf > $NEO4J_HOME/conf/neo4j.conf

echo "Finished init."

neo4j console
