<script>

    // svelte functions
    import { onMount, onDestroy, tick } from 'svelte';
    import { fade, slide } from 'svelte/transition';
    import { get } from 'svelte/store';

    // import backend functions
    import {
        OpenTranscriptionDialog,
        SaveTranscriptionDialog,
        ExportMatchTextDialog,
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
        showGoToMoveModalStore,
        showMetadataModalStore,
        showMetadataPanelStore,
        isAnyModalOpenStore,
        previousModeStore,
        showCandidateMovesStore,
        showMovesTableStore,
        showInitialPositionStore,
        showMoveSearchModalStore
    } from './stores/uiStore';

    import { metaStore } from './stores/metaStore'; // Import metaStore

    // import move search functions
    import { 
        executeSearch, 
        nextSearchResult, 
        previousSearchResult,
        clearSearch 
    } from './stores/moveSearchStore.js';

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
        swapPlayers,
        migrateCubeActions,
        validateGameInconsistencies,
        dumpTranscriptionState
    } from './stores/transcriptionStore.js';
    import { undoRedoStore } from './stores/undoRedoStore.js';
    import { clipboardStore } from './stores/clipboardStore.js';
    import { parseMatchFile } from './utils/matchParser.js';
    import { exportToMatchText } from './utils/matchExporter.js';
    import { checkCompatibility, migrateTranscription, validateTranscription } from './utils/versionUtils.js';
    import { 
        createInitialPosition, 
        calculatePositionAtMove,
        validatePosition,
        applyMove
    } from './utils/positionCalculator.js';

    // import components
    import Toolbar from './components/Toolbar.svelte';
    import Board from './components/Board.svelte';
    import CommandLine from './components/CommandLine.svelte';
    import StatusBar from './components/StatusBar.svelte';
    import HelpModal from './components/HelpModal.svelte';
    import GoToMoveModal from './components/GoToMoveModal.svelte';
    import MetadataModal from './components/MetadataModal.svelte';
    import MetadataPanel from './components/MetadataPanel.svelte';
    import MovesTable from './components/MovesTable.svelte';
    import CandidateMovesPanel from './components/CandidateMovesPanel.svelte';
    import EditMovePanel from './components/EditMovePanel.svelte';
    import EditPanel from './components/EditPanel.svelte';
    import MoveSearchPanel from './components/MoveSearchPanel.svelte';
    import MoveInsertPanel from './components/MoveInsertPanel.svelte';
    import GameInsertPanel from './components/GameInsertPanel.svelte';
    import PastePanel from './components/PastePanel.svelte';

    // Debug logging
    console.log('App.svelte: Script starting to execute');

    // Visibility variables
    let showCommand = false;
    let showHelp = false;
    let showGoToMoveModal = false;
    let showMoveSearchModal = false;
    let applicationVersion = '';
    let showMetadataModal = false;
    let showMetadataPanel = false;
    let mode = 'NORMAL';
    let isAnyModalOpen = false;
    let showCandidateMoves = false;
    let showMovesTable = true;
    let showEditPanel = false;
    let showInsertPanel = false;
    let showInsertGamePanel = false;
    let showPastePanel = false;
    
    console.log('App.svelte: Variables initialized');
    
    // Reference for various elements.
    let mainArea;
    let commandInput;
    let currentPositionIndex = 0;
    let positions = [];
    
    // Vim-like navigation state
    let lastGKeyTime = 0;
    const GG_TIMEOUT = 500; // milliseconds to wait for second 'g'
    
    // Vim-like 'dd' state for deletion
    let lastDKeyTime = 0;
    const DD_TIMEOUT = 500; // milliseconds to wait for second 'd'
    let pendingDeleteCommand = false; // Track if 'd' was pressed and waiting for second 'd'
    let movesTableRef = null; // Reference to MovesTable component

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

    showGoToMoveModalStore.subscribe(value => {
        showGoToMoveModal = value;
    });

    showMoveSearchModalStore.subscribe(value => {
        showMoveSearchModal = value;
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
    function convertPositionToBoard(position, dice = [0, 0], playerOnRoll = 0, score = [0, 0], cubeValue = 1, cubeOwner = -1, isCrawford = 0) {
        const points = [];
        
        // Initialize points array
        // Point 0: Player 2's bar (white, color=1, negative checkers)
        // Points 1-24: board points
        // Point 25: Player 1's bar (black, color=0, positive checkers)
        for (let i = 0; i <= 25; i++) {
            if (i === 0) {
                // Point 0 is Player 2's bar (opponent bar - negative checkers)
                const opponentBarCheckers = Math.abs(position.opponentBar);
                if (opponentBarCheckers > 0) {
                    points.push({ checkers: opponentBarCheckers, color: 1 });
                } else {
                    points.push({ checkers: 0, color: -1 });
                }
            } else if (i === 25) {
                // Point 25 is Player 1's bar (positive checkers)
                const barCheckers = position.bar;
                if (barCheckers > 0) {
                    points.push({ checkers: barCheckers, color: 0 });
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
        
        // Convert cube value to log2 for internal representation
        // cubeValue 1->0, 2->1, 4->2, 8->3, etc.
        const cubeValueLog = cubeValue > 0 ? Math.log2(cubeValue) : 0;
        
        return {
            id: 0,
            board: {
                points: points,
                bearoff: [position.off, position.opponentOff]
            },
            cube: {
                owner: cubeOwner,
                value: cubeValueLog
            },
            dice: dice,
            score: score,
            player_on_roll: playerOnRoll,
            decision_type: 0,
            has_jacoby: 0,
            has_beaver: 0,
            is_crawford: isCrawford
        };
    }

    // Subscribe to selected move changes to update position
    selectedMoveStore.subscribe(selectedMove => {
        const transcription = get(transcriptionStore);
        const positionsCache = get(positionsCacheStore);
        const showInitialPosition = get(showInitialPositionStore);
        
        if (transcription && transcription.games && transcription.games.length > 0) {
            const { gameIndex, moveIndex, player } = selectedMove;
            
            // Check cache first - include showInitialPosition in cache key
            const cacheKey = `${gameIndex}-${moveIndex}-${player}-${showInitialPosition ? 'initial' : 'final'}`;
            if (positionsCache[cacheKey]) {
                positionStore.set(positionsCache[cacheKey]);
            } else {
                // Calculate position by applying moves sequentially up to the selected move
                const game = transcription.games[gameIndex];
                if (game) {
                    let position = createInitialPosition();
                    
                    // Track cube state
                    let cubeValue = 1;  // Start at 1 (displayed as "1" at center)
                    let cubeOwner = -1; // -1 = center, 0 = player1, 1 = player2
                    
                    // Check if we should show the starting position (before any moves)
                    // This happens when: moveIndex is 0 AND player 1 is selected BUT player 2 starts the game
                    const firstMove = game.moves[0];
                    const player2Starts = firstMove && (!firstMove.player1Move || firstMove.player1Move === null) && firstMove.player2Move;
                    const showStartingPosition = (moveIndex === 0 && player === 1 && player2Starts);
                    
                    console.log('Position calculation:', {
                        moveIndex,
                        player,
                        player2Starts,
                        showStartingPosition,
                        firstMove: firstMove ? {
                            player1Move: firstMove.player1Move,
                            player2Move: firstMove.player2Move
                        } : null
                    });
                    
                    // If showing starting position, don't apply any moves
                    if (!showStartingPosition) {
                        // Apply all moves up to and including the selected move
                        for (let i = 0; i < game.moves.length; i++) {
                            const move = game.moves[i];
                            
                            // Check if this is the selected move index
                            if (i === moveIndex) {
                                // Only apply the selected player's move (unless showing initial position)
                                if (player === 1) {
                                    // Apply player1's move if it exists and we want final position
                                    if (!showInitialPosition && move.player1Move && move.player1Move.move) {
                                        const result = applyMove(position, move.player1Move.move, true);
                                        position = result.position;
                                    }
                                    // Process cube action if player 1 made it (only if not showing initial position for player 1)
                                    // New structure: in player move data
                                    if (!showInitialPosition && move.player1Move?.cubeAction) {
                                        if (move.player1Move.cubeAction === 'doubles') {
                                            cubeValue = move.player1Move.cubeValue || (cubeValue * 2);
                                            cubeOwner = -1; // Center when doubled
                                        } else if (move.player1Move.cubeAction === 'takes') {
                                            cubeOwner = 0; // Player 1 owns after taking
                                        }
                                    }
                                    // Old structure: at move level
                                    if (!showInitialPosition && move.cubeAction && move.cubeAction.player === 1) {
                                        if (move.cubeAction.action === 'doubles') {
                                            cubeValue = move.cubeAction.value;
                                            cubeOwner = -1; // Center when doubled
                                            // Don't process response here - it happens after player 1's turn
                                        } else if (move.cubeAction.action === 'takes') {
                                            cubeOwner = 0; // Player 1 owns after taking
                                        }
                                    }
                                    // Process cube action if player 2 made it (for viewing player 1's response)
                                    // Only process if both players have moves (player 2's action happened after player 1's move)
                                    // OR if there's a response in the same move entry
                                    if (move.cubeAction && move.cubeAction.player === 2) {
                                        const bothPlayersHaveMoves = move.player1Move && move.player2Move;
                                        const hasResponse = move.cubeAction.response;
                                        
                                        if (bothPlayersHaveMoves || hasResponse) {
                                            if (move.cubeAction.action === 'doubles') {
                                                // Always apply the double (player 2 doubled before player 1's decision)
                                                cubeValue = move.cubeAction.value;
                                                cubeOwner = -1; // Center when doubled
                                                // Only process player 1's response if not showing initial position
                                                if (!showInitialPosition) {
                                                    if (move.cubeAction.response === 'takes') {
                                                        cubeOwner = 0; // Player 1 owns after taking
                                                    } else if (move.cubeAction.response === 'drops') {
                                                        cubeOwner = -1;
                                                    }
                                                }
                                            }
                                        }
                                    }
                                    break; // Stop after the selected player's move
                                } else if (player === 2) {
                                    // First process any cube action by player 1 (if it has already occurred before player 2's turn)
                                    if (move.player1Move?.cubeAction) {
                                        if (move.player1Move.cubeAction === 'doubles') {
                                            cubeValue = move.player1Move.cubeValue || (cubeValue * 2);
                                            cubeOwner = -1; // Center when doubled
                                            // Don't process player 2's response here yet - handled below
                                        } else if (move.player1Move.cubeAction === 'takes') {
                                            cubeOwner = 0; // Player 1 owns after taking
                                        } else if (move.player1Move.cubeAction === 'drops') {
                                            cubeOwner = -1;
                                        }
                                    }
                                    // Then apply player1's move (it happened before player2's move)
                                    if (move.player1Move && move.player1Move.move) {
                                        const result1 = applyMove(position, move.player1Move.move, true);
                                        position = result1.position;
                                    }
                                    // Then apply player2's move (unless showing initial position)
                                    if (!showInitialPosition && move.player2Move && move.player2Move.move) {
                                        const result2 = applyMove(position, move.player2Move.move, false);
                                        position = result2.position;
                                    }
                                    // Process cube action if player 2 made it (only if not showing initial position)
                                    // New structure: in player move data
                                    if (!showInitialPosition && move.player2Move?.cubeAction) {
                                        if (move.player2Move.cubeAction === 'doubles') {
                                            cubeValue = move.player2Move.cubeValue || (cubeValue * 2);
                                            cubeOwner = -1; // Center when doubled
                                        } else if (move.player2Move.cubeAction === 'takes') {
                                            cubeOwner = 1; // Player 2 owns after taking
                                        }
                                    }
                                    // Old structure: at move level
                                    if (!showInitialPosition && move.cubeAction && move.cubeAction.player === 2) {
                                        if (move.cubeAction.action === 'doubles') {
                                            cubeValue = move.cubeAction.value;
                                            cubeOwner = -1; // Center when doubled
                                            // Check if player 1 responded in the same move
                                            if (move.cubeAction.response === 'takes') {
                                                cubeOwner = 0; // Player 1 owns after taking
                                            } else if (move.cubeAction.response === 'drops') {
                                                cubeOwner = -1;
                                            }
                                        } else if (move.cubeAction.action === 'takes') {
                                            cubeOwner = 1; // Player 2 owns after taking
                                        }
                                    }
                                    break; // Stop after the selected player's move
                                }
                            } else if (i < moveIndex) {
                                // For moves before the selected one, apply both players' moves
                                if (move.player1Move && move.player1Move.move) {
                                    const result = applyMove(position, move.player1Move.move, true);
                                    position = result.position;
                                }
                                if (move.player2Move && move.player2Move.move) {
                                    const result = applyMove(position, move.player2Move.move, false);
                                    position = result.position;
                                }
                            }
                            
                            // Process cube actions from completed moves (before the selected move)
                            // Handle both old structure (move.cubeAction) and new structure (playerMove.cubeAction)
                            if (i < moveIndex) {
                                // New structure: player1's cube action
                                if (move.player1Move?.cubeAction) {
                                    if (move.player1Move.cubeAction === 'doubles') {
                                        cubeValue = move.player1Move.cubeValue || (cubeValue * 2);
                                        cubeOwner = -1; // Center when doubled
                                    } else if (move.player1Move.cubeAction === 'takes') {
                                        cubeOwner = 0; // Player 1 owns after taking
                                    }
                                }
                                
                                // New structure: player2's cube action
                                if (move.player2Move?.cubeAction) {
                                    if (move.player2Move.cubeAction === 'doubles') {
                                        cubeValue = move.player2Move.cubeValue || (cubeValue * 2);
                                        cubeOwner = -1; // Center when doubled
                                    } else if (move.player2Move.cubeAction === 'takes') {
                                        cubeOwner = 1; // Player 2 owns after taking
                                    }
                                }
                                
                                // Old structure: move-level cube action
                                if (move.cubeAction) {
                                    if (move.cubeAction.action === 'doubles') {
                                        cubeValue = move.cubeAction.value;
                                        // After doubling, check if there's a response
                                        if (move.cubeAction.response === 'takes' || move.cubeAction.action === 'takes') {
                                            cubeOwner = move.cubeAction.player === 1 ? 1 : 0; // Opponent owns after taking
                                        } else if (move.cubeAction.response === 'drops') {
                                            cubeOwner = -1;
                                        } else {
                                            cubeOwner = -1; // Still at center if no response yet
                                        }
                                    } else if (move.cubeAction.action === 'takes') {
                                        cubeOwner = move.cubeAction.player - 1; // Convert 1/2 to 0/1
                                    }
                                }
                            }
                        }
                    }
                    
                    // Get the selected move data for dice and player on roll
                    const selectedMoveData = game.moves[moveIndex];
                    const playerMoveData = player === 1 ? selectedMoveData?.player1Move : selectedMoveData?.player2Move;
                    
                    // Parse dice from move data - only if player has dice (not just cube action)
                    let dice = [0, 0];
                    if (!showStartingPosition && playerMoveData && playerMoveData.dice && playerMoveData.dice.trim() !== '') {
                        const diceStr = playerMoveData.dice;
                        if (diceStr.length >= 2) {
                            dice = [parseInt(diceStr[0]), parseInt(diceStr[1])];
                        }
                    }
                    
                    // Calculate away scores
                    const matchLength = transcription.metadata.matchLength || 0;
                    const player1Score = game.player1Score || 0;
                    const player2Score = game.player2Score || 0;
                    
                    let awayScore1, awayScore2;
                    let isCrawfordGame = 0; // 0 = not Crawford, 1 = is Crawford game
                    
                    if (matchLength === 0) {
                        // Unlimited match
                        awayScore1 = -1;
                        awayScore2 = -1;
                    } else {
                        awayScore1 = matchLength - player1Score;
                        awayScore2 = matchLength - player2Score;
                        
                        // Check for Crawford game (one player is 1-away)
                        const isCrawford = (awayScore1 === 1 || awayScore2 === 1) && 
                                          transcription.metadata.crawford === 'On';
                        
                        if (isCrawford && gameIndex === transcription.games.findIndex(g => 
                            (matchLength - g.player1Score === 1 || matchLength - g.player2Score === 1))) {
                            // This is the Crawford game
                            isCrawfordGame = 1;
                            if (awayScore1 === 1) awayScore1 = 1;
                            if (awayScore2 === 1) awayScore2 = 1;
                        } else if (isCrawford && gameIndex > transcription.games.findIndex(g => 
                            (matchLength - g.player1Score === 1 || matchLength - g.player2Score === 1))) {
                            // Post-Crawford
                            if (awayScore1 === 1) awayScore1 = 0;
                            if (awayScore2 === 1) awayScore2 = 0;
                        }
                    }
                    
                    // Convert to board format expected by Board component
                    const boardPosition = convertPositionToBoard(position, dice, player - 1, [awayScore1, awayScore2], cubeValue, cubeOwner, isCrawfordGame);
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

    // Subscribe to show initial position toggle - refresh when it changes
    showInitialPositionStore.subscribe(() => {
        // Trigger a refresh by re-setting the selected move
        const currentSelectedMove = get(selectedMoveStore);
        const showInitial = get(showInitialPositionStore);
        // Force recalculation by clearing cache for both initial and final positions
        const cacheKeyInitial = `${currentSelectedMove.gameIndex}-${currentSelectedMove.moveIndex}-${currentSelectedMove.player}-initial`;
        const cacheKeyFinal = `${currentSelectedMove.gameIndex}-${currentSelectedMove.moveIndex}-${currentSelectedMove.player}-final`;
        positionsCacheStore.update(cache => {
            delete cache[cacheKeyInitial];
            delete cache[cacheKeyFinal];
            return cache;
        });
        // Trigger recalculation
        selectedMoveStore.set(currentSelectedMove);
    });

    // Subscribe to transcription store to detect match length changes
    let previousMatchLength = null;
    transcriptionStore.subscribe(async transcription => {
        if (transcription && transcription.metadata) {
            const currentMatchLength = transcription.metadata.matchLength;
            
            // Check if match length has changed (skip initial subscription)
            if (previousMatchLength !== null && previousMatchLength !== currentMatchLength) {
                // Match length changed - force recalculation of current position
                const currentSelectedMove = get(selectedMoveStore);
                const cacheKeyInitial = `${currentSelectedMove.gameIndex}-${currentSelectedMove.moveIndex}-${currentSelectedMove.player}-initial`;
                const cacheKeyFinal = `${currentSelectedMove.gameIndex}-${currentSelectedMove.moveIndex}-${currentSelectedMove.player}-final`;
                positionsCacheStore.update(cache => {
                    delete cache[cacheKeyInitial];
                    delete cache[cacheKeyFinal];
                    return cache;
                });
                // Trigger recalculation
                selectedMoveStore.set(currentSelectedMove);
                
                // Revalidate all games to detect impossible game scores
                for (let gameIndex = 0; gameIndex < transcription.games.length; gameIndex++) {
                    await validateGameInconsistencies(gameIndex, 0);
                }
            }
            
            previousMatchLength = currentMatchLength;
        }
    });

    //Global shortcuts
    
    // Helper function to check if focus is on a metadata panel input
    function isMetadataInputFocused() {
        const activeElement = document.activeElement;
        if (!activeElement || activeElement.tagName !== 'INPUT') {
            return false;
        }
        // Check if the input is within the metadata panel
        const metadataPanel = activeElement.closest('.metadata-panel');
        return metadataPanel !== null;
    }
    
    function handleKeyDown(event) {
        event.stopPropagation();

        // Prevent shortcuts if any modal is open
        if ($isAnyModalOpenStore) {
            return;
        }
        
        // Prevent shortcuts if editing metadata (except Escape and Ctrl+M to close panel)
        const editingMetadata = isMetadataInputFocused();
        if (editingMetadata) {
            if (event.key === 'Escape') {
                event.preventDefault();
                showMetadataPanelStore.set(false);
                return;
            } else if (!(event.ctrlKey && event.code === 'KeyM')) {
                return;
            }
        }

        if (event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
        } else if(event.ctrlKey && event.code == 'KeyN') {
            newMatch();
        } else if(event.ctrlKey && event.code == 'KeyO') {
            loadMatchFromText();
        } else if(event.ctrlKey && event.code == 'KeyS') {
            event.preventDefault();
            exportMatchText();
        } else if (event.ctrlKey && event.code === 'KeyQ') {
            exitApp();
        } else if (!event.ctrlKey && event.key === 'PageUp') {
            event.preventDefault();
            previousGame();
        } else if (!event.ctrlKey && event.key === 'h') {
            previousGame();
        } else if (!event.ctrlKey && event.key === 'ArrowLeft') {
            // Don't intercept if in EDIT mode (allow cursor movement in input fields)
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                previousPosition();
            }
        } else if (!event.ctrlKey && event.key === 'ArrowRight') {
            // Don't intercept if in EDIT mode (allow cursor movement in input fields)
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                nextPosition();
            }
        } else if (!event.ctrlKey && event.key === 'PageDown') {
            event.preventDefault();
            nextGame();
        } else if (!event.ctrlKey && event.key === 'l') {
            nextGame();
        } else if (!event.ctrlKey && !event.shiftKey && event.key === 'g') {
            // Vim-like 'gg' - go to first move of current game
            const currentTime = Date.now();
            if (currentTime - lastGKeyTime < GG_TIMEOUT) {
                event.preventDefault();
                firstMoveOfCurrentGame();
                lastGKeyTime = 0; // Reset
            } else {
                lastGKeyTime = currentTime;
            }
        } else if (!event.ctrlKey && event.shiftKey && event.key === 'G') {
            // Vim-like 'G' - go to last move of current game
            event.preventDefault();
            lastMoveOfCurrentGame();
        } else if(event.ctrlKey && event.code == 'KeyK') {
            gotoMove();
        } else if(!event.ctrlKey && event.code === 'Tab') {
            // Don't intercept Tab if in metadata panel (allow field cycling)
            if (!isMetadataInputFocused()) {
                event.preventDefault();
                if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                    setStatusBarMessage('No transcription opened');
                    return;
                }
                toggleEditMode();
            }
        } else if (!event.ctrlKey && event.code === 'Space') {
            // Don't open command mode if in EDIT mode (allow space in input fields)
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                showCommandStore.set(true); // Show command line
            }
        } else if (event.ctrlKey && event.shiftKey && event.code === 'KeyD') {
            // Debug: Dump transcription state to console
            event.preventDefault();
            dumpTranscriptionState();
        } else if (event.ctrlKey && event.code === 'KeyH') {
            toggleHelpModal();
        } else if (!event.ctrlKey && event.key === '?') {
            toggleHelpModal();
        } else if (event.ctrlKey && event.code === 'KeyM') {
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            showMetadataPanelStore.update(v => !v);
        } else if (event.ctrlKey && event.code === 'KeyI') {
            loadMatchFromText();
        } else if (event.ctrlKey && event.code === 'KeyP') {
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            showMovesTableStore.update(v => !v);
        } else if (event.ctrlKey && event.code === 'KeyL') {
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            showCandidateMovesStore.update(v => !v);
        } else if (!event.ctrlKey && event.key === 't') {
            // 't' - toggle initial/final position display
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                togglePositionDisplay();
            }
        } else if (event.ctrlKey && event.code === 'KeyC') {
            // Ctrl+C - copy decisions
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            copyDecisions();
        } else if (!event.ctrlKey && event.key === 'y') {
            // 'y' (vim-like yank) - copy decisions
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                    setStatusBarMessage('No transcription opened');
                    return;
                }
                copyDecisions();
            }
        } else if (event.ctrlKey && event.code === 'KeyX') {
            // Ctrl+X - cut decisions
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            cutDecisions();
        } else if (event.ctrlKey && event.code === 'KeyV') {
            // Ctrl+V - paste decisions after
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            const clipboard = get(clipboardStore);
            if (!clipboard.decisions || clipboard.decisions.length === 0) {
                setStatusBarMessage('Clipboard is empty');
                return;
            }
            pasteDecisionsDirectly('after');
        } else if (!event.ctrlKey && !event.shiftKey && event.key === 'p') {
            // 'p' - paste decisions after (directly)
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                    setStatusBarMessage('No transcription opened');
                    return;
                }
                const clipboard = get(clipboardStore);
                if (!clipboard.decisions || clipboard.decisions.length === 0) {
                    setStatusBarMessage('Clipboard is empty');
                    return;
                }
                pasteDecisionsDirectly('after');
            }
        } else if (!event.ctrlKey && event.shiftKey && event.key === 'P') {
            // 'P' - paste decisions before (directly)
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                    setStatusBarMessage('No transcription opened');
                    return;
                }
                const clipboard = get(clipboardStore);
                if (!clipboard.decisions || clipboard.decisions.length === 0) {
                    setStatusBarMessage('Clipboard is empty');
                    return;
                }
                pasteDecisionsDirectly('before');
            }
        } else if (!event.ctrlKey && event.shiftKey && event.key === 'O') {
            // Vim-like 'O' - insert empty decision BEFORE current
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            insertDecisionDirectly('before');
        } else if (!event.ctrlKey && !event.shiftKey && event.key === 'o') {
            // Vim-like 'o' - insert empty decision AFTER current
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            insertDecisionDirectly('after');
        } else if (!event.ctrlKey && event.shiftKey && event.key === 'N') {
            // 'N' - insert new game BEFORE current
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            insertGameDirectly('before');
        } else if (!event.ctrlKey && !event.shiftKey && event.key === 'n') {
            // 'n' - insert new game AFTER current
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            insertGameDirectly('after');
        } else if (!event.ctrlKey && event.shiftKey && event.key === 'D') {
            // 'D' - delete current game
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            deleteCurrentGame();
        } else if (event.ctrlKey && event.code === 'KeyZ') {
            event.preventDefault();
            handleUndo();
        } else if (event.ctrlKey && (event.code === 'KeyY' || event.code === 'KeyR')) {
            event.preventDefault();
            handleRedo();
        } else if (!event.ctrlKey && event.key === 'u') {
            // Vim-like 'u' for undo
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                handleUndo();
            }
        } else if (event.ctrlKey && event.code === 'KeyF') {
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            // If panel is open but not focused, focus it. Otherwise toggle.
            if ($showMoveSearchModalStore) {
                // Panel is open - try to focus it
                const event = new CustomEvent('focusSearchPanel');
                window.dispatchEvent(event);
            } else {
                // Panel is closed - open it
                showMoveSearchModalStore.set(true);
            }
        } else if (event.ctrlKey && event.key === ']') {
            event.preventDefault();
            nextSearchResult();
        } else if (event.ctrlKey && event.key === '[') {
            event.preventDefault();
            previousSearchResult();
        } else if (!event.ctrlKey && !event.shiftKey && event.key === 'd') {
            // Don't intercept if in EDIT mode (allow typing 'd' for double/drop)
            if ($statusBarModeStore !== 'EDIT') {
                event.preventDefault();
                const currentTime = Date.now();
                
                if (pendingDeleteCommand) {
                    // Second 'd' pressed - execute dd (cut/delete)
                    console.log('[App.handleKeyDown] dd detected - cut/delete');
                    if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                        setStatusBarMessage('No transcription opened');
                        pendingDeleteCommand = false;
                        return;
                    }
                    // dd cuts the decision (which also deletes it)
                    cutDecisions();
                    pendingDeleteCommand = false;
                } else {
                    // First 'd' pressed - wait for next key
                    console.log('[App.handleKeyDown] First d pressed, waiting for next key');
                    pendingDeleteCommand = true;
                    lastDKeyTime = currentTime;
                    setTimeout(() => {
                        if (pendingDeleteCommand && Date.now() - lastDKeyTime >= DD_TIMEOUT) {
                            console.log('[App.handleKeyDown] d command timed out - ignored');
                            // Timeout: just clear the pending state
                            pendingDeleteCommand = false;
                        }
                    }, DD_TIMEOUT);
                }
            }
        } else if (!event.ctrlKey && event.key === 'Delete') {
            // Delete key - delete current decision
            event.preventDefault();
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            deleteCurrentDecision();
        } else if (!event.ctrlKey && event.shiftKey && event.key === 'K') {
            // Shift+k - extend selection upward
            console.log('[App.handleKeyDown] Shift+K detected - extend selection up');
            event.preventDefault();
            extendSelectionUp();
        } else if (!event.ctrlKey && event.shiftKey && event.key === 'J') {
            // Shift+j - extend selection downward
            console.log('[App.handleKeyDown] Shift+J detected - extend selection down');
            event.preventDefault();
            extendSelectionDown();
        } else if (!event.ctrlKey && event.key === 'k') {
            // k for up navigation (delete mode handled earlier)
            previousPosition();
        } else if (!event.ctrlKey && event.key === 'j') {
            // j for down navigation (delete mode handled earlier)
            nextPosition();
        } else if (pendingDeleteCommand) {
            // Any other key pressed while waiting for delete command - cancel
            console.log(`[App.handleKeyDown] Canceling delete mode on other key: ${event.key}`);
            pendingDeleteCommand = false;
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
            
            // Add the first game with 0-0 score
            const { addGame } = await import('./stores/transcriptionStore.js');
            addGame(1, 0, 0);
            
            // Set selected move to the first empty decision of player 1
            selectedMoveStore.set({ gameIndex: 0, moveIndex: 0, player: 1 });
            
            // Get the transcription with initial metadata and first game
            const transcription = get(transcriptionStore);
            
            // Save the initial transcription file
            const jsonContent = JSON.stringify(transcription, null, 2);
            await WriteTextFile(finalPath, jsonContent);
            
            // Store the file path
            transcriptionFilePathStore.set(finalPath);
            
            // Clear undo/redo history for new transcription
            undoRedoStore.clear();
            
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
            if (!transcription || !transcription.games || transcription.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }

            console.log('[saveTranscription] First 3 moves:', JSON.stringify(transcription.games[0]?.moves.slice(0, 3), null, 2));
            
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

    async function exportMatchText() {
        console.log('exportMatchText');
        try {
            // Get current transcription
            const transcription = get(transcriptionStore);
            if (!transcription || !transcription.games || transcription.games.length === 0) {
                setStatusBarMessage('No transcription to export');
                return;
            }
            
            // Show save dialog
            let filePath = await ExportMatchTextDialog();
            if (!filePath) {
                console.log('Export cancelled');
                return;
            }
            
            // Ensure .txt extension
            if (!filePath.endsWith('.txt')) {
                filePath = filePath + '.txt';
            }
            
            // Convert transcription to match text format
            const textContent = exportToMatchText(transcription);
            
            // Save file
            await WriteTextFile(filePath, textContent);
            
            const filename = filePath.split('/').pop();
            setStatusBarMessage(`Match exported to ${filename}`);
            console.log('Match exported to:', filePath);
        } catch (error) {
            console.error('Error exporting match:', error);
            setStatusBarMessage('Error exporting match');
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
                let transcription = parseMatchFile(content);
                
                // Migrate old cube action structure if present
                transcription = migrateCubeActions(transcription);
                
                // Update the transcription store with the complete transcription
                clearTranscription();
                transcriptionStore.set(transcription);
                
                // Clear undo/redo history for loaded transcription
                undoRedoStore.clear();
                
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
                
                // Migrate old cube action structure if present
                transcription = migrateCubeActions(transcription);
                
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
                
                // Clear undo/redo history for loaded transcription
                undoRedoStore.clear();
                
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

    function firstMoveOfCurrentGame() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate to first move of current game
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex } = $selectedMoveStore;
            selectedMoveStore.set({ gameIndex, moveIndex: 0, player: 1 });
        }
    }

    function lastMoveOfCurrentGame() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate to last move of current game
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex } = $selectedMoveStore;
            const game = $transcriptionStore.games[gameIndex];
            if (game && game.moves.length > 0) {
                const lastMoveIndex = game.moves.length - 1;
                const lastMove = game.moves[lastMoveIndex];
                // Check if player 2 has something to show (move or cube action)
                const hasPlayer2Action = lastMove && (lastMove.player2Move || (lastMove.cubeAction && lastMove.cubeAction.player === 2));
                const lastPlayer = hasPlayer2Action ? 2 : 1;
                selectedMoveStore.set({ gameIndex, moveIndex: lastMoveIndex, player: lastPlayer });
            }
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
        
        // Clear multi-selection when navigating
        clearMultiSelection();
        
        // Navigate in transcription mode
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex, moveIndex, player } = $selectedMoveStore;
            
            if (player === 2) {
                // Move from player 2 to player 1 of same move
                selectedMoveStore.set({ gameIndex, moveIndex, player: 1 });
            } else if (moveIndex > 0) {
                // Move to previous move (player 2)
                // Always navigate to player 2 slot, even if empty (null)
                selectedMoveStore.set({ gameIndex, moveIndex: moveIndex - 1, player: 2 });
            }
            // Stop at first move of current game - do not go to previous game
        }
    }

    function nextPosition() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Clear multi-selection when navigating
        clearMultiSelection();
        
        // Navigate in transcription mode
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            const { gameIndex, moveIndex, player } = $selectedMoveStore;
            const game = $transcriptionStore.games[gameIndex];
            
            if (player === 1) {
                // Move from player 1 to player 2 of same move
                // Navigate to player 2 slot even if empty (null)
                selectedMoveStore.set({ gameIndex, moveIndex, player: 2 });
            } else if (moveIndex < game.moves.length - 1) {
                // Move to next move (player 1)
                selectedMoveStore.set({ gameIndex, moveIndex: moveIndex + 1, player: 1 });
            }
            // Stop at last move of current game - do not go to next game
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
            // Check if player 2 has something to show (move or cube action)
            const hasPlayer2Action = lastMove && (lastMove.player2Move || (lastMove.cubeAction && lastMove.cubeAction.player === 2));
            const lastPlayer = hasPlayer2Action ? 2 : 1;
            selectedMoveStore.set({ gameIndex: lastGameIndex, moveIndex: lastMoveIndex, player: lastPlayer });
        }
    }

    function gotoMove() {
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        if ($statusBarModeStore !== 'NORMAL') {
            setStatusBarMessage('Cannot go to move in current mode');
            return;
        }
        showGoToMoveModalStore.set(true);
    }

    function toggleEditMode() {
        console.log('toggleEditMode');
        if ($statusBarModeStore !== "EDIT") {
            if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
                setStatusBarMessage('No transcription opened');
                return;
            }
            previousModeStore.set($statusBarModeStore);
            statusBarModeStore.set('EDIT');
            
            // If moves table is NOT open, show the edit panel
            if (!showMovesTable) {
                showEditPanel = true;
            }
            // If moves table IS open, inline editing will be handled by MovesTable component
        } else {
            previousModeStore.set($statusBarModeStore);
            statusBarModeStore.set('NORMAL');
            showEditPanel = false;
            // Refresh board and display position associated with currentPositionIndexStore
            const currentIndex = $currentPositionIndexStore;
            currentPositionIndexStore.set(-1); // Temporarily set to a different value to force redraw
            currentPositionIndexStore.set(currentIndex); // Set back to the original value
        }
    }

    async function handleUndo() {
        console.log('handleUndo');
        const success = undoRedoStore.undo();
        if (success) {
            // Clear the entire position cache since transcription structure may have changed
            positionsCacheStore.set({});
            
            // Revalidate all games to update inconsistencies
            const transcription = get(transcriptionStore);
            if (transcription && transcription.games) {
                for (let gameIndex = 0; gameIndex < transcription.games.length; gameIndex++) {
                    await validateGameInconsistencies(gameIndex, 0);
                }
            }
            
            setStatusBarMessage('Undo completed');
            
            // Force board update by updating the selectedMoveStore
            const currentMove = get(selectedMoveStore);
            selectedMoveStore.set({ ...currentMove });
        } else {
            setStatusBarMessage('Nothing to undo');
        }
    }

    async function handleRedo() {
        console.log('handleRedo');
        const success = undoRedoStore.redo();
        if (success) {
            // Clear the entire position cache since transcription structure may have changed
            positionsCacheStore.set({});
            
            // Revalidate all games to update inconsistencies
            const transcription = get(transcriptionStore);
            if (transcription && transcription.games) {
                for (let gameIndex = 0; gameIndex < transcription.games.length; gameIndex++) {
                    await validateGameInconsistencies(gameIndex, 0);
                }
            }
            
            setStatusBarMessage('Redo completed');
            
            // Force board update by updating the selectedMoveStore
            const currentMove = get(selectedMoveStore);
            selectedMoveStore.set({ ...currentMove });
        } else {
            setStatusBarMessage('Nothing to redo');
        }
    }
    
    // Helper function to save snapshot AFTER an operation
    function saveSnapshotBeforeOperation() {
        console.log('[App.svelte] saveSnapshotBeforeOperation called');
        undoRedoStore.saveSnapshot();
    }

    function showInsertDecisionPanel() {
        console.log('showInsertDecisionPanel');
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        showInsertPanel = true;
    }

    function showInsertGamePanelFunc() {
        console.log('showInsertGamePanelFunc');
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        showInsertGamePanel = true;
    }

    async function insertGameBefore(gameIndex) {
        saveSnapshotBeforeOperation();
        const { insertGameBefore: insertGameBeforeStore, validateGameInconsistencies } = await import('./stores/transcriptionStore.js');
        insertGameBeforeStore(gameIndex);
        
        // Validate the newly inserted game and all subsequent games for score inconsistencies
        const transcription = get(transcriptionStore);
        for (let i = gameIndex; i < transcription.games.length; i++) {
            await validateGameInconsistencies(i, 0);
        }
        
        // Select first move of the newly inserted game
        selectedMoveStore.set({ gameIndex: gameIndex, moveIndex: 0, player: 1 });
        setStatusBarMessage(`New game inserted before game ${gameIndex + 2}`);
    }

    async function insertGameAfter(gameIndex) {
        saveSnapshotBeforeOperation();
        const { insertGameAfter: insertGameAfterStore, validateGameInconsistencies } = await import('./stores/transcriptionStore.js');
        insertGameAfterStore(gameIndex);
        
        // Force update to ensure UI reflects new scores
        await tick();
        
        // Validate the newly inserted game and all subsequent games for score inconsistencies
        const transcription = get(transcriptionStore);
        for (let i = gameIndex + 1; i < transcription.games.length; i++) {
            await validateGameInconsistencies(i, 0);
        }
        
        // Select first move of the newly inserted game
        selectedMoveStore.set({ gameIndex: gameIndex + 1, moveIndex: 0, player: 1 });
        setStatusBarMessage(`New game inserted after game ${gameIndex + 1}`);
    }

    async function insertGameDirectly(position) {
        const selectedMove = get(selectedMoveStore);
        if (!selectedMove) return;
        
        const { gameIndex } = selectedMove;
        
        if (position === 'before') {
            await insertGameBefore(gameIndex);
        } else {
            await insertGameAfter(gameIndex);
        }
    }

    async function deleteCurrentGame() {
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        
        if ($transcriptionStore.games.length === 1) {
            setStatusBarMessage('Cannot delete the last game');
            return;
        }
        
        const selectedMove = get(selectedMoveStore);
        if (!selectedMove) return;
        
        const { gameIndex } = selectedMove;
        
        // Save state before operation
        saveSnapshotBeforeOperation();
        
        // Import and call deleteGame
        const { deleteGame: deleteGameStore } = await import('./stores/transcriptionStore.js');
        const success = deleteGameStore(gameIndex);
        
        if (success) {
            // Adjust selection to previous game or stay at same index if first game was deleted
            const newGameIndex = gameIndex > 0 ? gameIndex - 1 : 0;
            const newTranscription = get(transcriptionStore);
            
            // Make sure the new game index is valid
            if (newTranscription.games[newGameIndex]) {
                selectedMoveStore.set({ gameIndex: newGameIndex, moveIndex: 0, player: 1 });
            }
            
            setStatusBarMessage(`Game ${gameIndex + 1} deleted`);
        } else {
            setStatusBarMessage('Failed to delete game');
        }
    }

    async function insertDecisionDirectly(position) {
        const selectedMove = get(selectedMoveStore);
        
        if (!selectedMove) return;
        
        const { gameIndex, moveIndex, player } = selectedMove;
        
        // Check if game has ended before allowing insertion after
        if (position === 'after') {
            const transcription = get(transcriptionStore);
            const game = transcription?.games[gameIndex];
            if (game && game.moves && game.moves[moveIndex]) {
                const move = game.moves[moveIndex];
                const currentPlayerMove = player === 1 ? move.player1Move : move.player2Move;
                
                // Only block if current decision is actually a drop or resign
                if (currentPlayerMove && (currentPlayerMove.cubeAction === 'drops' || currentPlayerMove.resignAction)) {
                    setStatusBarMessage('Cannot insert decision after game has ended');
                    return;
                }
                
                // For player 1, also check if player 2 of same move has dropped/resigned
                if (player === 1 && move.player2Move && (move.player2Move.cubeAction === 'drops' || move.player2Move.resignAction)) {
                    setStatusBarMessage('Cannot insert decision after game has ended');
                    return;
                }
            }
        }
        
        // Save state before the operation
        saveSnapshotBeforeOperation();
        const { insertDecisionBefore, insertDecisionAfter, invalidatePositionsCacheFrom } = await import('./stores/transcriptionStore.js');
        
        console.log(`[insertDecisionDirectly] BEFORE - position=${position}, gameIndex=${gameIndex}, moveIndex=${moveIndex}, player=${player}`);
        console.log(`[insertDecisionDirectly] Current transcription games count:`, get(transcriptionStore)?.games?.length);
        console.log(`[insertDecisionDirectly] Current game moves count:`, get(transcriptionStore)?.games[gameIndex]?.moves?.length);
        
        // Insert empty decision
        if (position === 'before') {
            insertDecisionBefore(gameIndex, moveIndex, player);
        } else {
            insertDecisionAfter(gameIndex, moveIndex, player);
        }
        
        console.log(`[insertDecisionDirectly] AFTER INSERT - New moves count:`, get(transcriptionStore)?.games[gameIndex]?.moves?.length);
        
        // Calculate new move index and player
        // When inserting before player1, it stays in same move as player1
        // When inserting before player2, it stays in same move as player2
        // When inserting after player1, it becomes player2 in same move
        // When inserting after player2, it becomes player1 in next move
        let newMoveIndex = moveIndex;
        let newPlayer = player;
        
        if (position === 'after') {
            if (player === 1) {
                // After player1 -> becomes player2 in same move
                newPlayer = 2;
            } else {
                // After player2 -> becomes player1 in next move
                newMoveIndex = moveIndex + 1;
                newPlayer = 1;
            }
        }
        // For 'before', newMoveIndex and newPlayer stay the same
        
        console.log(`[insertDecisionDirectly] AFTER - Selecting newMoveIndex=${newMoveIndex}, newPlayer=${newPlayer}`);
        
        // Invalidate position cache from the insertion point onwards
        await invalidatePositionsCacheFrom(gameIndex, moveIndex);
        
        // Update selection to point to the newly inserted empty decision
        selectedMoveStore.set({ gameIndex, moveIndex: newMoveIndex, player: newPlayer });
        
        const posText = position === 'before' ? 'before' : 'after';
        setStatusBarMessage(`Empty decision inserted ${posText} at game ${gameIndex + 1}, move ${newMoveIndex + 1}. Use Tab to edit.`);
    }

    async function deleteCurrentDecision() {
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        
        // Delegate to MovesTable to handle both single and multi-selection deletion
        if (movesTableRef && movesTableRef.deleteSelectedDecisions) {
            await movesTableRef.deleteSelectedDecisions();
        } else {
            console.error('[deleteCurrentDecision] MovesTable reference not available');
        }
    }

    function clearMultiSelection() {
        if (movesTableRef && movesTableRef.clearSelection) {
            movesTableRef.clearSelection();
        }
    }

    function copyDecisions() {
        console.log('[App.copyDecisions] Copy decisions');
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        
        if (movesTableRef && movesTableRef.copySelectedDecisions) {
            movesTableRef.copySelectedDecisions();
        } else {
            console.error('[copyDecisions] MovesTable reference not available');
        }
    }

    function cutDecisions() {
        console.log('[App.cutDecisions] Cut decisions');
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        
        if (movesTableRef && movesTableRef.cutSelectedDecisions) {
            movesTableRef.cutSelectedDecisions();
        } else {
            console.error('[cutDecisions] MovesTable reference not available');
        }
    }

    function pasteDecisions() {
        console.log('[App.pasteDecisions] Paste decisions');
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        
        // Check if clipboard has content
        const clipboard = get(clipboardStore);
        if (!clipboard.decisions || clipboard.decisions.length === 0) {
            setStatusBarMessage('Clipboard is empty');
            return;
        }
        
        // Open paste panel to choose before/after
        showPastePanel = true;
    }

    function pasteDecisionsDirectly(position) {
        console.log('[App.pasteDecisionsDirectly] Paste decisions', position);
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        
        // Check if clipboard has content
        const clipboard = get(clipboardStore);
        if (!clipboard.decisions || clipboard.decisions.length === 0) {
            setStatusBarMessage('Clipboard is empty');
            return;
        }

        // Get current selection
        const selectedMove = get(selectedMoveStore);
        if (!selectedMove) {
            setStatusBarMessage('No decision selected');
            return;
        }
        
        // Delegate to MovesTable to paste at current position
        if (movesTableRef && movesTableRef.pasteDecisionsAt) {
            movesTableRef.pasteDecisionsAt(selectedMove.gameIndex, selectedMove.moveIndex, selectedMove.player, position);
        } else {
            console.error('[pasteDecisionsDirectly] MovesTable reference not available');
        }
    }

    function extendSelectionUp() {
        console.log('[App.extendSelectionUp] Delegating to MovesTable');
        if (movesTableRef && movesTableRef.extendSelectionUp) {
            movesTableRef.extendSelectionUp();
        } else {
            console.error('[App.extendSelectionUp] MovesTable reference not available');
        }
    }

    function extendSelectionDown() {
        console.log('[App.extendSelectionDown] Delegating to MovesTable');
        if (movesTableRef && movesTableRef.extendSelectionDown) {
            movesTableRef.extendSelectionDown();
        } else {
            console.error('[App.extendSelectionDown] MovesTable reference not available');
        }
    }

    function toggleMetadataModal() {
        if (mode === 'EDIT') {
            setStatusBarMessage('Cannot show metadata modal in edit mode');
            return;
        }
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        showMetadataModalStore.set(!showMetadataModal);
    }

    function toggleMovesTable() {
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        showMovesTableStore.set(!showMovesTable);
    }

    function toggleCandidateMovesPanel() {
        if (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0) {
            setStatusBarMessage('No transcription opened');
            return;
        }
        showCandidateMovesStore.set(!$showCandidateMovesStore);
    }

    function togglePositionDisplay() {
        if (mode === 'EDIT') {
            setStatusBarMessage('Cannot toggle position display in edit mode');
        } else {
            showInitialPositionStore.update(v => !v);
        }
    }

    function handleShowPastePanel(event) {
        showPastePanel = true;
    }

    onMount(() => {
        console.log('App.svelte: onMount called');
        // @ts-ignore
        console.log('Wails runtime:', runtime);
        window.addEventListener("keydown", handleKeyDown);
        mainArea.addEventListener("wheel", handleWheel); // Add wheel event listener to main container
        window.addEventListener("resize", handleResize);
        window.addEventListener("showPastePanel", handleShowPastePanel);
        console.log('App.svelte: Event listeners added');
    });

    onDestroy(() => {
        window.removeEventListener("keydown", handleKeyDown);
        mainArea.removeEventListener("wheel", handleWheel); // Remove wheel event listener from main container
        window.removeEventListener("resize", handleResize);
        window.removeEventListener("showPastePanel", handleShowPastePanel);
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
        onExportMatch={exportMatchText}
        onExit={exitApp}
        onPreviousGame={previousGame}
        onFirstPosition={firstPosition}
        onPreviousPosition={previousPosition}
        onNextPosition={nextPosition}
        onNextGame={nextGame}
        onLastPosition={lastPosition}
        onGoToMove={gotoMove}
        onTogglePositionDisplay={togglePositionDisplay}
        onToggleEditMode={toggleEditMode}
        onToggleCommandMode={() => showCommandStore.set(true)}
        onToggleMovesPanel={() => showMovesTableStore.update(v => !v)}
        onShowCandidateMoves={toggleCandidateMovesPanel}
        onToggleHelp={toggleHelpModal}
        onShowMetadata={toggleMetadataModal}
        onShowMoveSearch={() => showMoveSearchModalStore.update(v => !v)}
        onCopyDecisions={copyDecisions}
        onCutDecisions={cutDecisions}
        onPasteDecisions={pasteDecisions}
        onUndo={handleUndo}
        onRedo={handleRedo}
        onShowInsertPanel={showInsertDecisionPanel}
        onShowInsertGamePanel={showInsertGamePanelFunc}
        onDeleteGame={deleteCurrentGame}
        onDeleteDecision={deleteCurrentDecision}
    />

    <div class="transcription-layout">
        {#if showMovesTable}
        <div class="moves-table-column" transition:slide={{ duration: 50, axis: 'x' }}>
            <MovesTable bind:this={movesTableRef} />
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
                onExportMatch={exportMatchText}
                exitApp={exitApp}
            />
        </div>

        {#if showCandidateMoves}
        <div class="candidate-moves-column" transition:slide={{ duration: 50, axis: 'x' }}>
            <CandidateMovesPanel />
        </div>
        {/if}
    </div>

    {#if showMetadataPanel}
    <div class="metadata-panel-container">
        <MetadataPanel visible={showMetadataPanel} onClose={() => showMetadataPanelStore.set(false)} />
    </div>
    {/if}

    <GoToMoveModal
        visible={showGoToMoveModal}
        onClose={() => showGoToMoveModalStore.set(false)}
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

    <EditMovePanel />

    <EditPanel
        visible={showEditPanel}
        onClose={() => {
            showEditPanel = false;
            statusBarModeStore.set('NORMAL');
        }}
    />

    <MoveInsertPanel
        visible={showInsertPanel}
        onClose={() => {
            showInsertPanel = false;
        }}
    />

    <GameInsertPanel
        visible={showInsertGamePanel}
        onClose={() => {
            showInsertGamePanel = false;
        }}
        onInsertGameBefore={insertGameBefore}
        onInsertGameAfter={insertGameAfter}
    />

    <PastePanel
        visible={showPastePanel}
        onClose={() => {
            showPastePanel = false;
        }}
    />

    <MoveSearchPanel
        visible={showMoveSearchModal}
        onClose={() => showMoveSearchModalStore.set(false)}
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
