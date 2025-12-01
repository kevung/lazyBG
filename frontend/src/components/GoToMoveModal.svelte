<script>
    import { onMount } from 'svelte';
    import { selectedMoveStore, transcriptionStore } from '../stores/transcriptionStore';
    import { statusBarModeStore } from '../stores/uiStore';

    export let visible = false;
    export let onClose;

    let moveNumber = 1;
    let inputField;
    let maxMoveNumber = 1;
    let currentGameIndex = 0;

    // Calculate current move number and max based on current game
    $: if ($transcriptionStore && $selectedMoveStore) {
        currentGameIndex = $selectedMoveStore.gameIndex;
        const game = $transcriptionStore.games[currentGameIndex];
        
        if (game && game.moves && game.moves.length > 0) {
            // Max move number is the last move's moveNumber
            const lastMove = game.moves[game.moves.length - 1];
            maxMoveNumber = lastMove.moveNumber;
            
            // Current move number is from the selected move
            const { moveIndex, player } = $selectedMoveStore;
            const currentMove = game.moves[moveIndex];
            if (currentMove) {
                moveNumber = currentMove.moveNumber;
            } else {
                moveNumber = 1;
            }
        } else {
            maxMoveNumber = 1;
            moveNumber = 1;
        }
    }

    function handleGoToMove() {
        let targetMoveNumber = moveNumber;
        
        // Clamp to valid range
        if (targetMoveNumber < 1) {
            targetMoveNumber = 1;
        } else if (targetMoveNumber > maxMoveNumber) {
            targetMoveNumber = maxMoveNumber;
        }
        
        // Find the move with the target moveNumber
        const game = $transcriptionStore.games[currentGameIndex];
        if (game && game.moves && game.moves.length > 0) {
            // Find the move index that matches the moveNumber
            const moveIndex = game.moves.findIndex(m => m.moveNumber === targetMoveNumber);
            
            if (moveIndex !== -1) {
                // Default to player 1
                selectedMoveStore.set({ gameIndex: currentGameIndex, moveIndex, player: 1 });
            }
        }
        
        onClose(); // Close the modal after going to the move
    }

    function handleKeyDown(event) {
        if (event.key === 'Enter') {
            handleGoToMove();
        } else if (event.key === 'Escape') {
            onClose();
        }
    }

    onMount(() => {
        if (visible && inputField) {
            // moveNumber is already calculated in reactive statement
            inputField.focus();
            inputField.select(); // Select the text to allow direct replacement
        }
    });

    $: if (visible && inputField) {
        inputField.focus();
        inputField.select(); // Select the text to allow direct replacement
    }

    $: if (visible && $statusBarModeStore !== 'NORMAL') {
        onClose(); // Close the modal if not in normal mode
    }
</script>

{#if visible}
<div class="modal-overlay" on:click={onClose}>
    <div class="modal-content" on:click|stopPropagation>
        <div class="close-button" on:click={onClose}>×</div>
        <h2>Go To Move</h2>
        <input type="number" bind:value={moveNumber} min="1" max={maxMoveNumber} placeholder="Enter move number" class="input-field" bind:this={inputField} on:keydown={handleKeyDown} />
        <div class="modal-buttons">
            <button class="primary-button" on:click={handleGoToMove}>Go</button>
            <button class="secondary-button" on:click={onClose}>Cancel</button>
        </div>
    </div>
</div>
{/if}

<style>
    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.5);
        display: flex;
        justify-content: center;
        align-items: center;
        z-index: 1000;
    }

    .modal-content {
        background-color: white;
        padding: 1rem;
        border-radius: 4px;
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
        z-index: 1000;
        width: 90%;
        max-width: 300px; /* Decrease the max-width */
        max-height: 80vh; /* Limit the height of the modal */
        overflow-y: auto; /* Add vertical scrollbar if content exceeds max height */
        padding-left: 1rem; /* Add left padding to make it symmetric */
        position: relative;
        display: flex;
        flex-direction: column;
        text-align: center;
    }

    .close-button {
        position: absolute;
        top: 8px;
        right: 8px;
        font-size: 1.5rem;
        font-weight: bold;
        color: #666;
        cursor: pointer;
        z-index: 10;
        transition: background-color 0.3s ease, opacity 0.3s ease;
    }

    .input-field {
        width: 80%; /* Adjust the width */
        padding: 8px;
        margin: 8px auto; /* Center the input field */
        border: 1px solid #ccc;
        border-radius: 4px;
        box-sizing: border-box;
        font-size: 18px; /* Set font size */
    }

    .input-field:focus {
        outline: none;
        border-color: #6c757d; /* Sober grey color */
        box-shadow: 0 0 5px rgba(108, 117, 125, 0.5); /* Slight shadow for focus */
    }

    .modal-buttons {
        margin-top: 10px;
        display: flex;
        justify-content: center;
        gap: 10px; /* Add space between buttons */
    }

    .modal-buttons button {
        padding: 8px 14px; /* Slightly increase padding */
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 15px; /* Slightly increase font size */
    }

    .primary-button {
        background-color: #6c757d; /* Sober grey color */
        color: white;
    }

    .secondary-button {
        background-color: #ccc;
    }

    .primary-button:hover {
        background-color: #5a6268; /* Slightly darker grey on hover */
    }

    .secondary-button:hover {
        background-color: #999;
    }
</style>