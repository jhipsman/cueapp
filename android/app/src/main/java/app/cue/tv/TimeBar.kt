package app.cue.tv

import android.content.Context
import android.graphics.Canvas
import android.graphics.Paint
import android.graphics.Rect
import android.view.KeyEvent
import android.view.View

// The player's time bar: what has played in Cue's cyan, what has loaded in
// grey. Focused, it grows a little and shows a knob; left and right then
// move through the video, faster the longer they are held.
class TimeBar(context: Context) : View(context) {
    var duration = 0L
        private set
    var position = 0L
        private set
    private var buffered = 0L

    // Called with where to go when the remote moves the knob.
    var onSeek: ((Long) -> Unit)? = null

    private val dp = screenUnit(context)
    private val track = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = 0x40FFFFFF }
    private val loaded = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = 0x66FFFFFF }
    private val played = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = context.getColor(R.color.cue_teal) }
    private val glow = Paint(Paint.ANTI_ALIAS_FLAG).apply { color = 0x5534D1BF }

    // setAccent colors the bar and its knob's glow (the profile's color).
    fun setAccent(color: Int) {
        played.color = color
        glow.color = (color and 0x00FFFFFF) or 0x55000000
        invalidate()
    }

    init {
        isFocusable = true
        isFocusableInTouchMode = true
    }

    fun update(position: Long, duration: Long, buffered: Long) {
        this.position = position
        this.duration = duration
        this.buffered = buffered
        invalidate()
    }

    override fun onMeasure(widthMeasureSpec: Int, heightMeasureSpec: Int) {
        setMeasuredDimension(MeasureSpec.getSize(widthMeasureSpec), (30 * dp).toInt())
    }

    private fun xOf(t: Long, left: Float, width: Float): Float =
        if (duration <= 0) left else left + width * (t.coerceIn(0, duration).toFloat() / duration)

    override fun onDraw(canvas: Canvas) {
        val h = if (isFocused) 7 * dp else 4 * dp
        val cy = height / 2f
        val left = 12 * dp
        val width = this.width - 24 * dp
        val r = h / 2
        canvas.drawRoundRect(left, cy - r, left + width, cy + r, r, r, track)
        canvas.drawRoundRect(left, cy - r, xOf(buffered, left, width), cy + r, r, r, loaded)
        val x = xOf(position, left, width)
        canvas.drawRoundRect(left, cy - r, x, cy + r, r, r, played)
        if (isFocused) {
            canvas.drawCircle(x, cy, 14 * dp, glow)
            canvas.drawCircle(x, cy, 9 * dp, played)
        }
    }

    override fun onFocusChanged(gainFocus: Boolean, direction: Int, previouslyFocusedRect: Rect?) {
        super.onFocusChanged(gainFocus, direction, previouslyFocusedRect)
        invalidate()
    }

    override fun onKeyDown(keyCode: Int, event: KeyEvent): Boolean {
        if (duration <= 0) return super.onKeyDown(keyCode, event)
        val sign = when (keyCode) {
            KeyEvent.KEYCODE_DPAD_LEFT -> -1
            KeyEvent.KEYCODE_DPAD_RIGHT -> 1
            else -> return super.onKeyDown(keyCode, event)
        }
        // 10 seconds a press; held down it speeds up to a minute a step.
        val step = 10_000L * (1 + event.repeatCount / 3).coerceAtMost(6)
        val to = (position + sign * step).coerceIn(0, duration)
        position = to
        invalidate()
        onSeek?.invoke(to)
        return true
    }
}
