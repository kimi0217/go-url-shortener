# Demo GIF for the README. Regenerate from the repo root with:
#   vhs docs/demo.tape
# Requires VHS (https://github.com/charmbracelet/vhs) and a free port 8080.

Output docs/demo.gif

Set Shell zsh
Set FontSize 20
Set Width 1000
Set Height 600
Set Padding 20
Set TypingSpeed 35ms
Set Theme "Catppuccin Mocha"

# --- Hidden setup: build and start the server, logs discarded ---
Hide
Type "go build -o server ./cmd/server" Enter
Wait
Type "./server >/dev/null 2>&1 &" Enter
Type "until curl -s localhost:8080/ >/dev/null; do sleep 0.1; done" Enter
Wait
# The short code is random, so the tape cannot hard-code it. curl's output is
# copied to a temp file, and Ctrl+G appends the code from it to the command
# being typed. Everything curl prints on screen is still its real output.
# -s is added because piping into tee would otherwise make curl show a
# progress meter that it never shows when writing straight to a terminal.
Type `DEMO_OUT=$(mktemp); curl() { command curl -s "$@" | tee "$DEMO_OUT"; }` Enter
Type `_code() { LBUFFER+=$(grep -o '"code":"[^"]*"' "$DEMO_OUT" | cut -d'"' -f4); }` Enter
Type `zle -N _code; bindkey '^G' _code` Enter
Type "clear" Enter
Sleep 300ms
Show

# --- 1. Create a short URL ---
Type "curl -s -X POST localhost:8080/shorten \" Enter
Type `  -d '{"url":"https://github.com/kimi0217/go-url-shortener"}'`
Sleep 300ms
Enter
Sleep 2s

# --- 2. Follow it: 302 Found + Location ---
Type "curl -i localhost:8080/"
Ctrl+G
Sleep 300ms
Enter
Sleep 2.5s

# --- 3. Invalid URL: 400 ---
Type `curl -i localhost:8080/shorten -d '{"url":"ftp://example.com"}'`
Sleep 300ms
Enter
Sleep 3s

# --- Hidden teardown: stop the server and clean up ---
Hide
Type `kill %1; rm -f server "$DEMO_OUT"` Enter
Sleep 300ms
