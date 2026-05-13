package bypass.whitelist.iran.tunnel

enum class TunnelMode(val label: String, val wire: String) {
    VIDEO("Video", "vp8"),
    DC("DC", "dc");

    fun relayMode(platform: CallPlatform): String = "${platform.id}-video-joiner"
}
