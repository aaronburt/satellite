import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.ksp)
    alias(libs.plugins.hilt)
}

// Optional release signing. keystore.properties is gitignored; when absent the
// release build falls back to the debug keystore (handy for CI and dev builds).
val releaseKeystoreFile = rootProject.file("keystore.properties")
val releaseKeystoreExists = releaseKeystoreFile.exists()
val releaseKeystore = Properties().apply {
    if (releaseKeystoreExists) {
        releaseKeystoreFile.inputStream().use { load(it) }
    }
}
val releaseStoreFilePath = releaseKeystore.getProperty("storeFile")
val releaseStorePassword = releaseKeystore.getProperty("storePassword")
val releaseKeyAliasValue = releaseKeystore.getProperty("keyAlias")
val releaseKeyPasswordValue = releaseKeystore.getProperty("keyPassword")

android {
    namespace = "uk.co.aaronburt.satellite.app"
    compileSdk = 37

    defaultConfig {
        applicationId = "uk.co.aaronburt.satellite"
        minSdk = 26
        targetSdk = 36
        // Release builds override these from the git tag (`-PversionName=…`,
        // `-PversionCode=…`); local/CI debug builds fall back to the defaults.
        versionCode = (project.findProperty("versionCode") as String?)?.toIntOrNull() ?: 1
        versionName = (project.findProperty("versionName") as String?) ?: "0.1.0"
    }

    signingConfigs {
        // Fallback used when no release keystore is configured (CI / dev).
        create("dev") {
            storeFile = file("${System.getProperty("user.home")}/.android/debug.keystore")
            storePassword = "android"
            keyAlias = "androiddebugkey"
            keyPassword = "android"
        }

        // Real release signing, from keystore.properties (gitignored).
        if (releaseKeystoreExists) {
            create("release") {
                storeFile = rootProject.file(releaseStoreFilePath)
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAliasValue
                keyPassword = releaseKeyPasswordValue
            }
        }
    }

    buildTypes {
        release {
            signingConfig = if (releaseKeystoreExists) {
                signingConfigs.getByName("release")
            } else {
                signingConfigs.getByName("dev")
            }
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    buildFeatures {
        compose = true
    }

    packaging {
        resources {
            excludes += "META-INF/INDEX.LIST"
            excludes += "META-INF/io.netty.versions.properties"
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

dependencies {
    implementation(project(":core:common"))
    implementation(project(":core:model"))
    implementation(project(":core:datastore"))
    implementation(project(":core:mqtt"))
    implementation(project(":core:telemetry"))
    implementation(project(":core:discovery"))
    implementation(project(":core:reporter"))
    implementation(project(":core:designsystem"))
    implementation(project(":feature:dashboard"))
    implementation(project(":feature:settings"))

    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.lifecycle.runtime.ktx)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.activity.compose)
    implementation(libs.kotlinx.coroutines.android)

    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.material3)
    implementation(libs.compose.ui.tooling.preview)
    debugImplementation(libs.compose.ui.tooling)

    implementation(libs.hilt.android)
    ksp(libs.hilt.compiler)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)

    androidTestImplementation(platform(libs.compose.bom))
    androidTestImplementation(libs.compose.ui.test.junit4)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.test.rules)
    debugImplementation(libs.compose.ui.test.manifest)
}
