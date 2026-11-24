#!/usr/bin/env bash
# The nightly polish walk of the TV app, run inside an Android TV emulator
# (from .github/workflows/polish.yml, in the android directory): the same
# walk-through as CI, with the screen recorded the whole time, the log kept
# with times (each step and key press is a "CueWalkThrough" line), and any
# crash pulled out.
#
#   tools/polish/tv-walk.sh <out-dir>
set -u
out=${1:?out dir}
mkdir -p "$out/rec"
chmod +x gradlew

# Build first, so the recording is all walk.
./gradlew --no-daemon assembleDebug assembleDebugAndroidTest

adb shell rm -rf /data/local/tmp/cue-rec
adb shell mkdir -p /data/local/tmp/cue-rec
adb logcat -c
rm -f /tmp/cue-stop-rec
# screenrecord stops at 3 minutes, so record in parts, noting when each began.
(
  i=0
  while [ ! -f /tmp/cue-stop-rec ]; do
    i=$((i + 1))
    part=$(printf 'part-%02d' "$i")
    echo "$part $(adb shell date +%s.%N | tr -d '\r')" >> "$out/rec/starts.txt"
    adb shell screenrecord --size 1280x720 --bit-rate 8000000 --time-limit 170 "/data/local/tmp/cue-rec/$part.mp4"
  done
) &
rec=$!
sleep 2

status=0
./gradlew --no-daemon connectedDebugAndroidTest || status=1

sleep 2
touch /tmp/cue-stop-rec
adb shell pkill -INT screenrecord || true
wait "$rec" 2>/dev/null || true
sleep 1
adb pull /data/local/tmp/cue-rec "$out/" >/dev/null 2>&1 || true
mv "$out"/cue-rec/*.mp4 "$out/rec/" 2>/dev/null || true
adb pull /data/local/tmp/cue-shots "$out/" >/dev/null 2>&1 || true
adb logcat -d -v epoch > "$out/logcat.txt" || true
grep -E "CueWalkThrough" "$out/logcat.txt" > "$out/steps.txt" || true
# Only real crashes: each "FATAL EXCEPTION" and the error lines of its
# stack trace. (Every "am" command also logs harmless AndroidRuntime lines.)
awk '/ E AndroidRuntime: FATAL EXCEPTION/ { on = 1 } on && / E AndroidRuntime:/ { print; next } on { on = 0; print "" }' \
  "$out/logcat.txt" > "$out/crashes.txt" || true
[ -s "$out/crashes.txt" ] || rm -f "$out/crashes.txt"
echo "$status" > "$out/walkthrough-status.txt"
exit 0
