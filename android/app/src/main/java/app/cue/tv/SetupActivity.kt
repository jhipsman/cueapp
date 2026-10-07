package app.cue.tv

import android.app.Activity
import android.content.Intent
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.text.InputType
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.View
import android.view.inputmethod.EditorInfo
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView

// The first screen: where is your Cue server? Shown on first start, when the
// server can't be reached, and from "Change server" in Watch's menu.
class SetupActivity : Activity() {
    private lateinit var input: EditText
    private lateinit var status: TextView
    private lateinit var connect: Button

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val dp = { v: Int -> TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v.toFloat(), resources.displayMetrics).toInt() }
        val teal = getColor(R.color.cue_teal)

        val title = TextView(this).apply {
            text = "Cue"
            setTextColor(teal)
            textSize = 44f
            typeface = Typeface.DEFAULT_BOLD
        }
        val hint = TextView(this).apply {
            text = "Enter the address of your Cue server. At home that's your computer's address, like 192.168.1.20 (Cue's port 8264 is added for you). Away from home, use its Tailscale name."
            setTextColor(getColor(R.color.cue_dim))
            textSize = 18f
            setPadding(0, dp(12), 0, dp(20))
        }
        input = EditText(this).apply {
            setText(intent.getStringExtra(EXTRA_ADDRESS) ?: Server.address(this@SetupActivity) ?: "")
            hint = "192.168.1.20"
            setHintTextColor(getColor(R.color.cue_dim))
            setTextColor(getColor(R.color.cue_text))
            textSize = 22f
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
            imeOptions = EditorInfo.IME_ACTION_GO
            setSingleLine()
            background = GradientDrawable().apply {
                setColor(getColor(R.color.cue_panel))
                cornerRadius = dp(8).toFloat()
                setStroke(dp(2), teal)
            }
            setPadding(dp(16), dp(12), dp(16), dp(12))
            setOnEditorActionListener { _, action, event ->
                if (action == EditorInfo.IME_ACTION_GO || event?.keyCode == KeyEvent.KEYCODE_ENTER) {
                    tryConnect(); true
                } else false
            }
        }
        connect = Button(this).apply {
            text = "Connect"
            textSize = 20f
            isAllCaps = false
            setTextColor(getColor(R.color.cue_bg))
            background = GradientDrawable().apply {
                setColor(getColor(R.color.cue_text))
                cornerRadius = dp(8).toFloat()
            }
            setPadding(dp(32), dp(12), dp(32), dp(12))
            setOnClickListener { tryConnect() }
            setOnFocusChangeListener { v, has -> v.alpha = if (has) 1f else 0.8f }
        }
        status = TextView(this).apply {
            setTextColor(0xFFFF8A80.toInt())
            textSize = 18f
            setPadding(0, dp(16), 0, 0)
            text = intent.getStringExtra(EXTRA_PROBLEM) ?: ""
        }

        val column = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(64), dp(48), dp(64), dp(48))
            gravity = Gravity.CENTER_VERTICAL
            addView(title)
            addView(hint)
            addView(input, LinearLayout.LayoutParams(dp(560), LinearLayout.LayoutParams.WRAP_CONTENT))
            addView(connect, LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(20) })
            addView(status)
        }
        setContentView(column)
        if (input.text.isNullOrBlank()) input.requestFocus() else connect.requestFocus()
    }

    private fun tryConnect() {
        val address = Server.normalize(input.text.toString())
        if (address.isEmpty()) {
            status.text = "Type your server's address first."
            return
        }
        connect.isEnabled = false
        status.setTextColor(getColor(R.color.cue_dim))
        status.text = "Connecting to $address…"
        Thread {
            val problem = Server.check(address)
            runOnUiThread {
                connect.isEnabled = true
                if (problem == null) {
                    Server.save(this, address)
                    startActivity(Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TASK or Intent.FLAG_ACTIVITY_NEW_TASK))
                    finish()
                } else {
                    status.setTextColor(0xFFFF8A80.toInt())
                    status.text = problem
                    connect.visibility = View.VISIBLE
                }
            }
        }.start()
    }

    companion object {
        const val EXTRA_ADDRESS = "address"
        const val EXTRA_PROBLEM = "problem"
    }
}
