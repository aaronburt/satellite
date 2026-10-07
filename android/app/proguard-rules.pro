# App-specific R8 rules.

# HiveMQ MQTT client / Netty (see the HiveMQ Android installation guide).
-keepclassmembernames class io.netty.** { *; }
-keepclassmembers class org.jctools.** { *; }

# Netty references optional transports / SSL providers that are not present on
# Android. They are only loaded reflectively, so silence the missing-class
# warnings instead of failing the release build.
-dontwarn io.netty.channel.epoll.**
-dontwarn io.netty.channel.kqueue.**
-dontwarn io.netty.channel.unix.**
-dontwarn io.netty.handler.codec.http.**
-dontwarn io.netty.handler.proxy.**
-dontwarn io.netty.handler.ssl.**
-dontwarn io.netty.internal.tcnative.**
-dontwarn org.bouncycastle.**
-dontwarn org.conscrypt.**
-dontwarn org.eclipse.jetty.**
-dontwarn org.slf4j.**
-dontwarn org.apache.log4j.**
-dontwarn org.apache.logging.log4j.**
-dontwarn reactor.blockhound.**
-dontwarn com.aayushatharva.brotli4j.**
-dontwarn com.github.luben.zstd.**
-dontwarn com.google.protobuf.**
-dontwarn nl.jqno.equalsverifier.**
-dontwarn java.lang.management.**
-dontwarn javax.annotation.**
-dontwarn sun.misc.**
