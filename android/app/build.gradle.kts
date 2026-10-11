plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
}

// Each build on GitHub gets the next version number, so a new APK installs
// over the old one.
val build = System.getenv("GITHUB_RUN_NUMBER")?.toIntOrNull() ?: 1

android {
    namespace = "app.cue.tv"
    compileSdk = 35

    defaultConfig {
        applicationId = "app.cue.tv"
        minSdk = 23 // Android 6 and later: every Fire TV Cube, Fire TV since 2017, Google TV
        targetSdk = 35
        versionCode = build
        versionName = "0.1.$build"
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
            isMinifyEnabled = false
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
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.7")
    implementation("androidx.tv:tv-material:1.0.0")
    implementation("io.coil-kt:coil-compose:2.7.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")
}
