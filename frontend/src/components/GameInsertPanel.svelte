<script>
    import { onMount, onDestroy } from 'svelte';
    import { slide } from 'svelte/transition';
    import { get } from 'svelte/store';
    import { 
        transcriptionStore, 
        selectedMoveStore
    } from '../stores/transcriptionStore.js';
    import { 
        statusBarTextStore
    } from '../stores/uiStore.js';

    export let visible = false;
    export let onClose;
    export let onInsertGameBefore;
    export let onInsertGameAfter;

    // State for insertion
    let insertPosition = 'after'; // 'before' or 'after'
    
    let panelEl;

    let hasAutoFocused = false;

    // Initialize when panel becomes visible
    $: if (visible && !hasAutoFocused) {
        startInserting();
    } else if (!visible) {
        hasAutoFocused = false;
    }

    function startInserting() {
        insertPosition = 'after'; // Reset to default
        hasAutoFocused = true;
        statusBarTextStore.set('INSERT GAME MODE: Choose position and press Enter to insert new game, Esc=cancel');
    }

    function cancelInserting() {
        onClose();
    }

    async function validateInserting() {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        
        if (!transcription || !selectedMove) {
            return;
        }

        const { gameIndex } = selectedMove;
        
        // Insert new game
        if (insertPosition === 'before') {
            await onInsertGameBefore(gameIndex);
        } else {
            await onInsertGameAfter(gameIndex);
        }

        const posText = insertPosition === 'before' ? 'before' : 'after';
        statusBarTextStore.set(`New game inserted ${posText} game ${gameIndex + 1}.`);

        // Close panel
        cancelInserting();
    }

    function handleKeyDown(event) {
        if (!visible) return;

        event.stopPropagation();

        if (event.key === 'Escape') {
            event.preventDefault();
            cancelInserting();
        } else if (event.key === 'Enter') {
            event.preventDefault();
            validateInserting();
        }
    }

    onMount(() => {
    });

    onDestroy(() => {
    });
</script>

{#if visible}
    <div class="insert-panel" transition:slide={{ duration: 50 }} bind:this={panelEl}>
        <div class="field-group">
            <label class="radio-label">
                <input
                    type="radio"
                    bind:group={insertPosition}
                    value="after"
                    name="position"
                />
                <span>Insert After</span>
            </label>
            <label class="radio-label">
                <input
                    type="radio"
                    bind:group={insertPosition}
                    value="before"
                    name="position"
                />
                <span>Insert Before</span>
            </label>
        </div>

        <button on:click={validateInserting} class="validate-button" title="Insert (Enter)" aria-label="Insert">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="button-icon">
                <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
            </svg>
        </button>
    </div>
{/if}

<svelte:window on:keydown={handleKeyDown} />

<style>
    .insert-panel {
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
