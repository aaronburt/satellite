package uk.co.aaronburt.satellite.app.devicecheck

import org.junit.Assert.assertEquals
import org.junit.Test
import uk.co.aaronburt.satellite.model.Transport

class DeviceCheckReportTest {

    @Test
    fun `formats every field as sorted key value pairs the script can split`() {
        val report = DeviceCheckReport(
            deviceId = "abc123",
            reporting = true,
            transport = Transport.WEBHOOK,
            enabledControls = setOf("volume_media", "mic_muted"),
            micMuted = false,
            volumePercent = 40,
        )

        assertEquals(
            "device_id=abc123;reporting=true;transport=webhook;controls=mic_muted,volume_media;mic_muted=false;volume=40",
            report.format(),
        )
    }

    @Test
    fun `unknown audio state is reported as unknown, not as false`() {
        val report = DeviceCheckReport("id", false, Transport.MQTT, emptySet(), null, null)

        assertEquals(
            "device_id=id;reporting=false;transport=mqtt;controls=;mic_muted=unknown;volume=unknown",
            report.format(),
        )
    }
}
