// Shared test doubles (fake settings, fake MQTT client, test dispatchers).
// Only ever used as a testImplementation dependency.
plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "uk.co.aaronburt.satellite.testing"
    compileSdk = 37

    defaultConfig {
        minSdk = 26
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17)
    }
}

dependencies {
    api(project(":core:common"))
    api(project(":core:model"))
    api(project(":core:datastore"))
    api(project(":core:mqtt"))

    api(libs.kotlinx.coroutines.core)
    api(libs.kotlinx.coroutines.test)
    api(libs.robolectric)
    api(libs.junit)
}
