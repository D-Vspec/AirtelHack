#!/bin/bash
# Creates every topic listed in topics.txt (generated from contracts/topics.json by `make contracts`).
set -euo pipefail
BOOTSTRAP="${BOOTSTRAP:-kafka:19092}"
KT=/opt/kafka/bin/kafka-topics.sh
while IFS=: read -r name partitions; do
  [ -z "$name" ] && continue
  "$KT" --bootstrap-server "$BOOTSTRAP" --create --if-not-exists \
        --topic "$name" --partitions "$partitions" --replication-factor 1
done < /init/topics.txt
echo "topics ready:"
"$KT" --bootstrap-server "$BOOTSTRAP" --list
