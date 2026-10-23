plugins {
    alias(libs.plugins.android.library)
}

android {
    namespace = "uk.co.aaronburt.satellite.mqtt"
    compileSdk = 37

    defaultConfig {
        minSdk = 26
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    testOptions {
        unitTests.all {
            // The in-process HiveMQ broker used by HiveMqttClientTest reaches
            // into these JDK internals, as its own launcher allows.
            it.jvmArgs(
                "--add-opens", "java.base/java.lang=ALL-UNNAMED",
                "--add-opens", "java.base/java.nio=ALL-UNNAMED",
                "--add-opens", "java.base/sun.nio.ch=ALL-UNNAMED",
                "--add-exports", "java.base/jdk.internal.misc=ALL-UNNAMED",
            )
        }
    }

    packaging {
        resources {
            excludes += "META-INF/INDEX.LIST"
            excludes += "META-INF/io.netty.versions.properties"
        }
    }
}

dependencies {
    implementation(project(":core:common"))
    implementation(project(":core:model"))
    implementation(project(":core:datastore"))
    implementation(project(":core:discovery"))

    implementation(libs.hivemq.mqtt.client)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.javax.inject)

    testImplementation(project(":core:testing"))
    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.hivemq.embedded)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
}
