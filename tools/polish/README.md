# The nightly polish run

Every night, `.github/workflows/polish.yml` walks the TV app on an Android TV
emulator and the web app in a browser (desktop and phone) with the screen
recorded, and cuts each transition into a contact sheet. Every morning a
Claude Code session reviews the run against
[docs/motion-rulebook.md](../../docs/motion-rulebook.md), checks for new
crashes, fixes small issues in one pull request and suggests the bigger ones.

| File | What it does |
|---|---|
| `tv-walk.sh` | Inside the emulator: builds, records the screen in 170 s parts, runs the walk-through (`android/app/src/androidTest`), and keeps the log with times. |
| `web-walk.mjs` | Signs in to a throwaway Cue in demo mode and visits the main pages with video on, marking when each step starts. |
| `sheets.mjs` | Cuts each step out of the recordings onto a contact sheet (frames left to right, top to bottom; `index.json` gives the frame interval). |

## The `polish` artifact and the `polish-latest` branch

The branch holds the same files without the videos and the full log.

```
summary.txt              walkthrough status, crash and error counts, commit
sheets/index.json        one entry per sheet: app, viewport, step, frameMs
sheets/tv-NN-<step>.jpg  a TV step, 10 fps, from the key press until it settled
sheets/web-<vp>-NN-<step>.jpg  a web step, 12 fps
tv/cue-shots/*.png       the settled screen at each TV step
tv/crashes.txt           FATAL EXCEPTION blocks from the log (only if any)
tv/steps.txt             each step and key press, with times
tv/rec/part-NN.mp4       the TV recording
web/<vp>/NN-<step>.png   the settled page at each web step
web/<vp>/walk.webm       the web recording
web/errors.json          console errors and page crashes
```

To run a walk yourself, start the workflow by hand (Actions > Polish
recordings > Run workflow). The web walk also runs locally against any Cue
with an administrator `polish` (password in `POLISH_PASSWORD`):

```bash
node tools/polish/web-walk.mjs http://localhost:8264 polish-out/web && node tools/polish/sheets.mjs polish-out
```

## The morning review (what the scheduled Claude session does)

1. **Get the latest run.** `git fetch origin polish-latest` and
   `git worktree add ../polish-latest FETCH_HEAD` (the run's sheets, screenshots and
   logs, without the videos, which stay in the run's `polish` artifact). Check its
   date with `git log -1 FETCH_HEAD` and the run with
   `gh api "repos/jhipsman/cueapp/actions/workflows/polish.yml/runs?per_page=1"`.
   If the run failed or is older than 36 hours, say so first and read its job logs.
2. **Crashes first.** Read `summary.txt`, `tv/crashes.txt`, `web/errors.json`, and
   the walk-through status. Check open GitHub issues labelled `crash`. A crash is
   fixed in this morning's PR if the cause is clear and the fix is small;
   otherwise it heads the suggestions.
3. **Motion and look.** Open every contact sheet and its settled screenshot. For each
   transition, check the rulebook (S, T, W and L rules): the curve, the timing
   (count frames × `frameMs`), flashes, blank frames, lost focus, jumps after
   settling, clipping, misalignment, and phone width. Note each finding with the
   rule, the sheet and the frame number.
4. **Fix the small ones.** Only findings the rulebook calls *Small*: local, safe, a
   few lines, and no change of behaviour. Make each its own commit, run the checks
   (`go test ./...`, `gofmt -l .`, and in `web/`: `npm run lint`, `npm test`,
   `npm run build`), add one CHANGELOG line, and open **one** PR titled
   `Polish: <date>`. The PR body lists each fix with its rule and the sheet it came
   from. No PR if there's nothing small.
5. **Suggest the bigger ones.** Anything *Bigger* goes in the PR body under
   "Suggestions" (or, with no PR, in an issue titled `Polish suggestions: <date>`,
   labelled `polish`), each with the rule, what was seen, which sheet, and a
   proposed approach. Don't repeat a suggestion that is already in an open
   `polish` issue: add to that issue instead.
6. **Report.** End with a short summary: run status, crashes, what was fixed (with
   the PR link), and the top suggestions.
