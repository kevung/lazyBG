# gnubg Candidate Moves - All Fixes Implementation

## Date
2025-12-15

## Issues Fixed

### 1. Player Perspective Bug (CRITICAL)
**Problem**: White player (player1) moves showing wrong point numbers (e.g., "6/8 6/7" instead of proper notation from 24→1 direction)

**Root Cause**: Misunderstanding of gnubg's coordinate system. gnubg ALWAYS returns moves from the player's home board perspective where:
- gnubg index 0 = player's home point (where they bear off)
- gnubg index 23 = player's furthest point from home
- gnubg index 24 = bar

**Solution**: Fixed conversion in `analysis.go::gnubgMoveToLazyBG()`:
- For Black (player 0): gnubg 0 → lazyBG 1, gnubg 23 → lazyBG 24
  - Formula: `lazyBG = gnubg + 1`
- For White (player 1): gnubg 0 → lazyBG 24, gnubg 23 → lazyBG 1
  - Formula: `lazyBG = 24 - gnubg`

This correctly maps gnubg's player-centric perspective to lazyBG's absolute board notation.

### 2. Asynchronous Analysis Trigger
**Problem**: Analysis only triggered on position changes, not when selecting a move in edit mode

**Solution**: Changed `CandidateMovesPanel.svelte` to subscribe to `selectedMoveStore` instead of `positionStore`
- Analysis now triggers immediately when selecting any decision
- Uses async call to avoid blocking UI
- Removed `unsubscribePosition` as it's no longer needed

### 3. j/k Navigation in Move Input Field
**Problem**: j/k keys not working when focus is in the move input field during edit mode

**Solution**: 
- Added event listener in `EditPanel.svelte` to catch j/k keypresses in move input
- Dispatches custom `candidateNavigate` event to CandidateMovesPanel
- CandidateMovesPanel listens for this event in `onMount()`
- Calls `navigateNext()` or `navigatePrevious()` which update `selectedIndex` and call `applyMoveToInput()`
- `applyMoveToInput()` updates the move input field value directly

### 4. Auto-Select Best Move After Dice Entry
**Problem**: After entering dice, user had to manually select the first gnubg move

**Solution**:
- Exported `getBestMove()` function from CandidateMovesPanel
- In EditPanel, after validating dice input and focusing move field, calls `getBestMove()` and sets `moveInput` to the best candidate
- Uses 100ms timeout to allow gnubg analysis to complete

### 5. Orange Highlighting Removal
**Problem**: Some moves showing orange highlighting unexpectedly (current move indicator)

**Solution**:
- Removed `{normalizeMove(move.move) === normalizeMove(currentPlayerMove) ? 'current' : ''}` class from move items
- Removed `.move-item.current` CSS rules
- All moves now have uniform styling, only selected move is highlighted

### 6. Board Update After Candidate Selection
**Problem**: Board arrows not refreshing after selecting a candidate move

**Solution**: Already working correctly through existing mechanisms:
- `selectMove()` calls `updateMove()` which updates transcription
- Calls `invalidatePositionsCacheFrom()` to clear cached positions
- Calls `selectedMoveStore.set({ ...selMove })` to trigger reactivity
- Position calculation runs automatically
- Board subscribes to positionStore and redraws

## Technical Details

### Files Modified
1. **analysis.go**
   - `gnubgMoveToLazyBG()`: Fixed player perspective conversion formulas

2. **frontend/src/components/CandidateMovesPanel.svelte**
   - Changed from `positionStore.subscribe` to `selectedMoveStore.subscribe`
   - Added `getBestMove()` export function
   - Added event listener for `candidateNavigate` custom events
   - Moved `navigateNext()` and `navigatePrevious()` to top of script
   - Removed duplicate function definitions
   - Removed current move highlighting from template and CSS

3. **frontend/src/components/EditPanel.svelte**
   - Added j/k keypress detection in `handleKeyDown()` when in move input
   - Dispatches custom event to CandidateMovesPanel
   - Auto-selects best move after dice validation using `getBestMove()`

### Key Concepts

#### gnubg Coordinate System
- **Always player-relative**: gnubg doesn't care which physical player (Black/White) is moving
- **Home is 0**: The player's home board starts at index 0
- **Away is 23**: The opponent's home board is at index 23
- **Bar is 24**: Entering from bar

#### lazyBG Coordinate System
- **Absolute board positions**: Point 1 is always bottom-right, Point 24 is always top-right
- **Black**: Home = 1-6, moves 24→1, bears off at 1
- **White**: Home = 19-24, moves 1→24, bears off at 24

#### Conversion Logic
When gnubg analyzes for a player, it gives moves as if that player is playing from home (0) to away (23).
We must convert these to absolute lazyBG positions based on which physical player is moving.

## Testing Verification

Test case from user: match2, game1, move2, player1 (White) with dice 21
- Expected: Moves from White's perspective (e.g., "13/11 6/5*")
- gnubg now returns correct moves with proper point numbers
- Before fix: "6/8 6/7" (wrong direction)
- After fix: Correct notation matching player's perspective

## Known Behaviors

1. **First move auto-selection**: After entering dice and pressing Enter, the best gnubg move is automatically filled in the move input field
2. **j/k navigation**: Works in move input field to cycle through candidate moves, updates input field in real-time
3. **Async analysis**: Analysis happens in background without blocking UI
4. **Immediate trigger**: Analysis runs immediately when selecting a decision, not waiting for position changes
5. **Clean UI**: No orange highlighting, only selected move has visual distinction

## Build Status
✅ Application builds successfully
✅ All gnubg data files embedded
✅ No compilation errors
✅ Frontend compiled without critical errors (only accessibility warnings)
