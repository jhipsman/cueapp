plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
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
    lint {
        // Builds on GitHub must not stop over a lint opinion; real errors still fail compiling.
        checkReleaseBuilds = false
        abortOnError = false
    }
}

dependencies {
    val media3 = "1.4.1"
    implementation("androidx.media3:media3-exoplayer:$media3")
    implementation("androidx.media3:media3-exoplayer-hls:$media3")
    implementation("androidx.media3:media3-ui:$media3")
}
