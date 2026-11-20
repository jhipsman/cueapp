# Cue motion rulebook

How Cue moves on a TV (Fire TV / Google TV) and on the web. The nightly polish run reviews the recorded walk-throughs against these rules. A finding names the rule it breaks (for example **T3**), so each rule has a number.

The house style comes from Apple TV: **nothing snaps, nothing bounces, and everything settles.** Motion explains where something came from and where it went. It is never decoration.

## Shared

| | Rule |
|---|---|
| **S1** | **One curve.** Movement uses *Settle*, `cubic-bezier(0.2, 0.8, 0.2, 1)`: quick to start, with a long gentle stop. It is `Settle` in `android/.../home/Look.kt` and `cubic-bezier(0.2, 0.8, 0.2, 1)` in CSS. Linear is only for things that loop or track time (a progress bar, a slow background drift). |
| **S2** | **No overshoot.** No springs that pass their target and no bouncy curves (`cubic-bezier` with a y value above 1). The one exception is a deliberate "pop" on a success tick, which is at most 1.1× and 200 ms. |
| **S3** | **Arrivals fade and rise.** New content fades in while rising 8–16 dp/px into place. It never just appears, and it never slides in from off-screen unless it really came from there (a side menu, a sheet). |
| **S4** | **Stagger lightly.** Items arriving together go one after another, 30–60 ms apart, with at most 6 staggered. Anything after the sixth arrives with the sixth. |
| **S5** | **Leaving is faster than arriving**, about two-thirds of the arrival time. |
| **S6** | **Nothing moves twice.** Content that arrives does not shift again once it has settled. A late image or count must not push its neighbours, so space is reserved or it fades in place. Layout jumps after the first 600 ms are bugs. |
| **S7** | **No flashes.** No white or empty frame between two screens, no skeleton that blinks for under 150 ms (show it only if loading takes longer), and no spinner that replaces content which was already on screen. |
| **S8** | **Respect reduced motion.** With `prefers-reduced-motion` (web) or animator scale 0 (Android), movement becomes a fade of 150 ms or less, and nothing loops. |

## TV (Fire TV / Google TV / Android TV)

Viewed from 3 m away and driven by a D-pad: focus is the main character.

| | Rule |
|---|---|
| **T1** | **Focus is always visible.** Exactly one thing has the highlight at every moment, including mid-transition and after Back. A frame with no highlight, or with two, is a bug. |
| **T2** | **The highlight lifts.** A focused card grows, rises a little, and its shadow deepens over **190–240 ms** with Settle. Losing focus reverses it in about two-thirds of that time. |
| **T3** | **Lift by size.** Posters and small cards use 1.08–1.10×, wide (16:9) cards 1.05–1.06×, very wide banners and buttons 1.02–1.04×, and avatars up to 1.15×. A lifted card must not be clipped by its row: the row keeps vertical room for it. |
| **T4** | **Rows glide; they don't jump.** Moving along a row scrolls it with Settle over **320–450 ms**, keeping the highlight near the start of the row (the pivot). Moving between rows scrolls the page the same way. Holding a direction may speed up, but it never skips frames to empty. |
| **T5** | **Rows remember.** Coming back to a row (up, down, or from the side menu) lands on the card that last had the highlight when it is still in view, otherwise on the nearest one (`keepRowFocus`). |
| **T6** | **Screens cross-fade.** Opening a title or a page fades the new screen in over 320–420 ms, with the backdrop fading in last. Back returns to the same card, already highlighted, with no visible re-layout. |
| **T7** | **Side menu.** Moving onto the menu opens it with labels over 240–320 ms. The content behind dims but does not move. Leaving closes it in about 200 ms. |
| **T8** | **Hero and backdrop.** Backdrops change with a 500–700 ms cross-fade, never a cut. A slow drift (Ken Burns) is linear, at most 1.05× over 9 s or more. |
| **T9** | **Text never animates position on its own.** Titles and descriptions fade. They don't slide letter by letter, scale, or marquee unless the text is truncated and focused (then marquee after a 1 s pause). |
| **T10** | **Smooth on a Fire TV Stick.** No transition may drop below about 50 fps in the recording. Blur, large shadows and full-screen alpha layers are the usual causes. |

## Web

| | Rule |
|---|---|
| **W1** | **Short and quiet.** Hover and press feedback takes 120–180 ms, and panels and menus 160–240 ms. Nothing takes over 400 ms except a page's first arrival (up to 600 ms). |
| **W2** | **Page changes don't flash.** Moving between pages keeps the shell (sidebar, top bar) still. Only the content area changes, by a fade (with S3 for its first content). |
| **W3** | **Menus and sheets come from their source.** A menu grows from its button, and a sheet on a phone slides up from the bottom edge. Both close the way they came. |
| **W4** | **Hover never moves layout.** A hover may lift (`translateY` of at most 2 px, or a scale of at most 1.03 on cards), deepen a shadow, or change colour. It never changes size in the flow. |
| **W5** | **Toasts** slide up 8 px and fade in over 180 ms, stay at least 3 s, and fade out. Two toasts stack and don't overlap. |
| **W6** | **Phone width.** At 390 px nothing scrolls sideways, tap targets are at least 40 px, and a sticky bar doesn't cover content after a transition. |

## Look (checked on the same recordings)

| | Rule |
|---|---|
| **L1** | **Edges line up.** TV: everything starts at the 48 dp gutter, with card corners 12 dp and panels 20 dp. Web: the page grid is followed, with no element 1–3 px off its neighbours. |
| **L2** | **Contrast.** Text over pictures sits on a scrim, and body text meets 4.5:1. Dim text is never dimmer than `--text-dim`. |
| **L3** | **No truncated words** in labels or buttons at 1080p (TV) or 390 px (web). Titles may be truncated with an ellipsis. |
| **L4** | **Empty, loading and error states** each have a designed look. A blank area is a bug. |

## Severity, for the nightly review

- **Small**: a value outside its range, a missing fade, a 1–3 px misalignment, or a flash under 100 ms. If the fix is local (one file, a few lines) and safe, it goes in the morning's polish PR.
- **Bigger**: anything that needs a new component or a change across many screens, that changes how something behaves, or that is a matter of taste. It is written up as a suggestion with frames from the recording, not changed.
- **Broken**: a crash, lost focus (T1), or a screen that never shows. This is always reported first, even when it is not fixed.
