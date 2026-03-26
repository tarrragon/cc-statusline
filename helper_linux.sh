#!/bin/bash
# Input method detection
im=""
if command -v ibus &>/dev/null && [ -n "$DBUS_SESSION_BUS_ADDRESS" ]; then
    im=$(ibus engine 2>/dev/null)
elif command -v fcitx5-remote &>/dev/null; then
    im=$(fcitx5-remote -n 2>/dev/null)
elif command -v fcitx-remote &>/dev/null; then
    im=$(fcitx-remote -n 2>/dev/null)
else
    im=$(setxkbmap -query 2>/dev/null | awk '/layout/{print $2}')
fi

# Caps Lock detection
caps="false"
brightness=$(cat /sys/class/leds/input*::capslock/brightness 2>/dev/null | head -1)
if [ "$brightness" = "1" ]; then
    caps="true"
elif command -v xset &>/dev/null; then
    xset_out=$(xset q 2>/dev/null | grep "Caps Lock")
    if echo "$xset_out" | grep -q "on"; then
        caps="true"
    fi
fi

echo "${im:-unknown}|${caps}"
