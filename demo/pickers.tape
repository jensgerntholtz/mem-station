# Profiles: the saved-profile picker with fuzzy filtering, then the JSON import
# browser. Record with ./demo/record.sh pickers.tape
Output demo/pickers.gif

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

# ctrl+l opens the picker; "/" filters, enter accepts, enter loads.
Ctrl+L
Sleep 1500ms
Type "/"
Sleep 300ms
Type "under"
Sleep 1000ms
Enter
Sleep 500ms
Enter
Sleep 1500ms

# ctrl+o browses for a profile JSON in the working directory.
Ctrl+O
Sleep 1500ms
Down
Sleep 400ms
Down
Sleep 400ms
Escape
Sleep 500ms

Type "q"
