# Cue's TV app. R8 shrinks and optimises; names stay as they are, so crash
# reports read plainly and anything found by name is still there.
-dontobfuscate

# The web view's bridge (window.CueTV): called by name from JavaScript.
-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

# Media3 finds its FFmpeg decoders (the Jellyfin build) by name.
-keep class androidx.media3.decoder.ffmpeg.** { *; }
-keep class org.jellyfin.media3.** { *; }

# The app's own classes are small; keep them whole to be safe.
-keep class app.cue.tv.** { *; }

-dontwarn org.jellyfin.**
-dontwarn javax.annotation.**
