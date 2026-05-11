package bypass.whitelist.iran.ui

import bypass.whitelist.iran.tunnel.VpnStatus

interface JoinFragmentHost {
    fun appendLog(message: String)
    fun onJoinStatus(status: VpnStatus)
    fun onJoinStatusText(text: String)
    fun requestVpn()
}
