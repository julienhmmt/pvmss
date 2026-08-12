#!/bin/sh

# Run the SonarScanner CLI against the local SonarQube container.
# Expects .sonar/token to exist (created by `make sonar-bootstrap`).

set -u

TOKEN_FILE=".sonar/token"
SONAR_URL="http://localhost:9000"

if [ ! -f "$TOKEN_FILE" ]; then
    echo "SonarQube token not found. Run 'make sonar-bootstrap' first." >&2
    exit 1
fi

SONAR_TOKEN=$(tr -d '[:space:]' < "$TOKEN_FILE")
export SONAR_TOKEN

echo "Waiting for SonarQube at $SONAR_URL..."
status=""
attempts=0
max_attempts=60
while [ "$status" != "UP" ] && [ $attempts -lt $max_attempts ]; do
    status=$(curl -s "$SONAR_URL/api/system/status" 2>/dev/null \
        | python3 -c "import sys, json; print(json.load(sys.stdin).get('status', ''))" 2>/dev/null || true)
    if [ "$status" != "UP" ]; then
        sleep 2
    fi
    attempts=$((attempts + 1))
done

if [ "$status" != "UP" ]; then
    echo "SonarQube is not ready. Run 'make sonar-up' first." >&2
    exit 1
fi

echo "Running SonarScanner..."
docker compose -f docker-compose.sonarqube.yml run --rm \
    -e SONAR_TOKEN \
    sonar-scanner

echo "Scan complete. Open $SONAR_URL/dashboard?id=pvmss"
