#!/usr/bin/env bash
# Home Assistant smoke test (issue #21): starts an isolated Mosquitto + Home
# Assistant, onboards Home Assistant and adds the MQTT integration over its
# API, runs HomeAssistantSmokeTest against them, keeps evidence, then removes
# everything it created.
#
# Usage (from anywhere):  android/docker/smoke/run.sh
# Env:
#   HA_IMAGE      Home Assistant image (default: ghcr.io/home-assistant/home-assistant)
#   HA_VERSION    Home Assistant image tag (default: stable)
#   EVIDENCE_DIR  where logs and broker/HA dumps go (default: android/build/ha-smoke)
#   KEEP_STACK=1  leave the containers running after the test, for debugging
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
android_dir="$(cd "$here/../.." && pwd)"
evidence="${EVIDENCE_DIR:-$android_dir/build/ha-smoke}"
project="satellite-smoke-$(date +%s)-$$"
mkdir -p "$evidence"

export MQTT_PASSWORD="smoke-$(head -c 12 /dev/urandom | od -An -tx1 | tr -d ' \n')"
ha_user="smoke"
ha_password="smoke-$(head -c 12 /dev/urandom | od -An -tx1 | tr -d ' \n')"
client_id="http://satellite-smoke.local/"

compose() { docker compose -p "$project" -f "$here/compose.yaml" "$@"; }
log() { echo "[ha-smoke] $*"; }

ha_url=""
token=""
mqtt_port=""

collect_evidence() {
    log "Saving evidence to $evidence"
    compose ps -a > "$evidence/compose-ps.txt" 2>&1 || true
    compose logs --no-color mosquitto > "$evidence/mosquitto.log" 2>&1 || true
    compose logs --no-color homeassistant > "$evidence/homeassistant.log" 2>&1 || true
    # Everything still retained on the broker (discovery, availability, state).
    compose exec -T mosquitto mosquitto_sub -h localhost -u satellite -P "$MQTT_PASSWORD" \
        -v -t '#' --retained-only -W 3 > "$evidence/retained-topics.txt" 2>&1 || true
    if [[ -n "$token" ]]; then
        curl -fsS -H "Authorization: Bearer $token" "$ha_url/api/states" \
            > "$evidence/ha-states.json" 2>&1 || true
        curl -fsS -H "Authorization: Bearer $token" "$ha_url/api/error_log" \
            > "$evidence/ha-error.log" 2>&1 || true
    fi
}

cleanup() {
    status=$?
    collect_evidence
    if [[ "${KEEP_STACK:-0}" == "1" ]]; then
        log "KEEP_STACK=1: leaving project $project running (HA at $ha_url)"
        log "  HA user $ha_user / $ha_password, MQTT port ${mqtt_port:-?} satellite / $MQTT_PASSWORD"
        log "  API token: ${token:-none}"
    else
        compose down -v --remove-orphans > /dev/null 2>&1 || true
    fi
    exit $status
}
trap cleanup EXIT

log "Creating broker account"
compose run --rm --no-deps -T mosquitto \
    mosquitto_passwd -b -c /mosquitto/data/passwd satellite "$MQTT_PASSWORD"

log "Starting project $project"
compose up -d --wait mosquitto
compose up -d homeassistant

mqtt_port="$(compose port mosquitto 1883 | sed 's/.*://')"
ha_port="$(compose port homeassistant 8123 | sed 's/.*://')"
ha_url="http://127.0.0.1:$ha_port"

log "Waiting for Home Assistant at $ha_url"
for _ in $(seq 1 180); do
    if curl -fsS "$ha_url/api/onboarding" > /dev/null 2>&1; then break; fi
    sleep 2
done
curl -fsS "$ha_url/api/onboarding" > /dev/null

log "Onboarding Home Assistant"
auth_code="$(curl -fsS -X POST "$ha_url/api/onboarding/users" \
    -H 'Content-Type: application/json' \
    -d "{\"client_id\":\"$client_id\",\"name\":\"Smoke\",\"username\":\"$ha_user\",\"password\":\"$ha_password\",\"language\":\"en\"}" \
    | jq -er .auth_code)"
token="$(curl -fsS -X POST "$ha_url/auth/token" \
    --data-urlencode grant_type=authorization_code \
    --data-urlencode "code=$auth_code" \
    --data-urlencode "client_id=$client_id" \
    | jq -er .access_token)"

log "Adding the MQTT integration"
api() { curl -fsS -H "Authorization: Bearer $token" -H 'Content-Type: application/json' "$@"; }
flow="$(api -X POST "$ha_url/api/config/config_entries/flow" -d '{"handler":"mqtt","show_advanced_options":false}')"
echo "$flow" > "$evidence/mqtt-flow-start.json"
flow_id="$(echo "$flow" | jq -er .flow_id)"
# Some Home Assistant versions open the flow with a menu; pick manual broker setup.
if [[ "$(echo "$flow" | jq -r .type)" == "menu" ]]; then
    flow="$(api -X POST "$ha_url/api/config/config_entries/flow/$flow_id" -d '{"next_step_id":"broker"}')"
fi
# No -f here: on a 400 the body says which field Home Assistant rejected.
result="$(curl -sS -H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
    -X POST "$ha_url/api/config/config_entries/flow/$flow_id" \
    -d "{\"broker\":\"mosquitto\",\"port\":1883,\"protocol\":\"5\",\"username\":\"satellite\",\"password\":\"$MQTT_PASSWORD\",\"other_settings\":{\"set_client_cert\":false,\"set_ca_cert\":\"off\",\"transport\":\"tcp\"}}")"
echo "$result" > "$evidence/mqtt-flow-result.json"
if [[ "$(echo "$result" | jq -r .type)" != "create_entry" ]]; then
    log "MQTT integration setup failed: $result"
    exit 1
fi

log "Running HomeAssistantSmokeTest"
cd "$android_dir"
SATELLITE_HA_URL="$ha_url" \
SATELLITE_HA_TOKEN="$token" \
SATELLITE_MQTT_HOST=127.0.0.1 \
SATELLITE_MQTT_PORT="$mqtt_port" \
SATELLITE_MQTT_USERNAME=satellite \
SATELLITE_MQTT_PASSWORD="$MQTT_PASSWORD" \
./gradlew :core:reporter:testDebugUnitTest \
    --tests 'uk.co.aaronburt.satellite.reporter.HomeAssistantSmokeTest' \
    --rerun --stacktrace
