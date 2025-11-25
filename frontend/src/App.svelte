<script>

    // svelte functions
    import { onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import { get } from 'svelte/store';

    // import backend functions
    import {
        SaveDatabaseDialog,
        OpenDatabaseDialog,
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

    // import stores
    import {
        databasePathStore,
        databaseLoadedStore // Import databaseLoadedStore
    } from './stores/databaseStore';

    import {
        pastePositionTextStore,
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
    let databaseLoaded = false;
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

    async function showPosition(position) {
        // Database functionality removed
        setStatusBarMessage('Database functionality has been removed');
        return;
    }

    // Subscribe to the metaStore
    metaStore.subscribe(value => {
        applicationVersion = value.applicationVersion;
    });

    // Subscribe to the derived store
    isAnyModalOpenStore.subscribe(value => {
        isAnyModalOpen = value;
    });

    // Subscribe to the databaseLoadedStore
    databaseLoadedStore.subscribe(value => {
        databaseLoaded = value;
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
            await showPosition(positions[currentPositionIndex]);
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

    databasePathStore.subscribe(value => {
        databaseLoaded = !!value;
    });

    statusBarModeStore.subscribe(value => {
        mode = value;
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
        } else if(event.ctrlKey && event.code == 'KeyS') {
            event.preventDefault();
            saveTranscription();
        } else if(event.ctrlKey && event.code == 'KeyO') {
            loadMatchFromText();
        } else if (event.ctrlKey && event.code === 'KeyQ') {
            exitApp();
        } else if (!event.ctrlKey && event.key === 'PageUp') {
            event.preventDefault();
            firstPosition();
        } else if (!event.ctrlKey && event.key === 'h') {
            firstPosition();
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
            lastPosition();
        } else if (!event.ctrlKey && event.key === 'l') {
            lastPosition();
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

    // Test function to load a match file
    async function loadTestMatch() {
        try {
            const response = await fetch('/match1.txt');
            const content = await response.text();
            const transcription = parseMatchFile(content);
            
            // Update the transcription store
            updateMetadata(transcription.metadata);
            
            // Clear existing games and add new ones
            clearTranscription();
            updateMetadata(transcription.metadata);
            
            for (const game of transcription.games) {
                addGame(game.gameNumber, game.player1Score, game.player2Score);
                const gameIndex = transcription.games.indexOf(game);
                
                for (const move of game.moves) {
                    if (move.cubeAction) {
                        // TODO: Handle cube actions
                    } else {
                        if (move.player1Move) {
                            addMove(gameIndex, move.moveNumber, 1, 
                                move.player1Move.dice, move.player1Move.move,
                                move.player1Move.isIllegal, move.player1Move.isGala);
                        }
                        if (move.player2Move) {
                            addMove(gameIndex, move.moveNumber, 2,
                                move.player2Move.dice, move.player2Move.move,
                                move.player2Move.isIllegal, move.player2Move.isGala);
                        }
                    }
                }
            }
            
            setStatusBarMessage('Test match loaded successfully');
            console.log('Test match loaded:', transcription);
        } catch (error) {
            console.error('Error loading test match:', error);
            setStatusBarMessage('Error loading test match');
        }
    }

    async function newMatch() {
        console.log('newMatch');
        try {
            // Create a new empty transcription
            clearTranscription();
            updateMetadata({
                transcriber: 'User',
                variation: 'Backgammon',
                crawford: 'On',
                matchLength: 7
            });
            transcriptionFilePathStore.set('');
            WindowSetTitle('lazyBG - New Match');
            setStatusBarMessage('New match created - Fill in match details below');
            console.log('New match created');
            
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
                setStatusBarMessage('No transcription to save');
                return;
            }

            // Convert transcription to JSON string
            const jsonContent = JSON.stringify(transcription, null, 2);
            
            // Show save dialog
            const filePath = await SaveTranscriptionDialog();
            if (!filePath) {
                console.log('Save cancelled');
                return;
            }
            
            // Ensure .lbg extension
            const finalPath = filePath.endsWith('.lbg') ? filePath : filePath + '.lbg';
            
            // Save file
            await WriteTextFile(finalPath, jsonContent);
            
            // Update the stored file path
            transcriptionFilePathStore.set(finalPath);
            
            // Update window title
            const filename = finalPath.split('/').pop();
            WindowSetTitle(`lazyBG - ${filename}`);
            
            setStatusBarMessage(`Transcription saved to ${filename}`);
            console.log('Transcription saved to:', finalPath);
        } catch (error) {
            console.error('Error saving transcription:', error);
            setStatusBarMessage('Error saving transcription');
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
                
                // Update the transcription store
                clearTranscription();
                updateMetadata(transcription.metadata);
                
                for (const game of transcription.games) {
                    addGame(game.gameNumber, game.player1Score, game.player2Score);
                    const gameIndex = transcription.games.indexOf(game);
                    
                    for (const move of game.moves) {
                        if (move.cubeAction) {
                            // TODO: Handle cube actions properly
                        } else {
                            if (move.player1Move) {
                                addMove(gameIndex, move.moveNumber, 1, 
                                    move.player1Move.dice, move.player1Move.move,
                                    move.player1Move.isIllegal, move.player1Move.isGala);
                            }
                            if (move.player2Move) {
                                addMove(gameIndex, move.moveNumber, 2,
                                    move.player2Move.dice, move.player2Move.move,
                                    move.player2Move.isIllegal, move.player2Move.isGala);
                            }
                        }
                    }
                }
                
                // Save as .lbg file
                const lbgPath = filePath.replace('.txt', '.lbg');
                // TODO: Save to lbgPath
                transcriptionFilePathStore.set(lbgPath);
                
                WindowSetTitle(`lazyBG - ${getFilenameFromPath(lbgPath)}`);
                setStatusBarMessage('Match loaded successfully');
                
            } else if (filePath.endsWith('.lbg')) {
                // Load .lbg JSON file using backend
                const jsonContent = await ReadTextFile(filePath);
                const transcription = JSON.parse(jsonContent);
                
                clearTranscription();
                transcriptionStore.set(transcription);
                transcriptionFilePathStore.set(filePath);
                
                WindowSetTitle(`lazyBG - ${getFilenameFromPath(filePath)}`);
                setStatusBarMessage('Transcription loaded successfully');
            }
            
        } catch (error) {
            console.error('Error loading match:', error);
            setStatusBarMessage('Error loading match: ' + error.message);
        } finally {
            previousModeStore.set('NORMAL');
            statusBarModeStore.set('NORMAL');
        }
    }

    function getMajorVersion(version) {
        return version.split('.')[0];
    }



    function exitApp() {
        Quit();
    }

    async function savePositionAndAnalysis(positionData, parsedAnalysis, successMessage) {
        // Database functionality removed
        setStatusBarMessage('Database functionality has been removed');
        return;
    }

    export async function importPosition() {
        if ($statusBarModeStore !== 'NORMAL' && !($statusBarModeStore === 'COMMAND' && $previousModeStore === 'NORMAL')) {
            setStatusBarMessage('Cannot import position in current mode');
            return;
        }
        if (!$databasePathStore) {
            setStatusBarMessage('No database opened');
            return;
        }
        try {
            const response = await OpenPositionDialog();

            if (response.error) {
                console.error("Error:", response.error);
                return;
            }

            console.log("File path:", response.file_path);
            console.log("File content:", response.content);

            // Now you can parse and use the file content
            const { positionData, parsedAnalysis } = parsePosition(response.content);
            positionStore.set({ ...positionData, id: 0, board: { ...positionData.board, bearoff: [15, 15] } });
            analysisStore.set({
                positionId: null,
                xgid: parsedAnalysis.xgid,
                player1: '',
                player2: '',
                analysisType: parsedAnalysis.analysisType,
                analysisEngineVersion: parsedAnalysis.analysisEngineVersion,
                checkerAnalysis: { moves: parsedAnalysis.checkerAnalysis },
                doublingCubeAnalysis: {
                    analysisDepth: parsedAnalysis.doublingCubeAnalysis.analysisDepth || '',
                    playerWinChances: parsedAnalysis.doublingCubeAnalysis.playerWinChances || 0,
                    playerGammonChances: parsedAnalysis.doublingCubeAnalysis.playerGammonChances || 0,
                    playerBackgammonChances: parsedAnalysis.doublingCubeAnalysis.playerBackgammonChances || 0,
                    opponentWinChances: parsedAnalysis.doublingCubeAnalysis.opponentWinChances || 0,
                    opponentGammonChances: parsedAnalysis.doublingCubeAnalysis.opponentGammonChances || 0,
                    opponentBackgammonChances: parsedAnalysis.doublingCubeAnalysis.opponentBackgammonChances || 0,
                    cubelessNoDoubleEquity: parsedAnalysis.doublingCubeAnalysis.cubelessNoDoubleEquity || 0,
                    cubelessDoubleEquity: parsedAnalysis.doublingCubeAnalysis.cubelessDoubleEquity || 0,
                    cubefulNoDoubleEquity: parsedAnalysis.doublingCubeAnalysis.cubefulNoDoubleEquity || 0,
                    cubefulNoDoubleError: parsedAnalysis.doublingCubeAnalysis.cubefulNoDoubleError || 0,
                    cubefulDoubleTakeEquity: parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeEquity || 0,
                    cubefulDoubleTakeError: parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeError || 0,
                    cubefulDoublePassEquity: parsedAnalysis.doublingCubeAnalysis.cubefulDoublePassEquity || 0,
                    cubefulDoublePassError: parsedAnalysis.doublingCubeAnalysis.cubefulDoublePassError || 0,
                    bestCubeAction: parsedAnalysis.doublingCubeAnalysis.bestCubeAction || '',
                    wrongPassPercentage: parsedAnalysis.doublingCubeAnalysis.wrongPassPercentage || 0,
                    wrongTakePercentage: parsedAnalysis.doublingCubeAnalysis.wrongTakePercentage || 0
                },
                creationDate: '',
                lastModifiedDate: ''
            });
            console.log('positionStore:', $positionStore);
            console.log('analysisStore:', $analysisStore);

            await savePositionAndAnalysis(positionData, parsedAnalysis, 'Imported position and analysis saved successfully');
        } catch (error) {
            console.error("Error importing position:", error);
        } finally {
            previousModeStore.set('NORMAL');
            statusBarModeStore.set('NORMAL');
        }
    }

    async function pastePosition() {
        if ($statusBarModeStore !== 'NORMAL' && !($statusBarModeStore === 'COMMAND' && $previousModeStore === 'NORMAL')) {
            setStatusBarMessage('Cannot paste position in current mode');
            return;
        }
        if (!$databasePathStore) {
            setStatusBarMessage('No database opened');
            return;
        }
        console.log('pastePosition');
        let promise = ClipboardGetText();
        promise.then(
            async (result) => {
                pastePositionTextStore.set(result);
                console.log('pastePositionTextStore:', $pastePositionTextStore);
                const { positionData, parsedAnalysis } = parsePosition(result);
                await savePositionAndAnalysis(positionData, parsedAnalysis, 'Pasted position and analysis saved successfully');
                statusBarModeStore.set('NORMAL');
            })
            .catch((error) => {
                console.error('Error pasting from clipboard:', error);
            });
    }

    async function saveCurrentPosition() {
        if (!$databasePathStore) {
            setStatusBarMessage('No database opened');
            return;
        }
        if ($statusBarModeStore !== 'EDIT' && !($statusBarModeStore === 'COMMAND' && $previousModeStore === 'EDIT')) {
            setStatusBarMessage('Save is only possible in edit mode');
            return;
        }

        console.log('saveCurrentPosition');

        const position = $positionStore;
        const analysis = $analysisStore;

        if (!isValidPosition(position)) {
            return;
        }

        console.log('Position to save:', position);
        console.log('Analysis to save:', analysis);

        // Reset all fields of analysis to initialized values
        analysis.xgid = generateXGID(position);
        analysis.analysisType = "";
        analysis.checkerAnalysis = { moves: [] };
        analysis.doublingCubeAnalysis = {
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
        };
        analysis.analysisEngineVersion = "";

        await savePositionAndAnalysis(position, analysis, 'Position and analysis saved successfully');
        statusBarModeStore.set('NORMAL');
        previousModeStore.set('NORMAL');
    }

    function parsePosition(fileContent) {
        if (!fileContent || fileContent.trim().length === 0) {
            throw new Error("File is empty or invalid.");
        }

        let normalizedContent = fileContent.replace(/\r\n|\r/g, '\n').trim();
        const lines = normalizedContent.split('\n').map(line => line.trim());

        const isFrench = normalizedContent.includes("Joueur") || normalizedContent.includes("Adversaire") || normalizedContent.includes("Videau");
        const isJapanese = normalizedContent.includes("プレーヤー") || normalizedContent.includes("対戦相手") || normalizedContent.includes("キューブ");
        const isInternalCheckerAnalysisFormat = normalizedContent.includes("Analysis:\nChecker Move Analysis:");
        const isInternalDoublingAnalysisFormat = normalizedContent.includes("Analysis:\nDoubling Cube Analysis:");
        const isGerman = normalizedContent.includes("Spieler") || normalizedContent.includes("Gegner") || normalizedContent.includes("Dopplerwürfel");

        // Replace commas with dots. Enable to treat decimal numbers with commas as well.
        normalizedContent = normalizedContent.replace(/,/g, '.');

        const xgidLine = lines.find(line => line.startsWith("XGID="));
        const xgid = xgidLine ? xgidLine.split('=')[1] : null;

        if (!xgid) {
            throw new Error("XGID not found in the file content.");
        }

        const [
            positionPart, 
            cubeValue, 
            cubeOwner, 
            playerDownOnDiagram, 
            dicePart, 
            score1, 
            score2, 
            isCrawford, 
            matchLength, 
            dummy
        ] = xgid.split(":");

        const board = { points: Array(26).fill({ checkers: 0, color: -1 }) };

        if (positionPart) {
            const pointChars = positionPart.split('');
            let pointIndex = 0;
            pointChars.forEach(char => {
                if (char >= 'A' && char <= 'Z') {
                    const numCheckers = char.charCodeAt(0) - 'A'.charCodeAt(0) + 1;
                    board.points[pointIndex] = { checkers: numCheckers, color: 0 };
                } else if (char >= 'a' && char <= 'z') {
                    const numCheckers = char.charCodeAt(0) - 'a'.charCodeAt(0) + 1;
                    board.points[pointIndex] = { checkers: numCheckers, color: 1 };
                }
                pointIndex++;
            });
        }

        const diceValues = dicePart ? dicePart.split("").map(num => parseInt(num)) : [0, 0];
        const dice = [diceValues[0], diceValues[1]];

        const player1Score = parseInt(score1);
        const player2Score = parseInt(score2);
        const matchLengthValue = parseInt(matchLength);
        const playerOnRoll = parseInt(playerDownOnDiagram) === 1 ? 0 : 1;

        let hasJacoby = 0, hasBeaver = 0, awayScores = [matchLengthValue - player1Score, matchLengthValue - player2Score];
        if (parseInt(isCrawford) === 0) {
            awayScores = awayScores.map(score => score === 1 ? 0 : score);
        }
        if (matchLengthValue === 0) {
            awayScores = [-1, -1];
            switch (parseInt(isCrawford)) {
                case 1: hasJacoby = 1; break;
                case 2: hasBeaver = 1; break;
                case 3: hasJacoby = 1; hasBeaver = 1; break;
            }
        }

        const player1Bearoff = 15 - board.points.reduce((sum, point) => sum + (point.color === 0 ? point.checkers : 0), 0);
        const player2Bearoff = 15 - board.points.reduce((sum, point) => sum + (point.color === 1 ? point.checkers : 0), 0);
        board.bearoff = [player1Bearoff, player2Bearoff];

        const decisionLine = lines.find(line => line.includes(isFrench ? "jouer" : isJapanese ? "to play" : isGerman ? "spielen" : "to play"));
        const decisionType = decisionLine || isInternalCheckerAnalysisFormat ? 0 : 1;

        const positionData = {
            board: board,
            cube: {
                owner: parseInt(cubeOwner) === 1 ? 0 : parseInt(cubeOwner) === -1 ? 1 : -1,
                value: parseInt(cubeValue)
            },
            dice: dice,
            score: awayScores,
            player_on_roll: playerOnRoll,
            decision_type: decisionType,
            has_jacoby: hasJacoby,
            has_beaver: hasBeaver,
        };

        const parsedAnalysis = { xgid, analysisType: "", checkerAnalysis: [], doublingCubeAnalysis: {}, analysisEngineVersion: "" };

        const engineVersionMatch = normalizedContent.match(/eXtreme Gammon Version: ([\d.]+)(?:. MET: (.+))?/); // remember comma transformed in dot
        if (engineVersionMatch) {
            parsedAnalysis.analysisEngineVersion = `eXtreme Gammon Version: ${engineVersionMatch[1]}`;
            if (engineVersionMatch[2]) {
                parsedAnalysis.analysisEngineVersion += `, MET: ${engineVersionMatch[2]}`;
            }
        }

        if (isInternalDoublingAnalysisFormat) {
            parsedAnalysis.analysisType = "DoublingCube";
            const doublingCubeAnalysisRegex = /Doubling Cube Analysis:\nAnalysis Depth: "(.+)"\nPlayer Win Chances: ([-.\d]+)%\nPlayer Gammon Chances: ([-.\d]+)%\nPlayer Backgammon Chances: ([-.\d]+)%\nOpponent Win Chances: ([-.\d]+)%\nOpponent Gammon Chances: ([-.\d]+)%\nOpponent Backgammon Chances: ([-.\d]+)%\nCubeless No Double Equity: ([-.\d]+)\nCubeless Double Equity: ([-.\d]+)\nCubeful No Double Equity: ([-.\d]+)\nCubeful No Double Error: ([-.\d]+)\nCubeful Double Take Equity: ([-.\d]+)\nCubeful Double Take Error: ([-.\d]+)\nCubeful Double Pass Equity: ([-.\d]+)\nCubeful Double Pass Error: ([-.\d]+)\nBest Cube Action: (.+)\nWrong Pass Percentage: ([-.\d]+)%\nWrong Take Percentage: ([-.\d]+)%/;
            const doublingCubeMatch = doublingCubeAnalysisRegex.exec(normalizedContent);
            if (doublingCubeMatch) {
                parsedAnalysis.doublingCubeAnalysis = {
                    analysisDepth: doublingCubeMatch[1].trim(),
                    playerWinChances: parseFloat(doublingCubeMatch[2]),
                    playerGammonChances: parseFloat(doublingCubeMatch[3]),
                    playerBackgammonChances: parseFloat(doublingCubeMatch[4]),
                    opponentWinChances: parseFloat(doublingCubeMatch[5]),
                    opponentGammonChances: parseFloat(doublingCubeMatch[6]),
                    opponentBackgammonChances: parseFloat(doublingCubeMatch[7]),
                    cubelessNoDoubleEquity: parseFloat(doublingCubeMatch[8]),
                    cubelessDoubleEquity: parseFloat(doublingCubeMatch[9]),
                    cubefulNoDoubleEquity: parseFloat(doublingCubeMatch[10]),
                    cubefulNoDoubleError: parseFloat(doublingCubeMatch[11]),
                    cubefulDoubleTakeEquity: parseFloat(doublingCubeMatch[12]),
                    cubefulDoubleTakeError: parseFloat(doublingCubeMatch[13]),
                    cubefulDoublePassEquity: parseFloat(doublingCubeMatch[14]),
                    cubefulDoublePassError: parseFloat(doublingCubeMatch[15]),
                    bestCubeAction: doublingCubeMatch[16].trim(),
                    wrongPassPercentage: parseFloat(doublingCubeMatch[17]),
                    wrongTakePercentage: parseFloat(doublingCubeMatch[18])
                };
            }
        } else if (isInternalCheckerAnalysisFormat) {
            parsedAnalysis.analysisType = "CheckerMove";
            const moveRegex = /^Move (\d+): (.+)\nAnalysis Depth: "(.+)"\nEquity: ([-.\d]+)\nEquity Error: ([-.\d]+)\nPlayer Win Chance: ([-.\d]+)%\nPlayer Gammon Chance: ([-.\d]+)%\nPlayer Backgammon Chance: ([-.\d]+)%\nOpponent Win Chance: ([-.\d]+)%\nOpponent Gammon Chance: ([-.\d]+)%\nOpponent Backgammon Chance: ([-.\d]+)%/gm;
            let moveMatch;
            while ((moveMatch = moveRegex.exec(normalizedContent)) !== null) {
                const moveDetails = {
                    index: parseInt(moveMatch[1], 10),
                    move: moveMatch[2].trim(),
                    analysisDepth: moveMatch[3].trim(),
                    equity: parseFloat(moveMatch[4]),
                    equityError: parseFloat(moveMatch[5]),
                    playerWinChance: parseFloat(moveMatch[6]),
                    playerGammonChance: parseFloat(moveMatch[7]),
                    playerBackgammonChance: parseFloat(moveMatch[8]),
                    opponentWinChance: parseFloat(moveMatch[9]),
                    opponentGammonChance: parseFloat(moveMatch[10]),
                    opponentBackgammonChance: parseFloat(moveMatch[11]),
                };
                parsedAnalysis.checkerAnalysis.push(moveDetails);
            }
        } else if (/^ {4}(\d+)\./gm.test(normalizedContent)) {
            parsedAnalysis.analysisType = "CheckerMove";
            const moveRegex = new RegExp(
                isFrench
                ? /^ {4}(\d+)\.\s(.{11})\s(.{28})\séq:(.{5,7})\s(?:\((-?[-.\d]{5,7})\))?/
                : isJapanese
                ? /^ {4}(\d+)\.\s(.{11})\s(.{28})\seq:(.{5,7})\s(?:\((-?[-.\d]{5,7})\))?/
                : isGerman
                ? /^ {4}(\d+)\.\s(.{11})\s(.{28})\seq:(.{5,7})\s(?:\((-?[-.\d]{5,7})\))?/
                : /^ {4}(\d+)\.\s(.{11})\s(.{28})\seq:(.{5,7})\s(?:\((-?[-.\d]{5,7})\))?/,
                'gm'
            );
            let moveMatch;
            while ((moveMatch = moveRegex.exec(normalizedContent)) !== null) {
                const moveDetails = {
                    index: parseInt(moveMatch[1], 10),
                    analysisDepth: moveMatch[2].trim(),
                    move: moveMatch[3].trim(),
                    equity: parseFloat(moveMatch[4]),
                    equityError: moveMatch[5] ? parseFloat(moveMatch[5]) : 0,
                };
                const lineStart = moveMatch.index + moveMatch[0].length;
                const remainingContent = normalizedContent.slice(lineStart);
                const playerRegex = isFrench
                    ? /Joueur:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/
                    : isJapanese
                    ? /プレーヤー:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/
                    : isGerman
                    ? /Spieler:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/
                    : /Player:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/;
                const opponentRegex = isFrench
                    ? /Adversaire:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/
                    : isJapanese
                    ? /対戦相手:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/
                    : isGerman
                    ? /Gegner:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/
                    : /Opponent:\s*(\d+\.\d+)%.*\(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/;
                const playerMatch = playerRegex.exec(remainingContent);
                const opponentMatch = opponentRegex.exec(remainingContent);
                if (playerMatch) {
                    moveDetails.playerWinChance = parseFloat(playerMatch[1]);
                    moveDetails.playerGammonChance = parseFloat(playerMatch[2]);
                    moveDetails.playerBackgammonChance = parseFloat(playerMatch[3]);
                }
                if (opponentMatch) {
                    moveDetails.opponentWinChance = parseFloat(opponentMatch[1]);
                    moveDetails.opponentGammonChance = parseFloat(opponentMatch[2]);
                    moveDetails.opponentBackgammonChance = parseFloat(opponentMatch[3]);
                }
                parsedAnalysis.checkerAnalysis.push(moveDetails);
            }
            if (playerOnRoll === 1) {
                // Swap win, gammon, and backgammon chances between player and opponent for each move
                parsedAnalysis.checkerAnalysis.forEach(move => {
                    const tempWinChance = move.playerWinChance;
                    const tempGammonChance = move.playerGammonChance;
                    const tempBackgammonChance = move.playerBackgammonChance;

                    move.playerWinChance = move.opponentWinChance;
                    move.playerGammonChance = move.opponentGammonChance;
                    move.playerBackgammonChance = move.opponentBackgammonChance;

                    move.opponentWinChance = tempWinChance;
                    move.opponentGammonChance = tempGammonChance;
                    move.opponentBackgammonChance = tempBackgammonChance;
                });
            }
        } else if (
            (isFrench && (normalizedContent.includes("Equités sans videau") || normalizedContent.includes("Equités avec videau"))) ||
            (isJapanese && (normalizedContent.includes("Cubeless Equities") || normalizedContent.includes("Cubeful Equities"))) ||
            (isGerman && (normalizedContent.includes("Equities ohne Dopplerwürfel") || normalizedContent.includes("Equities mit Dopplerwürfel"))) ||
            (!isFrench && !isJapanese && !isGerman && (normalizedContent.includes("Cubeless Equities") || "Cubeful Equities"))
        ) {
            parsedAnalysis.analysisType = "DoublingCube";

            const analysisDepthMatch = normalizedContent.match(new RegExp(isFrench ? /Analysé avec\s+([^\n]*)/ : isJapanese ? /Analyzed in\s+([^\n]*)/ : isGerman ? /Analysiert in\s+([^\n]*)/ : /Analyzed in\s+([^\n]*)/));

            const playerWinMatch = normalizedContent.match(new RegExp(isFrench ? /Chance de gain du joueur:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/ : isJapanese ? /Player Winning Chances:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/ : isGerman ? /Spieler Gewinnchancen:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/ : /Player Winning Chances:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/));

            const opponentWinMatch = normalizedContent.match(new RegExp(isFrench ? /Chance de gain de l'adversaire:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/ : isJapanese ? /Opponent Winning Chances:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/ : isGerman ? /Gewinnchancen des Gegners:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/ : /Opponent Winning Chances:\s+(\d+\.\d+)% \(G:(\d+\.\d+)% B:(\d+\.\d+)%\)/));

            const cubelessMatch = normalizedContent.match(new RegExp(isFrench ? /Equités sans videau\s*:\s*Pas de double=([\+\-\d.]+).\s*Double=([\+\-\d.]+)/ : isJapanese ? /Cubeless Equities:\s*No Double=([\+\-\d.]+).\s*Double=([\+\-\d.]+)./ : isGerman ? /Equities ohne Dopplerwürfel\s*:\s*Nicht Doppeln=([\+\-\d.]+).\s*Doppeln=([\+\-\d.]+)/ : /Cubeless Equities:\s*No Double=([\+\-\d.]+).\s*Double=([\+\-\d.]+)/));

            const cubefulNoDoubleMatch = normalizedContent.match(new RegExp(isFrench ? /Pas de double\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isJapanese ? /ノーダブル\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isGerman ? /Nicht Doppeln\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : /No double\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/));
            const cubefulDoubleTakeMatch = normalizedContent.match(new RegExp(isFrench ? /Double\/Prend:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isJapanese ? /ダブル\/テイク:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isGerman ? /Doppeln\/Annehmen:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : /Double\/Take:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/));
            const cubefulDoublePassMatch = normalizedContent.match(new RegExp(isFrench ? /Double\/Passe:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isJapanese ? /ダブル\/パス:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isGerman ? /Doppeln\/Ablehnen:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : /Double\/Pass:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/));
            const redoubleNoMatch = normalizedContent.match(new RegExp(isFrench ? /Pas de redouble\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isJapanese ? /ノーリダブル\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isGerman ? /Nicht Redoppeln\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : /No redouble\s*:\s*([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/));
            const redoubleTakeMatch = normalizedContent.match(new RegExp(isFrench ? /Redouble\/Prend:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isJapanese ? /リダブル\/テイク:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isGerman ? /Redoppeln\/Annehmen:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : /Redouble\/Take:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/));
            const redoublePassMatch = normalizedContent.match(new RegExp(isFrench ? /Redouble\/Passe:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isJapanese ? /リダブル\/パス:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isGerman ? /Redoppeln\/Ablehnen:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : /Redouble\/Pass:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/));

            const cubefulDoubleBeaverMatch = normalizedContent.match(new RegExp(isFrench ? /Double\/Beaver:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isJapanese ? /ダブル\/ビーバー:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : isGerman ? /Doppeln\/Beaver:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/ : /Double\/Beaver:\s+([\+\-\d.]+)(?: \(([\+\-\d.]+)\))?/));

            const bestCubeActionMatch = normalizedContent.match(new RegExp(isFrench ? /Meilleur action du videau:\s*(.*)/ : isJapanese ? /ベストキューブアクション：\s*(.*)/ : isGerman ? /Beste Dopplerwürfel Aktion\s*(.*)/ : /Best Cube action:\s*(.*)/));

            const wrongPassPercentageMatch = normalizedContent.match(new RegExp(isFrench ? /Pourcentage de passes incorrectes pour rendre la décision de double correcte:\s*(\d+\.\d+)%/ : isJapanese ? /ダブルを正当化するのに必要な相手がパスする確率:\s*(\d+\.\d+)%/ : isGerman ? /Prozent von falschen Ablehnen gebraucht damit Doppelentscheidung richtig wäre.:\s*(\d+\.\d+)%/ : /Percentage of wrong pass needed to make the double decision right:\s*(\d+\.\d+)%/));
            const wrongTakePercentageMatch = normalizedContent.match(new RegExp(isFrench ? /Pourcentage de prises incorrectes pour rendre la décision de double correcte:\s*(\d+\.\d+)%/ : isJapanese ? /ダブルを正当化するのに必要な相手がテイクする確率:\s*(\d+\.\d+)%/ : isGerman ? /Prozent von falschen Annehmen gebraucht damit Doppelentscheidung richtig wäre.:\s*(\d+\.\d+)%/ : /Percentage of wrong take needed to make the double decision right:\s*(\d+\.\d+)%/));

            if (analysisDepthMatch) {
                parsedAnalysis.doublingCubeAnalysis.analysisDepth = analysisDepthMatch[1].trim();
            }
            if (playerWinMatch) {
                parsedAnalysis.doublingCubeAnalysis.playerWinChances = parseFloat(playerWinMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.playerGammonChances = parseFloat(playerWinMatch[2]);
                parsedAnalysis.doublingCubeAnalysis.playerBackgammonChances = parseFloat(playerWinMatch[3]);
            }
            if (opponentWinMatch) {
                parsedAnalysis.doublingCubeAnalysis.opponentWinChances = parseFloat(opponentWinMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.opponentGammonChances = parseFloat(opponentWinMatch[2]);
                parsedAnalysis.doublingCubeAnalysis.opponentBackgammonChances = parseFloat(opponentWinMatch[3]);
            }
            if (cubelessMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubelessNoDoubleEquity = parseFloat(cubelessMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubelessDoubleEquity = parseFloat(cubelessMatch[2]);
            }
            if (cubefulNoDoubleMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubefulNoDoubleEquity = parseFloat(cubefulNoDoubleMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubefulNoDoubleError = cubefulNoDoubleMatch[2] ? parseFloat(cubefulNoDoubleMatch[2]) : 0;
            }
            if (cubefulDoubleTakeMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeEquity = parseFloat(cubefulDoubleTakeMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeError = cubefulDoubleTakeMatch[2] ? parseFloat(cubefulDoubleTakeMatch[2]) : 0;
            }
            if (cubefulDoublePassMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubefulDoublePassEquity = parseFloat(cubefulDoublePassMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubefulDoublePassError = cubefulDoublePassMatch[2] ? parseFloat(cubefulDoublePassMatch[2]) : 0;
            }
            if (redoubleNoMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubefulNoDoubleEquity = parseFloat(redoubleNoMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubefulNoDoubleError = redoubleNoMatch[2] ? parseFloat(redoubleNoMatch[2]) : 0;
            }
            if (redoubleTakeMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeEquity = parseFloat(redoubleTakeMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeError = redoubleTakeMatch[2] ? parseFloat(redoubleTakeMatch[2]) : 0;
            }
            if (redoublePassMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubefulDoublePassEquity = parseFloat(redoublePassMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubefulDoublePassError = redoublePassMatch[2] ? parseFloat(redoublePassMatch[2]) : 0;
            }
            if (cubefulDoubleBeaverMatch) {
                parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeEquity = parseFloat(cubefulDoubleBeaverMatch[1]);
                parsedAnalysis.doublingCubeAnalysis.cubefulDoubleTakeError = cubefulDoubleBeaverMatch[2] ? parseFloat(cubefulDoubleBeaverMatch[2]) : 0;
            }
            if (bestCubeActionMatch) {
                parsedAnalysis.doublingCubeAnalysis.bestCubeAction = bestCubeActionMatch[1].trim();
            }
            if (wrongPassPercentageMatch) {
                parsedAnalysis.doublingCubeAnalysis.wrongPassPercentage = parseFloat(wrongPassPercentageMatch[1]);
            }
            if (wrongTakePercentageMatch) {
                parsedAnalysis.doublingCubeAnalysis.wrongTakePercentage = parseFloat(wrongTakePercentageMatch[1]);
            }

            if (playerOnRoll === 1) {
                // Swap win, gammon, and backgammon chances between player and opponent
                const tempWinChances = parsedAnalysis.doublingCubeAnalysis.playerWinChances;
                const tempGammonChances = parsedAnalysis.doublingCubeAnalysis.playerGammonChances;
                const tempBackgammonChances = parsedAnalysis.doublingCubeAnalysis.playerBackgammonChances;

                parsedAnalysis.doublingCubeAnalysis.playerWinChances = parsedAnalysis.doublingCubeAnalysis.opponentWinChances;
                parsedAnalysis.doublingCubeAnalysis.playerGammonChances = parsedAnalysis.doublingCubeAnalysis.opponentGammonChances;
                parsedAnalysis.doublingCubeAnalysis.playerBackgammonChances = parsedAnalysis.doublingCubeAnalysis.opponentBackgammonChances;

                parsedAnalysis.doublingCubeAnalysis.opponentWinChances = tempWinChances;
                parsedAnalysis.doublingCubeAnalysis.opponentGammonChances = tempGammonChances;
                parsedAnalysis.doublingCubeAnalysis.opponentBackgammonChances = tempBackgammonChances;
            }
        }

        // Extract comment section
        const commentSection = extractCommentSection(normalizedContent, parsedAnalysis.analysisType === "DoublingCube");
        parsedAnalysis.comment = commentSection;

        return { positionData, parsedAnalysis };
    }

    function extractCommentSection(content, isDoublingCube) {
        if (isDoublingCube) {
            const commentRegex = /(?:Best Cube action: .+|Meilleur action du videau: .+|Percentage of wrong .+|Pourcentage de passes incorrectes .+%)\n\n([\s\S]+?)\n\neXtreme Gammon Version:/;

            let match = commentRegex.exec(content);
            return match ? match[1].trim() : '';
        } else {
            const lines = content.split('\n');
            let lastOpponentIndex = -1;

            // Find the last line where "Opponent" appears
            for (let i = lines.length - 1; i >= 0; i--) {
                if (lines[i].includes('Opponent') || lines[i].includes('Adversaire')) {
                    lastOpponentIndex = i;
                    break;
                }
            }

            if (lastOpponentIndex === -1) {
                return '';
            }

            // Count 2 blank lines after the last "Opponent" line
            let blankLineCount = 0;
            let commentStartIndex = -1;
            for (let i = lastOpponentIndex + 1; i < lines.length; i++) {
                if (lines[i].trim() === '') {
                    blankLineCount++;
                } else {
                    blankLineCount = 0;
                }

                if (blankLineCount === 2) {
                    commentStartIndex = i + 1;
                    break;
                }
            }

            if (commentStartIndex === -1) {
                return '';
            }

            // Extract the comment section until the next blank line before the engine version
            let commentEndIndex = -1;
            for (let i = commentStartIndex; i < lines.length; i++) {
                if (lines[i].trim() === '' && lines[i + 1] && lines[i + 1].startsWith('eXtreme Gammon Version:')) {
                    commentEndIndex = i;
                    break;
                }
            }

            if (commentEndIndex === -1) {
                return '';
            }

            const commentLines = lines.slice(commentStartIndex, commentEndIndex);
            return commentLines.join('\n').trim();
        }
    }

    function generateXGID(position) {
        const { board, cube, dice, score, player_on_roll, decision_type } = position;

        // Encode board positions
        let positionPart = '';
        for (let i = 0; i < 26; i++) {
            const point = board.points[i];
            if (point.checkers > 0) {
                const charCode = point.color === 0 ? 'A'.charCodeAt(0) : 'a'.charCodeAt(0);
                positionPart += String.fromCharCode(charCode + point.checkers - 1);
            } else {
                positionPart += '-';
            }
        }

        // Encode cube value and owner
        const cubeValue = cube.value;
        const cubeOwner = cube.owner === 0 ? 1 : cube.owner === 1 ? -1 : 0;

        // Encode dice
        const dicePart = decision_type === 1 ? '00' : dice.join('');

        // Compute minimal match length
        const matchLength = score[0] === -1 || score[1] === -1 ? 0 : Math.max(score[0], score[1]);

        // Deduce actual scores
        const actualScore1 = score[0] === -1 ? 0 : matchLength - score[0];
        const actualScore2 = score[1] === -1 ? 0 : matchLength - score[1];

        // Encode Crawford status
        const isCrawford = score[0] === 1 || score[1] === 1 ? 1 : 0;

        // Encode dummy value (not used)
        const dummy = 0;

        // Correctly encode player on roll
        const playerOnRoll = player_on_roll === 0 ? 1 : -1;

        // Combine all parts to form the XGID
        const xgid = `${positionPart}:${cubeValue}:${cubeOwner}:${playerOnRoll}:${dicePart}:${actualScore1}:${actualScore2}:${isCrawford}:${matchLength}:${dummy}`;
        return xgid;
    }

    function copyPosition() {
        if ($statusBarModeStore !== 'NORMAL' && !($statusBarModeStore === 'COMMAND' && $previousModeStore === 'NORMAL')) {
            setStatusBarMessage('Cannot copy position in current mode');
            return;
        }
        if (!$databasePathStore) {
            setStatusBarMessage('No database opened');
            return;
        }
        // @ts-ignore
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot copy position in edit mode');
            return;
        }
        console.log('copyPosition');
        const position = $positionStore;
        const analysis = $analysisStore;

        // Generate XGID if not present in the analysis
        const xgid = analysis.xgid || generateXGID(position);

        // Construct the clipboard content
        let clipboardContent = `XGID=${xgid}\n\n`;

        // Add position details
        clipboardContent += `Position:\n`;
        clipboardContent += `Board: ${JSON.stringify(position.board)}\n`;
        clipboardContent += `Cube: ${JSON.stringify(position.cube)}\n`;
        clipboardContent += `Dice: ${position.dice.join(', ')}\n`;
        clipboardContent += `Score: ${position.score.join(', ')}\n`;
        clipboardContent += `Player on roll: ${position.player_on_roll}\n`;
        clipboardContent += `Decision type: ${position.decision_type}\n\n`;

        // Add analysis details
        clipboardContent += `Analysis:\n`;
        if (analysis.analysisType === "DoublingCube") {
            clipboardContent += `Doubling Cube Analysis:\n`;
            clipboardContent += `Analysis Depth: "${analysis.doublingCubeAnalysis.analysisDepth}"\n`;
            clipboardContent += `Player Win Chances: ${analysis.doublingCubeAnalysis.playerWinChances}%\n`;
            clipboardContent += `Player Gammon Chances: ${analysis.doublingCubeAnalysis.playerGammonChances}%\n`;
            clipboardContent += `Player Backgammon Chances: ${analysis.doublingCubeAnalysis.playerBackgammonChances}%\n`;
            clipboardContent += `Opponent Win Chances: ${analysis.doublingCubeAnalysis.opponentWinChances}%\n`;
            clipboardContent += `Opponent Gammon Chances: ${analysis.doublingCubeAnalysis.opponentGammonChances}%\n`;
            clipboardContent += `Opponent Backgammon Chances: ${analysis.doublingCubeAnalysis.opponentBackgammonChances}%\n`;
            clipboardContent += `Cubeless No Double Equity: ${analysis.doublingCubeAnalysis.cubelessNoDoubleEquity}\n`;
            clipboardContent += `Cubeless Double Equity: ${analysis.doublingCubeAnalysis.cubelessDoubleEquity}\n`;
            clipboardContent += `Cubeful No Double Equity: ${analysis.doublingCubeAnalysis.cubefulNoDoubleEquity}\n`;
            clipboardContent += `Cubeful No Double Error: ${analysis.doublingCubeAnalysis.cubefulNoDoubleError}\n`;
            clipboardContent += `Cubeful Double Take Equity: ${analysis.doublingCubeAnalysis.cubefulDoubleTakeEquity}\n`;
            clipboardContent += `Cubeful Double Take Error: ${analysis.doublingCubeAnalysis.cubefulDoubleTakeError}\n`;
            clipboardContent += `Cubeful Double Pass Equity: ${analysis.doublingCubeAnalysis.cubefulDoublePassEquity}\n`;
            clipboardContent += `Cubeful Double Pass Error: ${analysis.doublingCubeAnalysis.cubefulDoublePassError}\n`;
            clipboardContent += `Best Cube Action: ${analysis.doublingCubeAnalysis.bestCubeAction}\n`;
            clipboardContent += `Wrong Pass Percentage: ${analysis.doublingCubeAnalysis.wrongPassPercentage}%\n`;
            clipboardContent += `Wrong Take Percentage: ${analysis.doublingCubeAnalysis.wrongTakePercentage}%\n`;
        } else if (analysis.analysisType === "CheckerMove") {
            clipboardContent += `Checker Move Analysis:\n`;
            analysis.checkerAnalysis.moves.forEach(move => {
                clipboardContent += `Move ${move.index}: ${move.move}\n`;
                clipboardContent += `Analysis Depth: "${move.analysisDepth}"\n`;
                clipboardContent += `Equity: ${move.equity}\n`;
                if (move.equityError !== undefined) {
                    clipboardContent += `Equity Error: ${move.equityError}\n`;
                }
                clipboardContent += `Player Win Chance: ${move.playerWinChance}%\n`;
                clipboardContent += `Player Gammon Chance: ${move.playerGammonChance}%\n`;
                clipboardContent += `Player Backgammon Chance: ${move.playerBackgammonChance}%\n`;
                clipboardContent += `Opponent Win Chance: ${move.opponentWinChance}%\n`;
                clipboardContent += `Opponent Gammon Chance: ${move.opponentGammonChance}%\n`;
                clipboardContent += `Opponent Backgammon Chance: ${move.opponentBackgammonChance}%\n\n`;
            });
        }

        // Add engine version
        if (analysis.analysisEngineVersion) {
            clipboardContent += `Engine Version: ${analysis.analysisEngineVersion}\n`;
        }

        // Copy to clipboard
        navigator.clipboard.writeText(clipboardContent).then(() => {
            console.log('Position and analysis copied to clipboard');
            setStatusBarMessage('Position and analysis copied to clipboard');
        }).catch(err => {
            console.error('Error copying to clipboard:', err);
            setStatusBarMessage('Error copying to clipboard');
        });
    }

    function isValidPosition(position) {
        const player1Checkers = position.board.points.reduce((acc, point) => acc + (point.color === 0 ? point.checkers : 0), 0);
        const player2Checkers = position.board.points.reduce((acc, point) => acc + (point.color === 1 ? point.checkers : 0), 0);

        if (player1Checkers > 15) {
            setStatusBarMessage('Invalid position: Player 1 has more than 15 checkers');
            return false;
        }

        if (player2Checkers > 15) {
            setStatusBarMessage('Invalid position: Player 2 has more than 15 checkers');
            return false;
        }

        if (player1Checkers === 0) {
            setStatusBarMessage('Invalid position: Player 1 has already borne off all checkers');
            return false;
        }

        if (player2Checkers === 0) {
            setStatusBarMessage('Invalid position: Player 2 has already borne off all checkers');
            return false;
        }

        if (position.decision_type === 1) {
            if (position.cube.owner !== position.player_on_roll && position.cube.owner !== -1) {
                setStatusBarMessage('Invalid position: Cube is not available for doubling');
                return false;
            }
            if (position.score[position.player_on_roll] === 1) {
                setStatusBarMessage('Invalid position: Crawford rule prevents doubling');
                return false;
            }
        }

        if ((position.score[0] === -1 && position.score[1] !== -1) || (position.score[1] === -1 && position.score[0] !== -1)) {
            setStatusBarMessage('Invalid position: Both players must have unlimited score or neither');
            return false;
        }

        return true;
    }

    async function deletePosition() {
        // Database functionality removed
        setStatusBarMessage('Database functionality has been removed');
        return;
    }

    async function updatePosition() {
        // Database functionality removed
        setStatusBarMessage('Database functionality has been removed');
        return;
    }

    function firstPosition() {
        if ($statusBarModeStore === 'EDIT') {
            setStatusBarMessage('Cannot browse positions in edit mode');
            return;
        }
        
        // Navigate to first move of first game in transcription
        if ($transcriptionStore && $transcriptionStore.games && $transcriptionStore.games.length > 0) {
            selectedMoveStore.set({ gameIndex: 0, moveIndex: 0, player: 1 });
        } else if ($databasePathStore && positions && positions.length > 0) {
            // Fallback to database mode
            currentPositionIndexStore.set(0);
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
        } else if ($databasePathStore && positions && $currentPositionIndexStore > 0) {
            // Fallback to database mode
            currentPositionIndexStore.set($currentPositionIndexStore - 1);
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
        } else if ($databasePathStore && positions && $currentPositionIndexStore < positions.length - 1) {
            // Fallback to database mode
            currentPositionIndexStore.set($currentPositionIndexStore + 1);
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
        } else if ($databasePathStore && positions && positions.length > 0) {
            // Fallback to database mode
            currentPositionIndexStore.set(positions.length - 1);
        }
    }

    function gotoPosition() {
        if (!$databasePathStore && (!$transcriptionStore || !$transcriptionStore.games || $transcriptionStore.games.length === 0)) {
            setStatusBarMessage('No transcription or database opened');
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
        if (!$databasePathStore) {
            setStatusBarMessage('No database opened');
            statusBarModeStore.set('NORMAL');
            return;
        }
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
        if (databaseLoaded) {
            if (mode === 'EDIT') {
                setStatusBarMessage('Cannot show metadata modal in edit mode');
            } else {
                showMetadataModalStore.set(!showMetadataModal);
            }
        }
    }

    function toggleCandidateMovesPanel() {
        console.log('toggleCandidateMovesPanel');
        if (!databaseLoaded) {
            statusBarTextStore.set('No database loaded');
            return;
        }
        showCandidateMovesStore.set(!showCandidateMoves);
    }

    function toggleMovesTablePanel() {
        console.log('toggleMovesTablePanel');
        if (!databaseLoaded) {
            statusBarTextStore.set('No database loaded');
            return;
        }
        showMovesTableStore.set(!showMovesTable);
    }


    async function loadAllPositions() {
        // Database functionality removed
        setStatusBarMessage('Database functionality has been removed');
        return;
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
        onNewDatabase={newMatch}
        onOpenDatabase={loadMatchFromText}
        onExit={exitApp}
        onFirstPosition={firstPosition}
        onPreviousPosition={previousPosition}
        onNextPosition={nextPosition}
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
                onNewDatabase={newMatch}
                onOpenDatabase={loadMatchFromText}
                exitApp={exitApp}
            />
        </div>

        {#if showCandidateMoves}
        <div class="candidate-moves-column">
            <CandidateMovesPanel />
        </div>
        {/if}
    </div>

    <MetadataPanel visible={showMetadataPanel} onClose={() => showMetadataPanelStore.set(false)} />

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
        min-height: 100vh;
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
    }

    .moves-table-column {
        width: 300px;
        min-width: 200px;
        max-width: 400px;
        border-right: 1px solid #ccc;
        overflow-y: auto;
        background-color: #f9f9f9;
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
        padding: 5px;
        box-sizing: border-box;
        width: 100%;
    }

    .candidate-moves-column {
        width: 350px;
        min-width: 250px;
        max-width: 500px;
        border-left: 1px solid #ccc;
        overflow-y: auto;
        background-color: #f9f9f9;
    }
</style>
