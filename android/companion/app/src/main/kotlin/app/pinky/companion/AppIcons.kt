package app.pinky.companion

import android.content.Context
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.Canvas
import android.util.Base64
import java.io.ByteArrayOutputStream

/**
 * Icons of installed apps, for the desktop to show next to their names.
 * Package visibility is the same as for AppLabels: on Android 11 and later
 * the manifest's launcher query makes every app with a launcher entry
 * visible, and others read as not installed.
 */
class AppIcons(private val context: Context) {

    /** The `app_icon` result: the package's icon as a square PNG, base64 in `png`. */
    fun icon(pkg: String, size: Int): String {
        if (pkg.isEmpty()) throw RequestError("bad_request", "missing package")
        val png = Base64.encodeToString(render(pkg, size), Base64.NO_WRAP)
        return Json().beginObject().field("png", png).endObject().toString()
    }

    /**
     * Draws the icon into a size by size bitmap. An adaptive icon draws its
     * layers clipped to the phone's icon mask, as the launcher shows it.
     */
    private fun render(pkg: String, size: Int): ByteArray {
        val drawable = try {
            context.packageManager.getApplicationIcon(pkg)
        } catch (_: PackageManager.NameNotFoundException) {
            throw RequestError("not_found", "no app with package $pkg")
        }
        val bitmap = Bitmap.createBitmap(size, size, Bitmap.Config.ARGB_8888)
        try {
            // A fresh drawable from the package manager: its bounds are ours to set.
            drawable.setBounds(0, 0, size, size)
            drawable.draw(Canvas(bitmap))
            val out = ByteArrayOutputStream()
            if (!bitmap.compress(Bitmap.CompressFormat.PNG, 100, out)) {
                throw RequestError("internal", "cannot encode the icon of $pkg as PNG")
            }
            return out.toByteArray()
        } finally {
            bitmap.recycle()
        }
    }

    companion object {
        const val DEFAULT_SIZE = 192
        const val MIN_SIZE = 16
        const val MAX_SIZE = 192
    }
}
