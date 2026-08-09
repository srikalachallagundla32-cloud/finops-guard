# demo.tape — Charm VHS specification for the FinOps-Guard "Cost Reactor" TUI.
#
# Generate the GIF with:
#   brew install vhs        # (also pulls ttyd + ffmpeg)
#   vhs demo.tape           # writes assets/demo.gif
# or simply:  make demo-gif
#
# Records the default animated TUI: boot sequence → Doom-fire cost canvas →
# findings navigation → quit.

Output assets/demo.gif

Set Shell        "bash"
Set FontSize     16
Set Width        1200
Set Height       700
Set Padding      24
Set Theme        "Catppuccin Mocha"
Set TypingSpeed  55ms
Set PlaybackSpeed 1.0

# --- Build silently so the recording opens on a clean prompt ---
Hide
Type "make build" Enter
Wait
Ctrl+L
Show

# --- Launch the Cost Reactor against the sample "AI slop" file ---
Type "./bin/finops-guard --pricing=pricing.json --scan=examples/ingest_pipeline.py"
Sleep 700ms
Enter

# Boot sequence: ASCII logo + scanline sweep (~2s)
Sleep 2500ms

# Doom-fire cost canvas + dollar-rain particles + rotating CFO quote
Sleep 3s

# Navigate the detected leaks
Down@900ms 2
Sleep 1500ms
Up@900ms 1
Sleep 2500ms

# Quit cleanly
Type "q"
Sleep 800ms
