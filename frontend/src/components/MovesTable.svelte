<script>
    import { 
        transcriptionStore, 
        selectedMoveStore, 
        matchValidationStore,
        moveInconsistenciesStore,
        insertMoveBefore,
        insertMoveAfter,
        deleteMove,
        updateMove,
        invalidatePositionsCacheFrom
    } from '../stores/transcriptionStore.js';
    import { statusBarModeStore, statusBarTextStore } from '../stores/uiStore.js';
    
    export let visible = true;

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

    $: games = $transcriptionStore?.games || [];
    $: selectedGameIndex = $selectedMoveStore?.gameIndex ?? 0;
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
        { gameIndex: selectedGameIndex, moveIndex: mIdx, moveNumber: move.moveNumber, player: 1, moveData: move.player1Move, cubeAction: move.cubeAction },
        { gameIndex: selectedGameIndex, moveIndex: mIdx, moveNumber: move.moveNumber, player: 2, moveData: move.player2Move, cubeAction: move.cubeAction }
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

    function selectMove(gameIndex, moveIndex, player) {
        selectedMoveStore.set({ gameIndex, moveIndex, player });
    }

    function startEdit(gameIndex, moveIndex, field, player) {
        editingMove = { gameIndex, moveIndex };
        editingField = field;
        selectMove(gameIndex, moveIndex, player);
    }

    function finishEdit(event, gameIndex, moveIndex, player) {
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
        
        updateMove(gameIndex, move.moveNumber, player, dice, moveStr, isIllegal, isGala);
        
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
                insertMoveAfter(gameIndex, currentMoves[moveIndex].moveNumber);
                break;
                
            case 'i':
                event.preventDefault();
                const moveEntry = currentMoves[moveIndex];
                const playerMove = player === 1 ? moveEntry.player1Move : moveEntry.player2Move;
                if (playerMove) {
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
                
            case 'Delete':
                event.preventDefault();
                deleteMove(gameIndex, currentMoves[moveIndex].moveNumber);
                break;
        }
    }
    
    function handleInlineEditKeyDown(event) {
        if (event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
            cancelInlineEdit();
        } else if (event.key === 'Enter') {
            event.preventDefault();
            event.stopPropagation();
            
            // If dice input is focused
            if (document.activeElement === diceInputElement) {
                const dice = inlineEditDice.toLowerCase();
                // Check for cube decisions
                if (dice === 'd' || dice === 't' || dice === 'p') {
                    // Validate immediately for cube decisions
                    validateInlineEdit();
                } else if (validateDiceInput(inlineEditDice)) {
                    // Valid dice, move to move input
                    moveInputElement?.focus();
                } else {
                    statusBarTextStore.set('Invalid dice: enter d/t/p or 2 digits between 1 and 6');
                }
            } else if (document.activeElement === moveInputElement) {
                // Validate and save
                const dice = inlineEditDice.toLowerCase();
                if (dice === 'd' || dice === 't' || dice === 'p' || validateDiceInput(inlineEditDice)) {
                    validateInlineEdit();
                } else {
                    statusBarTextStore.set('Invalid dice: enter d/t/p or 2 digits between 1 and 6');
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
        const game = games[gameIndex];
        if (!game || !game.moves[moveIndex]) return;
        
        const move = game.moves[moveIndex];
        const playerMove = player === 1 ? move.player1Move : move.player2Move;
        
        editingMove = { gameIndex, moveIndex, player };
        
        if (playerMove) {
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
        
        statusBarTextStore.set('EDIT MODE: Enter dice (d=double, t=take, p=pass) or 2 digits 1-6, then move. Enter=validate, Esc=cancel');
    }
    
    function cancelInlineEdit() {
        editingMove = null;
        inlineEditDice = '';
        inlineEditMove = '';
        statusBarModeStore.set('NORMAL');
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
        
        updateMove(gameIndex, move.moveNumber, player, inlineEditDice, inlineEditMove, isIllegal, isGala);
        
        // Invalidate position cache from this move onwards
        await invalidatePositionsCacheFrom(gameIndex, moveIndex);
        
        // Force recalculation
        selectedMoveStore.set({ gameIndex, moveIndex, player });
        
        statusBarTextStore.set(`Move updated at game ${gameIndex + 1}, move ${move.moveNumber}`);
        
        cancelInlineEdit();
    }
    
    function handleDiceInput(event) {
        let value = event.target.value.toLowerCase();
        
        // Check for cube decisions first
        if (value === 'd' || value === 't' || value === 'p') {
            inlineEditDice = value;
            // Clear move input for cube decisions
            inlineEditMove = '';
            return;
        }
        
        // Only allow digits
        value = value.replace(/[^1-6]/g, '');
        
        // Limit to 2 characters
        if (value.length > 2) {
            value = value.slice(0, 2);
        }
        
        inlineEditDice = value;
        
        // Auto-advance to move input when 2 valid digits are entered
        if (value.length === 2 && validateDiceInput(value)) {
            setTimeout(() => {
                moveInputElement?.focus();
            }, 50);
        }
    }
    
    function validateDiceInput(dice) {
        if (dice.length !== 2) return false;
        const d1 = parseInt(dice[0]);
        const d2 = parseInt(dice[1]);
        return d1 >= 1 && d1 <= 6 && d2 >= 1 && d2 <= 6;
    }
    
    // Watch for EDIT mode changes - start inline editing when entering EDIT mode
    $: if ($statusBarModeStore === 'EDIT' && $selectedMoveStore && !editingMove && visible) {
        const { gameIndex, moveIndex, player } = $selectedMoveStore;
        startInlineEdit(gameIndex, moveIndex, player);
    } else if ($statusBarModeStore !== 'EDIT' && editingMove) {
        // Exit edit mode if mode changes
        editingMove = null;
        inlineEditDice = '';
        inlineEditMove = '';
    }
    
    // Check if dice is a cube decision
    $: isCubeDecision = inlineEditDice.toLowerCase() === 'd' || inlineEditDice.toLowerCase() === 't' || inlineEditDice.toLowerCase() === 'p';

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

    function getMoveClass(moveData, moveIndex, player) {
        if (!moveData) return '';
        const classes = [];
        
        // Check for inconsistencies first (highest priority)
        const inconsistencyKey = `${moveIndex}-${player}`;
        if (gameInconsistencies[inconsistencyKey]) {
            classes.push('inconsistent');
        }
        
        // Don't highlight "Cannot Move"
        if (moveData.move === 'Cannot Move') return classes.join(' ');
        if (moveData.isIllegal) classes.push('illegal');
        if (moveData.isGala) classes.push('gala');
        return classes.join(' ');
    }
    
    function hasInconsistency(moveIndex, player) {
        const inconsistencyKey = `${moveIndex}-${player}`;
        return gameInconsistencies[inconsistencyKey];
    }
</script>

<svelte:window on:keydown={handleKeyDown} />

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
                class:player1={row.player === 1}
                class:player2={row.player === 2}
                class={getMoveClass(row.moveData, row.moveIndex, row.player)}
                on:click={() => selectMove(row.gameIndex, row.moveIndex, row.player)}
                title={hasInconsistency(row.moveIndex, row.player) ? hasInconsistency(row.moveIndex, row.player).reason : ''}
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
                    {:else if row.cubeAction && row.cubeAction.player === row.player}
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
                        disabled={isCubeDecision}
                    />
                    {:else if row.cubeAction && row.cubeAction.player === row.player}
                        {#if row.cubeAction.action === 'doubles'}
                        <span class="cube-action">Doubles → {row.cubeAction.value}</span>
                        {:else if row.cubeAction.action === 'takes'}
                        <span class="cube-response">Takes</span>
                        {:else if row.cubeAction.action === 'drops'}
                        <span class="cube-response">Drops</span>
                        {/if}
                    {:else if row.cubeAction && row.cubeAction.player !== row.player && row.cubeAction.response}
                        {#if row.cubeAction.response === 'takes'}
                        <span class="cube-response">Takes</span>
                        {:else if row.cubeAction.response === 'drops'}
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

<style>
    .moves-table {
        display: flex;
        flex-direction: column;
        height: 100%;
        background-color: #f9f9f9;
        overflow: hidden;
    }



    .table-wrapper {
        overflow-y: auto;
        flex: 1;
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
    }

    tbody tr:hover {
        background-color: #f5f5f5;
    }

    tbody tr.selected {
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
        background-color: #ff6b6b !important;
        color: white !important;
    }
    
    tr.inconsistent:hover {
        background-color: #ff5252 !important;
    }
    
    tr.inconsistent.selected {
        background-color: #ff4444 !important;
        outline: 2px solid #cc0000;
    }
    
    tr.inconsistent .dice-cell,
    tr.inconsistent .move-number,
    tr.inconsistent .empty {
        color: white !important;
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
</style>
