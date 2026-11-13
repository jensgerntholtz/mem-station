# Front page recording: the Timings workspace and the apply review. No runs, so
# nothing here depends on the fake stress-ng. Record with:
#   ./demo/record.sh frontpage.tape
Output demo/frontpage.gif

Set Shell bash
Set FontSize 14
Set Padding 10
Set Columns 120
Set Rows 34
Set Theme "Dracula"
Set TypingSpeed 60ms
Set PlaybackSpeed 1.2

# Launch while hidden: the first visible frame is the app already painted, not a
# shell prompt.
Hide
Type "clear && ./demo/.build/mem-station"
Enter
Sleep 2s
Show

Sleep 800ms

# Channels moves by one, DRAM frequency by 100 MT/s; the ratio lock rescales the
# primary timings, which the review below shows as a diff.
Right
Sleep 350ms
Right
Sleep 250ms
Down
Down
Right
Sleep 500ms

# The review is the last frame: it carries the profile-only promise.
Ctrl+S
Sleep 3000ms
