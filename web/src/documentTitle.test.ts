// Run with: npm test (uses node's built-in test runner, no extra packages).
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { titleFor } from './documentTitle.ts'

test('titleFor puts the page first and the app second', () => {
  assert.equal(titleFor('Library'), 'Library · Cue')
  assert.equal(titleFor('Server and backup'), 'Server and backup · Cue')
  assert.equal(titleFor('  Dashboard '), 'Dashboard · Cue')
})

test('titleFor never says Cue twice', () => {
  assert.equal(titleFor(''), 'Cue')
  assert.equal(titleFor('Cue'), 'Cue')
})
