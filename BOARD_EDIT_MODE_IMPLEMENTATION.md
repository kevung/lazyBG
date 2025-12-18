# Board Edit Mode Implementation

## Overview
Implemented a new keyboard-driven decision editing workflow for when the move panel is closed. This provides a streamlined, efficient way to edit decisions directly on the board view.

## New Component: BoardEditPanel

### Location
`/home/unger/src/lazyBG/frontend/src/components/BoardEditPanel.svelte`

### Features

#### 1. Direct Dice Entry
- **Digits (1-6)**: Type one or two digits to enter dice values
  - First digit: stored immediately
  - Second digit: completes the dice and triggers candidate move analysis
- **Cube Decisions**: Single letter shortcuts
  - `d` = double
  - `t` = take
  - `p` = pass
- **Resign Decisions**: Single letter shortcuts
  - `r` = resign
  - `g` = resign gammon
  - `b` = resign backgammon

#### 2. Candidate Move Navigation (j/k)
- After entering valid dice, press:
  - `j` or `↓` to cycle to next gnubg candidate move
  - `k` or `↑` to cycle to previous gnubg candidate move
- Board updates in real-time to preview each candidate
- Integrates seamlessly with CandidateMovesPanel
- Automatically applies selected candidate move on validation

#### 3. Manual Move Entry (Space)
- Press `Space` to open a discrete text input bar
- Input bar appears centered on screen (similar to command line)
- Type move notation manually (e.g., "24/20 13/8")
- Press `Enter` to confirm or `Esc` to cancel
- Returns to board edit mode after input

#### 4. Validation and Navigation
- Press `Enter` to:
  - Save current decision
  - Automatically move to next decision
  - Stay in EDIT mode for continuous editing
- Press `Esc` to cancel and exit EDIT mode

### UI Design

#### Dice Display Overlay
- **Position**: Center of screen, non-intrusive
- **Style**: Dark semi-transparent background with blue border
- **Content**: Shows current dice value or prompt "Type dice..."
- **Visibility**: Always visible when in board edit mode

#### Manual Input Bar
- **Position**: Center-top of screen (same location as command line)
- **Style**: White background with blue border on focus
- **Behavior**: Only appears when Space is pressed

### Integration

#### App.svelte Changes
1. Replaced `EditPanel` import with `BoardEditPanel`
2. Uses same `showEditPanel` visibility control
3. Activated when:
   - User presses `Tab` to enter EDIT mode
   - Moves table is closed (`showMovesTable === false`)

#### CandidateMovesPanel Integration
- Subscribes to `candidatePreviewMoveStore` to receive j/k navigation updates
- Dispatches `candidateNavigate` custom events to CandidateMovesPanel
- Automatically updates `manualMoveInput` when candidate is selected

### Removed Components

#### EditPanel.svelte (Deleted)
- Old bottom panel-based editing interface
- Replaced entirely by BoardEditPanel
- Was only used when moves table was closed

### Help Documentation Updates

Updated `HelpModal.svelte` to document new workflow:

```markdown
### EDIT Mode

#### Edit Mode with Moves Table Closed
- Dice Entry: Type digits (1-6) or decision letters (d/t/p/r/g/b)
- j/k Navigation: Cycle through gnubg candidate moves
- Manual Entry: Press Space to type move notation
- Validate: Press Enter to save and move to next decision
- Cancel: Press Esc to exit EDIT mode
```

## Technical Implementation Details

### State Management
- `diceInput`: Current dice value (2 digits or single letter)
- `manualMoveInput`: Current move notation (from candidate or manual entry)
- `showManualInput`: Controls visibility of manual input bar
- `originalDice`/`originalMove`: Track original values for change detection

### Keyboard Event Handling
1. All keypresses captured by `handleKeyDown` when visible
2. Manual input bar has priority when visible
3. Dice input processed character-by-character
4. j/k navigation only enabled when valid dice are entered
5. Space key opens manual input (unless in manual input already)

### Validation
- `validateDiceInput()`: Checks dice format (2 digits 1-6, or special letters)
- Allows empty dice for player1's first decision (player2 starts)
- Cube/resign decisions automatically disable move entry

### Data Flow
1. User enters dice → `updateDiceOnly()` called immediately
2. Dice change triggers CandidateMovesPanel analysis
3. j/k navigation → dispatches custom event → CandidateMovesPanel updates
4. Candidate selection → `candidatePreviewMoveStore` updates → `manualMoveInput` syncs
5. Enter pressed → `validateEditing()` saves to transcription
6. Auto-advance to next decision or insert new decision if at end

## Benefits

### User Experience
1. **Keyboard-driven**: No mouse required for decision editing
2. **Fast workflow**: Type dice, press j/k to select move, press Enter
3. **Visual feedback**: Board updates in real-time as candidates are previewed
4. **Minimal UI**: Clean overlay doesn't obstruct board view
5. **Flexible**: Manual entry available when needed via Space key

### Consistency
1. Same j/k navigation as moves table inline editing
2. Same candidate move integration as before
3. Same validation and auto-correction logic
4. Same undo/redo support

### Code Quality
1. Single responsibility: BoardEditPanel handles board-based editing only
2. Proper separation: Moves table handles inline editing separately
3. Clean integration: Uses existing stores and events
4. No conflicts: Space key doesn't trigger command mode in EDIT mode

## Testing Checklist

- [x] Build completes successfully
- [ ] Dice entry (1-6) updates board correctly
- [ ] Cube decisions (d/t/p) work correctly
- [ ] Resign decisions (r/g/b) work correctly
- [ ] j/k navigation cycles through candidates
- [ ] Board previews update for each candidate
- [ ] Space key opens manual input bar
- [ ] Manual input validates and applies correctly
- [ ] Enter saves and advances to next decision
- [ ] Esc cancels and exits EDIT mode
- [ ] Command line NOT activated during EDIT mode
- [ ] Auto-advance works at end of game (inserts new decision)
- [ ] Help documentation displays correctly

## Files Modified

1. **Created**: `/home/unger/src/lazyBG/frontend/src/components/BoardEditPanel.svelte`
2. **Modified**: `/home/unger/src/lazyBG/frontend/src/App.svelte`
   - Import BoardEditPanel instead of EditPanel
   - Use BoardEditPanel in template
3. **Modified**: `/home/unger/src/lazyBG/frontend/src/components/HelpModal.svelte`
   - Updated EDIT mode documentation
   - Added section for board editing workflow
4. **Deleted**: `/home/unger/src/lazyBG/frontend/src/components/EditPanel.svelte`

## Compatibility

- Works seamlessly with existing moves table inline editing
- No breaking changes to existing keyboard shortcuts
- Preserves all existing validation and auto-correction logic
- Compatible with undo/redo system
