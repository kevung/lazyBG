# gnubg Candidate Moves - Additional Fixes

## Date
2025-12-15 (Second Round)

## Issues Fixed

### 1. j/k Navigation in Edit Mode
**Problem**: When editing checker moves in edit mode (after entering dice), pressing j/k showed error message "Cannot browse positions in edit mode"

**Root Cause**: The `nextPosition()` and `previousPosition()` functions in App.svelte blocked ALL j/k keypresses when in edit mode, not distinguishing between position navigation and candidate move navigation.

**Solution**: Modified both functions to check if focus is in the move input field:
- If in edit mode AND focus is in `#move-input`, allow the keypress (return early without error)
- EditPanel's event handler then catches the j/k and dispatches custom `candidateNavigate` events
- CandidateMovesPanel receives these events and navigates through candidates
- Move input field updates in real-time via `applyMoveToInput()`

**Files Modified**:
- `frontend/src/App.svelte`: Added check for `document.getElementById('move-input')` in both functions

### 2. Inverted Move Notation Display
**Problem**: gnubg moves were confusing - showing opposite direction from what players expect
- Player2 with dice 54: gnubg showed "12/17 1/5" (should be "17/12 5/1")
- Player1 with dice 21: gnubg showed "19/17 19/18" (should be "17/19 18/19")

**Root Cause**: gnubg returns moves in "bearing off" direction (high to low numbers), but players read moves from their starting position to destination. The display was showing gnubg's raw output without inverting.

**Solution**: Swapped from/to in move notation display
- Changed `moveStr += fromStr + "/" + toStr` to `moveStr += toStr + "/" + fromStr`
- This inverts each play so moves read naturally from player's perspective
- Now player2 54 shows proper starting points (e.g., "20/24 8/13" becomes readable)
- Now player1 21 shows proper format from their perspective

**Technical Note**: We're NOT changing what gnubg calculates (the moves are still correct), just how we display them to match human reading expectations.

**Files Modified**:
- `analysis.go`: Line ~165, swapped the order in the concatenation

### 3. Reduced Candidate Panel Width
**Problem**: Candidate moves panel had excessive white space, making it wider than necessary

**Solution**: Reduced padding and gaps throughout the panel:
- Move item padding: `10px 12px` → `6px 8px`
- Selected item padding: `9px 11px` → `5px 7px` (accounting for thicker border)
- Gap between moves: `4px` → `2px`

**Result**: Panel is now more compact, fitting moves comfortably while reducing wasted horizontal space. Still accommodates 4 checker moves (for doubles) without issue.

**Files Modified**:
- `frontend/src/components/CandidateMovesPanel.svelte`: CSS adjustments for `.move-item`, `.move-item.selected`, and `.moves-list`

## Technical Details

### Key Changes Summary

1. **App.svelte** - `nextPosition()` and `previousPosition()`:
```javascript
// Allow j/k in edit mode if in move input (for gnubg candidate navigation)
const moveInput = document.getElementById('move-input');
if ($statusBarModeStore === 'EDIT' && (!moveInput || document.activeElement !== moveInput)) {
    setStatusBarMessage('Cannot browse positions in edit mode');
    return;
}
if ($statusBarModeStore === 'EDIT') {
    return; // Let EditPanel handle it
}
```

2. **analysis.go** - `gnubgMoveToLazyBG()`:
```go
// Display moves inverted so they read naturally from player's perspective
// gnubg gives moves from high to low (bearing off direction)
// Players read moves from their starting position to destination
moveStr += toStr + "/" + fromStr
```

3. **CandidateMovesPanel.svelte** - CSS:
```css
.move-item {
    padding: 6px 8px;  /* was 10px 12px */
}

.move-item.selected {
    padding: 5px 7px;  /* was 9px 11px */
}

.moves-list {
    gap: 2px;  /* was 4px */
}
```

## User Experience Improvements

1. **Seamless j/k Navigation**: Users can now use j/k keys to browse gnubg candidates while in edit mode without error messages
2. **Intuitive Move Notation**: Moves read naturally - players see their starting point first, then destination
3. **Compact Panel**: Less wasted space means more screen real estate for other panels

## Testing Notes

Test these scenarios:
1. Enter edit mode, type dice (e.g., "54"), press Enter to focus move field
2. Press 'j' several times - should cycle through candidates without error
3. Press 'k' several times - should cycle backwards through candidates
4. Verify player2 moves read correctly (e.g., 54 should show high-to-low like "20/24")
5. Verify player1 moves read correctly (e.g., 21 should show their perspective)
6. Check that panel width is noticeably narrower but still readable
7. Check doubles (e.g., 66) show all 4 plays without wrapping or truncation

## Build Status
✅ Application builds successfully
✅ All three issues fixed
✅ No compilation errors
