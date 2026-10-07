pluginManagement {
    repositories {
        google {
            content {
                includeGroupByRegex("com\\.android.*")
                includeGroupByRegex("com\\.google.*")
                includeGroupByRegex("androidx.*")
            }
        }
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "SatelliteAndroid"

include(":app")
include(":core:common")
include(":core:model")
include(":core:datastore")
include(":core:mqtt")
include(":core:telemetry")
include(":core:discovery")
include(":core:reporter")
include(":core:designsystem")
include(":feature:dashboard")
include(":feature:settings")
