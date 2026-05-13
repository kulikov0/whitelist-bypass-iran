package bypass.whitelist.iran.ui

import android.os.Bundle
import android.util.Log
import android.view.LayoutInflater
import android.view.View
import android.view.ViewGroup
import androidx.fragment.app.Fragment
import bypass.whitelist.iran.R
import bypass.whitelist.iran.tunnel.HeadlessRelayController
import bypass.whitelist.iran.tunnel.VpnStatus
import bypass.whitelist.iran.util.Prefs
import org.json.JSONObject

class HeadlessBaleFragment : Fragment() {

    private lateinit var relay: HeadlessRelayController

    private val host: JoinFragmentHost?
        get() = activity as? JoinFragmentHost

    override fun onCreateView(
        inflater: LayoutInflater,
        container: ViewGroup?,
        savedInstanceState: Bundle?,
    ): View = inflater.inflate(R.layout.fragment_headless_bale, container, false)

    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        val joinLink = requireArguments().getString(ARG_URL, "")
        val displayName = if (Prefs.useCustomName) Prefs.displayName else "Joiner"

        relay = HeadlessRelayController(
            nativeLibDir = requireContext().applicationInfo.nativeLibraryDir,
            relayMode = "bale-headless-joiner",
            onLog = { host?.appendLog(it) },
            onStatus = { status ->
                Log.d("BALE-HEADLESS", "status: $status")
                host?.onJoinStatus(status)
                when (status) {
                    VpnStatus.STARTING -> {
                        val params = JSONObject().apply {
                            put("joinLink", joinLink)
                            put("displayName", displayName)
                            put("resources", "default")
                            put("vp8Fps", Prefs.vp8Fps)
                            put("vp8Batch", Prefs.vp8Batch)
                            put("tunnelMode", Prefs.tunnelMode.wire)
                        }
                        relay.sendJoinParams(params.toString())
                    }
                    VpnStatus.TUNNEL_ACTIVE -> activity?.runOnUiThread { host?.requestVpn() }
                    else -> {}
                }
            },
        )
        relay.start()
    }

    override fun onDestroyView() {
        if (::relay.isInitialized) relay.stop()
        super.onDestroyView()
    }

    companion object {
        const val ARG_URL = "url"

        fun newInstance(url: String): HeadlessBaleFragment {
            return HeadlessBaleFragment().apply {
                arguments = Bundle().apply {
                    putString(ARG_URL, url)
                }
            }
        }
    }
}
