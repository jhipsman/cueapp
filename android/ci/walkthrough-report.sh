#!/bin/sh
# After the emulator walk-through fails: its reason at the top of the run
# page (GitHub's Annotations), not only deep in the log. Which screen it
# waited for and what was on screen instead, and any crash. Run from
# android/, with the logcat already pulled to ../walkthrough/logcat.txt.
[ -f ../walkthrough-failed ] || exit 0
for f in app/build/outputs/androidTest-results/connected/*/*.xml app/build/outputs/androidTest-results/connected/*.xml; do
  [ -f "$f" ] || continue
  grep -o '<failure message="[^"]*"' "$f" | head -3 |
    sed -e 's/^<failure message="//' -e 's/"$//' -e 's/&#10;/ /g' -e 's/&quot;/"/g' -e 's/&amp;/\&/g' |
    cut -c1-900 | while read -r m; do echo "::error title=Walkthrough failed::$m"; done
done
grep -E "FATAL EXCEPTION" -A3 ../walkthrough/logcat.txt 2>/dev/null | head -8 |
  while read -r m; do echo "::error title=App crash::$m"; done
exit 0
