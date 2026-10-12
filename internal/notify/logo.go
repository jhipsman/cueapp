package notify

import _ "embed"

// logoCID is the Content-ID the email's HTML uses to show the logo.
const logoCID = "cue-logo@cue"

// logoPNG is the full Cue logo, sent inside every HTML email.
//
//go:embed assets/cue-logo.png
var logoPNG []byte
