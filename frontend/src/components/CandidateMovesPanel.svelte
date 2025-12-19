<script>
    import { onMount, onDestroy } from 'svelte';
    import { get } from 'svelte/store';
    import { positionStore } from '../stores/positionStore.js';
    import { selectedMoveStore, transcriptionStore } from '../stores/transcriptionStore.js';
    import { candidatePreviewMoveStore } from '../stores/uiStore.js';
    import { GetCandidateMoves } from '../../wailsjs/go/main/App.js';
    import { updateMove, invalidatePositionsCacheFrom } from '../stores/transcriptionStore.js';
    import { undoRedoStore } from '../stores/undoRedoStore.js';
    import { calculatePositionAtMove } from '../utils/positionCalculator.js';
    
    // Helper function to parse dice string (e.g., "54" -> [5, 4])
    function parseDice(diceStr) {
        if (!diceStr || diceStr.trim() === '') return [0, 0];
        if (diceStr.length >= 2) {
            return [parseInt(diceStr[0]), parseInt(diceStr[1])];
        }
        return [0, 0];
    }
    
    export let visible = true;
    
    let candidateMoves = [];
    let loading = false;
    let error = null;
    let selectedIndex = 0;
    let currentPlayerMove = '';
    let basePosition = null; // Store the position before any move is applied (for cycling through candidates)
    
    // Cache for candidate moves analysis results
    // Key format: "gameIndex:moveIndex:player"
    let analysisCache = new Map();
    
    // Subscribe to selected move changes
    let unsubscribeSelectedMove;
    
    // Export to allow parent to get best move
    export function getBestMove() {
        return candidateMoves.length > 0 ? candidateMoves[0].move : '';
    }
    
    // Export function to invalidate cache when moves are modified
    export function invalidateCache(gameIndex, moveIndex, player = null) {
        if (player !== null) {
            // Invalidate specific player's move
            const cacheKey = `${gameIndex}:${moveIndex}:${player}`;
            analysisCache.delete(cacheKey);
        } else {
            // Invalidate both players for this move
            analysisCache.delete(`${gameIndex}:${moveIndex}:1`);
            analysisCache.delete(`${gameIndex}:${moveIndex}:2`);
        }
    }
    
    // Export function to invalidate cache from a move onwards
    export function invalidateCacheFrom(gameIndex, moveIndex) {
        const keysToDelete = [];
        for (const key of analysisCache.keys()) {
            const [gIdx, mIdx] = key.split(':').map(Number);
            if (gIdx > gameIndex || (gIdx === gameIndex && mIdx >= moveIndex)) {
                keysToDelete.push(key);
            }
        }
        keysToDelete.forEach(key => analysisCache.delete(key));
    }
    
    $: position = $positionStore;
    $: selectedMove = $selectedMoveStore;
    
    function navigateNext() {
        console.log('[CandidateMovesPanel.navigateNext] candidateMoves:', candidateMoves.length, 'selectedIndex:', selectedIndex);
        if (candidateMoves.length === 0) return;
        
        // If no selection (selectedIndex = -1), start from first candidate
        if (selectedIndex === -1) {
            selectedIndex = 0;
            console.log('[CandidateMovesPanel.navigateNext] ➡️ Starting from first candidate:', selectedIndex);
            applyMoveToInput(true);
        } else if (selectedIndex < candidateMoves.length - 1) {
            const oldIndex = selectedIndex;
            selectedIndex = selectedIndex + 1;
            console.log('[CandidateMovesPanel.navigateNext] ➡️ Moving from', oldIndex, 'to', selectedIndex);
            applyMoveToInput(true);
        }
    }
    
    function navigatePrevious() {
        console.log('[CandidateMovesPanel.navigatePrevious] candidateMoves:', candidateMoves.length, 'selectedIndex:', selectedIndex);
        if (candidateMoves.length === 0) return;
        
        // If no selection (selectedIndex = -1), start from first candidate
        if (selectedIndex === -1) {
            selectedIndex = 0;
            console.log('[CandidateMovesPanel.navigatePrevious] ⬅️ Starting from first candidate:', selectedIndex);
            applyMoveToInput(true);
        } else if (selectedIndex > 0) {
            const oldIndex = selectedIndex;
            selectedIndex = selectedIndex - 1;
            console.log('[CandidateMovesPanel.navigatePrevious] ⬅️ Moving from', oldIndex, 'to', selectedIndex);
            applyMoveToInput(true);
        }
    }
    
    onMount(() => {
        // Track last seen transcription and dice to detect modifications
        let lastTranscriptionVersion = 0;
        let lastDiceMap = new Map(); // gameIndex:moveIndex:player -> diceStr
        let pendingAnalysisKeys = new Set(); // keys that need re-analysis due to dice changes
        
        // Subscribe to transcription changes to detect dice modifications
        const unsubscribeTranscription = transcriptionStore.subscribe((transcription) => {
            console.log('[CandidateMovesPanel] 📋 transcriptionStore changed');
            if (transcription && transcription.games) {
                const currentVersion = Date.now();
                const isNewLoad = lastTranscriptionVersion === 0 || 
                    (currentVersion - lastTranscriptionVersion) > 1000;
                console.log('[CandidateMovesPanel] isNewLoad:', isNewLoad, 'timeDiff:', currentVersion - lastTranscriptionVersion);
                
                if (isNewLoad) {
                    // New file loaded - clear everything
                    analysisCache.clear();
                    lastDiceMap.clear();
                    pendingAnalysisKeys.clear();
                    backgroundAnalyzeAllMoves(true); // Prioritize current move
                    lastTranscriptionVersion = currentVersion;
                } else {
                    // Check if any dice changed compared to what we cached
                    let currentMoveChanged = false;
                    const selMove = get(selectedMoveStore);
                    
                    transcription.games.forEach((game, gameIndex) => {
                        game.moves?.forEach((move, moveIndex) => {
                            // Check player 1
                            if (move.player1Move?.dice) {
                                const key = `${gameIndex}:${moveIndex}:1`;
                                const oldDice = lastDiceMap.get(key);
                                const newDice = move.player1Move.dice;
                                if (oldDice && oldDice !== newDice) {
                                    // Dice changed - invalidate cache
                                    analysisCache.delete(key);
                                    console.log(`[CandidateMovesPanel] Invalidated cache for ${key}: ${oldDice} -> ${newDice}`);
                                    console.log(`[CandidateMovesPanel] Current selMove:`, selMove);
                                    console.log(`[CandidateMovesPanel] Changed move: game=${gameIndex}, move=${moveIndex}, player=1`);
                                    
                                    // Check if this is the current move
                                    if (selMove && selMove.gameIndex === gameIndex && 
                                        selMove.moveIndex === moveIndex && selMove.player === 1) {
                                        currentMoveChanged = true;
                                        console.log(`[CandidateMovesPanel] ✓ This IS the current move - will trigger refresh`);
                                    } else {
                                        // Not current move - mark for pending analysis
                                        pendingAnalysisKeys.add(key);
                                        console.log(`[CandidateMovesPanel] ✗ NOT current move - added to pending`);
                                    }
                                }
                                lastDiceMap.set(key, newDice);
                            }
                            
                            // Check player 2
                            if (move.player2Move?.dice) {
                                const key = `${gameIndex}:${moveIndex}:2`;
                                const oldDice = lastDiceMap.get(key);
                                const newDice = move.player2Move.dice;
                                if (oldDice && oldDice !== newDice) {
                                    // Dice changed - invalidate cache
                                    analysisCache.delete(key);
                                    console.log(`[CandidateMovesPanel] Invalidated cache for ${key}: ${oldDice} -> ${newDice}`);
                                    console.log(`[CandidateMovesPanel] Current selMove:`, selMove);
                                    console.log(`[CandidateMovesPanel] Changed move: game=${gameIndex}, move=${moveIndex}, player=2`);
                                    
                                    // Check if this is the current move
                                    if (selMove && selMove.gameIndex === gameIndex && 
                                        selMove.moveIndex === moveIndex && selMove.player === 2) {
                                        currentMoveChanged = true;
                                        console.log(`[CandidateMovesPanel] ✓ This IS the current move - will trigger refresh`);
                                    } else {
                                        // Not current move - mark for pending analysis
                                        pendingAnalysisKeys.add(key);
                                        console.log(`[CandidateMovesPanel] ✗ NOT current move - added to pending`);
                                    }
                                }
                                lastDiceMap.set(key, newDice);
                            }
                        });
                    });
                    
                    // If current move's dice changed, trigger immediate re-analysis
                    if (currentMoveChanged && selMove) {
                        console.log('[CandidateMovesPanel] ⚡ Current move dice changed, forcing position refresh');
                        console.log('[CandidateMovesPanel] Setting selectedMoveStore to:', selMove);
                        
                        // Cancel any ongoing background analysis to prioritize immediate analysis
                        if (backgroundAnalysisTimeout) {
                            clearTimeout(backgroundAnalysisTimeout);
                            backgroundAnalysisTimeout = null;
                        }
                        isBackgroundAnalyzing = false;
                        
                        // Force positionStore to refresh by re-triggering selectedMoveStore
                        // This ensures the new dice values are reflected in the position
                        selectedMoveStore.set({...selMove});
                        console.log('[CandidateMovesPanel] ⚡ Position refresh triggered');
                    }
                    
                    lastTranscriptionVersion = currentVersion;
                }
            }
        });
        
        // Subscribe to selected move changes to trigger analysis immediately
        unsubscribeSelectedMove = selectedMoveStore.subscribe(async () => {
            updateCurrentMove();
            const selMove = get(selectedMoveStore);
            
            console.log('[CandidateMovesPanel] 🔔 selectedMoveStore changed:', selMove);
            
            // Get dice from transcription (not from positionStore which might be stale)
            const transcription = get(transcriptionStore);
            const game = transcription?.games[selMove.gameIndex];
            const move = game?.moves[selMove.moveIndex];
            const playerMove = selMove.player === 1 ? move?.player1Move : move?.player2Move;
            const diceStr = playerMove?.dice || '';
            
            console.log('[CandidateMovesPanel] Dice from transcription:', diceStr);
            console.log('[CandidateMovesPanel] Background analyzing:', isBackgroundAnalyzing);
            
            // Parse dice string to array
            const dice = parseDice(diceStr);
            
            if (dice && (dice[0] !== 0 || dice[1] !== 0)) {
                const cacheKey = getCacheKey(selMove);
                const cachedMoves = analysisCache.get(cacheKey);
                
                console.log('[CandidateMovesPanel] Cache key:', cacheKey);
                console.log('[CandidateMovesPanel] Current dice str:', diceStr);
                console.log('[CandidateMovesPanel] Has cache:', !!cachedMoves);
                console.log('[CandidateMovesPanel] Cache dice str:', cachedMoves?._diceStr);
                
                // Check if this move has pending analysis due to dice change
                const hasPendingAnalysis = pendingAnalysisKeys.has(cacheKey);
                if (hasPendingAnalysis) {
                    // Remove from pending and force re-analysis
                    pendingAnalysisKeys.delete(cacheKey);
                    console.log('[CandidateMovesPanel] 🔄 Processing pending analysis for', cacheKey);
                    
                    // Cancel background analysis to prioritize immediate analysis
                    if (isBackgroundAnalyzing) {
                        console.log('[CandidateMovesPanel] ⚠️ Canceling background analysis for immediate analysis');
                        isBackgroundAnalyzing = false;
                    }
                    
                    // Recalculate position with current dice from transcription
                    const pos = get(positionStore);
                    if (pos) {
                        // Update position dice with current dice from transcription
                        const posWithDice = { ...pos, dice: dice };
                        await analyzeMoves(posWithDice, selMove);
                    }
                    return;
                }
                
                // Check if cache exists and if dice match
                const cacheValid = cachedMoves && cachedMoves._diceStr === diceStr;
                
                if (cacheValid) {
                    // Use cached results
                    console.log('[CandidateMovesPanel] ✓ Using cached results');
                    candidateMoves = cachedMoves;
                    selectedIndex = findMatchingMoveIndex();
                } else {
                    // Cache invalid or missing - analyze
                    if (cachedMoves) {
                        // Dice changed - invalidate cache
                        analysisCache.delete(cacheKey);
                        console.log('[CandidateMovesPanel] 🔄 Dice changed, re-analyzing:', diceStr);
                    } else {
                        console.log('[CandidateMovesPanel] 🔄 No cache, analyzing:', diceStr);
                    }
                    
                    // Cancel background analysis to prioritize immediate analysis
                    if (isBackgroundAnalyzing) {
                        console.log('[CandidateMovesPanel] ⚠️ Canceling background analysis for immediate analysis');
                        isBackgroundAnalyzing = false;
                    }
                    
                    // Recalculate position with current dice from transcription
                    const pos = get(positionStore);
                    if (pos) {
                        // Update position dice with current dice from transcription
                        const posWithDice = { ...pos, dice: dice };
                        await analyzeMoves(posWithDice, selMove);
                    }
                }
            } else {
                console.log('[CandidateMovesPanel] No valid dice, clearing candidates');
                candidateMoves = [];
                selectedIndex = 0;
            }
        });
        
        // Listen for j/k navigation from App.svelte (dispatched when move input is focused)
        const handleCandidateNavigate = (event) => {
            console.log('[CandidateMovesPanel] candidateNavigate event received:', event.detail);
            const direction = event.detail.direction;
            if (direction === 'next') {
                console.log('[CandidateMovesPanel] Calling navigateNext from window event');
                navigateNext();
            } else if (direction === 'prev') {
                console.log('[CandidateMovesPanel] Calling navigatePrevious from window event');
                navigatePrevious();
            }
        };
        
        // Add event listener to window
        window.addEventListener('candidateNavigate', handleCandidateNavigate);
        
        // Cleanup on destroy
        return () => {
            window.removeEventListener('candidateNavigate', handleCandidateNavigate);
        };
    });
    
    onDestroy(() => {
        if (unsubscribeSelectedMove) unsubscribeSelectedMove();
    });
    
    function updateCurrentMove() {
        const transcription = get(transcriptionStore);
        const selMove = get(selectedMoveStore);
        
        if (!transcription || !selMove) {
            currentPlayerMove = '';
            return;
        }
        
        const { gameIndex, moveIndex, player } = selMove;
        const game = transcription.games[gameIndex];
        
        if (!game || !game.moves[moveIndex]) {
            currentPlayerMove = '';
            return;
        }
        
        const move = game.moves[moveIndex];
        const playerMove = player === 1 ? move.player1Move : move.player2Move;
        
        if (playerMove && playerMove.move) {
            currentPlayerMove = playerMove.move.trim();
        } else {
            currentPlayerMove = '';
        }
    }
    
    function getCacheKey(selMove) {
        if (!selMove) return null;
        return `${selMove.gameIndex}:${selMove.moveIndex}:${selMove.player}`;
    }
    
    function findMatchingMoveIndex(moveText) {
        if (candidateMoves.length === 0) return -1;
        
        // If move is empty or not provided, select first candidate
        if (!moveText || moveText.trim() === '') return 0;
        
        // Try to find matching move in candidates
        const matchIndex = candidateMoves.findIndex(m => 
            normalizeMove(m.move) === normalizeMove(moveText)
        );
        
        // Return -1 if no match (don't auto-select when move doesn't match)
        return matchIndex;
    }
    
    async function analyzeMoves(pos, selMove) {
        if (!pos || !pos.dice || (pos.dice[0] === 0 && pos.dice[1] === 0)) {
            candidateMoves = [];
            selectedIndex = 0;
            basePosition = null;
            return;
        }
        
        // Store the base position (NOT cloned - just keep reference for board updates)
        // We'll use it later to apply candidate moves for board visualization
        basePosition = pos;
        
        loading = true;
        error = null;
        
        try {
            console.log('[CandidateMovesPanel] Analyzing position with dice:', pos.dice);
            const moves = await GetCandidateMoves(pos);
            console.log('[CandidateMovesPanel] Got', moves?.length || 0, 'candidate moves');
            
            if (moves && moves.length > 0) {
                candidateMoves = moves;
                
                // Store in cache with dice string for validation
                if (selMove) {
                    const cacheKey = getCacheKey(selMove);
                    if (cacheKey) {
                        const transcription = get(transcriptionStore);
                        const game = transcription?.games[selMove.gameIndex];
                        const move = game?.moves[selMove.moveIndex];
                        const playerMove = selMove.player === 1 ? move?.player1Move : move?.player2Move;
                        const diceStr = playerMove?.dice || '';
                        
                        // Store moves with dice string for validation
                        moves._diceStr = diceStr;
                        analysisCache.set(cacheKey, moves);
                    }
                }
                
                console.log('[CandidateMovesPanel] First move:', moves[0].move);
                
                // Check current move input value
                const moveInputEl = document.getElementById('move-input') || document.querySelector('.inline-move-input');
                const currentInputValue = moveInputEl?.value?.trim() || '';
                
                // Check if this was an empty decision (no move existed in transcription before editing)
                // Use the selMove parameter that was passed in
                const transcription = get(transcriptionStore);
                const game = transcription?.games[selMove.gameIndex];
                const move = game?.moves[selMove.moveIndex];
                const playerMove = selMove.player === 1 ? move?.player1Move : move?.player2Move;
                const wasEmptyDecision = !playerMove?.move || playerMove.move.trim() === '';
                
                console.log('[CandidateMovesPanel] Analysis check:', {
                    gameIndex: selMove.gameIndex,
                    moveIndex: selMove.moveIndex,
                    player: selMove.player,
                    playerMove: playerMove,
                    move: playerMove?.move,
                    wasEmptyDecision: wasEmptyDecision,
                    currentInputValue: currentInputValue
                });
                
                // Handle empty input first (before calling findMatchingMoveIndex which also returns 0 for empty)
                if (currentInputValue === '') {
                    // Empty input - select first candidate and preview it if it was an empty decision
                    console.log('[CandidateMovesPanel] Empty input, selecting first candidate. Was empty decision:', wasEmptyDecision);
                    selectedIndex = 0;
                    
                    // If this was an empty decision, set preview to show arrows
                    if (wasEmptyDecision && moves[0]?.move) {
                        console.log('[CandidateMovesPanel] ✅ Setting preview for empty decision:', moves[0].move);
                        candidatePreviewMoveStore.set(moves[0].move);
                    } else {
                        console.log('[CandidateMovesPanel] ❌ NOT setting preview. wasEmptyDecision:', wasEmptyDecision, 'has move:', !!moves[0]?.move);
                    }
                } else {
                    // Non-empty input - try to find matching move
                    selectedIndex = findMatchingMoveIndex(currentInputValue);
                    
                    if (selectedIndex >= 0) {
                        // Found matching candidate - keep the input as-is, just highlight in panel
                        console.log('[CandidateMovesPanel] Found matching candidate at index:', selectedIndex);
                    } else {
                        // No matching candidate found (selectedIndex = -1), don't change the input
                        console.log('[CandidateMovesPanel] No matching candidate for:', currentInputValue);
                    }
                }
            } else {
                candidateMoves = [];
                selectedIndex = 0;
            }
        } catch (e) {
            console.error('[CandidateMovesPanel] Error analyzing moves:', e);
            error = e.message || 'Failed to analyze moves';
            candidateMoves = [];
            selectedIndex = 0;
        } finally {
            loading = false;
        }
    }
    
    let backgroundAnalysisTimeout = null;
    let isBackgroundAnalyzing = false;
    
    // Background analysis of all moves with dice in the transcription
    async function backgroundAnalyzeAllMoves(prioritizeCurrentMove = false) {
        // Debounce: cancel pending analysis and schedule new one
        if (backgroundAnalysisTimeout) {
            clearTimeout(backgroundAnalysisTimeout);
        }
        
        backgroundAnalysisTimeout = setTimeout(async () => {
            if (isBackgroundAnalyzing) {
                console.log('[CandidateMovesPanel] Background analysis already in progress, skipping');
                return;
            }
            
            isBackgroundAnalyzing = true;
            const transcription = get(transcriptionStore);
            if (!transcription || !transcription.games) {
                isBackgroundAnalyzing = false;
                return;
            }
            
            console.log('[CandidateMovesPanel] Starting background analysis...');
            let analyzedCount = 0;
            let errorCount = 0;
            
            // If prioritizing current move, analyze it first
            if (prioritizeCurrentMove) {
                const selMove = get(selectedMoveStore);
                if (selMove) {
                    const cacheKey = getCacheKey(selMove);
                    if (cacheKey && !analysisCache.has(cacheKey)) {
                        try {
                            await analyzeInBackground(selMove.gameIndex, selMove.moveIndex, selMove.player);
                            analyzedCount++;
                            console.log('[CandidateMovesPanel] Prioritized current move analysis');
                        } catch (e) {
                            errorCount++;
                        }
                    }
                }
            }
            
            // Use setTimeout to yield to UI thread between analyses
            for (let gameIndex = 0; gameIndex < transcription.games.length; gameIndex++) {
                // Check if background analysis was canceled
                if (!isBackgroundAnalyzing) {
                    console.log('[CandidateMovesPanel] Background analysis canceled by user action');
                    return;
                }
                
                const game = transcription.games[gameIndex];
                if (!game || !game.moves) continue;
                
                for (let moveIndex = 0; moveIndex < game.moves.length; moveIndex++) {
                    // Check if background analysis was canceled
                    if (!isBackgroundAnalyzing) {
                        console.log('[CandidateMovesPanel] Background analysis canceled by user action');
                        return;
                    }
                    
                    const move = game.moves[moveIndex];
                    
                    // Analyze player 1 move if it has dice
                    if (move.player1Move && move.player1Move.dice) {
                        const cacheKey = `${gameIndex}:${moveIndex}:1`;
                        if (!analysisCache.has(cacheKey)) {
                            try {
                                await analyzeInBackground(gameIndex, moveIndex, 1);
                                analyzedCount++;
                            } catch (e) {
                                errorCount++;
                            }
                        }
                    }
                    
                    // Analyze player 2 move if it has dice
                    if (move.player2Move && move.player2Move.dice) {
                        const cacheKey = `${gameIndex}:${moveIndex}:2`;
                        if (!analysisCache.has(cacheKey)) {
                            try {
                                await analyzeInBackground(gameIndex, moveIndex, 2);
                                analyzedCount++;
                            } catch (e) {
                                errorCount++;
                            }
                        }
                    }
                    
                    // Yield to UI thread every 5 analyses
                    if (analyzedCount % 5 === 0) {
                        await new Promise(resolve => setTimeout(resolve, 10));
                    }
                }
            }
            
            console.log(`[CandidateMovesPanel] Background analysis complete: ${analyzedCount} positions analyzed, ${errorCount} errors`);
            isBackgroundAnalyzing = false;
        }, 500); // Wait 500ms after last change before starting
    }
    
    async function analyzeInBackground(gameIndex, moveIndex, player) {
        try {
            const transcription = get(transcriptionStore);
            const game = transcription.games[gameIndex];
            if (!game || !game.moves[moveIndex]) return;
            
            const move = game.moves[moveIndex];
            const playerMove = player === 1 ? move.player1Move : move.player2Move;
            
            if (!playerMove || !playerMove.dice) return;
            
            // Parse dice first - if invalid, skip this position
            const dice = parseDice(playerMove.dice);
            if (!dice || (dice[0] === 0 && dice[1] === 0)) return;
            
            // Calculate position directly without changing selectedMoveStore
            const targetPlayer = player === 1 ? 'player1' : 'player2';
            let position;
            
            try {
                position = calculatePositionAtMove(
                    transcription,
                    game.gameNumber,
                    move.moveNumber,
                    targetPlayer
                );
            } catch (posError) {
                // Position calculation failed - skip this position
                console.warn(`[CandidateMovesPanel] Position calc failed for g${gameIndex}:m${moveIndex}:p${player}`, posError.message);
                return;
            }
            
            // Create position object with dice
            const pos = {
                ...position,
                dice: dice,
                playerOnRoll: player - 1  // Convert from 1-based to 0-based
            };
            
            // Analyze moves
            const moves = await GetCandidateMoves(pos);
            if (moves && moves.length > 0) {
                const cacheKey = `${gameIndex}:${moveIndex}:${player}`;
                // Store dice string with moves for validation
                moves._diceStr = playerMove.dice;
                analysisCache.set(cacheKey, moves);
            }
        } catch (e) {
            // Silently skip positions that cause errors during background analysis
            // console.error('[CandidateMovesPanel] Background analysis error:', e);
        }
    }
    
    function normalizeMove(move) {
        // Normalize move string for comparison (remove extra spaces, sort plays)
        if (!move) return '';
        return move.trim().toLowerCase().replace(/\s+/g, ' ');
    }
    
    function applyMoveToInput(forcePreview = false) {
        // If in edit mode with move input, update the input field
        // Try EditPanel's move input first, then MovesTable's inline input
        let moveInputElement = document.getElementById('move-input');
        if (!moveInputElement) {
            moveInputElement = document.querySelector('.inline-move-input');
        }
        
        console.log('[CandidateMovesPanel] applyMoveToInput - element:', !!moveInputElement, 'selectedIndex:', selectedIndex, 'forcePreview:', forcePreview);
        
        // Only update preview store if explicitly requested (for j/k navigation) or if there's an input element (edit mode)
        if ((forcePreview || moveInputElement) && selectedIndex >= 0 && selectedIndex < candidateMoves.length) {
            const move = candidateMoves[selectedIndex].move;
            console.log('[CandidateMovesPanel] Setting candidate preview move:', move);
            candidatePreviewMoveStore.set(move);
            
            // If there's an input element, also update it
            if (moveInputElement) {
                console.log('[CandidateMovesPanel] Setting value to:', move);
                moveInputElement.value = move;
                // Dispatch input event to update the bound value
                moveInputElement.dispatchEvent(new Event('input', { bubbles: true }));
                
                // Select the text so user can continue pressing j/k or type to replace
                // Use double requestAnimationFrame to ensure selection happens after all DOM updates
                requestAnimationFrame(() => {
                    requestAnimationFrame(() => {
                        console.log('[CandidateMovesPanel] requestAnimationFrame (2nd) - about to select');
                        if (moveInputElement) {
                            console.log('[CandidateMovesPanel] Value before select:', moveInputElement.value);
                            moveInputElement.setSelectionRange(0, moveInputElement.value.length);
                            console.log('[CandidateMovesPanel] Selection set:', moveInputElement.selectionStart, '-', moveInputElement.selectionEnd);
                        }
                    });
                });
            }
        }
    }
    
    async function selectMove(index) {
        if (index < 0 || index >= candidateMoves.length) return;
        
        selectedIndex = index;
        const move = candidateMoves[index];
        
        // Save undo state before making changes
        undoRedoStore.saveSnapshot();
        
        // Update the current move in the transcription
        const transcription = get(transcriptionStore);
        const selMove = get(selectedMoveStore);
        
        if (!transcription || !selMove) return;
        
        const { gameIndex, moveIndex, player } = selMove;
        const gameMove = transcription.games[gameIndex]?.moves[moveIndex];
        
        if (!gameMove) return;
        
        const playerMove = player === 1 ? gameMove.player1Move : gameMove.player2Move;
        const isIllegal = playerMove?.isIllegal || false;
        const isGala = playerMove?.isGala || false;
        const dice = playerMove?.dice || '';
        
        // Update the move
        updateMove(gameIndex, gameMove.moveNumber, player, dice, move.move, isIllegal, isGala);
        
        // Invalidate cache from this point
        await invalidatePositionsCacheFrom(gameIndex, moveIndex);
        
        // Trigger re-rendering
        selectedMoveStore.set({ ...selMove });
        
        console.log('[CandidateMovesPanel] Selected move:', move.move);
    }
    
    function handleKeyDown(event) {
        // This component now handles navigation exclusively through the candidateNavigate window event
        // dispatched by App.svelte. Direct keyboard handling is removed to prevent double-triggering.
        // Only keeping this function for potential future non-navigation shortcuts.
    }
</script>

<svelte:window on:keydown={handleKeyDown} />

{#if visible}
<div class="candidate-moves-panel">
    {#if loading}
        <div class="loading">Analyzing...</div>
    {:else if error}
        <div class="error">{error}</div>
    {:else if candidateMoves.length === 0}
        <div class="no-moves">—</div>
    {:else}
        <div class="moves-list">
            {#each candidateMoves as move, i}
                <div 
                    class="move-item {i === selectedIndex ? 'selected' : ''}"
                    on:click={() => selectMove(i)}
                    on:keydown={(e) => e.key === 'Enter' && selectMove(i)}
                    role="button"
                    tabindex="0"
                >
                    <span class="move-notation">{move.move}</span>
                </div>
            {/each}
        </div>
    {/if}
</div>
{/if}

<style>
    .candidate-moves-panel {
        padding: 2px 1px;
        background-color: white;
        border: 1px solid #ddd;
        flex: 1;
        min-height: 0;
        overflow-y: auto;
        overflow-x: hidden;
        display: flex;
        flex-direction: column;
        width: 170px;
    }

    .loading, .error, .no-moves {
        color: #999;
        font-size: 13px;
        font-style: italic;
        padding: 10px;
        text-align: center;
    }

    .error {
        color: #d9534f;
    }

    .moves-list {
        flex: 1;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
    }

    .move-item {
        padding: 2px 3px;
        cursor: pointer;
        transition: background-color 0.1s ease;
        text-align: left;
        border-left: 2px solid transparent;
    }

    .move-item:hover {
        background-color: #f0f0f0;
    }

    .move-item.selected {
        background-color: #e6f3ff;
        border-left-color: #4a9eff;
    }

    .move-item.current {
        background-color: #fff3cd;
        border-color: #ffc107;
    }

    .move-item.current.selected {
        background-color: #ffe69c;
        border-color: #ff9800;
    }

    .move-notation {
        font-family: monospace;
        font-size: 13px;
        font-weight: normal;
        color: #333;
        white-space: nowrap;
    }

    .move-item.selected .move-notation {
        font-weight: 600;
    }
    
    /* Removed .move-item.current styles - all moves should look the same */
</style>
