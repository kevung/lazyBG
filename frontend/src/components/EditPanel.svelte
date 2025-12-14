<script>
    import { onMount, onDestroy } from 'svelte';
    import { slide } from 'svelte/transition';
    import { get } from 'svelte/store';
    import { 
        transcriptionStore, 
        selectedMoveStore,
        updateMove,
        invalidatePositionsCacheFrom,
        insertDecisionAfter
    } from '../stores/transcriptionStore.js';
    import { 
        statusBarModeStore,
        statusBarTextStore
    } from '../stores/uiStore.js';

    export let visible = false;
    export let onClose;

    // State for editing
    let diceInput = '';
    let moveInput = '';
    let originalDice = '';
    let originalMove = '';
    
    let diceInputElement;
    let moveInputElement;
    let panelEl;

    let hasAutoFocused = false;

    // Initialize when panel becomes visible
    $: if (visible && !hasAutoFocused) {
        startEditing();
    } else if (!visible) {
        hasAutoFocused = false;
    }

    function startEditing() {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        
        if (!transcription || !selectedMove) {
            return;
        }

        const { gameIndex, moveIndex, player } = selectedMove;
        const game = transcription.games[gameIndex];
        
        if (!game || !game.moves[moveIndex]) {
            return;
        }

        const move = game.moves[moveIndex];
        const playerMove = player === 1 ? move.player1Move : move.player2Move;

        // Check if this is a cube action
        if (playerMove?.cubeAction) {
            if (playerMove.cubeAction === 'doubles') {
                originalDice = 'd';
            } else if (playerMove.cubeAction === 'takes') {
                originalDice = 't';
            } else if (playerMove.cubeAction === 'drops') {
                originalDice = 'p';
            }
            originalMove = '';
            diceInput = originalDice;
            moveInput = '';
        } else if (playerMove?.resignAction) {
            // Check if this is a resign action
            if (playerMove.resignAction === 'normal') {
                originalDice = 'r';
            } else if (playerMove.resignAction === 'gammon') {
                originalDice = 'g';
            } else if (playerMove.resignAction === 'backgammon') {
                originalDice = 'b';
            }
            originalMove = '';
            diceInput = originalDice;
            moveInput = '';
        } else if (playerMove) {
            originalDice = playerMove.dice || '';
            originalMove = playerMove.move || '';
            diceInput = originalDice;
            moveInput = originalMove;
        } else {
            originalDice = '';
            originalMove = '';
            diceInput = '';
            moveInput = '';
        }

        // Focus on dice input
        setTimeout(() => {
            if (diceInputElement) {
                diceInputElement.focus();
                hasAutoFocused = true;
            }
        }, 100);

        statusBarTextStore.set('EDIT MODE: Enter dice (d=double, t=take, p=pass, r=resign, g=resign gammon, b=resign backgammon) or 2 digits 1-6, then move. Enter=validate, Esc=cancel');
    }

    function cancelEditing() {
        diceInput = '';
        moveInput = '';
        onClose();
    }

    async function validateEditing() {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        
        if (!transcription || !selectedMove) {
            return;
        }

        const { gameIndex, moveIndex, player } = selectedMove;
        const game = transcription.games[gameIndex];
        
        if (!game || !game.moves[moveIndex]) {
            return;
        }

        const move = game.moves[moveIndex];
        
        // Check if anything changed
        const hasChanges = diceInput !== originalDice || moveInput !== originalMove;

        if (hasChanges) {
            // Update the move
            const playerMove = player === 1 ? move.player1Move : move.player2Move;
            const isIllegal = playerMove?.isIllegal || false;
            const isGala = playerMove?.isGala || false;
            
            // For cube decisions (d/t/p) and resign decisions (r/g/b), pass empty string as move
            const diceStr = diceInput.toLowerCase();
            const isCubeDecision = diceStr === 'd' || diceStr === 't' || diceStr === 'p';
            const isResignDecision = diceStr === 'r' || diceStr === 'g' || diceStr === 'b';
            const moveToSave = (isCubeDecision || isResignDecision) ? '' : moveInput;
            
            updateMove(gameIndex, move.moveNumber, player, diceInput, moveToSave, isIllegal, isGala);
            
            // Invalidate position cache from this move onwards
            await invalidatePositionsCacheFrom(gameIndex, moveIndex);
            
            // Force recalculation of all positions after this move
            selectedMoveStore.set({ ...selectedMove });
            
            statusBarTextStore.set(`Move updated at game ${gameIndex + 1}, move ${move.moveNumber}`);
        } else {
            statusBarTextStore.set('No changes made');
        }

        // Move to next decision and stay in edit mode
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
            // Reset and restart editing for the new decision
            hasAutoFocused = false;
            startEditing();
            return;
        }
        
        // Reset and restart editing for the next decision
        hasAutoFocused = false;
        startEditing();
    }

    function handleKeyDown(event) {
        if (!visible) return;

        event.stopPropagation();

        if (event.key === 'Escape') {
            event.preventDefault();
            cancelEditing();
        } else if (event.key === 'Enter') {
            event.preventDefault();
            
            // If dice input is focused
            if (document.activeElement === diceInputElement) {
                const dice = diceInput.toLowerCase();
                // Check for cube decisions and resign decisions
                if (dice === 'd' || dice === 't' || dice === 'p' || dice === 'r' || dice === 'g' || dice === 'b') {
                    // Validate immediately for cube and resign decisions
                    validateEditing();
                } else if (validateDiceInput(diceInput)) {
                    // Valid dice, move to move input
                    moveInputElement?.focus();
                } else {
                    statusBarTextStore.set('Invalid dice: enter d/t/p or 2 digits between 1 and 6');
                }
            } else if (document.activeElement === moveInputElement) {
                // Validate and save
                const dice = diceInput.toLowerCase();
                if (dice === 'd' || dice === 't' || dice === 'p' || validateDiceInput(diceInput)) {
                    validateEditing();
                } else {
                    statusBarTextStore.set('Invalid dice: enter d/t/p or 2 digits between 1 and 6');
                }
            }
        } else if (event.key === 'Backspace') {
            // If in move input and it's empty, focus on dice input
            if (document.activeElement === moveInputElement && moveInput === '') {
                event.preventDefault();
                diceInputElement?.focus();
                // Select all text in dice input for easy editing
                if (diceInputElement) {
                    diceInputElement.select();
                }
            }
        }
    }

    function handleDiceInput(event) {
        let value = event.target.value.toLowerCase();
        
        // Check for cube decisions and resign decisions first
        if (value === 'd' || value === 't' || value === 'p' || value === 'r' || value === 'g' || value === 'b') {
            diceInput = value;
            // Disable move input for cube and resign decisions
            moveInput = '';
            return;
        }
        
        // Only allow digits
        value = value.replace(/[^1-6]/g, '');
        
        // Limit to 2 characters
        if (value.length > 2) {
            value = value.slice(0, 2);
        }
        
        diceInput = value;
        
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

    function handleClickOutside(event) {
        if (panelEl && !panelEl.contains(event.target)) {
            // Click outside the panel - blur the input
            diceInputElement?.blur();
            moveInputElement?.blur();
        }
    }

    onMount(() => {
        document.addEventListener('mousedown', handleClickOutside);
    });

    onDestroy(() => {
        document.removeEventListener('mousedown', handleClickOutside);
    });

    // Check if dice is a cube decision or resign decision
    $: isCubeDecision = diceInput.toLowerCase() === 'd' || diceInput.toLowerCase() === 't' || diceInput.toLowerCase() === 'p';
    $: isResignDecision = diceInput.toLowerCase() === 'r' || diceInput.toLowerCase() === 'g' || diceInput.toLowerCase() === 'b';
</script>

{#if visible}
    <div class="edit-panel" transition:slide={{ duration: 50 }} bind:this={panelEl}>
        <div class="panel-content">
            <div class="field-group">
                <label for="dice-input">Dice:</label>
                <input
                    id="dice-input"
                    type="text"
                    bind:this={diceInputElement}
                    bind:value={diceInput}
                    on:input={handleDiceInput}
                    on:keydown={handleKeyDown}
                    maxlength="2"
                    placeholder="54 or d/t/p/r/g/b"
                    class="dice-input"
                />
            </div>
            
            <div class="field-group">
                <label for="move-input">Move:</label>
                <input
                    id="move-input"
                    type="text"
                    bind:this={moveInputElement}
                    bind:value={moveInput}
                    on:keydown={handleKeyDown}
                    placeholder="24/20 13/8"
                    class="move-input"
                    disabled={isCubeDecision || isResignDecision}
                />
            </div>

            <button on:click={validateEditing} class="validate-button" title="Validate (Enter)" aria-label="Validate">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="button-icon">
                    <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                </svg>
            </button>

            <button class="close-button" on:click={cancelEditing} title="Close (Esc)" aria-label="Close">×</button>
        </div>
    </div>
{/if}

<style>
    .edit-panel {
        position: fixed;
        bottom: 32px; /* Above status bar */
        left: 0;
        right: 0;
        background-color: white;
        border-top: 1px solid #ccc;
        box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.1);
        z-index: 100;
    }

    .panel-content {
        display: flex;
        gap: 8px;
        align-items: center;
        padding: 8px 15px;
    }

    .field-group {
        display: flex;
        align-items: center;
        gap: 6px;
    }

    .field-group label {
        color: #333;
        font-size: 14px;
        font-weight: 500;
    }

    .dice-input {
        width: 100px;
        padding: 6px 10px;
        font-size: 14px;
        font-weight: bold;
        text-align: center;
        border: 1px solid #ccc;
        border-radius: 4px;
        outline: none;
        font-family: 'Courier New', monospace;
    }

    .move-input {
        width: 200px;
        padding: 6px 10px;
        font-size: 14px;
        border: 1px solid #ccc;
        border-radius: 4px;
        outline: none;
        font-family: 'Courier New', monospace;
    }

    .move-input:disabled {
        background-color: #f0f0f0;
        color: #999;
        cursor: not-allowed;
    }

    .dice-input:focus,
    .move-input:focus {
        border-color: #999;
        box-shadow: none;
    }

    .validate-button {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 6px;
        background-color: #888;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .validate-button:hover {
        background-color: #777;
    }

    .close-button {
        background: none;
        border: none;
        font-size: 20px;
        font-weight: bold;
        color: #666;
        cursor: pointer;
        padding: 0 6px;
        line-height: 1;
        margin-left: 8px;
    }

    .close-button:hover {
        color: #000;
    }

    .button-icon {
        width: 16px;
        height: 16px;
    }
</style>
