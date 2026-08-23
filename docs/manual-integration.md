# Manual integration checklist

Do this only after `tmux -V` succeeds. Run it in a disposable tmux session with
no valuable command sitting at a shell prompt.

## Fixed-string safety proof

1. Create two panes and capture the target pane explicitly with
   `target="$TMUX_PANE"` in the target pane.
2. Focus the other pane before each insertion to prove focus changes do not
   redirect text.
3. Insert ordinary text:

   ```bash
   ./vox insert --target "$target" --text 'hello café 🚀'
   ```

4. Insert multiline text without a final newline:

   ```bash
   printf 'first line\nsecond line' | ./vox insert --target "$target" --stdin
   ```

5. At a shell prompt, insert this dangerous-looking literal and verify it is
   visible but has not executed:

   ```bash
   printf '%s' 'printf DANGEROUS_COMMAND_WAS_EXECUTED > /tmp/vox-must-not-exist' | ./vox insert --target "$target" --stdin
   ```

6. Press Ctrl-C to clear the literal. Confirm `/tmp/vox-must-not-exist` does not
   exist. The application itself must never create or remove that sentinel.
7. Confirm Unicode, spaces, and both lines are preserved and no Enter was sent.

## Interactive CLIs

Repeat ordinary and multiline cases in:

- Codex CLI;
- one other locally available interactive CLI.

For each CLI, verify the full text remains in the editable input buffer, focus
can move during transcription, and nothing is submitted until Enter is pressed
manually.

## Push-to-talk

1. Run `./vox toggle --target "$TMUX_PANE"` and speak.
2. Run the exact command again to stop.
3. Verify the text appears in the original pane and is not submitted.
4. Start one recording and try starting another for a different pane; verify it
   is rejected.
5. Force transcription failure (for example, temporarily set an invalid
   `VOX_WHISPER_MODEL`) and verify the pane remains untouched.
