package bypass.whitelist.iran.tunnel

import androidx.annotation.StringRes
import bypass.whitelist.iran.R

enum class VpnStatus(@StringRes val labelRes: Int) {
    STARTING(R.string.vpn_starting),
    CONNECTING(R.string.vpn_connecting),
    CALL_CONNECTED(R.string.vpn_call_connected),
    TUNNEL_ACTIVE(R.string.vpn_tunnel_active),
    TUNNEL_LOST(R.string.vpn_tunnel_lost),
    CALL_DISCONNECTED(R.string.vpn_call_disconnected),
    CALL_FAILED(R.string.vpn_call_failed),
    PORT_BUSY(R.string.vpn_port_busy)
}
