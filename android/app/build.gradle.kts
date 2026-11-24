plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

// Each build on GitHub gets the next version number, so a new APK installs
// over the old one.
val build = System.getenv("GITHUB_RUN_NUMBER")?.toIntOrNull() ?: 1

// The Android that Robolectric runs the screenshots on (Android 15).
val robolectricJars: Configuration by configurations.creating
val robolectricAndroidDir = layout.buildDirectory.dir("robolectric-android")
val robolectricAndroid = tasks.register<Copy>("robolectricAndroid") {
    from(robolectricJars)
    into(robolectricAndroidDir)
}

android {
    namespace = "app.cue.tv"
    compileSdk = 35

    defaultConfig {
        applicationId = "app.cue.tv"
        minSdk = 23 // Android 6 and later: every Fire TV Cube, Fire TV since 2017, Google TV
        targetSdk = 35
        versionCode = build
        versionName = "0.1.$build"
        // The Cue server the app opens with, so there's no address to type:
        // -PcueServer=https://... builds it for another server. Profile >
        // Change server still points it elsewhere.
        // The walk-through on an Android TV emulator (app/src/androidTest).
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        buildConfigField("String", "DEFAULT_SERVER", "\"" + (project.findProperty("cueServer") ?: "https://cuetv.me") + "\"")
    }

    // One fixed key, so every new APK can update the one already installed.
    // See README.md for why it lives in the repository.
    signingConfigs {
        create("cue") {
            storeFile = file("cue-tv.keystore")
            storePassword = "cue-tv-sideload"
            keyAlias = "cue"
            keyPassword = "cue-tv-sideload"
        }
    }
    buildTypes {
        getByName("release") {
            // R8 optimises the app and Compose (much smoother on a TV's slow
            // processor); names are kept (proguard-rules.pro) so crash reports
            // and anything looked up by name still work.
            isMinifyEnabled = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            signingConfig = signingConfigs.getByName("cue")
        }
        getByName("debug") {
            signingConfig = signingConfigs.getByName("cue")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    // The stand-in server is shared by the emulator walk-through and the
    // screenshots taken on the build machine (app/src/test).
    sourceSets {
        getByName("test").java.srcDir("src/sharedTest/java")
        getByName("androidTest").java.srcDir("src/sharedTest/java")
    }
    // Screenshots of the real screens on the build machine (no emulator):
    // Robolectric runs the app, drawing with Android's own graphics, and
    // Roborazzi saves each screen as a picture (app/build/screens).
    testOptions {
        unitTests {
            isIncludeAndroidResources = true
            all {
                // Robolectric's Android, fetched by Gradle (its own downloader
                // gives up when Maven Central asks it to slow down).
                it.dependsOn(robolectricAndroid)
                it.systemProperty("robolectric.offline", "true")
                it.systemProperty("robolectric.dependency.dir", robolectricAndroidDir.get().asFile.absolutePath)
                it.systemProperty("robolectric.graphicsMode", "NATIVE")
                it.systemProperty("robolectric.pixelCopyRenderMode", "hardware")
                it.systemProperty("roborazzi.test.record", "true")
                it.maxHeapSize = "3g"
            }
        }
    }
    lint {
        // assembleRelease doesn't run lint; the TV app workflow runs lintRelease
        // as its own step, which fails on lint errors (warnings only report).
        checkReleaseBuilds = false
        abortOnError = true
    }
}

dependencies {
    val media3 = "1.6.1"
    implementation("androidx.media3:media3-exoplayer:$media3")
    implementation("androidx.media3:media3-exoplayer-hls:$media3")
    implementation("androidx.media3:media3-ui:$media3")
    // FFmpeg's audio decoders (Jellyfin's build of Media3's extension): Dolby
    // (AC3, E-AC3, TrueHD) and DTS play as ordinary sound on any TV, instead
    // of silence where the TV or box can't take them.
    implementation("org.jellyfin.media3:media3-ffmpeg-decoder:$media3+2")

    // The app's own screens (Home, titles, search, Live TV): Jetpack Compose
    // with Compose for TV, the toolkit Google TV's own apps use.
    implementation(platform("androidx.compose:compose-bom:2024.10.01"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.foundation:foundation")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.activity:activity-compose:1.9.3")
    // FileProvider, to hand a downloaded update to Android's installer.
    implementation("androidx.core:core-ktx:1.13.1")
    // The QR code on Search that opens search on a phone.
    implementation("com.google.zxing:core:3.5.3")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    implementation("androidx.tv:tv-material:1.0.0")
    // Installs Compose's ready-made startup profiles on a sideloaded APK (the
    // Play Store would do this itself): screens are compiled ahead, not on
    // first use, so scrolling is smooth from the start.
    implementation("androidx.profileinstaller:profileinstaller:1.4.1")
    implementation("io.coil-kt:coil-compose:2.7.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")

    // The walk-through test: the app on an Android TV emulator, driven with
    // the remote's keys against a stand-in Cue server.
    androidTestImplementation("androidx.test:runner:1.6.2")
    androidTestImplementation("androidx.test:core-ktx:1.6.1")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
    androidTestImplementation("androidx.test.uiautomator:uiautomator:2.3.0")
    androidTestImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")
    // The screenshots (app/src/test).
    testImplementation("junit:junit:4.13.2")
    testImplementation("org.robolectric:robolectric:4.14.1")
    testImplementation("io.github.takahirom.roborazzi:roborazzi:1.36.0")
    testImplementation("io.github.takahirom.roborazzi:roborazzi-compose:1.36.0")
    testImplementation("androidx.compose.ui:ui-test-junit4")
    testImplementation("androidx.test:core-ktx:1.6.1")
    testImplementation("androidx.test.ext:junit:1.2.1")
    testImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
    robolectricJars("org.robolectric:android-all-instrumented:15-robolectric-12650502-i7") { isTransitive = false }
}
