plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "dev.satelliteandroid.discovery"
    compileSdk = 37

    defaultConfig {
        minSdk = 26
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

dependencies {
    testImplementation(libs.junit)
    // Real org.json implementation for JVM unit tests (the Android one is a stub).
    testImplementation(libs.org.json)
}
