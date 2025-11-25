<script>
    import { transcriptionStore, updateMetadata } from '../stores/transcriptionStore.js';
    
    export let visible = false;
    export let onClose;
    
    $: metadata = $transcriptionStore?.metadata || {};
    
    function handleInput(field, value) {
        updateMetadata({ [field]: value });
    }
    
    // Always set crawford to "On" when panel is visible
    $: if (visible && (!metadata.crawford || metadata.crawford !== 'On')) {
        handleInput('crawford', 'On');
    }
</script>

{#if visible}
<div class="metadata-panel">
    <button class="close-button" on:click={onClose} title="Close (Ctrl+M)">×</button>
    
    <div class="metadata-grid">
        <div class="field-group">
            <label for="player1">Player 1*:</label>
            <input 
                id="player1" 
                type="text" 
                value={metadata.player1 || ''} 
                on:input={(e) => handleInput('player1', e.target.value)}
                placeholder="Enter player 1 name"
            />
        </div>
        
        <div class="field-group">
            <label for="player2">Player 2*:</label>
            <input 
                id="player2" 
                type="text" 
                value={metadata.player2 || ''} 
                on:input={(e) => handleInput('player2', e.target.value)}
                placeholder="Enter player 2 name"
            />
        </div>
        
        <div class="field-group">
            <label for="matchLength">Match Length*:</label>
            <input 
                id="matchLength" 
                type="number" 
                min="1" 
                max="99" 
                value={metadata.matchLength || 7} 
                on:input={(e) => handleInput('matchLength', parseInt(e.target.value) || 7)}
            />
        </div>
        
        <div class="field-group">
            <label for="site">Site:</label>
            <input 
                id="site" 
                type="text" 
                value={metadata.site || ''} 
                on:input={(e) => handleInput('site', e.target.value)}
                placeholder="Location"
            />
        </div>
        
        <div class="field-group">
            <label for="event">Event:</label>
            <input 
                id="event" 
                type="text" 
                value={metadata.event || ''} 
                on:input={(e) => handleInput('event', e.target.value)}
                placeholder="Tournament name"
            />
        </div>
        
        <div class="field-group">
            <label for="round">Round:</label>
            <input 
                id="round" 
                type="text" 
                value={metadata.round || ''} 
                on:input={(e) => handleInput('round', e.target.value)}
                placeholder="Round number"
            />
        </div>
        
        <div class="field-group">
            <label for="eventDate">Date:</label>
            <input 
                id="eventDate" 
                type="date" 
                value={metadata.eventDate || ''} 
                on:input={(e) => handleInput('eventDate', e.target.value)}
            />
        </div>
        
        <div class="field-group">
            <label for="transcriber">Transcriber:</label>
            <input 
                id="transcriber" 
                type="text" 
                value={metadata.transcriber || ''} 
                on:input={(e) => handleInput('transcriber', e.target.value)}
                placeholder="Your name"
            />
        </div>
    </div>
</div>
{/if}

<style>
    .metadata-panel {
        position: relative;
        background-color: #f5f5f5;
        border-top: 2px solid #ddd;
        padding: 10px 20px;
        height: 150px;
        overflow-y: auto;
    }
    
    .close-button {
        position: absolute;
        top: 8px;
        right: 10px;
        background: none;
        border: none;
        font-size: 24px;
        color: #888;
        cursor: pointer;
        padding: 0;
        width: 24px;
        height: 24px;
        line-height: 24px;
        text-align: center;
    }
    
    .close-button:hover {
        color: #333;
    }
    
    .metadata-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
        gap: 12px;
    }
    
    .field-group {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }
    
    label {
        font-size: 12px;
        font-weight: 600;
        color: #555;
    }
    
    input {
        padding: 6px 8px;
        border: 1px solid #ccc;
        border-radius: 3px;
        font-size: 13px;
        font-family: inherit;
    }
    
    input:focus {
        outline: none;
        border-color: #4a90e2;
        box-shadow: 0 0 0 2px rgba(74, 144, 226, 0.1);
    }
    
    input::placeholder {
        color: #999;
    }
</style>
