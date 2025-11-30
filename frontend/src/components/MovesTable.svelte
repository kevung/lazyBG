<script>
    import { 
        transcriptionStore, 
        selectedMoveStore, 
        matchValidationStore,
        insertMoveBefore,
        insertMoveAfter,
        deleteMove,
        updateMove
    } from '../stores/transcriptionStore.js';
    
    export let visible = true;

    let selectedGameIndex = 0;
    let editingMove = null;
    let editingField = null;

    $: games = $transcriptionStore?.games || [];
    $: currentGame = games[selectedGameIndex];
    $: moves = currentGame?.moves || [];
    $: validation = $matchValidationStore;
    
    // Flatten moves into individual player rows
    $: playerRows = moves.flatMap((move, mIdx) => [
        { gameIndex: selectedGameIndex, moveIndex: mIdx, moveNumber: move.moveNumber, player: 1, moveData: move.player1Move, cubeAction: move.cubeAction },
        { gameIndex: selectedGameIndex, moveIndex: mIdx, moveNumber: move.moveNumber, player: 2, moveData: move.player2Move, cubeAction: move.cubeAction }
    ]);

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

    function getMoveClass(moveData) {
        if (!moveData) return '';
        const classes = [];
        // Don't highlight "Cannot Move"
        if (moveData.move === 'Cannot Move') return '';
        if (moveData.isIllegal) classes.push('illegal');
        if (moveData.isGala) classes.push('gala');
        return classes.join(' ');
    }
</script>

<svelte:window on:keydown={handleKeyDown} />

{#if visible}
<div class="moves-table">
    <!-- Moves table -->
    {#if currentGame}
    <div class="table-wrapper">
    <table>
        <tbody>
            {#each playerRows as row, rowIdx}
            <tr 
                class:selected={$selectedMoveStore?.gameIndex === row.gameIndex && $selectedMoveStore?.moveIndex === row.moveIndex && $selectedMoveStore?.player === row.player}
                class:player1={row.player === 1}
                class:player2={row.player === 2}
                class={getMoveClass(row.moveData)}
                on:click={() => selectMove(row.gameIndex, row.moveIndex, row.player)}
            >
                <td class="move-number">{row.player === 1 ? row.moveNumber : ''}</td>
                <td class="dice-cell">
                    {#if row.cubeAction && row.player === 1}
                    <span class="empty"></span>
                    {:else if row.cubeAction && row.player === 2}
                    <span class="empty"></span>
                    {:else if row.moveData?.dice}
                    {row.moveData.dice}
                    {:else}
                    <span class="empty">-</span>
                    {/if}
                </td>
                <td class="move-cell">
                    {#if row.cubeAction && row.player === 1}
                    <span class="cube-action">Doubles → {row.cubeAction.value}</span>
                    {:else if row.cubeAction && row.player === 2}
                    <span class="cube-response">{row.cubeAction.response}</span>
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
</style>
