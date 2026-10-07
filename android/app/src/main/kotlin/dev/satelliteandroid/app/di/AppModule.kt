package dev.satelliteandroid.app.di

import android.content.Context
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import dev.satelliteandroid.common.coroutines.ApplicationScope
import dev.satelliteandroid.common.coroutines.DefaultDispatchersProvider
import dev.satelliteandroid.common.coroutines.DispatchersProvider
import dev.satelliteandroid.datastore.DataStoreSettingsRepository
import dev.satelliteandroid.datastore.SettingsRepository
import dev.satelliteandroid.mqtt.HiveMqttClient
import dev.satelliteandroid.mqtt.MqttClient
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object AppModule {

    @Provides
    @Singleton
    fun provideDispatchersProvider(): DispatchersProvider = DefaultDispatchersProvider()

    @Provides
    @Singleton
    @ApplicationScope
    fun provideApplicationScope(): CoroutineScope =
        CoroutineScope(SupervisorJob() + Dispatchers.Default)

    @Provides
    @Singleton
    fun provideSettingsRepository(
        @ApplicationContext context: Context,
    ): SettingsRepository = DataStoreSettingsRepository(context)

    @Provides
    @Singleton
    fun provideMqttClient(impl: HiveMqttClient): MqttClient = impl
}
