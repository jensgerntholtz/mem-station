# Benchmark tab: run the memrate benchmark and watch the live log, parsed
# results and history fill in. Record with ./demo/record.sh bench.tape
Output demo/bench.gif

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

# Tab 2 is Benchmark; enter starts the run.
Type "2"
Sleep 1200ms
Enter
Sleep 7s

Type "q"
