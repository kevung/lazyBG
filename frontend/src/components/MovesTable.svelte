<script>
    import { onMount } from 'svelte';
    import { get } from 'svelte/store';
    import { 
        transcriptionStore, 
        selectedMoveStore, 
        matchValidationStore,
        moveInconsistenciesStore,
        insertMoveBefore,
        insertMoveAfter,
        deleteMove,
        updateMove,
        invalidatePositionsCacheFrom,
        insertDecisionBefore,
        insertDecisionAfter,
        deleteDecision,
        deleteDecisions
    } from '../stores/transcriptionStore.js';
    import { statusBarModeStore, statusBarTextStore, candidatePreviewMoveStore } from '../stores/uiStore.js';
    import { undoRedoStore } from '../stores/undoRedoStore.js';
    import { clipboardStore } from '../stores/clipboardStore.js';
    import { positionsCacheStore } from '../stores/transcriptionStore.js';
    
    console.log('[MovesTable] Component script loaded');
    
    export let visible = true;
    console.log('[MovesTable] visible prop:', visible);
    
    onMount(() => {
        console.log('[MovesTable] MOUNTED - component is now in DOM');
        return () => console.log('[MovesTable] UNMOUNTING');
    });

    let selectedGameIndex = 0;
    let editingMove = null;
    let editingField = null;
    let tableWrapper;
    let previousGameIndex = 0;
    
    // Inline edit mode state
    let inlineEditDice = '';
    let inlineEditMove = '';
    let diceInputElement;
    let moveInputElement;
    
    // Original state for cancellation
    let originalDice = '';
    let originalMove = '';
    let originalIsIllegal = false;
    let originalIsGala = false;

    // Context menu state
    let showContextMenu = false;
    let contextMenuX = 0;
    let contextMenuY = 0;
    let contextMenuGameIndex = 0;
    let contextMenuMoveIndex = 0;
    let contextMenuPlayer = 1;

    // Multi-select state
    let selectionStart = null; // { gameIndex, moveIndex, player }
    let selectionEnd = null; // { gameIndex, moveIndex, player }
    let isDragging = false; // Track if user is dragging for selection
    
    // Undo/redo state
    let canUndo = false;
    let canRedo = false;
    
    // Force reactive update when selection changes
    $: selectionRange = { start: selectionStart, end: selectionEnd };

    $: games = $transcriptionStore?.games || [];
    $: selectedGameIndex = $selectedMoveStore?.gameIndex ?? 0;
    
    // Subscribe to undo/redo store for menu state
    undoRedoStore.subscribe(() => {
        canUndo = undoRedoStore.canUndo();
        canRedo = undoRedoStore.canRedo();
    });
    $: currentGame = games[selectedGameIndex];
    $: moves = currentGame?.moves || [];
    $: validation = $matchValidationStore;
    $: gameInconsistencies = $moveInconsistenciesStore?.[selectedGameIndex] || {};
    
    // Scroll to top when game changes
    $: if (selectedGameIndex !== previousGameIndex && tableWrapper) {
        tableWrapper.scrollTop = 0;
        previousGameIndex = selectedGameIndex;
    }
    
    // Flatten moves into individual player rows
    $: playerRows = moves.flatMap((move, mIdx) => [
        { gameIndex: selectedGameIndex, moveIndex: mIdx, moveNumber: move.moveNumber, player: 1, moveData: move.player1Move },
        { gameIndex: selectedGameIndex, moveIndex: mIdx, moveNumber: move.moveNumber, player: 2, moveData: move.player2Move }
    ]);

    // Scroll selected move into view when selection changes
    $: if ($selectedMoveStore && tableWrapper && playerRows.length > 0) {
        const { gameIndex, moveIndex, player } = $selectedMoveStore;
        // Find the row index that matches the selected move
        const selectedRowIndex = playerRows.findIndex(r => 
            r.gameIndex === gameIndex && r.moveIndex === moveIndex && r.player === player
        );
        
        if (selectedRowIndex >= 0) {
            // Use setTimeout to ensure DOM is updated
            setTimeout(() => {
                const rows = tableWrapper.querySelectorAll('tbody tr');
                const selectedRow = rows[selectedRowIndex];
                if (selectedRow) {
                    selectedRow.scrollIntoView({ 
                        behavior: 'smooth', 
                        block: 'nearest',
                        inline: 'nearest'
                    });
                }
            }, 0);
        }
    }

    function selectMove(gameIndex, moveIndex, player, isShiftClick = false) {
        console.log(`[MovesTable.selectMove] Called with gameIndex=${gameIndex}, moveIndex=${moveIndex}, player=${player}, isShiftClick=${isShiftClick}`);
        console.log(`[MovesTable.selectMove] Current selectionStart:`, selectionStart, `selectionEnd:`, selectionEnd);
        const game = games[gameIndex];
        if (game) {
            const move = game.moves[moveIndex];
            const moveData = player === 1 ? move?.player1Move : move?.player2Move;
            console.log(`[MovesTable.selectMove] moveData:`, JSON.stringify(moveData));
        }
        
        if (isShiftClick && selectionStart) {
            // Extend selection from start to current position
            selectionEnd = { gameIndex, moveIndex, player };
            console.log(`[MovesTable.selectMove] ✓ Extended selection to:`, selectionEnd);
        } else {
            // Start new selection
            selectionStart = { gameIndex, moveIndex, player };
            selectionEnd = null;
            console.log(`[MovesTable.selectMove] ✓ Started new selection at:`, selectionStart);
        }
        
        selectedMoveStore.set({ gameIndex, moveIndex, player });
    }

    function handleMouseDown(gameIndex, moveIndex, player, event) {
        if (event.button !== 0) return; // Only left mouse button
        if (event.shiftKey) {
            // Shift-click extends selection
            selectMove(gameIndex, moveIndex, player, true);
        } else {
            // Start new selection and begin drag
            selectMove(gameIndex, moveIndex, player, false);
            isDragging = true;
        }
    }

    function handleMouseEnter(gameIndex, moveIndex, player) {
        if (isDragging && selectionStart) {
            // Extend selection while dragging
            selectionEnd = { gameIndex, moveIndex, player };
            selectedMoveStore.set({ gameIndex, moveIndex, player });
        }
    }

    function handleMouseUp() {
        isDragging = false;
    }

    // Helper to check if a decision is currently selected (single or in range)
    function isDecisionSelected(gameIndex, moveIndex, player) {
        const start = selectionRange.start;
        const end = selectionRange.end;
        
        if (!start) return false;
        
        // Check if this is the start decision
        if (start.gameIndex === gameIndex && start.moveIndex === moveIndex && start.player === player) {
            return true;
        }
        
        // If no range, only the start is selected
        if (!end) return false;
        
        // Check if in multi-selection range
        if (gameIndex !== start.gameIndex) return false;
        
        const startRowIdx = playerRows.findIndex(r => 
            r.gameIndex === start.gameIndex && 
            r.moveIndex === start.moveIndex && 
            r.player === start.player
        );
        const endRowIdx = playerRows.findIndex(r => 
            r.gameIndex === end.gameIndex && 
            r.moveIndex === end.moveIndex && 
            r.player === end.player
        );
        const currentRowIdx = playerRows.findIndex(r => 
            r.gameIndex === gameIndex && 
            r.moveIndex === moveIndex && 
            r.player === player
        );
        
        if (startRowIdx === -1 || endRowIdx === -1 || currentRowIdx === -1) return false;
        
        const minIdx = Math.min(startRowIdx, endRowIdx);
        const maxIdx = Math.max(startRowIdx, endRowIdx);
        
        return currentRowIdx >= minIdx && currentRowIdx <= maxIdx;
    }

    // Helper to check if a decision is in the selected range (for visual highlighting)
    function isInSelectedRange(gameIndex, moveIndex, player) {
        // Use selectionRange to ensure reactivity
        const start = selectionRange.start;
        const end = selectionRange.end;
        
        if (!start) return false;
        
        // Single selection (no range) - don't highlight as range
        if (!end) {
            return false;
        }
        
        // Multi-selection range (only within same game)
        if (gameIndex !== start.gameIndex) return false;
        
        // Find row indices for start and end
        const startRowIdx = playerRows.findIndex(r => 
            r.gameIndex === start.gameIndex && 
            r.moveIndex === start.moveIndex && 
            r.player === start.player
        );
        const endRowIdx = playerRows.findIndex(r => 
            r.gameIndex === end.gameIndex && 
            r.moveIndex === end.moveIndex && 
            r.player === end.player
        );
        const currentRowIdx = playerRows.findIndex(r => 
            r.gameIndex === gameIndex && 
            r.moveIndex === moveIndex && 
            r.player === player
        );
        
        if (startRowIdx === -1 || endRowIdx === -1 || currentRowIdx === -1) return false;
        
        const minIdx = Math.min(startRowIdx, endRowIdx);
        const maxIdx = Math.max(startRowIdx, endRowIdx);
        
        const inRange = currentRowIdx >= minIdx && currentRowIdx <= maxIdx;
        
        // Debug logging for visual highlighting
        if (inRange) {
            console.log(`[isInSelectedRange] Decision at ${gameIndex},${moveIndex},${player} IS in range [${minIdx}-${maxIdx}], currentIdx=${currentRowIdx}`);
        }
        
        return inRange;
    }

    // Check if multi-selection is active (more than one decision selected)
    function isMultiSelectionActive() {
        return selectionStart !== null && selectionEnd !== null;
    }

    // Get array of all decisions in the selected range
    function getSelectedDecisions() {
        console.log(`[getSelectedDecisions] selectionStart:`, selectionStart, `selectionEnd:`, selectionEnd);
        
        // If no selection state, fall back to selectedMoveStore (for keyboard navigation)
        if (!selectionStart) {
            if ($selectedMoveStore) {
                const result = [{ 
                    gameIndex: $selectedMoveStore.gameIndex, 
                    moveIndex: $selectedMoveStore.moveIndex, 
                    player: $selectedMoveStore.player 
                }];
                console.log(`[getSelectedDecisions] Using selectedMoveStore:`, result);
                return result;
            }
            return [];
        }
        
        if (!selectionEnd) {
            // Single selection
            const result = [{ gameIndex: selectionStart.gameIndex, moveIndex: selectionStart.moveIndex, player: selectionStart.player }];
            console.log(`[getSelectedDecisions] Single selection:`, result);
            return result;
        }
        
        // Multi-selection - get all decisions in range
        const startRowIdx = playerRows.findIndex(r => 
            r.gameIndex === selectionStart.gameIndex && 
            r.moveIndex === selectionStart.moveIndex && 
            r.player === selectionStart.player
        );
        const endRowIdx = playerRows.findIndex(r => 
            r.gameIndex === selectionEnd.gameIndex && 
            r.moveIndex === selectionEnd.moveIndex && 
            r.player === selectionEnd.player
        );
        
        if (startRowIdx === -1 || endRowIdx === -1) return [];
        
        const minIdx = Math.min(startRowIdx, endRowIdx);
        const maxIdx = Math.max(startRowIdx, endRowIdx);
        
        const decisions = [];
        for (let i = minIdx; i <= maxIdx; i++) {
            const row = playerRows[i];
            decisions.push({ gameIndex: row.gameIndex, moveIndex: row.moveIndex, player: row.player });
        }
        
        console.log(`[getSelectedDecisions] Multi-selection: ${decisions.length} decisions`);
        return decisions;
    }

    // Clear multi-selection
    export function clearSelection() {
        selectionStart = null;
        selectionEnd = null;
    }

    // Extend selection upward (shift+k)
    export function extendSelectionUp() {
        console.log('[MovesTable.extendSelectionUp] Called');
        if (!$selectedMoveStore) return;
        
        // Find current selection in playerRows
        const currentIdx = playerRows.findIndex(r => 
            r.gameIndex === $selectedMoveStore.gameIndex && 
            r.moveIndex === $selectedMoveStore.moveIndex && 
            r.player === $selectedMoveStore.player
        );
        
        if (currentIdx === -1 || currentIdx === 0) return;
        
        // If no range selection yet, start one
        if (!selectionStart) {
            selectionStart = { ...$selectedMoveStore };
        }
        
        // Move to previous row
        const prevRow = playerRows[currentIdx - 1];
        selectionEnd = { gameIndex: prevRow.gameIndex, moveIndex: prevRow.moveIndex, player: prevRow.player };
        selectedMoveStore.set(selectionEnd);
        
        console.log('[MovesTable.extendSelectionUp] Extended to:', selectionEnd);
    }

    // Extend selection downward (shift+j)
    export function extendSelectionDown() {
        console.log('[MovesTable.extendSelectionDown] Called');
        if (!$selectedMoveStore) return;
        
        // Find current selection in playerRows
        const currentIdx = playerRows.findIndex(r => 
            r.gameIndex === $selectedMoveStore.gameIndex && 
            r.moveIndex === $selectedMoveStore.moveIndex && 
            r.player === $selectedMoveStore.player
        );
        
        if (currentIdx === -1 || currentIdx >= playerRows.length - 1) return;
        
        // If no range selection yet, start one
        if (!selectionStart) {
            selectionStart = { ...$selectedMoveStore };
        }
        
        // Move to next row
        const nextRow = playerRows[currentIdx + 1];
        selectionEnd = { gameIndex: nextRow.gameIndex, moveIndex: nextRow.moveIndex, player: nextRow.player };
        selectedMoveStore.set(selectionEnd);
        
        console.log('[MovesTable.extendSelectionDown] Extended to:', selectionEnd);
    }

    function handleContextMenu(event, gameIndex, moveIndex, player) {
        // Only show context menu in NORMAL mode (not in EDIT or INSERT modes)
        if ($statusBarModeStore === 'NORMAL') {
            event.preventDefault();
            
            // Estimated context menu dimensions (approximate)
            const menuWidth = 180;
            const menuHeight = 280; // Approximate height with all menu items
            
            // Calculate position ensuring menu stays within viewport
            let x = event.clientX;
            let y = event.clientY;
            
            // Check right boundary
            if (x + menuWidth > window.innerWidth) {
                x = window.innerWidth - menuWidth - 5; // 5px margin
            }
            
            // Check bottom boundary
            if (y + menuHeight > window.innerHeight) {
                y = window.innerHeight - menuHeight - 5; // 5px margin
            }
            
            // Ensure not off left edge
            if (x < 0) {
                x = 5;
            }
            
            // Ensure not off top edge
            if (y < 0) {
                y = 5;
            }
            
            contextMenuX = x;
            contextMenuY = y;
            contextMenuGameIndex = gameIndex;
            contextMenuMoveIndex = moveIndex;
            contextMenuPlayer = player;
            showContextMenu = true;
            
            // Only select the move if not already selected (single or in range)
            if (!isDecisionSelected(gameIndex, moveIndex, player)) {
                selectMove(gameIndex, moveIndex, player);
            }
        }
    }

    function closeContextMenu() {
        showContextMenu = false;
    }
    
    // Helper function to save state before an operation
    function saveSnapshotBeforeOperation() {
        console.log('[MovesTable.svelte] saveSnapshotBeforeOperation called');
        // Save current state BEFORE operation
        undoRedoStore.saveSnapshot();
    }

    async function handleInsertBefore() {
        // Save state before the operation
        saveSnapshotBeforeOperation();
        
        insertDecisionBefore(contextMenuGameIndex, contextMenuMoveIndex, contextMenuPlayer);
        
        // Invalidate position cache from this move onwards
        await invalidatePositionsCacheFrom(contextMenuGameIndex, contextMenuMoveIndex);
        
        statusBarTextStore.set(`Decision inserted before at game ${contextMenuGameIndex + 1}, move ${contextMenuMoveIndex}`);
        
        // Select the newly inserted decision
        selectedMoveStore.set({ 
            gameIndex: contextMenuGameIndex, 
            moveIndex: contextMenuMoveIndex, 
            player: contextMenuPlayer 
        });
        
        closeContextMenu();
    }

    async function handleInsertAfter() {
        // Check if current decision is a drop or resign - can't insert after game-ending moves
        const game = games[contextMenuGameIndex];
        if (game && game.moves && game.moves[contextMenuMoveIndex]) {
            const move = game.moves[contextMenuMoveIndex];
            const currentPlayerMove = contextMenuPlayer === 1 ? move.player1Move : move.player2Move;
            
            if (currentPlayerMove && (currentPlayerMove.cubeAction === 'drops' || currentPlayerMove.resignAction)) {
                statusBarTextStore.set('Cannot insert decision after game has ended');
                closeContextMenu();
                return;
            }
            
            // If inserting after player 1, also check if player 2 has already dropped/resigned in same move
            if (contextMenuPlayer === 1 && move.player2Move && (move.player2Move.cubeAction === 'drops' || move.player2Move.resignAction)) {
                statusBarTextStore.set('Cannot insert decision after game has ended');
                closeContextMenu();
                return;
            }
        }
        
        // Save state before the operation
        saveSnapshotBeforeOperation();
        
        insertDecisionAfter(contextMenuGameIndex, contextMenuMoveIndex, contextMenuPlayer);
        
        // Calculate the new move index after insertion
        const newMoveIndex = contextMenuPlayer === 2 ? contextMenuMoveIndex + 1 : contextMenuMoveIndex;
        const newPlayer = contextMenuPlayer === 2 ? 1 : 2;
        
        // Invalidate position cache from the new move onwards
        await invalidatePositionsCacheFrom(contextMenuGameIndex, newMoveIndex);
        
        statusBarTextStore.set(`Decision inserted after at game ${contextMenuGameIndex + 1}, move ${newMoveIndex}`);
        
        // Select the newly inserted decision
        selectedMoveStore.set({ 
            gameIndex: contextMenuGameIndex, 
            moveIndex: newMoveIndex, 
            player: newPlayer 
        });
        
        closeContextMenu();
    }

    function handleCopyDecision() {
        copySelectedDecisions();
        closeContextMenu();
    }

    async function handleCutDecision() {
        await cutSelectedDecisions();
        closeContextMenu();
    }

    async function handlePasteBeforeDecision() {
        // Check if clipboard has content
        const clipboard = get(clipboardStore);
        if (!clipboard.decisions || clipboard.decisions.length === 0) {
            statusBarTextStore.set('Clipboard is empty');
            closeContextMenu();
            return;
        }

        closeContextMenu();
        
        // Paste directly before the context menu decision
        await pasteDecisionsAt(contextMenuGameIndex, contextMenuMoveIndex, contextMenuPlayer, 'before');
    }

    async function handlePasteAfterDecision() {
        // Check if clipboard has content
        const clipboard = get(clipboardStore);
        if (!clipboard.decisions || clipboard.decisions.length === 0) {
            statusBarTextStore.set('Clipboard is empty');
            closeContextMenu();
            return;
        }

        closeContextMenu();
        
        // Paste directly after the context menu decision
        await pasteDecisionsAt(contextMenuGameIndex, contextMenuMoveIndex, contextMenuPlayer, 'after');
    }

    export async function pasteDecisionsAt(gameIndex, moveIndex, player, position) {
        const transcription = get(transcriptionStore);
        const clipboard = get(clipboardStore);
        
        if (!transcription || !clipboard.decisions || clipboard.decisions.length === 0) {
            return;
        }

        const game = transcription.games[gameIndex];
        if (!game) {
            return;
        }

        // Save state before pasting
        saveSnapshotBeforeOperation();

        // Determine where to start inserting
        let insertAtMoveIndex = moveIndex;
        let insertAtPlayer = player;
        
        if (position === 'after') {
            if (player === 1) {
                // After player1 -> insert at player2 of same move
                insertAtPlayer = 2;
            } else {
                // After player2 -> insert at player1 of next move
                insertAtMoveIndex = moveIndex + 1;
                insertAtPlayer = 1;
            }
        }
        // For 'before', we use current position

        // Insert all decisions from clipboard
        let currentMoveIndex = insertAtMoveIndex;
        let currentPlayer = insertAtPlayer;
        
        for (let i = 0; i < clipboard.decisions.length; i++) {
            const decisionData = clipboard.decisions[i];
            
            // Use the stored move data from clipboard (not from transcription store)
            // This is important because for cut operations, the source may already be deleted
            const sourceMoveData = decisionData.moveData;
            
            if (position === 'before') {
                insertDecisionBefore(gameIndex, currentMoveIndex, currentPlayer);
            } else {
                // For 'after', we insert at the calculated position
                // First insertion needs special handling
                if (i === 0 && insertAtPlayer === player && insertAtMoveIndex === moveIndex) {
                    insertDecisionAfter(gameIndex, moveIndex, player);
                } else {
                    insertDecisionBefore(gameIndex, currentMoveIndex, currentPlayer);
                }
            }
            
            // Update the newly inserted decision with data from clipboard
            const updatedGame = get(transcriptionStore).games[gameIndex];
            const targetMove = updatedGame.moves[currentMoveIndex];
            
            if (targetMove) {
                if (currentPlayer === 1) {
                    targetMove.player1Move = sourceMoveData ? JSON.parse(JSON.stringify(sourceMoveData)) : null;
                } else {
                    targetMove.player2Move = sourceMoveData ? JSON.parse(JSON.stringify(sourceMoveData)) : null;
                }
            }
            
            // Move to next position
            if (currentPlayer === 1) {
                currentPlayer = 2;
            } else {
                currentMoveIndex++;
                currentPlayer = 1;
            }
        }

        // Update transcription store
        transcriptionStore.set(get(transcriptionStore));

        // If it was a cut operation, convert it to a copy operation (keep in clipboard for multiple pastes)
        // But clear the isCut flag so subsequent pastes don't delete the content
        if (clipboard.isCut) {
            clipboardStore.copy(clipboard.decisions);
        }

        // Invalidate cache and force recalculation
        await invalidatePositionsCacheFrom(gameIndex, insertAtMoveIndex);
        
        // Select the first pasted decision
        selectedMoveStore.set({ gameIndex, moveIndex: insertAtMoveIndex, player: insertAtPlayer });
        
        const posText = position === 'before' ? 'before' : 'after';
        const count = clipboard.decisions.length;
        statusBarTextStore.set(`${count} decision(s) pasted ${posText} at game ${gameIndex + 1}, move ${insertAtMoveIndex + 1}`);
    }

    // Export this function so App.svelte can call it for dd/Del key
    export async function deleteSelectedDecisions() {
        await handleDeleteDecision();
    }

    // Export copy function
    export function copySelectedDecisions() {
        const decisions = getSelectedDecisions();
        console.log('[copySelectedDecisions] Copying decisions:', decisions);
        
        if (decisions.length === 0) {
            statusBarTextStore.set('No decisions selected');
            return;
        }
        
        // Enhance decisions with actual move data
        const transcription = get(transcriptionStore);
        const decisionsWithData = decisions.map(d => {
            const game = transcription.games[d.gameIndex];
            if (!game) return d;
            
            const move = game.moves[d.moveIndex];
            if (!move) return d;
            
            const moveData = d.player === 1 ? move.player1Move : move.player2Move;
            return { ...d, moveData: moveData ? JSON.parse(JSON.stringify(moveData)) : null };
        });
        
        clipboardStore.copy(decisionsWithData);
        statusBarTextStore.set(`${decisions.length} decision(s) copied to clipboard`);
    }

    // Export cut function
    export async function cutSelectedDecisions() {
        const decisions = getSelectedDecisions();
        console.log('[cutSelectedDecisions] Cutting decisions:', decisions);
        
        if (decisions.length === 0) {
            statusBarTextStore.set('No decisions selected');
            return;
        }
        
        // Enhance decisions with actual move data BEFORE deleting
        const transcription = get(transcriptionStore);
        const decisionsWithData = decisions.map(d => {
            const game = transcription.games[d.gameIndex];
            if (!game) return d;
            
            const move = game.moves[d.moveIndex];
            if (!move) return d;
            
            const moveData = d.player === 1 ? move.player1Move : move.player2Move;
            return { ...d, moveData: moveData ? JSON.parse(JSON.stringify(moveData)) : null };
        });
        
        // Copy to clipboard with cut flag
        clipboardStore.cut(decisionsWithData);
        
        // Delete the decisions
        await handleDeleteDecision();
        
        statusBarTextStore.set(`${decisions.length} decision(s) cut to clipboard`);
    }
    
    async function handleUndo() {
        const success = undoRedoStore.undo();
        if (success) {
            // Clear the entire position cache
            positionsCacheStore.set({});
            
            // Revalidate all games to update inconsistencies
            const transcription = $transcriptionStore;
            if (transcription && transcription.games) {
                const { validateGameInconsistencies } = await import('../stores/transcriptionStore.js');
                for (let gameIndex = 0; gameIndex < transcription.games.length; gameIndex++) {
                    await validateGameInconsistencies(gameIndex, 0);
                }
            }
            
            statusBarTextStore.set('Undo completed');
            
            // Force board update
            const currentMove = $selectedMoveStore;
            selectedMoveStore.set({ ...currentMove });
        } else {
            statusBarTextStore.set('Nothing to undo');
        }
        closeContextMenu();
    }
    
    async function handleRedo() {
        const success = undoRedoStore.redo();
        if (success) {
            // Clear the entire position cache
            positionsCacheStore.set({});
            
            // Revalidate all games to update inconsistencies
            const transcription = $transcriptionStore;
            if (transcription && transcription.games) {
                const { validateGameInconsistencies } = await import('../stores/transcriptionStore.js');
                for (let gameIndex = 0; gameIndex < transcription.games.length; gameIndex++) {
                    await validateGameInconsistencies(gameIndex, 0);
                }
            }
            
            statusBarTextStore.set('Redo completed');
            
            // Force board update
            const currentMove = $selectedMoveStore;
            selectedMoveStore.set({ ...currentMove });
        } else {
            statusBarTextStore.set('Nothing to redo');
        }
        closeContextMenu();
    }

    async function handleDeleteDecision() {
        const decisions = getSelectedDecisions();
        console.log('[handleDeleteDecision] Deleting decisions:', decisions);
        
        if (decisions.length === 0) return;
        
        // Enhance decisions with actual move data BEFORE deleting (for clipboard)
        const transcription = get(transcriptionStore);
        const decisionsWithData = decisions.map(d => {
            const game = transcription.games[d.gameIndex];
            if (!game) return d;
            
            const move = game.moves[d.moveIndex];
            if (!move) return d;
            
            const moveData = d.player === 1 ? move.player1Move : move.player2Move;
            return { ...d, moveData: moveData ? JSON.parse(JSON.stringify(moveData)) : null };
        });
        
        // Save deleted decisions to clipboard (for potential paste/undo)
        clipboardStore.cut(decisionsWithData);
        
        // Save state before the operation
        saveSnapshotBeforeOperation();
        
        if (decisions.length === 1) {
            const { gameIndex, moveIndex, player } = decisions[0];
            deleteDecision(gameIndex, moveIndex, player);
            await invalidatePositionsCacheFrom(gameIndex, moveIndex);
            statusBarTextStore.set(`Decision deleted at game ${gameIndex + 1}, move ${moveIndex + 1}, player ${player}`);
        } else {
            deleteDecisions(decisions);
            statusBarTextStore.set(`${decisions.length} decisions deleted`);
        }
        
        // Clear selection range completely
        selectionStart = null;
        selectionEnd = null;
        
        // Force board update
        selectedMoveStore.update(current => ({ ...current }));
        
        closeContextMenu();
    }

    function startEdit(gameIndex, moveIndex, field, player) {
        editingMove = { gameIndex, moveIndex };
        editingField = field;
        selectMove(gameIndex, moveIndex, player);
    }

    function finishEdit(event, gameIndex, moveIndex, player) {
        // Only update if this cell was actually being edited
        if (!editingMove || 
            editingMove.gameIndex !== gameIndex || 
            editingMove.moveIndex !== moveIndex || 
            editingMove.player !== player) {
            return;
        }
        
        const newValue = event.target.textContent.trim();
        const move = games[gameIndex].moves[moveIndex];
        
        // Parse the new value (format: "dice: move" or just "move")
        let dice = '';
        let moveStr = newValue;
        
        const parts = newValue.split(':');
        if (parts.length === 2) {
            dice = parts[0].trim();
            moveStr = parts[1].trim();
        }
        
        // Get existing move data
        const existingMove = player === 1 ? move.player1Move : move.player2Move;
        const isIllegal = existingMove?.isIllegal || false;
        const isGala = existingMove?.isGala || false;
        
        // For cube decisions (d/t/p) and resign decisions (r/g/b), pass empty string as move
        const diceStr = dice.toLowerCase();
        const isCubeDecision = diceStr === 'd' || diceStr === 't' || diceStr === 'p';
        const isResignDecision = diceStr === 'r' || diceStr === 'g' || diceStr === 'b';
        const moveToSave = (isCubeDecision || isResignDecision) ? '' : moveStr;
        
        // Save state before the operation
        saveSnapshotBeforeOperation();
        
        updateMove(gameIndex, move.moveNumber, player, dice, moveToSave, isIllegal, isGala);
        
        editingMove = null;
        editingField = null;
    }

    function handleKeyDown(event) {
        if (!$selectedMoveStore) return;
        
        // If in EDIT mode with inline editing, handle edit-specific keys
        if ($statusBarModeStore === 'EDIT' && editingMove) {
            handleInlineEditKeyDown(event);
            return;
        }
        
        const { gameIndex, moveIndex, player } = $selectedMoveStore;
        const currentMoves = games[gameIndex]?.moves || [];
        const currentRowIndex = playerRows.findIndex(r => 
            r.gameIndex === gameIndex && r.moveIndex === moveIndex && r.player === player
        );
        
        switch(event.key) {
            case 'j':
            case 'ArrowDown':
                event.preventDefault();
                if (currentRowIndex < playerRows.length - 1) {
                    const nextRow = playerRows[currentRowIndex + 1];
                    selectMove(nextRow.gameIndex, nextRow.moveIndex, nextRow.player);
                }
                break;
                
            case 'k':
            case 'ArrowUp':
                event.preventDefault();
                if (currentRowIndex > 0) {
                    const prevRow = playerRows[currentRowIndex - 1];
                    selectMove(prevRow.gameIndex, prevRow.moveIndex, prevRow.player);
                }
                break;
                
            case 'a':
                event.preventDefault();
                insertMoveBefore(gameIndex, currentMoves[moveIndex].moveNumber);
                break;
                
            case 'b':
                event.preventDefault();
                // For 'b' (insert full move after), check if game has ended in current move
                const currentMove = currentMoves[moveIndex];
                const gameEnded = (currentMove.player1Move && (currentMove.player1Move.cubeAction === 'drops' || currentMove.player1Move.resignAction)) ||
                                 (currentMove.player2Move && (currentMove.player2Move.cubeAction === 'drops' || currentMove.player2Move.resignAction));
                if (!gameEnded) {
                    insertMoveAfter(gameIndex, currentMoves[moveIndex].moveNumber);
                }
                break;
                
            case 'i':
                event.preventDefault();
                const moveEntry = currentMoves[moveIndex];
                const playerMove = player === 1 ? moveEntry.player1Move : moveEntry.player2Move;
                if (playerMove) {
                    saveSnapshotBeforeOperation();
                    updateMove(
                        gameIndex, 
                        moveEntry.moveNumber, 
                        player,
                        playerMove.dice,
                        playerMove.move,
                        !playerMove.isIllegal,
                        playerMove.isGala
                    );
                }
                break;
                
            case 'g':
                event.preventDefault();
                const moveEntryG = currentMoves[moveIndex];
                const playerMoveG = player === 1 ? moveEntryG.player1Move : moveEntryG.player2Move;
                if (playerMoveG) {
                    saveSnapshotBeforeOperation();
                    updateMove(
                        gameIndex, 
                        moveEntryG.moveNumber, 
                        player,
                        playerMoveG.dice,
                        playerMoveG.move,
                        playerMoveG.isIllegal,
                        !playerMoveG.isGala
                    );
                }
                break;
        }
    }
    
    function handleInlineEditKeyDown(event) {
        if (event.key === 'Tab') {
            // Allow Tab to propagate to App.svelte to toggle edit mode
            // Don't prevent default so it can exit edit mode
            event.stopPropagation();
            cancelInlineEdit();
            return;
        } else if (event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
            cancelInlineEdit();
        } else if (event.key === 'Enter') {
            event.preventDefault();
            event.stopPropagation();
            
            // If dice input is focused
            if (document.activeElement === diceInputElement) {
                const dice = inlineEditDice.toLowerCase();
                // Check for cube decisions and resign decisions
                if (dice === 'd' || dice === 't' || dice === 'p' || dice === 'r' || dice === 'g' || dice === 'b') {
                    // Validate immediately for cube and resign decisions
                    validateInlineEdit();
                } else if (validateDiceInput(inlineEditDice)) {
                    // Valid dice - validate immediately (skip move input)
                    validateInlineEdit();
                } else {
                    statusBarTextStore.set('Invalid dice: enter d/t/p/r/g/b or 2 digits between 1 and 6');
                }
            } else if (document.activeElement === moveInputElement) {
                // Validate and save
                const dice = inlineEditDice.toLowerCase();
                if (dice === 'd' || dice === 't' || dice === 'p' || dice === 'r' || dice === 'g' || dice === 'b' || validateDiceInput(inlineEditDice)) {
                    validateInlineEdit();
                } else {
                    statusBarTextStore.set('Invalid dice: enter d/t/p/r/g/b or 2 digits between 1 and 6');
                }
            }
        } else if (event.key === 'Backspace') {
            // If in move input and it's empty, focus on dice input
            if (document.activeElement === moveInputElement && inlineEditMove === '') {
                event.preventDefault();
                diceInputElement?.focus();
                // Select all text in dice input for easy editing
                if (diceInputElement) {
                    diceInputElement.select();
                }
            }
        }
    }
    
    function startInlineEdit(gameIndex, moveIndex, player) {
        console.log('[MovesTable] startInlineEdit called:', { gameIndex, moveIndex, player });
        const game = games[gameIndex];
        if (!game || !game.moves[moveIndex]) {
            console.log('[MovesTable] No game or move found');
            return;
        }
        
        const move = game.moves[moveIndex];
        const playerMove = player === 1 ? move.player1Move : move.player2Move;
        
        console.log('[MovesTable] Setting editingMove:', { gameIndex, moveIndex, player });
        editingMove = { gameIndex, moveIndex, player };
        
        // Store original state for cancellation
        originalDice = playerMove?.dice || '';
        originalMove = playerMove?.move || '';
        originalIsIllegal = playerMove?.isIllegal || false;
        originalIsGala = playerMove?.isGala || false;
        
        // Check for cube action in player move
        if (playerMove?.cubeAction) {
            if (playerMove.cubeAction === 'doubles') {
                inlineEditDice = 'd';
            } else if (playerMove.cubeAction === 'takes') {
                inlineEditDice = 't';
            } else if (playerMove.cubeAction === 'drops') {
                inlineEditDice = 'p';
            }
            inlineEditMove = '';
        } else if (playerMove?.resignAction) {
            // Check for resign action in player move
            if (playerMove.resignAction === 'normal') {
                inlineEditDice = 'r';
            } else if (playerMove.resignAction === 'gammon') {
                inlineEditDice = 'g';
            } else if (playerMove.resignAction === 'backgammon') {
                inlineEditDice = 'b';
            }
            inlineEditMove = '';
        } else if (playerMove) {
            inlineEditDice = playerMove.dice || '';
            inlineEditMove = playerMove.move || '';
        } else {
            inlineEditDice = '';
            inlineEditMove = '';
        }
        
        // Focus on dice input after render
        setTimeout(() => {
            if (diceInputElement) {
                diceInputElement.focus();
                diceInputElement.select();
            }
        }, 50);
        
        statusBarTextStore.set('EDIT MODE: Enter dice (d=double, t=take, p=pass, r=resign, g=resign gammon, b=resign backgammon) or 2 digits 1-6, then move. Enter=validate, Tab=exit edit mode');
    }
    
    function cancelInlineEdit() {
        // Restore original state if it was modified
        if (editingMove) {
            const { gameIndex, moveIndex, player } = editingMove;
            const transcription = get(transcriptionStore);
            const game = transcription?.games[gameIndex];
            const move = game?.moves[moveIndex];
            const playerMove = player === 1 ? move?.player1Move : move?.player2Move;
            
            // Check if dice was modified
            const currentDice = playerMove?.dice || '';
            if (currentDice !== originalDice) {
                console.log('[MovesTable] Restoring original dice:', originalDice);
                // Restore original state
                updateMove(gameIndex, move.moveNumber, player, originalDice, originalMove, originalIsIllegal, originalIsGala);
                
                // Invalidate cache and refresh position
                invalidatePositionsCacheFrom(gameIndex, moveIndex);
                const currentSelection = get(selectedMoveStore);
                selectedMoveStore.set({...currentSelection});
            }
        }
        
        editingMove = null;
        inlineEditDice = '';
        inlineEditMove = '';
        statusBarModeStore.set('NORMAL');
        
        // Clear candidate preview when exiting edit mode
        candidatePreviewMoveStore.set(null);
    }
    
    async function validateInlineEdit() {
        if (!editingMove) return;
        
        const { gameIndex, moveIndex, player } = editingMove;
        const game = games[gameIndex];
        if (!game || !game.moves[moveIndex]) return;
        
        const move = game.moves[moveIndex];
        const playerMove = player === 1 ? move.player1Move : move.player2Move;
        const isIllegal = playerMove?.isIllegal || false;
        const isGala = playerMove?.isGala || false;
        
        // For cube decisions (d/t/p) and resign decisions (r/g/b), pass empty string as move
        const diceStr = inlineEditDice.toLowerCase();
        const isCubeDecision = diceStr === 'd' || diceStr === 't' || diceStr === 'p';
        const isResignDecision = diceStr === 'r' || diceStr === 'g' || diceStr === 'b';
        const moveToSave = (isCubeDecision || isResignDecision) ? '' : inlineEditMove;
        
        // Save state before the operation
        saveSnapshotBeforeOperation();
        
        updateMove(gameIndex, move.moveNumber, player, inlineEditDice, moveToSave, isIllegal, isGala);
        
        // Invalidate position cache from this move onwards
        await invalidatePositionsCacheFrom(gameIndex, moveIndex);
        
        // Force recalculation
        selectedMoveStore.set({ gameIndex, moveIndex, player });
        
        statusBarTextStore.set(`Move updated at game ${gameIndex + 1}, move ${move.moveNumber}. Tab=exit edit mode`);
        
        // Clear candidate preview
        candidatePreviewMoveStore.set(null);
        
        // Clear current editing state before moving to next decision
        editingMove = null;
        inlineEditDice = '';
        inlineEditMove = '';
        
        // Move to next decision and stay in EDIT mode
        await moveToNextDecision(gameIndex, moveIndex, player);
    }
    
    async function moveToNextDecision(currentGameIndex, currentMoveIndex, currentPlayer) {
        const transcription = get(transcriptionStore);
        if (!transcription || !transcription.games) return;
        
        const currentGame = transcription.games[currentGameIndex];
        if (!currentGame) return;
        
        // Try to move to next decision
        if (currentPlayer === 1) {
            // Move to player 2 of same move
            selectedMoveStore.set({ 
                gameIndex: currentGameIndex, 
                moveIndex: currentMoveIndex, 
                player: 2 
            });
        } else if (currentMoveIndex < currentGame.moves.length - 1) {
            // Move to player 1 of next move in same game
            selectedMoveStore.set({ 
                gameIndex: currentGameIndex, 
                moveIndex: currentMoveIndex + 1, 
                player: 1 
            });
        } else if (currentGameIndex < transcription.games.length - 1) {
            // Move to first move of next game
            selectedMoveStore.set({ 
                gameIndex: currentGameIndex + 1, 
                moveIndex: 0, 
                player: 1 
            });
        } else {
            // We're at the last decision - insert a new empty decision after it
            saveSnapshotBeforeOperation();
            insertDecisionAfter(currentGameIndex, currentMoveIndex, currentPlayer);
            
            // Calculate the new move index for the inserted decision
            const newMoveIndex = currentMoveIndex + 1;
            
            // Invalidate cache and force recalculation
            await invalidatePositionsCacheFrom(currentGameIndex, newMoveIndex);
            
            // Select the newly inserted decision
            selectedMoveStore.set({ 
                gameIndex: currentGameIndex, 
                moveIndex: newMoveIndex, 
                player: 1 
            });
            
            statusBarTextStore.set('Last decision reached - new empty decision inserted. Tab=exit edit mode');
            return;
        }
        
        // The reactive statement will automatically start editing the new selection
    }
    
    function handleDiceInput(event) {
        console.log('[MovesTable] handleDiceInput called, value:', event.target.value);
        let value = event.target.value.toLowerCase();
        
        // Check for cube decisions and resign decisions first
        if (value === 'd' || value === 't' || value === 'p' || value === 'r' || value === 'g' || value === 'b') {
            console.log('[MovesTable] Special action detected:', value);
            inlineEditDice = value;
            // Clear move input for cube and resign decisions
            inlineEditMove = '';
            return;
        }
        
        // Only allow digits
        value = value.replace(/[^1-6]/g, '');
        console.log('[MovesTable] Filtered dice value:', value, 'length:', value.length);
        
        // Limit to 2 characters
        if (value.length > 2) {
            value = value.slice(0, 2);
        }
        
        inlineEditDice = value;
        
        // Immediately update transcription when 2 valid dice digits are entered
        if (value.length === 2 && validateDiceInput(value)) {
            console.log('[MovesTable] ═══════════════════════════════════════════════════════');
            console.log('[MovesTable] ⚡ 2 VALID DICE ENTERED:', value);
            console.log('[MovesTable] ═══════════════════════════════════════════════════════');
            updateDiceOnly(value);
            
            // Auto-advance to move input
            setTimeout(() => {
                console.log('[MovesTable] First timeout - moveInputElement:', !!moveInputElement);
                if (moveInputElement) {
                    // Only auto-fill if move is currently empty
                    if (!inlineEditMove || inlineEditMove.trim() === '') {
                        // Auto-fill with best candidate move (wait for analysis)
                        setTimeout(() => {
                            console.log('[MovesTable] Second timeout - getting best move from candidate panel');
                            // Get the best move by looking for the first move item in the candidate panel
                            const firstMoveItem = document.querySelector('.candidate-moves-panel .move-item');
                            console.log('[MovesTable] First move item element:', !!firstMoveItem);
                            
                            if (firstMoveItem) {
                                // Extract the move text from the first candidate
                                const moveText = firstMoveItem.querySelector('.move-notation')?.textContent?.trim();
                                console.log('[MovesTable] Best move from DOM:', moveText);
                                
                                if (moveText) {
                                    console.log('[MovesTable] Setting inlineEditMove to:', moveText);
                                    inlineEditMove = moveText;
                                }
                            }
                        }, 100);
                    } else {
                        console.log('[MovesTable] Move already exists, not auto-filling:', inlineEditMove);
                    }
                    
                    // Always focus and select the move input
                    setTimeout(() => {
                        if (moveInputElement) {
                            console.log('[MovesTable] Focusing and selecting move input');
                            moveInputElement.focus();
                            moveInputElement.setSelectionRange(0, moveInputElement.value.length);
                        }
                    }, 150);
                }
            }, 50);
        }
    }

    // Update dice immediately in transcription (without move) to trigger candidate moves analysis
    function updateDiceOnly(newDice) {
        console.log('[MovesTable] updateDiceOnly called with:', newDice);
        if (!editingMove) {
            console.log('[MovesTable] Not editing, skipping update');
            return;
        }

        const { gameIndex, moveIndex, player } = editingMove;
        const transcription = get(transcriptionStore);
        
        if (!transcription || !transcription.games[gameIndex]) {
            console.log('[MovesTable] No transcription or game');
            return;
        }

        const game = transcription.games[gameIndex];
        const move = game.moves[moveIndex];
        
        if (!move) {
            console.log('[MovesTable] No move at index');
            return;
        }

        const playerMove = player === 1 ? move.player1Move : move.player2Move;
        
        // Keep existing move and flags, just update dice
        const existingMove = playerMove?.move || '';
        const isIllegal = playerMove?.isIllegal || false;
        const isGala = playerMove?.isGala || false;
        
        console.log('[MovesTable] Calling updateMove with dice:', newDice, 'move:', existingMove);
        updateMove(gameIndex, move.moveNumber, player, newDice, existingMove, isIllegal, isGala);
        
        // Invalidate position cache so CandidateMovesPanel recalculates with new dice
        invalidatePositionsCacheFrom(gameIndex, moveIndex);
        console.log('[MovesTable] Position cache invalidated from moveIndex:', moveIndex);
        
        // Force selectedMoveStore to re-trigger so App.svelte recalculates position with new dice
        const currentSelection = get(selectedMoveStore);
        selectedMoveStore.set({...currentSelection});
        console.log('[MovesTable] Triggered selectedMoveStore refresh');
    }

    function validateDiceInput(dice) {
        if (dice.length !== 2) return false;
        const d1 = parseInt(dice[0]);
        const d2 = parseInt(dice[1]);
        return d1 >= 1 && d1 <= 6 && d2 >= 1 && d2 <= 6;
    }    // Watch for EDIT mode changes - start inline editing when entering EDIT mode
    $: {
        console.log('[MovesTable] Reactive check:', {
            mode: $statusBarModeStore,
            hasSelected: !!$selectedMoveStore,
            editing: editingMove,
            visible: visible
        });
        
        if ($statusBarModeStore === 'EDIT' && $selectedMoveStore && !editingMove && visible) {
            console.log('[MovesTable] EDIT mode reactive - triggering startInlineEdit');
            console.log('[MovesTable] visible:', visible, 'editingMove:', editingMove);
            const { gameIndex, moveIndex, player } = $selectedMoveStore;
            startInlineEdit(gameIndex, moveIndex, player);
        } else if ($statusBarModeStore !== 'EDIT' && editingMove) {
            console.log('[MovesTable] Exiting EDIT mode');
            // Exit edit mode if mode changes
            editingMove = null;
            inlineEditDice = '';
            inlineEditMove = '';
        }
    }
    
    // Check if dice is a cube decision or resign decision
    $: isCubeDecision = inlineEditDice.toLowerCase() === 'd' || inlineEditDice.toLowerCase() === 't' || inlineEditDice.toLowerCase() === 'p';
    $: isResignDecision = inlineEditDice.toLowerCase() === 'r' || inlineEditDice.toLowerCase() === 'g' || inlineEditDice.toLowerCase() === 'b';

    function isSelected(gIdx, mIdx) {
        return $selectedMoveStore?.gameIndex === gIdx && 
               $selectedMoveStore?.moveIndex === mIdx;
    }

    function isEditing(gIdx, mIdx, field) {
        return editingMove?.gameIndex === gIdx && 
               editingMove?.moveIndex === mIdx && 
               editingField === field;
    }

    function formatMove(moveData) {
        if (!moveData) return '-';
        if (moveData.dice && moveData.move) {
            return `${moveData.dice}: ${moveData.move}`;
        }
        return moveData.move || '-';
    }

    // Check if game has ended at or before a specific move
    function hasGameEnded(gameIndex, beforeMoveIndex) {
        const game = games[gameIndex];
        if (!game) return false;
        
        // Check if game ended naturally by bearoff - only consider if we're at/after the last move
        if (game.naturalBearoffWin && beforeMoveIndex >= game.moves.length) {
            return true;
        }
        
        for (let i = 0; i < beforeMoveIndex && i < game.moves.length; i++) {
            const move = game.moves[i];
            if (move.player1Move?.cubeAction === 'drops' || move.player1Move?.resignAction) {
                return true;
            }
            if (move.player2Move?.cubeAction === 'drops' || move.player2Move?.resignAction) {
                return true;
            }
        }
        return false;
    }

    function getMoveClass(moveData, moveIndex, player) {
        const classes = [];
        
        // Check for inconsistencies first (highest priority) - applies to all moves including cube actions
        const inconsistencyKey = `${moveIndex}-${player}`;
        if (gameInconsistencies[inconsistencyKey]) {
            classes.push('inconsistent');
        }
        
        // Get the move object for checking game-ending conditions
        const move = moves[moveIndex];
        
        // Check if moveData is null (empty decision that needs to be filled in)
        // Exception: If player2 starts (moveIndex 0, player 1 is null, player 2 has a move), 
        // player1's first decision is legitimately null
        if (!moveData) {
            // Check if this is player1's first decision when player2 starts
            const firstMove = moves[0];
            const player2Starts = moveIndex === 0 && player === 1 && firstMove && 
                                  (!firstMove.player1Move || firstMove.player1Move === null) && 
                                  firstMove.player2Move;
            
            // Check if previous player dropped or resigned (game ended, so null is legitimate)
            let gameEndedBefore = false;
            if (player === 2 && move?.player1Move) {
                // Player 2's slot after player 1 in same move
                gameEndedBefore = move.player1Move.cubeAction === 'drops' || move.player1Move.resignAction;
            } else if (player === 1 && moveIndex > 0) {
                // Player 1's slot after player 2 from previous move
                const prevMove = moves[moveIndex - 1];
                gameEndedBefore = prevMove?.player2Move && 
                                 (prevMove.player2Move.cubeAction === 'drops' || prevMove.player2Move.resignAction);
            }
            
            // Check for natural bearoff win
            const gameEndedNaturally = currentGame && currentGame.naturalBearoffWin;
            
            // Only mark as inconsistent if it's NOT a legitimate null
            if (!player2Starts && !gameEndedBefore && !gameEndedNaturally) {
                classes.push('inconsistent'); // Mark null decision as inconsistent/incomplete
            }
            return classes.join(' ');
        }
        
        // Check if this is an empty decision object (needs to be filled in)
        // Empty decision = no dice value and no move (and no cube action, no resign action)
        const isEmpty = !moveData.cubeAction && !moveData.resignAction && 
                       (!moveData.dice || moveData.dice === '') && 
                       (!moveData.move || moveData.move === '');
        
        if (isEmpty) {
            // Check if this is player1's first decision when player2 starts (legitimate empty)
            const firstMove = moves[0];
            const player2Starts = moveIndex === 0 && player === 1 && firstMove && 
                                  (!firstMove.player1Move || !firstMove.player1Move.dice || firstMove.player1Move.dice === '') && 
                                  firstMove.player2Move && firstMove.player2Move.dice;
            
            // Check if previous player dropped or resigned (game ended, so empty is legitimate)
            let gameEndedBefore = false;
            if (player === 2 && move?.player1Move) {
                // Player 2's decision after player 1 in same move
                gameEndedBefore = move.player1Move.cubeAction === 'drops' || move.player1Move.resignAction;
            } else if (player === 1 && moveIndex > 0) {
                // Player 1's decision after player 2 from previous move
                const prevMove = moves[moveIndex - 1];
                gameEndedBefore = prevMove?.player2Move && 
                                 (prevMove.player2Move.cubeAction === 'drops' || prevMove.player2Move.resignAction);
            }
            
            // Only mark as inconsistent if it's NOT a legitimate empty
            if (!player2Starts && !gameEndedBefore) {
                classes.push('inconsistent');
            }
        }
        
        // Don't highlight "Cannot Move"
        if (moveData.move === 'Cannot Move') return classes.join(' ');
        // Don't mark as illegal if it's a valid resign action or cube action
        if (moveData.isIllegal && !moveData.resignAction && !moveData.cubeAction) classes.push('illegal');
        if (moveData.isGala) classes.push('gala');
        return classes.join(' ');
    }
    
    function hasInconsistency(moveIndex, player) {
        const inconsistencyKey = `${moveIndex}-${player}`;
        return gameInconsistencies[inconsistencyKey];
    }
    
    function getTooltipMessage(moveIndex, player) {
        // Check for inconsistency first
        const inconsistency = hasInconsistency(moveIndex, player);
        if (inconsistency) {
            return inconsistency.reason;
        }
        
        // Check for empty decision
        const move = moves[moveIndex];
        if (!move) return '';
        
        const moveData = player === 1 ? move.player1Move : move.player2Move;
        
        // Check if it's null (empty slot)
        if (!moveData) {
            // Check if this is player1's first decision when player2 starts (legitimate)
            const firstMove = moves[0];
            const player2Starts = moveIndex === 0 && player === 1 && firstMove && 
                                  (!firstMove.player1Move || firstMove.player1Move === null) && 
                                  firstMove.player2Move;
            
            // Check if game ended (drop, resign, or natural bearoff) - empty decision is legitimate
            const currentMove = moves[moveIndex];
            const gameEndedByAction = (player === 2 && currentMove?.player1Move && (currentMove.player1Move.cubeAction === 'drops' || currentMove.player1Move.resignAction)) ||
                             (player === 1 && moveIndex > 0 && moves[moveIndex - 1]?.player2Move && 
                              (moves[moveIndex - 1].player2Move.cubeAction === 'drops' || moves[moveIndex - 1].player2Move.resignAction));
            
            // Check for natural bearoff win
            const gameEndedNaturally = currentGame && currentGame.naturalBearoffWin;
            
            if (!player2Starts && !gameEndedByAction && !gameEndedNaturally) {
                return 'Empty decision needs to be defined';
            }
        }
        
        // Check if it's an empty object (no cube action, no resign action, no dice, no move)
        const isEmpty = moveData && !moveData.cubeAction && !moveData.resignAction &&
                       (!moveData.dice || moveData.dice === '') && 
                       (!moveData.move || moveData.move === '');
        
        if (isEmpty) {
            // Check if this is player1's first decision when player2 starts (legitimate empty)
            const firstMove = moves[0];
            const player2Starts = moveIndex === 0 && player === 1 && firstMove && 
                                  (!firstMove.player1Move || !firstMove.player1Move.dice || firstMove.player1Move.dice === '') && 
                                  firstMove.player2Move && firstMove.player2Move.dice;
            
            // Check if game ended (drop, resign, or natural bearoff) - empty decision is legitimate
            const currentMove = moves[moveIndex];
            const gameEndedByAction = (player === 2 && currentMove?.player1Move && (currentMove.player1Move.cubeAction === 'drops' || currentMove.player1Move.resignAction)) ||
                             (player === 1 && moveIndex > 0 && moves[moveIndex - 1]?.player2Move && 
                              (moves[moveIndex - 1].player2Move.cubeAction === 'drops' || moves[moveIndex - 1].player2Move.resignAction));
            
            // Check for natural bearoff win
            const gameEndedNaturally = currentGame && currentGame.naturalBearoffWin;
            
            if (!player2Starts && !gameEndedByAction && !gameEndedNaturally) {
                return 'Empty decision needs to be defined';
            }
        }
        
        return '';
    }
</script>

<svelte:window on:keydown={handleKeyDown} on:click={closeContextMenu} on:mouseup={handleMouseUp} />

{#if visible}
<div class="moves-table">
    <!-- Moves table -->
    {#if currentGame}
    <div class="table-wrapper" bind:this={tableWrapper}>
    <table>
        <tbody>
            {#each playerRows as row, rowIdx}
            <tr 
                class:selected={$selectedMoveStore?.gameIndex === row.gameIndex && $selectedMoveStore?.moveIndex === row.moveIndex && $selectedMoveStore?.player === row.player}
                class:in-selection={isInSelectedRange(row.gameIndex, row.moveIndex, row.player)}
                class:player1={row.player === 1}
                class:player2={row.player === 2}
                class={getMoveClass(row.moveData, row.moveIndex, row.player)}
                on:mousedown={(e) => handleMouseDown(row.gameIndex, row.moveIndex, row.player, e)}
                on:mouseenter={() => handleMouseEnter(row.gameIndex, row.moveIndex, row.player)}
                on:contextmenu|preventDefault={(e) => handleContextMenu(e, row.gameIndex, row.moveIndex, row.player)}
                title={getTooltipMessage(row.moveIndex, row.player)}
            >
                <td class="move-number">{row.player === 1 ? row.moveNumber : ''}</td>
                <td class="dice-cell">
                    {#if editingMove?.gameIndex === row.gameIndex && editingMove?.moveIndex === row.moveIndex && editingMove?.player === row.player && $statusBarModeStore === 'EDIT'}
                    <input
                        type="text"
                        bind:this={diceInputElement}
                        bind:value={inlineEditDice}
                        on:input={handleDiceInput}
                        on:keydown={handleInlineEditKeyDown}
                        maxlength="2"
                        class="inline-dice-input"
                        placeholder="54"
                    />
                    {:else if row.moveData?.cubeAction || (row.cubeAction && row.cubeAction.player === row.player)}
                    <span class="empty"></span>
                    {:else if row.moveData?.dice}
                    {row.moveData.dice}
                    {:else}
                    <span class="empty">-</span>
                    {/if}
                </td>
                <td class="move-cell">
                    {#if editingMove?.gameIndex === row.gameIndex && editingMove?.moveIndex === row.moveIndex && editingMove?.player === row.player && $statusBarModeStore === 'EDIT'}
                    <input
                        type="text"
                        bind:this={moveInputElement}
                        bind:value={inlineEditMove}
                        on:keydown={handleInlineEditKeyDown}
                        class="inline-move-input"
                        placeholder="24/20 13/8"
                        disabled={isCubeDecision || isResignDecision}
                    />
                    {:else if row.moveData?.cubeAction}
                        {#if row.moveData.cubeAction === 'doubles'}
                        <span class="cube-action">Doubles → {row.moveData.cubeValue || 2}</span>
                        {:else if row.moveData.cubeAction === 'takes'}
                        <span class="cube-response">Takes</span>
                        {:else if row.moveData.cubeAction === 'drops'}
                        <span class="cube-response">Drops</span>
                        {/if}
                    {:else if row.moveData?.resignAction}
                        {#if row.moveData.resignAction === 'normal'}
                        <span class="cube-response">Resigns</span>
                        {:else if row.moveData.resignAction === 'gammon'}
                        <span class="cube-response">Resigns Gammon</span>
                        {:else if row.moveData.resignAction === 'backgammon'}
                        <span class="cube-response">Resigns Backgammon</span>
                        {/if}
                    {:else if row.cubeAction && row.cubeAction.player === row.player}
                        {#if row.cubeAction.action === 'doubles'}
                        <span class="cube-action">Doubles → {row.cubeAction.value}</span>
                        {:else if row.cubeAction.action === 'takes'}
                        <span class="cube-response">Takes</span>
                        {:else if row.cubeAction.action === 'drops'}
                        <span class="cube-response">Drops</span>
                        {/if}
                    {:else if row.moveData}
                    <span 
                        role="textbox"
                        tabindex="0"
                        contenteditable={editingMove?.gameIndex === row.gameIndex && editingMove?.moveIndex === row.moveIndex && editingMove?.player === row.player}
                        on:dblclick={() => { editingMove = { gameIndex: row.gameIndex, moveIndex: row.moveIndex, player: row.player }; }}
                        on:blur={(e) => finishEdit(e, row.gameIndex, row.moveIndex, row.player)}
                    >
                        {row.moveData.move || '-'}
                    </span>
                    {:else}
                    <span class="empty">-</span>
                    {/if}
                </td>
            </tr>
            {/each}
        </tbody>
    </table>
    </div>
    {:else}
    <div class="empty-state">
        <p>No match loaded. Open a match file to start transcription.</p>
    </div>
    {/if}
</div>
{/if}

<!-- Context Menu -->
{#if showContextMenu}
<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div 
    class="context-menu-overlay" 
    on:click={closeContextMenu}
    on:contextmenu|preventDefault={closeContextMenu}
    role="presentation"
>
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div 
        class="context-menu" 
        style="left: {contextMenuX}px; top: {contextMenuY}px;"
        on:click|stopPropagation
        role="menu"
        tabindex="-1"
    >
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" class:disabled={isMultiSelectionActive()} on:click={isMultiSelectionActive() ? null : handleInsertBefore} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
            Before
        </div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" class:disabled={isMultiSelectionActive()} on:click={isMultiSelectionActive() ? null : handleInsertAfter} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
            After
        </div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" on:click={handleDeleteDecision} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14" />
            </svg>
            Delete
        </div>
        <div class="context-menu-separator"></div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" on:click={handleCopyDecision} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 7.5V6.108c0-1.135.845-2.098 1.976-2.192.373-.03.748-.057 1.123-.08M15.75 18H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08M15.75 18.75v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5A3.375 3.375 0 0 0 6.375 7.5H5.25m11.9-3.664A2.251 2.251 0 0 0 15 2.25h-1.5a2.251 2.251 0 0 0-2.15 1.586m5.8 0c.065.21.1.433.1.664v.75h-6V4.5c0-.231.035-.454.1-.664M6.75 7.5H4.875c-.621 0-1.125.504-1.125 1.125v12c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V16.5a9 9 0 0 0-9-9Z" />
            </svg>
            Copy
        </div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" on:click={handleCutDecision} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="m7.848 8.25 1.536.887M7.848 8.25a3 3 0 1 1-5.196-3 3 3 0 0 1 5.196 3Zm1.536.887a2.165 2.165 0 0 1 1.083 1.839c.005.351.054.695.14 1.024M9.384 9.137l2.077 1.199M7.848 15.75l1.536-.887m-1.536.887a3 3 0 1 1-5.196 3 3 3 0 0 1 5.196-3Zm1.536-.887a2.165 2.165 0 0 0 1.083-1.838c.005-.352.054-.695.14-1.025m-1.223 2.863 2.077-1.199m0-3.328a4.323 4.323 0 0 1 2.068-1.379l5.325-1.628a4.5 4.5 0 0 1 2.48-.044l.803.215-7.794 4.5m-2.882-1.664A4.33 4.33 0 0 0 10.607 12m3.736 0 7.794 4.5-.802.215a4.5 4.5 0 0 1-2.48-.043l-5.326-1.629a4.324 4.324 0 0 1-2.068-1.379M14.343 12l-2.882 1.664" />
            </svg>
            Cut
        </div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" class:disabled={isMultiSelectionActive()} on:click={isMultiSelectionActive() ? null : handlePasteBeforeDecision} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
            </svg>
            Paste Before
        </div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" class:disabled={isMultiSelectionActive()} on:click={isMultiSelectionActive() ? null : handlePasteAfterDecision} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
            </svg>
            Paste After
        </div>
        <div class="context-menu-separator"></div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" class:disabled={!canUndo} on:click={canUndo ? handleUndo : null} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 15 3 9m0 0 6-6M3 9h12a6 6 0 0 1 0 12h-3" />
            </svg>
            Undo
        </div>
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="context-menu-item" class:disabled={!canRedo} on:click={canRedo ? handleRedo : null} role="menuitem" tabindex="0">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="context-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="m15 15 6-6m0 0-6-6m6 6H9a6 6 0 0 0 0 12h3" />
            </svg>
            Redo
        </div>
    </div>
</div>
{/if}



<style>
    .moves-table {
        display: flex;
        flex-direction: column;
        height: 100%;
        background-color: #f9f9f9;
        overflow: hidden;
        isolation: isolate;
    }



    .table-wrapper {
        overflow-y: auto;
        flex: 1;
        position: relative;
        z-index: 1;
    }

    /* Remove scrollbar transitions and ensure it stays below context menu */
    .table-wrapper::-webkit-scrollbar {
        width: 8px;
        transition: none;
    }

    .table-wrapper::-webkit-scrollbar-track {
        background: #f1f1f1;
        transition: none;
    }

    .table-wrapper::-webkit-scrollbar-thumb {
        background: #888;
        border-radius: 4px;
        transition: none;
    }

    .table-wrapper::-webkit-scrollbar-thumb:hover {
        background: #555;
    }

    table {
        width: 100%;
        border-collapse: collapse;
        background: white;
        table-layout: fixed;
    }



    td {
        padding: 3px 4px;
        border-bottom: 1px solid #eee;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 1.3;
    }

    tbody tr {
        cursor: pointer;
        transition: background-color 0.15s;
        user-select: none;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
    }

    tbody tr:hover {
        background-color: #f5f5f5;
    }

    tbody tr.selected {
        background-color: #d4e6f8 !important;
        outline: 2px solid #4a90e2;
    }

    tbody tr.in-selection {
        background-color: #e8f4fd !important;
        outline: 1px solid #7ab8f0;
    }

    tbody tr.selected.in-selection {
        background-color: #d4e6f8 !important;
        outline: 2px solid #4a90e2;
    }

    tr.illegal {
        background-color: #ffe0e0;
    }

    tr.gala {
        background-color: #fff3cd;
    }

    tr.inconsistent {
        background-color: #ffcccc !important;
        color: #333333 !important;
    }
    
    tr.inconsistent:hover {
        background-color: #ffaaaa !important;
        color: #333333 !important;
    }
    
    tr.inconsistent.selected {
        background-color: #ff8888 !important;
        color: #333333 !important;
        outline: 2px solid #ff6666;
    }
    
    tr.inconsistent .dice-cell,
    tr.inconsistent .move-number,
    tr.inconsistent .empty {
        color: inherit !important;
    }
    
    tr.inconsistent.selected .dice-cell,
    tr.inconsistent.selected .move-number,
    tr.inconsistent.selected .empty {
        color: inherit !important;
    }
    
    tr.player1 {
        border-bottom: none;
    }
    
    tr.player2 {
        border-bottom: 2px solid #ddd;
    }
    
    .move-number {
        font-weight: 400;
        color: #aaa;
        text-align: center;
        width: 20px;
        min-width: 20px;
        max-width: 20px;
        padding: 3px 2px;
        font-size: 10px;
    }
    
    .dice-cell {
        font-weight: 600;
        color: #2c5aa0;
        text-align: center;
        width: 45px;
        min-width: 45px;
        max-width: 45px;
        font-family: monospace;
        padding: 3px 3px;
        font-size: 12px;
    }
    
    .cube-icon {
        font-size: 14px;
    }
    
    .move-cell {
        font-family: monospace;
        white-space: nowrap;
        text-align: left;
        font-size: 13px;
    }
    
    .empty {
        color: #ccc;
    }

    span[contenteditable="true"] {
        display: inline-block;
        min-width: 50px;
        padding: 2px 4px;
        border: 1px solid #4a90e2;
        background-color: #fff;
        border-radius: 2px;
    }

    .cube-action {
        font-weight: normal;
        color: inherit;
    }

    .cube-response {
        font-weight: normal;
        color: inherit;
    }

    .validation-status {
        margin-top: 10px;
        padding: 8px 12px;
        border-radius: 4px;
        background-color: #fff3cd;
        color: #856404;
        font-size: 14px;
        text-align: center;
    }

    .validation-status.valid {
        background-color: #d4edda;
        color: #155724;
    }

    .empty-state {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        color: #666;
    }

    .empty-state p {
        font-size: 14px;
        text-align: center;
    }
    
    .inline-dice-input {
        width: 100%;
        padding: 2px 3px;
        font-size: 12px;
        font-weight: 600;
        text-align: center;
        border: 1px solid #4a90e2;
        border-radius: 2px;
        outline: none;
        font-family: monospace;
        background-color: #fff;
        color: #2c5aa0;
    }
    
    .inline-move-input {
        width: 100%;
        padding: 2px 4px;
        font-size: 13px;
        border: 1px solid #4a90e2;
        border-radius: 2px;
        outline: none;
        font-family: monospace;
        background-color: #fff;
    }
    
    .inline-move-input:disabled {
        background-color: #f0f0f0;
        color: #999;
        cursor: not-allowed;
    }
    
    .inline-dice-input:focus,
    .inline-move-input:focus {
        border-color: #2c5aa0;
        box-shadow: 0 0 0 2px rgba(74, 144, 226, 0.2);
    }

    /* Context Menu */
    .context-menu-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        z-index: 9999;
        background: transparent;
    }

    .context-menu {
        position: fixed;
        background: white;
        border: 1px solid #ccc;
        border-radius: 3px;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
        z-index: 10000;
        min-width: 140px;
        padding: 2px 0;
        font-size: 13px;
    }

    .context-menu-item {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 4px 10px;
        cursor: pointer;
        font-size: 13px;
        color: #333;
        transition: background-color 0.1s;
        line-height: 1.4;
    }

    .context-menu-item:hover {
        background-color: #f0f0f0;
    }

    .context-menu-item.disabled {
        opacity: 0.4;
        cursor: not-allowed;
        pointer-events: none;
    }
    
    .context-menu-separator {
        height: 1px;
        background-color: #ddd;
        margin: 4px 0;
    }

    .context-icon {
        width: 13px;
        height: 13px;
        color: #666;
        flex-shrink: 0;
    }
</style>
