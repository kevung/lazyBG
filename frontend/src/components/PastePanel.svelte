<script>
    import { onMount, onDestroy } from 'svelte';
    import { slide } from 'svelte/transition';
    import { get } from 'svelte/store';
    import { 
        transcriptionStore, 
        selectedMoveStore,
        insertDecisionBefore,
        insertDecisionAfter,
        invalidatePositionsCacheFrom
    } from '../stores/transcriptionStore.js';
    import { 
        statusBarTextStore
    } from '../stores/uiStore.js';
    import { clipboardStore } from '../stores/clipboardStore.js';
    import { undoRedoStore } from '../stores/undoRedoStore.js';

    export let visible = false;
    export let onClose;

    // State for pasting
    let pastePosition = 'after'; // 'before' or 'after'
    
    let panelEl;
    let hasAutoFocused = false;

    // Initialize when panel becomes visible
    $: if (visible && !hasAutoFocused) {
        startPasting();
    } else if (!visible) {
        hasAutoFocused = false;
    }

    function startPasting() {
        pastePosition = 'after'; // Reset to default
        hasAutoFocused = true;
        const clipboard = get(clipboardStore);
        const count = clipboard.decisions?.length || 0;
        statusBarTextStore.set(`PASTE MODE: Choose position to paste ${count} decision(s), Enter=paste, Esc=cancel`);
    }

    function cancelPasting() {
        onClose();
    }

    async function validatePasting() {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        const clipboard = get(clipboardStore);
        
        if (!transcription || !selectedMove || !clipboard.decisions || clipboard.decisions.length === 0) {
            return;
        }

        const { gameIndex, moveIndex, player } = selectedMove;
        const game = transcription.games[gameIndex];
        
        if (!game) {
            return;
        }

        // Save state before pasting
        undoRedoStore.saveSnapshot();

        // Determine where to start inserting
        let insertAtMoveIndex = moveIndex;
        let insertAtPlayer = player;
        
        if (pastePosition === 'after') {
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
            
            // Get the move data from the decision
            const sourceGame = transcription.games[decisionData.gameIndex];
            if (!sourceGame) continue;
            
            const sourceMove = sourceGame.moves[decisionData.moveIndex];
            if (!sourceMove) continue;
            
            const sourceMoveData = decisionData.player === 1 ? sourceMove.player1Move : sourceMove.player2Move;
            
            if (pastePosition === 'before') {
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

        // If it was a cut operation, clear the clipboard
        if (clipboard.isCut) {
            clipboardStore.clear();
        }

        // Invalidate cache and force recalculation
        await invalidatePositionsCacheFrom(gameIndex, insertAtMoveIndex);
        
        // Select the first pasted decision
        selectedMoveStore.set({ gameIndex, moveIndex: insertAtMoveIndex, player: insertAtPlayer });
        
        const posText = pastePosition === 'before' ? 'before' : 'after';
        const count = clipboard.decisions.length;
        statusBarTextStore.set(`${count} decision(s) pasted ${posText} at game ${gameIndex + 1}, move ${insertAtMoveIndex + 1}`);

        // Close panel
        cancelPasting();
    }

    function handleKeyDown(event) {
        if (!visible) return;

        event.stopPropagation();

        if (event.key === 'Escape') {
            event.preventDefault();
            cancelPasting();
        } else if (event.key === 'Enter') {
            event.preventDefault();
            validatePasting();
        }
    }

    onMount(() => {
    });

    onDestroy(() => {
    });
</script>

{#if visible}
    <div class="paste-panel" transition:slide={{ duration: 50 }} bind:this={panelEl}>
        <div class="field-group">
            <label class="radio-label">
                <input
                    type="radio"
                    bind:group={pastePosition}
                    value="after"
                    name="position"
                />
                <span>Paste After</span>
            </label>
            <label class="radio-label">
                <input
                    type="radio"
                    bind:group={pastePosition}
                    value="before"
                    name="position"
                />
                <span>Paste Before</span>
            </label>
        </div>

        <button on:click={validatePasting} class="validate-button" title="Paste (Enter)" aria-label="Paste">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="button-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
            </svg>
        </button>
    </div>
{/if}

<svelte:window on:keydown={handleKeyDown} />

<style>
    .paste-panel {
        position: fixed;
        bottom: 25px;
        left: 50%;
        transform: translateX(-50%);
        z-index: 100;
        background: #f0f0f0;
        border: 1px solid #ccc;
        border-radius: 3px;
        padding: 8px 12px;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .field-group {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .radio-label {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 13px;
        cursor: pointer;
        color: #333;
    }

    .radio-label input[type="radio"] {
        cursor: pointer;
    }

    .radio-label span {
        user-select: none;
    }

    .validate-button {
        padding: 4px 8px;
        background: #888;
        border: 1px solid #666;
        border-radius: 3px;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.15s;
    }

    .validate-button:hover {
        background: #666;
    }

    .button-icon {
        width: 18px;
        height: 18px;
        color: white;
    }
</style>
