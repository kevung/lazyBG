<script>
    import { onMount, onDestroy } from 'svelte';
    import { get } from 'svelte/store';
    import { 
        transcriptionStore, 
        selectedMoveStore,
        updateMove,
        invalidatePositionsCacheFrom,
        validateGameInconsistencies
    } from '../stores/transcriptionStore.js';
    import { 
        statusBarModeStore,
        statusBarTextStore
    } from '../stores/uiStore.js';
    import {
        applyMove,
        createInitialPosition,
        addHitMarker,
        removeHitMarker
    } from '../utils/positionCalculator.js';

    // State for editing
    let diceInput = '';
    let moveInput = '';
    let originalDice = '';
    let originalMove = '';
    let isEditing = false;
    
    let diceInputElement;
    let moveInputElement;

    // NOTE: This modal is now deprecated in favor of EditPanel and inline editing in MovesTable
    // Kept for backward compatibility but disabled
    // Subscribe to mode changes - DISABLED
    // $: if ($statusBarModeStore === 'EDIT' && !isEditing) {
    //     startEditing();
    // } else if ($statusBarModeStore !== 'EDIT' && isEditing) {
    //     cancelEditing();
    // }

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

        if (playerMove) {
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

        isEditing = true;

        // Focus on dice input
        setTimeout(() => {
            if (diceInputElement) {
                diceInputElement.focus();
            }
        }, 100);

        statusBarTextStore.set('EDIT MODE: Enter dice (2 digits 1-6), then move notation. Enter=validate, Esc/Tab=cancel');
    }

    function cancelEditing() {
        diceInput = '';
        moveInput = '';
        isEditing = false;
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
            
            // For cube decisions (d/t/p), pass empty string as move
            const diceStr = diceInput.toLowerCase();
            const isCubeDecision = diceStr === 'd' || diceStr === 't' || diceStr === 'p';
            const moveToSave = isCubeDecision ? '' : moveInput;
            
            updateMove(gameIndex, move.moveNumber, player, diceInput, moveToSave, isIllegal, isGala);
            
            // Invalidate position cache from this move onwards
            // This will automatically:
            // 1. Auto-correct hit markers (*) for all subsequent moves
            // 2. Validate game positions to detect inconsistencies
            await invalidatePositionsCacheFrom(gameIndex, moveIndex);
            
            // Force recalculation of all positions after this move
            selectedMoveStore.set({ ...selectedMove });
            
            statusBarTextStore.set(`Move updated at game ${gameIndex + 1}, move ${move.moveNumber} (auto-correcting subsequent moves...)`);
        } else {
            statusBarTextStore.set('No changes made');
        }

        // Exit edit mode
        statusBarModeStore.set('NORMAL');
        isEditing = false;
    }

    function handleKeyDown(event) {
        if (!isEditing) return;

        // Allow normal text editing keys when inside input fields
        const isInInputField = document.activeElement === diceInputElement || 
                              document.activeElement === moveInputElement;
        
        if (isInInputField) {
            // Allow arrow keys, space, and other navigation keys in input fields
            if (event.key === 'ArrowLeft' || event.key === 'ArrowRight' || 
                event.key === 'ArrowUp' || event.key === 'ArrowDown' ||
                event.key === 'Home' || event.key === 'End' ||
                event.key === 'Delete' || event.key === 'Backspace' ||
                event.key === ' ') {
                // Don't prevent default or stop propagation - allow normal editing
                return;
            }
        }

        if (event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
            cancelEditing();
            statusBarModeStore.set('NORMAL');
        } else if (event.key === 'Enter') {
            event.preventDefault();
            event.stopPropagation();
            
            // If dice input is focused and valid, move to move input
            if (document.activeElement === diceInputElement) {
                if (validateDiceInput(diceInput)) {
                    moveInputElement?.focus();
                } else {
                    statusBarTextStore.set('Invalid dice: enter 2 digits between 1 and 6');
                }
            } else if (document.activeElement === moveInputElement) {
                // Validate and save
                if (validateDiceInput(diceInput)) {
                    validateEditing();
                } else {
                    statusBarTextStore.set('Invalid dice: enter 2 digits between 1 and 6');
                }
            }
        } else if (event.key === 'Tab') {
            // Tab cancels editing
            event.preventDefault();
            event.stopPropagation();
            cancelEditing();
            statusBarModeStore.set('NORMAL');
        }
    }

    function handleDiceInput(event) {
        let value = event.target.value;
        
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

    onDestroy(() => {
        if (isEditing) {
            cancelEditing();
        }
    });
</script>

{#if false}
<!-- This modal is now deprecated - using EditPanel and inline editing instead -->
<div class="edit-move-panel">
    <div class="edit-container">
        <div class="edit-header">
            <h3>Edit Move</h3>
            <div class="edit-instructions">
                Enter dice (2 digits 1-6), then move notation
            </div>
        </div>
        
        <div class="edit-fields">
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
                    placeholder="e.g., 54"
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
                    placeholder="e.g., 24/20 13/8"
                    class="move-input"
                />
            </div>
        </div>
        
        <div class="edit-actions">
            <button on:click={validateEditing} class="btn-validate">
                ✓ Validate (Enter)
            </button>
            <button on:click={() => { cancelEditing(); statusBarModeStore.set('NORMAL'); }} class="btn-cancel">
                ✗ Cancel (Esc/Tab)
            </button>
        </div>
    </div>
</div>
{/if}

<svelte:window on:keydown={handleKeyDown} />

<style>
    .edit-move-panel {
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        z-index: 1000;
        background: rgba(30, 30, 30, 0.98);
        border: 2px solid #4a9eff;
        border-radius: 8px;
        padding: 20px;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
        min-width: 400px;
    }

    .edit-container {
        display: flex;
        flex-direction: column;
        gap: 15px;
    }

    .edit-header h3 {
        margin: 0 0 5px 0;
        color: #4a9eff;
        font-size: 18px;
    }

    .edit-instructions {
        color: #aaa;
        font-size: 12px;
    }

    .edit-fields {
        display: flex;
        flex-direction: column;
        gap: 12px;
    }

    .field-group {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .field-group label {
        color: #ccc;
        min-width: 60px;
        font-weight: bold;
    }

    .dice-input {
        width: 80px;
        padding: 8px;
        font-size: 16px;
        font-weight: bold;
        text-align: center;
        background: #2a2a2a;
        border: 1px solid #555;
        border-radius: 4px;
        color: #fff;
        font-family: 'Courier New', monospace;
    }

    .move-input {
        flex: 1;
        padding: 8px;
        font-size: 14px;
        background: #2a2a2a;
        border: 1px solid #555;
        border-radius: 4px;
        color: #fff;
        font-family: 'Courier New', monospace;
    }

    .dice-input:focus,
    .move-input:focus {
        outline: none;
        border-color: #4a9eff;
        background: #353535;
    }

    .edit-actions {
        display: flex;
        gap: 10px;
        justify-content: flex-end;
        margin-top: 10px;
    }

    .btn-validate,
    .btn-cancel {
        padding: 8px 16px;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 14px;
        transition: all 0.2s;
    }

    .btn-validate {
        background: #4a9eff;
        color: white;
    }

    .btn-validate:hover {
        background: #3a8eef;
    }

    .btn-cancel {
        background: #666;
        color: white;
    }

    .btn-cancel:hover {
        background: #555;
    }
</style>
