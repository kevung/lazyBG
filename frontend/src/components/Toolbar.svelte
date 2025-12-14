<script>
    export let onNewMatch;
    export let onOpenMatch;
    export let onExportMatch;
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
    export let onShowMoveSearch;
    export let onShowInsertPanel;
    export let onShowInsertGamePanel;
    export let onDeleteGame;
    export let onDeleteDecision;
    export let onUndo;
    export let onRedo;
    export let onCopyDecisions;
    export let onCutDecisions;
    export let onPasteDecisions;

    import { statusBarModeStore, statusBarTextStore, showInitialPositionStore, showMovesTableStore, showCandidateMovesStore, showCommandStore, showMoveSearchModalStore } from '../stores/uiStore';
    import { transcriptionStore } from '../stores/transcriptionStore';
    import { undoRedoStore } from '../stores/undoRedoStore';
    import { clipboardStore } from '../stores/clipboardStore';
    
    let statusBarMode;
    let hasTranscription = false;
    let showInitialPosition = false;
    let showMovesTable = false;
    let showCandidateMoves = false;
    let showCommand = false;
    let showMoveSearchModal = false;
    let canUndo = false;
    let canRedo = false;
    let hasClipboard = false;
    
    statusBarModeStore.subscribe(value => {
        statusBarMode = value;
    });
    transcriptionStore.subscribe(value => {
        hasTranscription = value && value.games && value.games.length > 0;
    });
    showInitialPositionStore.subscribe(value => {
        showInitialPosition = value;
    });
    showMovesTableStore.subscribe(value => {
        showMovesTable = value;
    });
    showCandidateMovesStore.subscribe(value => {
        showCandidateMoves = value;
    });
    showCommandStore.subscribe(value => {
        showCommand = value;
    });
    showMoveSearchModalStore.subscribe(value => {
        showMoveSearchModal = value;
    });
    undoRedoStore.subscribe(() => {
        canUndo = undoRedoStore.canUndo();
        canRedo = undoRedoStore.canRedo();
    });
    clipboardStore.subscribe(value => {
        hasClipboard = value.decisions && value.decisions.length > 0;
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

    <button on:click|stopPropagation={onExportMatch} aria-label="Export to Match Transcription Text" title="Export to Match Transcription Text (Ctrl-S)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
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

    <button on:click|stopPropagation={handlePreviousGame} aria-label="Previous Game" title="Previous Game (PageUp, h)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m18.75 4.5-7.5 7.5 7.5 7.5m-6-15L5.25 12l7.5 7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={handlePreviousPosition} aria-label="Previous Decision" title="Previous Decision (Left, k)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={handleNextPosition} aria-label="Next Decision" title="Next Decision (Right, j)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={handleNextGame} aria-label="Next Game" title="Next Game (PageDown, l)" disabled={statusBarMode === 'EDIT' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m5.25 4.5 7.5 7.5-7.5 7.5m6-15 7.5 7.5-7.5 7.5" />
        </svg>
    </button>  

    <button on:click|stopPropagation={onGoToMove} aria-label="Go To Move" title="Go To Move (Ctrl-K)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 6.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 12.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 18.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5Z" />
        </svg>
    </button>

    <button on:click|stopPropagation={onTogglePositionDisplay} aria-label="Toggle Position Display" title="Toggle Initial/Final Position Display (t)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription} class:active={!showInitialPosition}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
            <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
        </svg>
    </button>

    <div class="separator"></div>

    <button on:click|stopPropagation={onShowInsertGamePanel} aria-label="Insert Game To New Game" title="Insert Game To New Game (n, N)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m3.75 9v6m3-3H9m1.5-12H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
        </svg>
    </button>

    <button on:click|stopPropagation={onDeleteGame} aria-label="Delete Game" title="Delete Game (D)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m6.75 12H9m1.5-12H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
        </svg>
    </button>

    <button on:click|stopPropagation={onShowInsertPanel} aria-label="Insert Decision To New Decision" title="Insert Decision To New Decision (o, O)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
        </svg>
    </button>

    <button on:click|stopPropagation={onDeleteDecision} aria-label="Delete Decision" title="Delete Decision (Del)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14" />
        </svg>
    </button>

    <button on:click|stopPropagation={onCopyDecisions} aria-label="Copy Decisions" title="Copy Decisions (Ctrl-C, y)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 7.5V6.108c0-1.135.845-2.098 1.976-2.192.373-.03.748-.057 1.123-.08M15.75 18H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08M15.75 18.75v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5A3.375 3.375 0 0 0 6.375 7.5H5.25m11.9-3.664A2.251 2.251 0 0 0 15 2.25h-1.5a2.251 2.251 0 0 0-2.15 1.586m5.8 0c.065.21.1.433.1.664v.75h-6V4.5c0-.231.035-.454.1-.664M6.75 7.5H4.875c-.621 0-1.125.504-1.125 1.125v12c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V16.5a9 9 0 0 0-9-9Z" />
        </svg>
    </button>

    <button on:click|stopPropagation={onCutDecisions} aria-label="Cut Decisions" title="Cut Decisions (Ctrl-X, dd)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m7.848 8.25 1.536.887M7.848 8.25a3 3 0 1 1-5.196-3 3 3 0 0 1 5.196 3Zm1.536.887a2.165 2.165 0 0 1 1.083 1.839c.005.351.054.695.14 1.024M9.384 9.137l2.077 1.199M7.848 15.75l1.536-.887m-1.536.887a3 3 0 1 1-5.196 3 3 3 0 0 1 5.196-3Zm1.536-.887a2.165 2.165 0 0 0 1.083-1.838c.005-.352.054-.695.14-1.025m-1.223 2.863 2.077-1.199m0-3.328a4.323 4.323 0 0 1 2.068-1.379l5.325-1.628a4.5 4.5 0 0 1 2.48-.044l.803.215-7.794 4.5m-2.882-1.664A4.33 4.33 0 0 0 10.607 12m3.736 0 7.794 4.5-.802.215a4.5 4.5 0 0 1-2.48-.043l-5.326-1.629a4.324 4.324 0 0 1-2.068-1.379M14.343 12l-2.882 1.664" />
        </svg>
    </button>

    <button on:click|stopPropagation={onPasteDecisions} aria-label="Paste Decisions" title="Paste Decisions (Ctrl-V, p, P)" disabled={statusBarMode !== 'NORMAL' || !hasTranscription || !hasClipboard}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
        </svg>
    </button>

    <button on:click|stopPropagation={onUndo} aria-label="Undo" title="Undo (Ctrl-Z, u)" disabled={!canUndo}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 15 3 9m0 0 6-6M3 9h12a6 6 0 0 1 0 12h-3" />
        </svg>
    </button>

    <button on:click|stopPropagation={onRedo} aria-label="Redo" title="Redo (Ctrl-Y, Ctrl-R)" disabled={!canRedo}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m15 15 6-6m0 0-6-6m6 6H9a6 6 0 0 0 0 12h3" />
        </svg>
    </button>

    <div class="separator"></div>

    <button on:click|stopPropagation={onToggleEditMode} aria-label="Edit Mode" title="Edit Mode (Tab)" disabled={!hasTranscription} class:active={statusBarMode === 'EDIT'}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L6.832 19.82a4.5 4.5 0 0 1-1.897 1.13l-2.685.8.8-2.685a4.5 4.5 0 0 1 1.13-1.897L16.863 4.487Zm0 0L19.5 7.125" />
        </svg>
    </button>

    <button on:click|stopPropagation={onToggleCommandMode} aria-label="Command Mode" title="Command Mode (Space)" class:active={showCommand}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m6.75 7.5 3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0 0 21 18V6a2.25 2.25 0 0 0-2.25-2.25H5.25A2.25 2.25 0 0 0 3 6v12a2.25 2.25 0 0 0 2.25 2.25Z" />
        </svg>
    </button>

    <div class="separator"></div>
    
    <button on:click|stopPropagation={onToggleMovesPanel} aria-label="Toggle Moves Panel" title="Toggle Moves Panel (Ctrl-P)" disabled={statusBarMode === 'EDIT' || !hasTranscription} class:active={showMovesTable}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.375 19.5h17.25m-17.25 0a1.125 1.125 0 0 1-1.125-1.125M3.375 19.5h7.5c.621 0 1.125-.504 1.125-1.125m-9.75 0V5.625m0 12.75v-1.5c0-.621.504-1.125 1.125-1.125m18.375 2.625V5.625m0 12.75c0 .621-.504 1.125-1.125 1.125m1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125m0 3.75h-7.5A1.125 1.125 0 0 1 12 18.375m9.75-12.75c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125m19.5 0v1.5c0 .621-.504 1.125-1.125 1.125M2.25 5.625v1.5c0 .621.504 1.125 1.125 1.125m0 0h17.25m-17.25 0h7.5c.621 0 1.125.504 1.125 1.125M3.375 8.25c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125m17.25-3.75h-7.5c-.621 0-1.125.504-1.125 1.125m8.625-1.125c.621 0 1.125.504 1.125 1.125v1.5c0 .621-.504 1.125-1.125 1.125m-17.25 0h7.5m-7.5 0c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125M12 10.875v-1.5m0 1.5c0 .621-.504 1.125-1.125 1.125M12 10.875c0 .621.504 1.125 1.125 1.125m-2.25 0c.621 0 1.125.504 1.125 1.125M13.125 12h7.5m-7.5 0c-.621 0-1.125.504-1.125 1.125M20.625 12c.621 0 1.125.504 1.125 1.125v1.5c0 .621-.504 1.125-1.125 1.125m-17.25 0h7.5M12 14.625v-1.5m0 1.5c0 .621-.504 1.125-1.125 1.125M12 14.625c0 .621.504 1.125 1.125 1.125m-2.25 0c.621 0 1.125.504 1.125 1.125m0 1.5v-1.5m0 0c0-.621.504-1.125 1.125-1.125m0 0h7.5" />
        </svg>
    </button>

    <button on:click|stopPropagation={onShowCandidateMoves} aria-label="Show Candidate Moves" title="Show Candidate Moves (Ctrl-L)" disabled={statusBarMode === 'EDIT' || !hasTranscription} class:active={showCandidateMoves}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.242 5.992h12m-12 6.003H20.24m-12 5.999h12M4.117 7.495v-3.75H2.99m1.125 3.75H2.99m1.125 0H5.24m-1.92 2.577a1.125 1.125 0 1 1 1.591 1.59l-1.83 1.83h2.16M2.99 15.745h1.125a1.125 1.125 0 0 1 0 2.25H3.74m0-.002h.375a1.125 1.125 0 0 1 0 2.25H2.99" />
        </svg>
    </button>

    <button on:click|stopPropagation={onShowMoveSearch} aria-label="Search Moves" title="Search Moves (Ctrl-F)" disabled={statusBarMode === 'EDIT' || !hasTranscription} class:active={showMoveSearchModal}>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
            <path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
        </svg>
    </button>

    <div class="separator"></div>

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

