import { test } from 'node:test'
import assert from 'node:assert/strict'
import { matchCues, parseCues } from './watch/findLineText.ts'

test('find a line: reads WebVTT and SRT, matches every word', () => {
  const vtt = `WEBVTT

00:01:02.500 --> 00:01:04.000
<i>I'll be back.</i>

1:02:03.000 --> 1:02:05.000
Here's looking at you, kid.`
  const cues = parseCues(vtt)
  assert.equal(cues.length, 2)
  assert.equal(cues[0].start, 62.5)
  assert.equal(cues[0].text, "I'll be back.")
  assert.equal(cues[1].start, 3723)
  const srt = `1
00:00:10,250 --> 00:00:12,000
May the Force
be with you.`
  assert.equal(parseCues(srt)[0].text, 'May the Force be with you.')
  assert.equal(matchCues(cues, 'be back').length, 1)
  assert.equal(matchCues(cues, 'LOOKING kid').length, 1)
  assert.equal(matchCues(cues, 'back kid').length, 0)
  assert.equal(matchCues(cues, '   ').length, 0)
})
