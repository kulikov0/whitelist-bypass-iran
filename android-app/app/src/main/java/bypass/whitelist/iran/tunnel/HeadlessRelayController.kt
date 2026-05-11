package bypass.whitelist.iran.tunnel

import android.util.Log
import bypass.whitelist.iran.util.ParamCallback
import bypass.whitelist.iran.util.Prefs
import bypass.whitelist.iran.util.SocksAuth
import java.io.BufferedWriter
import java.io.File
import java.io.OutputStreamWriter
import java.net.Inet4Address
import java.net.InetAddress

class HeadlessRelayController(
    private val nativeLibDir: String,
    private val relayMode: String = "bale-headless-joiner",
    private val onLog: ParamCallback<String>,
    private val onStatus: ParamCallback<VpnStatus>,
) {
    private var process: Process? = null
    private var thread: Thread? = null
    private var stdinWriter: BufferedWriter? = null
    private val pendingCommands = mutableListOf<String>()

    @Volatile
    var isRunning = false
        private set

    fun start() {
        stop()
        isRunning = true

        val relayBin = File(nativeLibDir, "librelay.so")
        if (!relayBin.exists()) {
            onLog("Relay binary not found")
            onStatus(VpnStatus.CALL_FAILED)
            return
        }

        thread = Thread {
            val socksPort = Prefs.socksPort
            if (!PortGuard.ensurePortFree(socksPort)) {
                onLog("SOCKS5 port $socksPort is busy and could not be freed")
                onStatus(VpnStatus.PORT_BUSY)
                isRunning = false
                return@Thread
            }
            try {
                val pb = ProcessBuilder(
                    relayBin.absolutePath,
                    "--mode", relayMode,
                    "--socks-port", "$socksPort",
                    "--socks-user", SocksAuth.user,
                    "--socks-pass", SocksAuth.pass,
                )
                pb.redirectErrorStream(true)
                val proc = pb.start()
                synchronized(this) {
                    process = proc
                    stdinWriter = BufferedWriter(OutputStreamWriter(proc.outputStream))
                    pendingCommands.forEach { writeStdin(it) }
                    pendingCommands.clear()
                }
                onLog("Headless relay started mode=$relayMode SOCKS5 ${SocksAuth.user}:${SocksAuth.pass}@127.0.0.1:$socksPort")

                proc.inputStream.bufferedReader().forEachLine { line ->
                    when {
                        line.startsWith("RESOLVE:") -> {
                            val hostname = line.removePrefix("RESOLVE:")
                            try {
                                val all = InetAddress.getAllByName(hostname)
                                val address = all.firstOrNull { it is Inet4Address } ?: all.first()
                                val resolvedIP = address.hostAddress ?: ""
                                Log.d("RELAY", "Resolved $hostname -> $resolvedIP")
                                writeStdin(resolvedIP)
                            } catch (e: Exception) {
                                Log.e("RELAY", "DNS resolve failed for $hostname", e)
                                writeStdin("")
                            }
                        }
                        line.startsWith("STATUS:") -> {
                            val status = line.removePrefix("STATUS:")
                            Log.d("RELAY", "status: $status")
                            when {
                                status == "READY" -> onStatus(VpnStatus.STARTING)
                                status == "CONNECTING" -> onStatus(VpnStatus.CONNECTING)
                                status == "TUNNEL_CONNECTED" -> onStatus(VpnStatus.TUNNEL_ACTIVE)
                                status == "TUNNEL_LOST" -> onStatus(VpnStatus.TUNNEL_LOST)
                                status.startsWith("ERROR") -> {
                                    val msg = status.substringAfter("ERROR:", "")
                                    if (msg.isNotEmpty()) onLog("Relay error: $msg")
                                    onStatus(VpnStatus.CALL_FAILED)
                                }
                            }
                        }
                        else -> {
                            Log.d("RELAY", line)
                            onLog(line)
                            if (line.contains("pub PC state: connected")) onStatus(VpnStatus.CALL_CONNECTED)
                        }
                    }
                }
                proc.waitFor()
                Log.d("RELAY", "Headless relay exited: ${proc.exitValue()}")
            } catch (e: Exception) {
                if (isRunning) {
                    Log.e("RELAY", "Headless relay error", e)
                    onLog("Relay error: ${e.message}")
                    onStatus(VpnStatus.CALL_FAILED)
                }
            }
        }.also { it.start() }
    }

    fun sendJoinParams(joinJson: String) {
        writeStdin("JOIN:$joinJson")
    }

    @Synchronized
    fun stop() {
        isRunning = false
        process?.let {
            it.destroy()
            it.waitFor()
        }
        process = null
        stdinWriter = null
        thread?.interrupt()
        thread = null
    }

    @Synchronized
    private fun writeStdin(line: String) {
        if (stdinWriter == null) {
            pendingCommands.add(line)
            return
        }
        try {
            stdinWriter?.write(line)
            stdinWriter?.newLine()
            stdinWriter?.flush()
        } catch (e: Exception) {
            Log.e("RELAY", "writeStdin error: ${e.message}")
        }
    }
}
