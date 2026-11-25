#!/bin/sh
# After the screenshots (Robolectric tests) fail: each failure's reason at
# the top of the run page (GitHub's Annotations). Run from android/.
for f in app/build/test-results/testDebugUnitTest/*.xml; do
  [ -f "$f" ] || continue
  grep -o '<failure message="[^"]*"' "$f" | head -3 |
    sed -e 's/^<failure message="//' -e 's/"$//' -e 's/&#10;/ /g' -e 's/&quot;/"/g' -e 's/&amp;/\&/g' -e 's/&lt;/</g' -e 's/&gt;/>/g' |
    cut -c1-900 | while read -r m; do echo "::error title=$(basename "$f" .xml)::$m"; done
done
exit 0
