CUE BRAND KIT

cue-mark.svg                  the mark: a C around a play button, in Cue teal
cue-icon-512.png              the app icon tile (used for Docker, Unraid and app store icons)
cue-lockup-dark.svg           horizontal lockup for dark backgrounds
cue-lockup-light.svg          horizontal lockup for light backgrounds
cue-tokens.css                drop-in CSS variables, dark by default plus [data-theme="light"]
cue-tokens.json               the same values for code and design tools

The favicons and the fonts (Sora, IBM Plex Sans, IBM Plex Mono, with their
OFL licences) used by the app live in web/public.

Quick start
  <link rel="icon" href="/favicon.svg" type="image/svg+xml">
  <link rel="icon" href="/favicon.ico" sizes="any">
  Inline cue-mark.svg in the sidebar with fill="currentColor" and set  .logo-mark { color: var(--accent) }
  Use cue-icon-512.png for Docker / Unraid template icons.
