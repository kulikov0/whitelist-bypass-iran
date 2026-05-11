package bypass.whitelist.iran.tunnel

enum class TunnelMode(val label: String) {
    VIDEO("Video");

    fun relayMode(platform: CallPlatform): String = "${platform.id}-video-joiner"
}
