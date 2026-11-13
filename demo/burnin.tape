# Burn-in tab: cycle the job preset, start a run, and watch the error counter
# and verdict. Record with ./demo/record.sh burnin.tape
Output demo/burnin.gif

Set Shell bash
Set FontSize 16
Set Padding 12
Set Columns 120
Set Rows 40
Set Theme "Dracula"
Set TypingSpeed 70ms
Set PlaybackSpeed 1.2

Hide
Type "clear"
Enter
Show

Type "./demo/.build/mem-station"
Enter
Sleep 2s

# Tab 3 is Burn-in. The repo's burnin-memory.job is discovered next to the
# working directory, so right/left cycles between it and the embedded preset.
Type "3"
Sleep 1200ms
Right
Sleep 900ms
Left
Sleep 500ms
Enter
Sleep 7s

Type "q"
