package bypass.whitelist.iran.tunnel

enum class CallPlatform(val id: String) {
    BALE("bale");

    companion object {
        fun fromUrl(@Suppress("UNUSED_PARAMETER") url: String): CallPlatform = BALE
    }
}
