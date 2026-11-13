# Timings workspace: semantic arrow-key steps, the apply review, the SPD/IMC
# views and the help overlay. Record with ./demo/record.sh timings.tape
Output demo/timings.gif

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

# Channels moves by one, DRAM frequency by 100 MT/s.
Right
Sleep 400ms
Right
Sleep 300ms
Down
Down
Right
Sleep 500ms

# Review the diff, then back out without applying.
Ctrl+S
Sleep 2s
Escape
Sleep 600ms

# SPD and IMC sub-views, then back to the editor.
Type "]"
Sleep 1200ms
Type "]"
Sleep 1200ms
Type "["
Sleep 400ms
Type "["
Sleep 400ms

# Help overlay floats over the workspace.
Type "?"
Sleep 1500ms
Escape
Sleep 400ms

Type "q"
