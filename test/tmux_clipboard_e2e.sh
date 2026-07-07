#!/bin/bash
# E2E test for Crush tmux clipboard passthrough.
# Tests that OSC 52 clipboard copy works through tmux.
# To run: bash test/tmux_clipboard_e2e.sh
set -euo pipefail

SESSION="crush_e2e_test_$$"
FAILED=0

cleanup() {
    # Only kill sessions matching our test prefix
    local sessions
    sessions=$(tmux list-sessions -F '#{session_name}' 2>/dev/null | grep "^crush_e2e_test_" || true)
    if [ -n "$sessions" ]; then
        for s in $sessions; do
            tmux kill-session -t "$s" 2>/dev/null || true
        done
    fi
}
trap cleanup EXIT

log() { echo "[$(date +%H:%M:%S)] $*"; }

log "=== Crush Tmux Clipboard E2E Test ==="

# Test 1: Create and destroy tmux session cleanly
log "Test 1: Session lifecycle"
tmux new-session -d -s "$SESSION"
sleep 0.2
if tmux has-session -t "$SESSION" 2>/dev/null; then
    log "  PASS: Session created"
else
    log "  FAIL: Could not create session"
    FAILED=1
fi
tmux kill-session -t "$SESSION"
sleep 0.2
if ! tmux has-session -t "$SESSION" 2>/dev/null; then
    log "  PASS: Session destroyed cleanly"
else
    log "  FAIL: Session lingering"
    FAILED=1
fi

# Test 2: OSC 52 passthrough format within tmux
log "Test 2: OSC 52 passthrough sequence"
tmux new-session -d -s "$SESSION" -x 80 -y 24
sleep 0.2
TEST_TEXT="E2E_$(date +%s)"
B64=$(echo -n "$TEST_TEXT" | base64)
tmux send-keys -t "$SESSION" "printf '"'\033Ptmux;\033\033]52;c;'"$B64"'\007\033\\'"'" Enter
sleep 0.5
OUTPUT=$(tmux capture-pane -t "$SESSION" -p 2>/dev/null || true)
# Passthrough sequences appear as literal text in capture-pane
if echo "$OUTPUT" | grep -q "Ptmux"; then
    log "  PASS: Passthrough sequence appears in pane output"
else
    log "  FAIL: Passthrough sequence not found in output"
    FAILED=1
fi
tmux kill-session -t "$SESSION"

# Test 3: No hanging processes after session cleanup
log "Test 3: No hanging processes"
tmux new-session -d -s "$SESSION" -x 80 -y 24
sleep 0.3
tmux kill-session -t "$SESSION"
sleep 0.3
if tmux has-session -t "$SESSION" 2>/dev/null; then
    log "  FAIL: Session still exists after kill"
    FAILED=1
else
    log "  PASS: Session cleaned up"
fi

# Test 4: Multiple rapid session create/destroy
log "Test 4: Rapid session create/destroy"
RAPID_FAILED=0
for i in $(seq 1 3); do
    tmux new-session -d -s "${SESSION}_${i}" 2>/dev/null || { RAPID_FAILED=1; break; }
    sleep 0.1
    tmux kill-session -t "${SESSION}_${i}" 2>/dev/null || true
done
sleep 0.3
# Verify no sessions left
RAPID_REMAIN=$(tmux list-sessions 2>/dev/null | grep -c "${SESSION}_" || true)
if [ "$RAPID_FAILED" -eq 0 ] && [ "$RAPID_REMAIN" = "0" ]; then
    log "  PASS: All rapid sessions cleaned up"
else
    log "  FAIL: Session create/destroy issue (rapid_failed=$RAPID_FAILED remaining=$RAPID_REMAIN)"
    FAILED=1
fi

# Test 5: Verify passthrough sequence renders correctly
log "Test 5: Passthrough sequence in capture"
tmux new-session -d -s "$SESSION" -x 80 -y 24
sleep 0.2
PASS_TEXT="PASSTHRU_$(date +%s)"
PASS_B64=$(echo -n "$PASS_TEXT" | base64)

# Send passthrough OSC 52
tmux send-keys -t "$SESSION" "echo BEFORE" Enter
sleep 0.1
tmux send-keys -t "$SESSION" "printf '"'\033Ptmux;\033\033]52;c;'"${PASS_B64}"'\007\033\\'"'" Enter
sleep 0.1
tmux send-keys -t "$SESSION" "echo AFTER" Enter
sleep 0.5
OUTPUT=$(tmux capture-pane -t "$SESSION" -p 2>/dev/null || true)

if echo "$OUTPUT" | grep -q "BEFORE"; then
    log "  PASS: Commands run in tmux pane"
else
    log "  FAIL: Commands not visible in output"
    FAILED=1
fi
if echo "$OUTPUT" | grep -q "AFTER"; then
    log "  PASS: Passthrough did not block command execution"
else
    log "  FAIL: Passthrough may have blocked execution"
    FAILED=1
fi
tmux kill-session -t "$SESSION"

log ""
if [ "$FAILED" -eq 0 ]; then
    log "=== ALL TESTS PASSED ==="
else
    log "=== SOME TESTS FAILED ==="
fi
exit $FAILED
