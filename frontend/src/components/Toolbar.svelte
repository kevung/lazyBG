<script>
    export let onNewMatch;
    export let onOpenMatch;
    export let onExit;
    export let onPreviousGame;
    export let onFirstPosition;
    export let onPreviousPosition;
    export let onNextPosition;
    export let onNextGame;
    export let onLastPosition;
    export let onGoToMove;
    export let onTogglePositionDisplay;
    export let onToggleEditMode;
    export let onToggleCommandMode;
    export let onShowCandidateMoves;
    export let onToggleHelp;
    export let onShowMetadata;
    // export let onSwapPlayers; // Temporarily disabled
    export let onToggleMovesPanel;

    import { statusBarModeStore, statusBarTextStore, showInitialPositionStore } from '../stores/uiStore';
    import { transcriptionStore } from '../stores/transcriptionStore';
    
    let statusBarMode;
    let hasTranscription = false;
    let showInitialPosition = false;
    
    statusBarModeStore.subscribe(value => {
        statusBarMode = value;
    });
    transcriptionStore.subscribe(value => {
        hasTranscription = value && value.games && value.games.length > 0;
    });
    showInitialPositionStore.subscribe(value => {
        showInitialPosition = value;
    });

    function setStatusBarMessage(message) {
        statusBarTextStore.set(message);
    }

    function handlePreviousGame() {
        if (statusBarMode === 'EDIT') {
            console.error('Cannot browse positions in edit mode');
            setStatusBarMessage('Cannot browse positions in edit mode');
        } else {
            onPreviousGame();
        }
    }

    function handleFirstPosition() {
        if (statusBarMode === 'EDIT') {
            console.error('Cannot browse positions in edit mode');
            setStatusBarMessage('Cannot browse positions in edit mode');
        } else {
            onFirstPosition();
        }
    }

    function handlePreviousPosition() {
        if (statusBarMode === 'EDIT') {
            console.error('Cannot browse positions in edit mode');
            setStatusBarMessage('Cannot browse positions in edit mode');
        } else {
            onPreviousPosition();
        }
    }

    function handleNextPosition() {
        if (statusBarMode === 'EDIT') {
            console.error('Cannot browse positions in edit mode');
            setStatusBarMessage('Cannot browse positions in edit mode');
        } else {
            onNextPosition();
        }
    }

    function handleNextGame() {
        if (statusBarMode === 'EDIT') {
            console.error('Cannot browse positions in edit mode');
            setStatusBarMessage('Cannot browse positions in edit mode');
        } else {
            onNextGame();
        }
    }

    function handleLastPosition() {
        if (statusBarMode === 'EDIT') {
            console.error('Cannot browse positions in edit mode');
            setStatusBarMessage('Cannot browse positions in edit mode');
        } else {
            onLastPosition();
        }
    }

    function handleGoToMove() {
        if (statusBarMode !== 'NORMAL') {
            console.error('Cannot go to move in current mode');
            setStatusBarMessage('Cannot go to move in current mode');
        } else {
            onGoToMove();
        }
    }
</script>

<!--// https://heroicons.com/-->
<div class="toolbar">
    <button on:click|stopPropagation={onNewMatch} aria-label="New Transcription" title="New Transcription (Ctrl-N)">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 10.5v6m3-3H9m4.06-7.19-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
        </svg>
    </button>

    <button on:click|stopPropagation={onOpenMatch} aria-label="Open Transcription" title="Open Transcription (Ctrl-O)">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 9.776c.112-.017.227-.026.344-.026h15.812c.117 0 .232.009.344.026m-16.5 0a2.25 2.25 0 0 0-1.883 2.542l.857 6a2.25 2.25 0 0 0 2.227 1.932H19.05a2.25 2.25 0 0 0 2.227-1.932l.857-6a2.25 2.25 0 0 0-1.883-2.542m-16.5 0V6A2.25 2.25 0 0 1 6 3.75h3.879a1.5 1.5 0 0 1 1.06.44l2.122 2.12a1.5 1.5 0 0 0 1.06.44H18A2.25 2.25 0 0 1 20.25 9v.776" />
        </svg>
    </button>

    <button on:click|stopPropagation={onExit} aria-label="Exit" title="Exit lazyBG (Ctrl-Q)">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 9V5.25A2.25 2.25 0 0 1 10.5 3h6a2.25 2.25 0 0 1 2.25 2.25v13.5A2.25 2.25 0 0 1 16.5 21h-6a2.25 2.25 0 0 1-2.25-2.25V15M12 9l3 3m0 0-3 3m3-3H2.25" />
        </svg>
    </button>

    <button on:click|stopPropagation={onShowMetadata} aria-label="Transcription Metadata" title="Transcription Metadata (Ctrl-M)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
        </svg>
    </button>

    <!-- Temporarily disabled: Swap Players functionality
    <button on:click|stopPropagation={onSwapPlayers} aria-label="Swap Players" title="Swap Player 1 and Player 2">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
        </svg>
    </button>
    -->

    <div class="separator"></div>

    <button on:click|stopPropagation={handlePreviousGame} aria-label="Previous Game" title="Previous Game (First Move) (PageUp, h)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m18.75 4.5-7.5 7.5 7.5 7.5m-6-15L5.25 12l7.5 7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={handlePreviousPosition} aria-label="Previous Move" title="Previous Move (Left, k)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={handleNextPosition} aria-label="Next Move" title="Next Move (Right, j)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={handleNextGame} aria-label="Next Game" title="Next Game (First Move) (PageDown, l)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m5.25 4.5 7.5 7.5-7.5 7.5m6-15 7.5 7.5-7.5 7.5" />
        </svg>
    </button>  

    <button on:click|stopPropagation={handleGoToMove} aria-label="Go To Move" title="Go To Move (Ctrl-K)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 12.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 18.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5Z" />
        </svg>
    </button>

    <div class="separator"></div>

    <button on:click|stopPropagation={onTogglePositionDisplay} aria-label="Toggle Position Display" title="Toggle Initial/Final Position Display (p)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription} class:active={showInitialPosition}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
        </svg>
    </button>

    <div class="separator"></div>

    <button on:click|stopPropagation={onToggleEditMode} aria-label="Edit Mode" title="Edit Mode (Tab)" disabled={!hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125" />
        </svg>
    </button>

    <button on:click|stopPropagation={onToggleCommandMode} aria-label="Command Mode" title="Command Mode (Space)">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m6.75 7.5 3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0 0 21 18V6a2.25 2.25 0 0 0-2.25-2.25H5.25A2.25 2.25 0 0 0 3 6v12a2.25 2.25 0 0 0 2.25 2.25Z" />
        </svg>
    </button>

    <div class="separator"></div>
    
    <button on:click|stopPropagation={onToggleMovesPanel} aria-label="Toggle Moves Panel" title="Toggle Moves Panel (Ctrl-P)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.375 19.5h17.25m-17.25 0a1.125 1.125 0 0 1-1.125-1.125M3.375 19.5h7.5c.621 0 1.125-.504 1.125-1.125m-9.75 0V5.625m0 12.75v-1.5c0-.621.504-1.125 1.125-1.125m18.375 2.625V5.625m0 12.75c0 .621-.504 1.125-1.125 1.125m1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125m0 3.75h-7.5A1.125 1.125 0 0 1 12 18.375m9.75-12.75c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125m19.5 0v1.5c0 .621-.504 1.125-1.125 1.125M2.25 5.625v1.5c0 .621.504 1.125 1.125 1.125m0 0h17.25m-17.25 0h7.5c.621 0 1.125.504 1.125 1.125M3.375 8.25c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125m17.25-3.75h-7.5c-.621 0-1.125.504-1.125 1.125m8.625-1.125c.621 0 1.125.504 1.125 1.125v1.5c0 .621-.504 1.125-1.125 1.125m-17.25 0h7.5m-7.5 0c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125M12 10.875v-1.5m0 1.5c0 .621-.504 1.125-1.125 1.125M12 10.875c0 .621.504 1.125 1.125 1.125m-2.25 0c.621 0 1.125.504 1.125 1.125M13.125 12h7.5m-7.5 0c-.621 0-1.125.504-1.125 1.125M20.625 12c.621 0 1.125.504 1.125 1.125v1.5c0 .621-.504 1.125-1.125 1.125m-17.25 0h7.5M12 14.625v-1.5m0 1.5c0 .621-.504 1.125-1.125 1.125M12 14.625c0 .621.504 1.125 1.125 1.125m-2.25 0c.621 0 1.125.504 1.125 1.125m0 1.5v-1.5m0 0c0-.621.504-1.125 1.125-1.125m0 0h7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={onShowCandidateMoves} aria-label="Show Candidate Moves" title="Show Candidate Moves (Ctrl-L)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.242 5.992h12m-12 6.003H20.24m-12 5.999h12M4.117 7.495v-3.75H2.99m1.125 3.75H2.99m1.125 0H5.24m-1.92 2.577a1.125 1.125 0 1 1 1.591 1.59l-1.83 1.83h2.16M2.99 15.745h1.125a1.125 1.125 0 0 1 0 2.25H3.74m0-.002h.375a1.125 1.125 0 0 1 0 2.25H2.99" />
        </svg>
    </button>

    <button on:click|stopPropagation={onToggleHelp} aria-label="Help" title="Help (Ctrl-H, ?)">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 5.25h.008v.008H12v-.008Z" />
        </svg>
    </button>
</div>

<style>
    .toolbar {
        display: flex;
        align-items: center;
        padding: 4px;
        background-color: #f0f0f0;
        border-bottom: 1px solid #ccc;
        height: 22px; /* Ensure toolbar height is sufficient */
        width: 100%; /* Ensure toolbar takes up full available width */
    }

    .toolbar button {
        background: none;
        border: none;
        padding: 4px;
        margin: 0;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .toolbar button:first-child {
        margin-left: 4px; /* Add space to the left of the first button */
    }

    .toolbar button svg {
        width: 20px;
        height: 20px;
        stroke: currentColor; /* Ensure the stroke color matches the current text color */
    }

    .toolbar button:focus {
        outline: none;
    }

    .toolbar button:hover {
        background-color: #e0e0e0;
    }

    .toolbar button:disabled {
        opacity: 0.5; /* Make disabled buttons paler */
        cursor: not-allowed;
    }

    .toolbar button.active {
        background-color: #d0d0d0; /* Highlight active toggle buttons */
    }

    .separator {
        width: 1px;
        margin: 0 8px; /* Add some space between the icon groups */
        height: 80%; /* Ensure separator height matches toolbar height */
        border-left: 1px solid #d3d3d3; /* Use a lighter shade for the separator */
    }

</style>

