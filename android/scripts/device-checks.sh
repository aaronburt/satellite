#!/usr/bin/env bash
#
# Runs the device checklists from docs/testing.md (issue #8 MQTT recovery and
# issue #9 lifecycle/microphone) against a connected emulator, observing the
# real MQTT traffic on a throwaway Mosquitto container.
#
# Requirements: adb (one device/emulator connected, API 33+), docker, python3,
# and the debug APK installed (./gradlew :app:installDebug). The debug build
# includes DeviceCheckReceiver, which this script uses to change settings.
#
# Usage:  scripts/device-checks.sh [check ...]
#   With no arguments every check runs, in order. Names: see ALL_CHECKS below.
#
# Environment:
#   BACKGROUND_MINUTES  length of the screen-off background check (default 5;
#                       the manual checklist uses 30)
#   RESULTS_DIR         where logs and summary.md go (default build/device-checks)
#
# Each check prints PASS/FAIL/SKIP. The script exits non-zero if any check fails.

set -uo pipefail

PKG=uk.co.aaronburt.satellite
ACTIVITY="$PKG/.app.MainActivity"
SERVICE="$PKG/.app.service.SatelliteService"
RECEIVER="$PKG/.app.devicecheck.DeviceCheckReceiver"
INTERVAL=30
BACKGROUND_MINUTES=${BACKGROUND_MINUTES:-5}
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ANDROID_DIR=$(dirname "$SCRIPT_DIR")
RESULTS_DIR=${RESULTS_DIR:-$ANDROID_DIR/build/device-checks}
BROKER=sat-check-broker
BROKER_HOST_FROM_EMULATOR=10.0.2.2
WEBHOOK_PORT=8089
MQTT_USER=satellite
MQTT_PASS=satellite

ALL_CHECKS=(
  setup
  reboot_reporting_on
  reboot_reporting_paused
  background_screen_off
  doze
  rapid_toggle_and_rotate
  broker_down_at_start
  broker_restart
  network_loss_then_one_command
  ha_birth_republishes
  transport_switch
  microphone_restore_offline
)

mkdir -p "$RESULTS_DIR"
SUMMARY="$RESULTS_DIR/summary.md"
FAILED=0
DEVICE_ID=""

log() { printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*"; }

record() { # status check note
  local status=$1 name=$2 note=${3:-}
  log "$status $name${note:+ - $note}"
  printf '| %s | `%s` | %s |\n' "$status" "$name" "$note" >> "$SUMMARY"
  [[ $status == FAIL ]] && FAILED=1
  return 0
}

# ---------------------------------------------------------------- adb helpers

sh_adb() { adb shell "$@" | tr -d '\r'; }

# Sends settings to DeviceCheckReceiver; prints the receiver's state report.
app_set() {
  local args=() key
  while (($#)); do key=${1%%=*}; args+=(--es "$key" "${1#*=}"); shift; done
  local out
  out=$(sh_adb am broadcast -f 0x20 -n "$RECEIVER" "${args[@]}")
  if [[ $out != *"result=-1"* ]]; then
    log "receiver rejected request: $out"
    return 1
  fi
  sed -n 's/.*data="\(.*\)".*/\1/p' <<<"$out"
}

app_query() { app_set | tr ';' '\n' | sed -n "s/^$1=//p"; }

start_app() { sh_adb am start -W -n "$ACTIVITY" > /dev/null; }

service_running() { sh_adb dumpsys activity services "$SERVICE" | grep -q ServiceRecord; }

notification_count() {
  sh_adb dumpsys notification --noredact | grep -c "NotificationRecord(.*pkg=$PKG " || true
}

notification_text() { sh_adb dumpsys notification --noredact | grep -A40 "pkg=$PKG " | grep -m1 "android.text=" ; }

wait_for_boot() {
  adb wait-for-device
  local i
  for i in $(seq 1 120); do
    [[ $(sh_adb getprop sys.boot_completed) == 1 ]] && break
    sleep 2
  done
  # Settle: package manager and broadcast queue keep working after the flag.
  sleep 15
  sh_adb input keyevent KEYCODE_WAKEUP
  sh_adb wm dismiss-keyguard >/dev/null 2>&1 || true
}

reboot_device() {
  log "rebooting emulator"
  adb reboot
  sleep 5
  wait_for_boot
}

# ------------------------------------------------------------- broker helpers

topic_base() { echo "satellite/$DEVICE_ID"; }

mqtt_sub() { docker exec "$BROKER" mosquitto_sub -h localhost -u "$MQTT_USER" -P "$MQTT_PASS" "$@"; }
mqtt_pub() { docker exec "$BROKER" mosquitto_pub -h localhost -u "$MQTT_USER" -P "$MQTT_PASS" "$@"; }

# Retained value of a topic, or empty when none arrives within 3 seconds.
retained() { mqtt_sub -t "$1" -C 1 -W 3 2>/dev/null || true; }

wait_for_retained() { # topic expected timeout_seconds
  local deadline=$((SECONDS + $3))
  while ((SECONDS < deadline)); do
    [[ $(retained "$1") == "$2" ]] && return 0
    sleep 2
  done
  return 1
}

# Records new (non-retained) messages on a topic for N seconds into a file as
# "<unix time> <payload>" lines. Runs in the background; use `wait`.
capture() { # file topic seconds
  mqtt_sub -R -t "$2" -W "$3" -F '%U %p' > "$1" 2>/dev/null || true
}

count_lines() { [[ -s $1 ]] && wc -l < "$1" | tr -d ' ' || echo 0; }

# Smallest gap in seconds between consecutive captured messages (999 if <2).
min_gap() { awk 'NR>1 { g=$1-p; if (m=="" || g<m) m=g } { p=$1 } END { printf "%d", (m=="" ? 999 : m) }' "$1"; }

# Blocks until the next periodic state message, so the following window starts
# just after a tick and is not polluted by one.
wait_for_tick() { mqtt_sub -R -t "$(topic_base)/state/uptime" -C 1 -W $((INTERVAL + 30)) > /dev/null 2>&1; }

start_broker() {
  docker rm -f "$BROKER" > /dev/null 2>&1 || true
  # Kept out of RESULTS_DIR: files written by the container are not readable
  # by the artifact upload.
  local data; data=$(mktemp -d)
  chmod 777 "$data"
  # The broker refuses to start without the password file, so create it first.
  docker run --rm -v "$data:/mosquitto/data" eclipse-mosquitto:2 sh -c \
    "mosquitto_passwd -b -c /mosquitto/data/passwd $MQTT_USER $MQTT_PASS &&
     chown mosquitto:mosquitto /mosquitto/data/passwd && chmod 0700 /mosquitto/data/passwd"
  docker run -d --name "$BROKER" -p 1883:1883 \
    -v "$ANDROID_DIR/docker/mosquitto/config:/mosquitto/config:ro" \
    -v "$data:/mosquitto/data" eclipse-mosquitto:2 > /dev/null
  local i
  for i in $(seq 1 30); do
    mqtt_sub -t '$SYS/broker/version' -C 1 -W 2 > /dev/null 2>&1 && return 0
    sleep 1
  done
  log "broker did not start"; docker logs "$BROKER"; return 1
}

# ------------------------------------------------------------- webhook helper

WEBHOOK_LOG="$RESULTS_DIR/webhook.log"
start_webhook() {
  : > "$WEBHOOK_LOG"
  python3 - "$WEBHOOK_PORT" "$WEBHOOK_LOG" <<'PY' &
import sys, time
from http.server import BaseHTTPRequestHandler, HTTPServer
port, path = int(sys.argv[1]), sys.argv[2]
class H(BaseHTTPRequestHandler):
    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        with open(path, "a") as f:
            f.write(f"{time.time():.0f} {self.path} {body.decode(errors='replace')}\n")
        self.send_response(204); self.end_headers()
    def log_message(self, *a): pass
HTTPServer(("0.0.0.0", port), H).serve_forever()
PY
  WEBHOOK_PID=$!
  sleep 1
}

# ---------------------------------------------------------------- the checks

ensure_reporting_on() {
  app_set reporting=true transport=mqtt > /dev/null
  start_app
  wait_for_retained "$(topic_base)/availability" online 60
}

check_setup() {
  start_app
  sh_adb pm grant "$PKG" android.permission.POST_NOTIFICATIONS 2>/dev/null || true
  app_set host=$BROKER_HOST_FROM_EMULATOR port=1883 username=$MQTT_USER password=$MQTT_PASS \
    interval=$INTERVAL transport=mqtt reporting=true > /dev/null || { record FAIL setup "receiver unavailable"; return; }
  DEVICE_ID=$(app_query device_id)
  start_app
  if wait_for_retained "$(topic_base)/availability" online 60; then
    record PASS setup "device $DEVICE_ID online"
  else
    record FAIL setup "availability never became online"
  fi
}

# #9 step 1
check_reboot_reporting_on() {
  ensure_reporting_on
  sh_adb input keyevent KEYCODE_HOME
  reboot_device
  local f="$RESULTS_DIR/reboot_on.txt"
  capture "$f" "$(topic_base)/state/uptime" $((INTERVAL + 60))
  local n; n=$(count_lines "$f")
  if service_running && (( $(notification_count) >= 1 )) && ((n >= 1)); then
    record PASS reboot_reporting_on "service and notification back, $n state message(s)"
  else
    record FAIL reboot_reporting_on "service=$(service_running && echo up || echo down) notifications=$(notification_count) messages=$n"
  fi
}

# #9 step 2
check_reboot_reporting_paused() {
  ensure_reporting_on
  app_set reporting=false > /dev/null
  wait_for_retained "$(topic_base)/availability" offline 30 || true
  sh_adb input keyevent KEYCODE_HOME
  reboot_device
  local f="$RESULTS_DIR/reboot_paused.txt"
  capture "$f" "$(topic_base)/#" $((INTERVAL + 45))
  local n reporting; n=$(count_lines "$f"); reporting=$(app_query reporting)
  if ! service_running && (( $(notification_count) == 0 )) && ((n == 0)) && [[ $reporting == false ]]; then
    record PASS reboot_reporting_paused "no service, no notification, no traffic, still paused"
  else
    record FAIL reboot_reporting_paused "service=$(service_running && echo up || echo down) notifications=$(notification_count) messages=$n reporting=$reporting"
  fi
  ensure_reporting_on
}

# #9 step 3 (shortened; BACKGROUND_MINUTES)
check_background_screen_off() {
  ensure_reporting_on
  sh_adb input keyevent KEYCODE_HOME
  sh_adb input keyevent KEYCODE_SLEEP
  local seconds=$((BACKGROUND_MINUTES * 60)) f="$RESULTS_DIR/background.txt"
  capture "$f" "$(topic_base)/state/uptime" "$seconds"
  sh_adb input keyevent KEYCODE_WAKEUP
  local n expected; n=$(count_lines "$f"); expected=$((seconds / INTERVAL - 1))
  if ((n >= expected)) && (( $(notification_count) >= 1 )); then
    record PASS background_screen_off "$n messages in ${BACKGROUND_MINUTES} min (expected >= $expected)"
  else
    record FAIL background_screen_off "$n messages in ${BACKGROUND_MINUTES} min (expected >= $expected), notifications=$(notification_count)"
  fi
}

# #9 step 4 and the Doze half of #8 step 4
check_doze() {
  ensure_reporting_on
  sh_adb input keyevent KEYCODE_HOME
  sh_adb dumpsys battery unplug
  sh_adb input keyevent KEYCODE_SLEEP
  sh_adb dumpsys deviceidle force-idle > /dev/null
  local during="$RESULTS_DIR/doze_during.txt" after="$RESULTS_DIR/doze_after.txt"
  capture "$during" "$(topic_base)/state/uptime" 90
  sh_adb dumpsys deviceidle unforce > /dev/null
  sh_adb dumpsys battery reset
  sh_adb input keyevent KEYCODE_WAKEUP
  capture "$after" "$(topic_base)/state/uptime" $((INTERVAL + 45))
  local d a; d=$(count_lines "$during"); a=$(count_lines "$after")
  if ((a >= 1)); then
    record PASS doze "resumed after Doze ($d message(s) during 90 s of forced idle)"
  else
    record FAIL doze "no state after Doze ended ($d during idle)"
  fi
}

# #9 step 5
check_rapid_toggle_and_rotate() {
  ensure_reporting_on
  local i
  for i in $(seq 1 10); do
    app_set reporting=false > /dev/null
    app_set reporting=true > /dev/null
  done
  sh_adb settings put system accelerometer_rotation 0
  sh_adb settings put system user_rotation 1; sleep 3
  sh_adb settings put system user_rotation 0; sleep 3
  sh_adb input keyevent KEYCODE_HOME
  start_app
  sleep 5
  local f="$RESULTS_DIR/rapid_toggle.txt"
  capture "$f" "$(topic_base)/state/uptime" $((INTERVAL * 3 + 5))
  local notes=() ok=1 n gap
  n=$(count_lines "$f"); gap=$(min_gap "$f")
  (( $(notification_count) == 1 )) || { ok=0; notes+=("notifications=$(notification_count)"); }
  ((n >= 2 && n <= 4)) || { ok=0; notes+=("$n messages in 3 intervals"); }
  ((gap >= INTERVAL / 2)) || { ok=0; notes+=("two publishes ${gap}s apart (duplicate watcher?)"); }
  app_set reporting=false > /dev/null
  sleep 5
  (( $(notification_count) == 0 )) || { ok=0; notes+=("notification left after final pause"); }
  service_running && { ok=0; notes+=("service still running after final pause"); }
  if ((ok)); then
    record PASS rapid_toggle_and_rotate "one notification, $n messages, min gap ${gap}s, clean pause"
  else
    record FAIL rapid_toggle_and_rotate "${notes[*]}"
  fi
  ensure_reporting_on
}

# #9 step 6
check_broker_down_at_start() {
  ensure_reporting_on
  app_set reporting=false > /dev/null
  sleep 3
  docker stop "$BROKER" > /dev/null
  app_set reporting=true > /dev/null
  start_app
  sleep 20
  docker start "$BROKER" > /dev/null
  if wait_for_retained "$(topic_base)/availability" online 120 && notification_text | grep -q "Connected to"; then
    record PASS broker_down_at_start "recovered to connected without reopening the app"
  else
    record FAIL broker_down_at_start "availability=$(retained "$(topic_base)/availability") $(notification_text)"
  fi
}

# #8 step 4 (broker side): restart with state wiped is covered by
# ReconnectRecoveryTest; here the real Android network stack reconnects.
check_broker_restart() {
  ensure_reporting_on
  docker restart "$BROKER" > /dev/null
  if wait_for_retained "$(topic_base)/availability" online 120; then
    local f="$RESULTS_DIR/broker_restart.txt"
    capture "$f" "$(topic_base)/state/uptime" $((INTERVAL + 20))
    if (( $(count_lines "$f") >= 1 )); then
      record PASS broker_restart "availability online and state flowing after restart"
    else
      record FAIL broker_restart "online but no state after restart"
    fi
  else
    record FAIL broker_restart "availability not online within 120 s"
  fi
}

# #8 step 6 and the Wi-Fi half of step 4
check_network_loss_then_one_command() {
  ensure_reporting_on
  app_set control=volume_media control_enabled=true > /dev/null
  local i
  for i in 1 2 3; do
    # The emulator also has cellular data; turn both off for a real outage.
    sh_adb svc wifi disable; sh_adb svc data disable
    sleep 8
    sh_adb svc wifi enable; sh_adb svc data enable
    sleep 12
  done
  if ! wait_for_retained "$(topic_base)/availability" online 120; then
    record FAIL network_loss_then_one_command "availability not online after network returned"
    return
  fi
  local before target f="$RESULTS_DIR/one_command.txt"
  before=$(app_query volume)
  target=$(( before >= 50 ? 20 : 80 ))
  wait_for_tick
  capture "$f" "$(topic_base)/state/volume_media" 15 &
  local cap=$!
  sleep 1
  mqtt_pub -t "$(topic_base)/cmd/volume" -m "$target"
  wait $cap
  local n after; n=$(count_lines "$f"); after=$(app_query volume)
  app_set control=volume_media control_enabled=false > /dev/null
  if ((n == 1)) && ((after != before)); then
    record PASS network_loss_then_one_command "volume $before -> $after with exactly one confirming message"
  else
    record FAIL network_loss_then_one_command "$n confirming message(s), volume $before -> $after"
  fi
}

# #8 step 5, using Home Assistant's birth message instead of a real HA restart
check_ha_birth_republishes() {
  ensure_reporting_on
  local f="$RESULTS_DIR/ha_birth.txt"
  capture "$f" "homeassistant/device/satellite_$DEVICE_ID/config" 15 &
  local cap=$!
  sleep 1
  mqtt_pub -t homeassistant/status -m online
  wait $cap
  if (( $(count_lines "$f") >= 1 )); then
    record PASS ha_birth_republishes "discovery republished on homeassistant/status online"
  else
    record FAIL ha_birth_republishes "no discovery after birth message"
  fi
}

# #9 step 7
check_transport_switch() {
  ensure_reporting_on
  start_webhook
  local mqtt="$RESULTS_DIR/webhook_mqtt.txt" notes=() ok=1
  app_set webhook_url="http://$BROKER_HOST_FROM_EMULATOR:$WEBHOOK_PORT/hook" transport=webhook > /dev/null
  wait_for_retained "$(topic_base)/availability" offline 30 || { ok=0; notes+=("MQTT availability not offline on webhook"); }
  capture "$mqtt" "$(topic_base)/state/#" $((INTERVAL + 30))
  local posts; posts=$(count_lines "$WEBHOOK_LOG")
  ((posts >= 1)) || { ok=0; notes+=("no webhook POSTs"); }
  (( $(count_lines "$mqtt") == 0 )) || { ok=0; notes+=("MQTT state sent while on webhook"); }
  app_set transport=mqtt > /dev/null
  wait_for_retained "$(topic_base)/availability" online 60 || { ok=0; notes+=("availability not online after switching back"); }
  sleep 3
  local settled; settled=$(count_lines "$WEBHOOK_LOG")
  sleep $((INTERVAL + 15))
  (( $(count_lines "$WEBHOOK_LOG") == settled )) || { ok=0; notes+=("webhook still receiving after switching back"); }
  kill "$WEBHOOK_PID" 2>/dev/null || true
  if ((ok)); then
    record PASS transport_switch "$posts webhook POST(s) and silent MQTT on webhook; clean switch back"
  else
    record FAIL transport_switch "${notes[*]}"
  fi
}

# #9 step 8
check_microphone_restore_offline() {
  ensure_reporting_on
  app_set control=mic_muted control_enabled=true > /dev/null
  sleep 3
  mqtt_pub -t "$(topic_base)/cmd/mute" -m ON
  local i muted=""
  for i in $(seq 1 10); do muted=$(app_query mic_muted); [[ $muted == true ]] && break; sleep 1; done
  if [[ $muted != true ]]; then
    app_set control=mic_muted control_enabled=false > /dev/null
    record SKIP microphone_restore_offline "this system image refuses app microphone mute (mic_muted=$muted)"
    return
  fi
  sh_adb cmd connectivity airplane-mode enable
  sleep 5
  app_set control=mic_muted control_enabled=false > /dev/null
  for i in $(seq 1 15); do muted=$(app_query mic_muted); [[ $muted == false ]] && break; sleep 1; done
  sh_adb cmd connectivity airplane-mode disable
  if [[ $muted == false ]]; then
    record PASS microphone_restore_offline "microphone unmuted while in airplane mode"
  else
    record FAIL microphone_restore_offline "microphone still muted after disabling the control offline"
  fi
}

# ---------------------------------------------------------------------- main

cleanup() {
  [[ -n ${WEBHOOK_PID:-} ]] && kill "$WEBHOOK_PID" 2>/dev/null
  sh_adb svc wifi enable >/dev/null 2>&1; sh_adb svc data enable >/dev/null 2>&1
  sh_adb cmd connectivity airplane-mode disable >/dev/null 2>&1
  sh_adb dumpsys deviceidle unforce >/dev/null 2>&1; sh_adb dumpsys battery reset >/dev/null 2>&1
  docker logs "$BROKER" > "$RESULTS_DIR/broker.log" 2>&1
  adb logcat -d > "$RESULTS_DIR/logcat.txt" 2>/dev/null
  docker rm -f "$BROKER" > /dev/null 2>&1
}
trap cleanup EXIT

checks=("$@")
((${#checks[@]})) || checks=("${ALL_CHECKS[@]}")
[[ ${checks[0]} == setup ]] || checks=(setup "${checks[@]}")

{
  echo "## Device checks"
  echo
  echo "Device: $(sh_adb getprop ro.product.model), Android $(sh_adb getprop ro.build.version.release) (API $(sh_adb getprop ro.build.version.sdk)), interval ${INTERVAL}s"
  echo
  echo "| Result | Check | Notes |"
  echo "| --- | --- | --- |"
} > "$SUMMARY"

start_broker || exit 1
for check in "${checks[@]}"; do
  if ! declare -F "check_$check" > /dev/null; then
    record FAIL "$check" "unknown check"; continue
  fi
  log "=== $check"
  "check_$check"
  if [[ $check == setup && -z $DEVICE_ID ]]; then
    log "setup failed; stopping"; break
  fi
done

[[ -n ${GITHUB_STEP_SUMMARY:-} ]] && cat "$SUMMARY" >> "$GITHUB_STEP_SUMMARY"
cat "$SUMMARY"
exit $FAILED
