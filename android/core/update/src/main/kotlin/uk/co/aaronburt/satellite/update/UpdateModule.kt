package uk.co.aaronburt.satellite.update

import dagger.Binds
import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent

@Module
@InstallIn(SingletonComponent::class)
abstract class UpdateModule {

    @Binds
    abstract fun releaseSource(impl: UpdateChecker): ReleaseSource

    @Binds
    abstract fun downloader(impl: UpdateDownloader): ApkDownloader

    @Binds
    abstract fun verifier(impl: UpdateVerifier): ApkVerifier

    @Binds
    abstract fun installer(impl: UpdateInstaller): ApkInstaller

    @Binds
    abstract fun installedVersion(impl: PackageInstalledVersion): InstalledVersion

    @Binds
    abstract fun updateStates(impl: UpdateEntityStates): UpdateStatesSource
}
