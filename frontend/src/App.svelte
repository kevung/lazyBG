<script>

    // svelte functions
    import { onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import { get } from 'svelte/store';

    // import backend functions
    import {
        OpenTranscriptionDialog,
        SaveTranscriptionDialog,
        ReadTextFile,
        WriteTextFile,
        OpenPositionDialog,
        DeleteFile,

        ShowAlert

    } from '../wailsjs/go/main/App.js';

    import { WindowSetTitle, Quit, ClipboardGetText, WindowGetSize } from '../wailsjs/runtime/runtime.js';

    import { SaveWindowDimensions } from '../wailsjs/go/main/Config.js';

    // import stores (databaseStore removed)

    import {
        positionStore,
        positionsStore // Import positionsStore
    } from './stores/positionStore';

    import {
        analysisStore,
    } from './stores/analysisStore';

    import {
        currentPositionIndexStore,
        statusBarTextStore,
        statusBarModeStore,
        showCommandStore,
        showHelpStore,
        showGoToPositionModalStore,
        showMetadataModalStore,
        showMetadataPanelStore,
        isAnyModalOpenStore,
        previousModeStore,
        showCandidateMovesStore,
        showMovesTableStore
    } from './stores/uiStore';

    import { metaStore } from './stores/metaStore'; // Import metaStore

    // import transcription stores and utils
    import {
        LAZYBG_VERSION,
        transcriptionStore,
        selectedMoveStore,
        transcriptionFilePathStore,
        positionsCacheStore,
        updateMetadata,
        addGame,
        addMove,
        clearTranscription,
        swapPlayers
    } from './stores/transcriptionStore.js';
    import { parseMatchFile } from './utils/matchParser.js';
    import { checkCompatibility, migrateTranscription, validateTranscription } from './utils/versionUtils.js';
    import { 
        createInitialPosition, 
        calculatePositionAtMove,
        validatePosition 
    } from './utils/positionCalculator.js';

    // import components
    import Toolbar from './components/Toolbar.svelte';
    import Board from './components/Board.svelte';
    import CommandLine from './components/CommandLine.svelte';
    import StatusBar from './components/StatusBar.svelte';
    import HelpModal from './components/HelpModal.svelte';
    import GoToPositionModal from './components/GoToPositionModal.svelte';
    import MetadataModal from './components/MetadataModal.svelte';
    import MetadataPanel from './components/MetadataPanel.svelte';
    import MovesTable from './components/MovesTable.svelte';
    import CandidateMovesPanel from './components/CandidateMovesPanel.svelte';

    // Debug logging
    console.log('App.svelte: Script starting to execute');

    // Visibility variables
    let showCommand = false;
    let showHelp = false;
    let showGoToPositionModal = false;
    let applicationVersion = '';
    let showMetadataModal = false;
    let showMetadataPanel = false;
    let mode = 'NORMAL';
    let isAnyModalOpen = false;
    let showCandidateMoves = false;
    let showMovesTable = true;
    
    console.log('App.svelte: Variables initialized');
    
    // Reference for various elements.
    let mainArea;
    let commandInput;
    let currentPositionIndex = 0;
    let positions = [];

    // Declare functions before subscriptions that use them
    export function setStatusBarMessage(message) {
        statusBarTextStore.set(message);
    }

    // Subscribe to the metaStore
    metaStore.subscribe(value => {
        applicationVersion = value.applicationVersion;
    });

    // Subscribe to the derived store
    isAnyModalOpenStore.subscribe(value => {
        isAnyModalOpen = value;
    });

    // Subscribe to new panel stores
    showCandidateMovesStore.subscribe(value => {
        showCandidateMoves = value;
    });

    showMovesTableStore.subscribe(value => {
        showMovesTable = value;
    });

    // Subscribe to the stores
    positionsStore.subscribe(value => {
        positions = Array.isArray(value) ? value : [];
        if (positions.length === 0) {
            positionStore.set({
                id: 0, // Add a default id
                board: {
                    points: Array(26).fill({ checkers: 0, color: -1 }),
                    bearoff: [15, 15],
                },
                cube: {
                    owner: -1,
                    value: 0,
                },
                dice: [3, 1],
                score: [-1, -1],
                player_on_roll: 0,
                decision_type: 0,
                has_jacoby: 0,
                has_beaver: 0,
            });
            analysisStore.set({
                positionId: null,
                xgid: '',
                player1: '',
                player2: '',
                analysisType: '',
                analysisEngineVersion: '',
                checkerAnalysis: { moves: [] },
                doublingCubeAnalysis: {
                    analysisDepth: '',
                    playerWinChances: 0,
                    playerGammonChances: 0,
                    playerBackgammonChances: 0,
                    opponentWinChances: 0,
                    opponentGammonChances: 0,
                    opponentBackgammonChances: 0,
                    cubelessNoDoubleEquity: 0,
                    cubelessDoubleEquity: 0,
                    cubefulNoDoubleEquity: 0,
                    cubefulNoDoubleError: 0,
                    cubefulDoubleTakeEquity: 0,
                    cubefulDoubleTakeError: 0,
                    cubefulDoublePassEquity: 0,
                    cubefulDoublePassError: 0,
                    bestCubeAction: '',
                    wrongPassPercentage: 0,
                    wrongTakePercentage: 0
                },
                creationDate: '',
                lastModifiedDate: ''
            }); // Reset analysisStore when no positions
        }
    });

    currentPositionIndexStore.subscribe(async value => {
        currentPositionIndex = value;
        if (positions.length > 0 && currentPositionIndex >= 0 && currentPositionIndex < positions.length) {
            setStatusBarMessage(''); // Reset status bar message when changing position
        }
    });

    showCommandStore.subscribe(value => {
        showCommand = value;
    });

    showHelpStore.subscribe(value => {
        showHelp = value;
    });

    showGoToPositionModalStore.subscribe(value => {
        showGoToPositionModal = value;
    });

    showMetadataModalStore.subscribe(value => {
        showMetadataModal = value;
    });

    showMetadataPanelStore.subscribe(value => {
        showMetadataPanel = value;
    });

    statusBarModeStore.subscribe(value => {
        mode = value;
    });

    // Auto-save transcription when it changes (debounced)
    let autoSaveTimeout = null;
    transcriptionStore.subscribe(value => {
        // Clear any existing timeout
        if (autoSaveTimeout) {
            clearTimeout(autoSaveTimeout);
        }
        
        // Debounce auto-save by 1 second to avoid too frequent saves
        autoSaveTimeout = setTimeout(() => {
            autoSaveTranscription();
        }, 1000);
    });

    /**
     * Convert position calculator format to Board component format
     * Position calculator: points[1..24], bar, off, opponentBar, opponentOff
     * Board format: board.points[0..25], board.bearoff[0,1]
     */
    function convertPositionToBoard(position) {
        const points = [];
        
        // Initialize points array (index 0 is unused, 1-24 are the board points, 25 is bar)
        for (let i = 0; i <= 25; i++) {
            if (i === 0) {
                // Point 0 is unused
                points.push({ checkers: 0, color: -1 });
            } else if (i === 25) {
                // Point 25 is the bar
                const barCheckers = position.bar;
                const opponentBarCheckers = Math.abs(position.opponentBar);
                
                if (barCheckers > 0) {
                    points.push({ checkers: barCheckers, color: 0 });
                } else if (opponentBarCheckers > 0) {
                    points.push({ checkers: opponentBarCheckers, color: 1 });
                } else {
                    points.push({ checkers: 0, color: -1 });
                }
            } else {
                // Regular board points (1-24)
                const checkersOnPoint = position.points[i];
                
                if (checkersOnPoint > 0) {
                    points.push({ checkers: checkersOnPoint, color: 0 });
                } else if (checkersOnPoint < 0) {
                    points.push({ checkers: Math.abs(checkersOnPoint), color: 1 });
                } else {
                    points.push({ checkers: 0, color: -1 });
                }
            }
        }
        
        return {
            id: 0,
            board: {
                points: points,
                bearoff: [position.off, position.opponentOff]
            },
            cube: {
                owner: -1,
                value: 1
            },
            dice: [0, 0],
            score: [0, 0],
            player_on_roll: 0,
            decision_type: 0,
            has_jacoby: 0,
            has_beaver: 0
        };
    }

    // Subscribe to selected move changes to update position
    // Temporarily commented out to debug
    /*
    selectedMoveStore.subscribe(selectedMove => {
        const transcription = get(transcriptionStore);
        const positionsCache = get(positionsCacheStore);
        
        if (transcription && transcription.games && transcription.games.length > 0) {
            const { gameIndex, moveIndex, player } = selectedMove;
            
            // Check cache first
            const cacheKey = `${gameIndex}-${moveIndex}-${player}`;
            if (positionsCache[cacheKey]) {
                positionStore.set(positionsCache[cacheKey]);
            } else {
                // Calculate position
                const game = transcription.games[gameIndex];
                if (game) {
                    const gameNumber = game.gameNumber;
                    const position = calculatePositionAtMove(
                        transcription,
                        gameNumber,
                        moveIndex,
                        player === 1 ? 'player1' : 'player2'
                    );
                    
                    // Convert to board format expected by Board component
                    const boardPosition = convertPositionToBoard(position);
                    positionStore.set(boardPosition);
                    
                    // Cache the calculated position
                    positionsCacheStore.update(cache => {
                        cache[cacheKey] = boardPosition;
                        return cache;
                    });
                    
                    // Validate position
                    const validation = validatePosition(position);
                    if (!validation.valid) {
                        console.warn(`Position validation failed: ${validation.playerCount} vs ${validation.opponentCount} checkers`);
                    }
                }
            }
        }
    });
    */

    //Global shortcuts
    function handleKeyDown(event) {
        event.stopPropagation();

        // Prevent shortcuts if any modal is open
        if ($isAnyModalOpenStore) {
            return;
        }

        if (event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
        } else if(event.ctrlKey && event.code == 'KeyN') {
            newMatch();
        } else if(event.ctrlKey && event.code == 'KeyO') {
            loadMatchFromText();
        } else if (event.ctrlKey && event.code === 'KeyQ') {
            exitApp();
        } else if (!event.ctrlKey && event.key === 'PageUp') {
            event.preventDefault();
            previousGame();
        } else if (!event.ctrlKey && event.key === 'h') {
            previousGame();
        } else if (!event.ctrlKey && event.key === 'ArrowLeft') {
            event.preventDefault();
            previousPosition();
        } else if (!event.ctrlKey && event.key === 'k') {
            previousPosition();
        } else if (!event.ctrlKey && event.key === 'ArrowRight') {
            event.preventDefault();
            nextPosition();
        } else if (!event.ctrlKey && event.key === 'j') {
            nextPosition();
        } else if (!event.ctrlKey && event.key === 'PageDown') {
            event.preventDefault();
            nextGame();
        } else if (!event.ctrlKey && event.key === 'l') {
            nextGame();
        } else if(event.ctrlKey && event.code == 'KeyK') {
            gotoPosition();
        } else if(!event.ctrlKey && event.code === 'Tab') {
            toggleEditMode();
        } else if (!event.ctrlKey && event.code === 'Space') {        
            event.preventDefault();
            showCommandStore.set(true); // Show command line
        } else if (event.ctrlKey && event.code === 'KeyH') {
            toggleHelpModal();
        } else if (!event.ctrlKey && event.key === '?') {
            toggleHelpModal();
        } else if (event.ctrlKey && event.code === 'KeyM') {
            event.preventDefault();
            showMetadataPanelStore.update(v => !v);
        } else if (event.ctrlKey && event.code === 'KeyI') {
            loadMatchFromText();
        } else if (event.ctrlKey && event.code === 'KeyP') {
            event.preventDefault();
            showMovesTableStore.update(v => !v);
        } else if (event.ctrlKey && event.code === 'KeyL') {
            event.preventDefault();
            showCandidateMovesStore.update(v => !v);
        }
    }

    function getFilenameFromPath(filePath) {
        return filePath.split('/').pop();
    }

    async function newMatch() {
        console.log('newMatch');
        try {
            // Show save dialog to let user choose where to save the new transcription
            const filePath = await SaveTranscriptionDialog();
            if (!filePath) {
                console.log('New match cancelled - no file selected');
                return;
            }
            
            // Ensure .lbg extension
            const finalPath = filePath.endsWith('.lbg') ? filePath : filePath + '.lbg';
            
            // Create a new empty transcription with default metadata
            clearTranscription();
            updateMetadata({
                transcriber: 'User',
                variation: 'Backgammon',
                crawford: 'On',
                matchLength: 7
            });
            
            // Get the transcription with initial metadata
            const transcription = get(transcriptionStore);
            
            // Save the initial transcription file
            const jsonContent = JSON.stringify(transcription, null, 2);
            await WriteTextFile(finalPath, jsonContent);
            
            // Store the file path
            transcriptionFilePathStore.set(finalPath);
            
            // Update window title with filename
            const filename = finalPath.split('/').pop();
            WindowSetTitle(`lazyBG - ${filename}`);
            
            setStatusBarMessage(`New match created: ${filename} - Fill in match details below`);
            console.log('New match created and saved at:', finalPath);
            
            // Show metadata panel
            showMetadataPanelStore.set(true);
        } catch (error) {
            console.error('Error creating new match:', error);
            setStatusBarMessage('Error creating new match');
        } finally {
            previousModeStore.set('NORMAL');
            statusBarModeStore.set('NORMAL');
        }
    }

    async function saveTranscription() {
        console.log('saveTranscription');
        try {
            // Get current transcription
            const transcription = get(transcriptionStore);
            if (!transcription) {
                setStatusBarMessage('No transcription to save');
                return;
            }

            // Convert transcription to JSON string
            const jsonContent = JSON.stringify(transcription, null, 2);
            
            // Check if we have an existing file path
            let filePath = get(transcriptionFilePathStore);
            
            // If no existing file path, show save dialog
            if (!filePath) {
                filePath = await SaveTranscriptionDialog();
                if (!filePath) {
                    console.log('Save cancelled');
                    return;
                }
                // Ensure .lbg extension
                filePath = filePath.endsWith('.lbg') ? filePath : filePath + '.lbg';
                transcriptionFilePathStore.set(filePath);
            }
            
            // Save file
            await WriteTextFile(filePath, jsonContent);
            
            // Update window title
            const filename = filePath.split('/').pop();
            WindowSetTitle(`lazyBG - ${filename}`);
            
            setStatusBarMessage(`Transcription saved to ${filename}`);
            console.log('Transcription saved to:', filePath);
        } catch (error) {
            console.error('Error saving transcription:', error);
            setStatusBarMessage('Error saving transcription');
        }
    }

    // Auto-save function when transcription changes
    async function autoSaveTranscription() {
        const filePath = get(transcriptionFilePathStore);
        if (!filePath) {
            return; // Don't auto-save if no file path is set
        }
        
        try {
            const transcription = get(transcriptionStore);
            const jsonContent = JSON.stringify(transcription, null, 2);
            await WriteTextFile(filePath, jsonContent);
            console.log('Auto-saved transcription to:', filePath);
        } catch (error) {
            console.error('Error auto-saving transcription:', error);
        }
    }

    async function loadMatchFromText() {
        console.log('loadMatchFromText');
        try {
            const filePath = await OpenTranscriptionDialog();
            if (!filePath) {
                console.log('No file selected');
                return;
            }
            
            console.log('Selected file:', filePath);
            
            // Check file extension
            if (filePath.endsWith('.txt')) {
                // Load .txt match file using backend
                const content = await ReadTextFile(filePath);
                const transcription = parseMatchFile(content);
                
                // Update the transcription store with the complete transcription
                clearTranscription();
                transcriptionStore.set(transcription);
                
                // Save as .lbg file with all the parsed information
                const lbgPath = filePath.replace('.txt', '.lbg');
                const jsonContent = JSON.stringify(transcription, null, 2);
                await WriteTextFile(lbgPath, jsonContent);
                
                transcriptionFilePathStore.set(lbgPath);
                
                WindowSetTitle(`lazyBG - ${getFilenameFromPath(lbgPath)}`);
                setStatusBarMessage(`Match loaded and saved as ${getFilenameFromPath(lbgPath)}`);
                
            } else if (filePath.endsWith('.lbg')) {
                // Load .lbg JSON file using backend
                const jsonContent = await ReadTextFile(filePath);
                let transcription = JSON.parse(jsonContent);
                
                // Handle version compatibility
                const fileVersion = transcription.version || '1.0.0';
                const compatibility = checkCompatibility(fileVersion, LAZYBG_VERSION);
                
                if (!compatibility.compatible) {
                    setStatusBarMessage(compatibility.message);
                    await ShowAlert(compatibility.message);
                    return;
                }
                
                // Add version if missing (backward compatibility)
                if (!transcription.version) {
                    console.warn('File missing version field, adding v1.0.0');
                    transcription.version = '1.0.0';
                }
                
                // Migrate if necessary
                if (compatibility.needsMigration) {
                    console.log(compatibility.message);
                    transcription = migrateTranscription(transcription, LAZYBG_VERSION);
                }
                
                // Validate structure
                const validation = validateTranscription(transcription);
                if (!validation.valid) {
                    const errorMsg = 'Invalid file structure: ' + validation.errors.join(', ');
                    setStatusBarMessage(errorMsg);
                    await ShowAlert(errorMsg);
                    return;
                }
                
                clearTranscription();
                transcriptionStore.set(transcription);
                transcriptionFilePathStore.set(filePath);
                
                WindowSetTitle(`lazyBG - ${getFilenameFromPath(filePath)}`);
                const versionMsg = compatibility.needsMigration ? 
                    ` (migrated from v${fileVersion})` : '';
                setStatusBarMessage(`Transcription loaded successfully${versionMsg}`);
            }
            
        } catch (error) {
            console.error('Error loading match:', error);
            setStatusBarMessage('Error loading match: ' + error.message);
        } finally {
            previousModeStore.set('NORMAL');
            statusBarModeStore.set('NORMAL');
        }
    }

    function exitApp() {
        Quit();
    }

    function firstPosition() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate to first move of first game in transcription
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            selectedMoveStore.set({ gameIndex: 0, moveIndex: 0, player: 1 });
        }
    }

    function previousGame() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate to first move of previous game
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex } = $selectedMoveStore;
            if (gameIndex > 0) {
                selectedMoveStore.set({ gameIndex: gameIndex - 1, moveIndex: 0, player: 1 });
            }
        }
    }

    function previousPosition() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate in transcription mode
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex, moveIndex, player } = $selectedMoveStore;
            
            if (player === 2) {
                // Move from player 2 to player 1 of same move
                selectedMoveStore.set({ gameIndex, moveIndex, player: 1 });
            } else if (moveIndex > 0) {
                // Move to previous move (player 2)
                const prevMove = $transcriptionStore.games[gameIndex].moves[moveIndex - 1];
                if (prevMove && prevMove.player2Move) {
                    selectedMoveStore.set({ gameIndex, moveIndex: moveIndex - 1, player: 2 });
                } else {
                    selectedMoveStore.set({ gameIndex, moveIndex: moveIndex - 1, player: 1 });
                }
            } else if (gameIndex > 0) {
                // Move to last move of previous game
                const prevGame = $transcriptionStore.games[gameIndex - 1];
                const lastMoveIndex = prevGame.moves.length - 1;
                const lastMove = prevGame.moves[lastMoveIndex];
                const lastPlayer = lastMove && lastMove.player2Move ? 2 : 1;
                selectedMoveStore.set({ gameIndex: gameIndex - 1, moveIndex: lastMoveIndex, player: lastPlayer });
            }
        }
    }

    function nextPosition() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate in transcription mode
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex, moveIndex, player } = $selectedMoveStore;
            const game = $transcriptionStore.games[gameIndex];
            const move = game.moves[moveIndex];
            
            if (player === 1 && move && move.player2Move) {
                // Move from player 1 to player 2 of same move
                selectedMoveStore.set({ gameIndex, moveIndex, player: 2 });
            } else if (moveIndex < game.moves.length - 1) {
                // Move to next move (player 1)
                selectedMoveStore.set({ gameIndex, moveIndex: moveIndex + 1, player: 1 });
            } else if (gameIndex < $transcriptionStore.games.length - 1) {
                // Move to first move of next game
                selectedMoveStore.set({ gameIndex: gameIndex + 1, moveIndex: 0, player: 1 });
            }
        }
    }

    function nextGame() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate to first move of next game
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex } = $selectedMoveStore;
            if (gameIndex < $transcriptionStore.games.length - 1) {
                selectedMoveStore.set({ gameIndex: gameIndex + 1, moveIndex: 0, player: 1 });
            }
        }
    }

    function lastPosition() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate to last move of last game in transcription
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const lastGameIndex = $transcriptionStore.games.length - 1;
            const lastGame = $transcriptionStore.games[lastGameIndex];
            const lastMoveIndex = lastGame.moves.length - 1;
            const lastMove = lastGame.moves[lastMoveIndex];
            const lastPlayer = lastMove && lastMove.player2Move ? 2 : 1;
            selectedMoveStore.set({ gameIndex: lastGameIndex, moveIndex: lastMoveIndex, player: lastPlayer });
        }
    }

    function gotoPosition() {
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        if ($statusBarModeStore !== 'NORMAL') {
            setStatusBarMessage('Cannot go to position in current mode');
            return;
        }
        showGoToPositionModalStore.set(true);
    }

    function toggleEditMode() {
        console.log('toggleEditMode');
        if ($statusBarModeStore !== "EDIT") {
            previousModeStore.set($statusBarModeStore);
            statusBarModeStore.set('EDIT');
        } else {
            previousModeStore.set($statusBarModeStore);
            statusBarModeStore.set('NORMAL');
            // Refresh board and display position associated with currentPositionIndexStore
            const currentIndex = $currentPositionIndexStore;
            currentPositionIndexStore.set(-1); // Temporarily set to a different value to force redraw
            currentPositionIndexStore.set(currentIndex); // Set back to the original value
        }
    }

    function toggleMetadataModal() {
        if (mode === 'EDIT') {
            setStatusBarMessage('Cannot show metadata modal in edit mode');
        } else {
            showMetadataModalStore.set(!showMetadataModal);
        }
    }

    function toggleMovesTable() {
        showMovesTableStore.set(!showMovesTable);
    }

    function toggleCandidateMovesPanel() {
        showCandidateMovesStore.set(!$showCandidateMovesStore);
    }

    onMount(() => {
        console.log('App.svelte: onMount called');
        // @ts-ignore
        console.log('Wails runtime:', runtime);
        window.addEventListener("keydown", handleKeyDown);
        mainArea.addEventListener("wheel", handleWheel); // Add wheel event listener to main container
        window.addEventListener("resize", handleResize);
        console.log('App.svelte: Event listeners added');
    });

    onDestroy(() => {
        window.removeEventListener("keydown", handleKeyDown);
        mainArea.removeEventListener("wheel", handleWheel); // Remove wheel event listener from main container
        window.removeEventListener("resize", handleResize);
    });

    function toggleHelpModal() {
        console.log('Help button clicked!');
        showHelpStore.set(!showHelp);

        // Focus the command input when closing the Help modal
        if (!showHelp) {
            setTimeout(() => {
                if(showCommand) {
                    const commandInput = document.querySelector('.command-input');
                    if (commandInput) {
                        // @ts-ignore
                        commandInput.focus();
                    }
                }
            }, 0);
        }
    }

    // Function to handle mouse wheel events
    function handleWheel(event) {
        if ($isAnyModalOpenStore || $statusBarModeStore === 'EDIT') {
            return; // Prevent changing position when any modal is open or in edit mode
        }

        if (positions && positions.length > 0) {
            if (event.deltaY < 0) {
                previousPosition();
            } else if (event.deltaY > 0) {
                nextPosition();
            }
        }
    }

    async function handleResize() {
        try {
            const size = await WindowGetSize();
            if (size) {
                const { w, h } = size;
                console.log('Window dimensions:', w, h);
                await SaveWindowDimensions(w, h);
            } else {
                console.error('Error: WindowGetSize returned undefined size');
            }
        } catch (err) {
            console.error('Error getting window dimensions:', err);
        }
    }

</script>

<main class="main-container" bind:this={mainArea}>
    <Toolbar
        onNewMatch={newMatch}
        onOpenMatch={loadMatchFromText}
        onExit={exitApp}
        onPreviousGame={previousGame}
        onFirstPosition={firstPosition}
        onPreviousPosition={previousPosition}
        onNextPosition={nextPosition}
        onNextGame={nextGame}
        onLastPosition={lastPosition}
        onGoToPosition={gotoPosition}
        onToggleEditMode={toggleEditMode}
        onToggleCommandMode={() => showCommandStore.set(true)}
        onToggleMovesPanel={() => showMovesTableStore.update(v => !v)}
        onShowCandidateMoves={toggleCandidateMovesPanel}
        onToggleHelp={toggleHelpModal}
        onShowMetadata={toggleMetadataModal}
        onSwapPlayers={swapPlayers}
    />

    <div class="transcription-layout">
        {#if showMovesTable}
        <div class="moves-table-column">
            <MovesTable />
        </div>
        {/if}
        
        <div class="board-column">
            <div class="board-container">
                <Board />
            </div>
            
            <CommandLine
                onToggleHelp={toggleHelpModal}
                bind:this={commandInput}
                onNewMatch={newMatch}
                onOpenMatch={loadMatchFromText}
                exitApp={exitApp}
            />
        </div>

        {#if showCandidateMoves}
        <div class="candidate-moves-column">
            <CandidateMovesPanel />
        </div>
        {/if}
    </div>

    {#if showMetadataPanel}
    <div class="metadata-panel-container">
        <MetadataPanel visible={showMetadataPanel} onClose={() => showMetadataPanelStore.set(false)} />
    </div>
    {/if}

    <GoToPositionModal
        visible={showGoToPositionModal}
        onClose={() => showGoToPositionModalStore.set(false)}
    />

    <MetadataModal
        visible={showMetadataModal}
        onClose={() => showMetadataModalStore.set(false)}
    />

    <HelpModal
        visible={showHelp}
        onClose={toggleHelpModal}
        handleGlobalKeydown={handleKeyDown}
    />

    <StatusBar />

</main>

<style>
    .main-container {
        display: flex;
        flex-direction: column;
        height: 100vh;
        padding: 0;
        box-sizing: border-box;
        position: relative;
        overflow: hidden;
        width: 100vw;
    }

    .transcription-layout {
        flex-grow: 1;
        display: flex;
        flex-direction: row;
        overflow: hidden;
        width: 100%;
        height: 100%;
        padding-bottom: 32px;
    }

    .moves-table-column {
        width: 250px;
        min-width: 200px;
        max-width: 400px;
        border-right: 1px solid #ccc;
        overflow: hidden;
        background-color: #f9f9f9;
        display: flex;
        flex-direction: column;
    }

    .board-column {
        flex-grow: 1;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        box-sizing: border-box;
    }

    .board-container {
        flex-grow: 1;
        display: flex;
        justify-content: center;
        align-items: center;
        overflow: hidden;
        padding: 2px;
        box-sizing: border-box;
        width: 100%;
    }

    .candidate-moves-column {
        width: 350px;
        min-width: 250px;
        max-width: 500px;
        border-left: 1px solid #ccc;
        background-color: #f9f9f9;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .metadata-panel-container {
        position: relative;
        width: 100%;
        margin-bottom: 32px;
        z-index: 5;
    }
</style>
