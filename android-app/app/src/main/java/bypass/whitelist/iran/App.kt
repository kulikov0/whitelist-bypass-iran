package bypass.whitelist.iran

import android.app.Application
import bypass.whitelist.iran.util.Prefs

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        Prefs.init(this)
    }
}
