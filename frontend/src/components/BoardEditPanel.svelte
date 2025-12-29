<script>
    import { onMount, onDestroy } from 'svelte';
    import { get } from 'svelte/store';
    import { 
        transcriptionStore, 
        selectedMoveStore,
        updateMove,
        invalidatePositionsCacheFrom,
        insertDecisionAfter
    } from '../stores/transcriptionStore.js';
    import { 
        statusBarModeStore,
        statusBarTextStore,
        candidatePreviewMoveStore
    } from '../stores/uiStore.js';

    export let visible = false;
    export let onClose;
    
    // Subscribe to candidate preview moves to update manual input when j/k navigation happens
    // This also updates when dice are first entered and best move is auto-selected
    candidatePreviewMoveStore.subscribe(move => {
        if (visible && move) {
            // Update manualMoveInput unless user is in manual entry mode
            if (!showManualInput) {
                manualMoveInput = move;
                console.log('[BoardEditPanel] Auto-updated move from candidate:', move);
            }
        }
    });

    // State for editing
    let diceInput = '';
    let manualMoveInput = '';
    let showManualInput = false;
    let originalDice = '';
    let originalMove = '';
    
    let manualInputElement;

    let hasAutoFocused = false;

    // Initialize when panel becomes visible
    $: if (visible && !hasAutoFocused) {
        startEditing();
    } else if (!visible) {
        hasAutoFocused = false;
        diceInput = '';
        manualMoveInput = '';
        showManualInput = false;
    }

    function startEditing() {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        
        if (!transcription || !selectedMove) {
            return;
        }

        const { gameIndex, moveIndex, player } = selectedMove;
        const game = transcription.games[gameIndex];
        
        if (!game || !game.moves[moveIndex]) {
            return;
        }

        const move = game.moves[moveIndex];
        const playerMove = player === 1 ? move.player1Move : move.player2Move;

        // Check if this is a cube action
        if (playerMove?.cubeAction) {
            if (playerMove.cubeAction === 'doubles') {
                originalDice = 'd';
            } else if (playerMove.cubeAction === 'takes') {
                originalDice = 't';
            } else if (playerMove.cubeAction === 'drops') {
                originalDice = 'p';
            }
            originalMove = '';
            diceInput = originalDice;
        } else if (playerMove?.resignAction) {
            // Check if this is a resign action
            if (playerMove.resignAction === 'normal') {
                originalDice = 'r';
            } else if (playerMove.resignAction === 'gammon') {
                originalDice = 'g';
            } else if (playerMove.resignAction === 'backgammon') {
                originalDice = 'b';
            }
            originalMove = '';
            diceInput = originalDice;
        } else if (playerMove) {
            originalDice = playerMove.dice || '';
            originalMove = playerMove.move || '';
            diceInput = originalDice;
        } else {
            originalDice = '';
            originalMove = '';
            diceInput = '';
        }

        hasAutoFocused = true;

        // Set status bar message based on edit mode
        const currentMove = get(selectedMoveStore);
        const isRapidMode = isEditingLastNonEmptyDecision();
        
        let statusMessage;
        if (isRapidMode) {
            // Rapid editing mode for last non-empty decision
            statusMessage = 'EDIT MODE (RAPID): Type dice, j/k=moves, Enter=next. After move selected, type next dice to continue. Backspace=clear, Esc=exit';
        } else if (currentMove && currentMove.moveIndex === 0 && currentMove.player === 1) {
            // First move special case
            statusMessage = 'EDIT MODE: Type dice (1-6) or leave empty if player2 starts. d/t/p=cube, r/g/b=resign. j/k=cycle, f=fail, space=manual, Enter=next, Esc=exit';
        } else {
            // Normal editing mode
            statusMessage = 'EDIT MODE: Type dice (d/t/p/r/g/b or 1-6), j/k=cycle moves, f=fail, space=manual, Enter=next, Esc=exit';
        }
        statusBarTextStore.set(statusMessage);
    }

    function cancelEditing() {
        diceInput = '';
        manualMoveInput = '';
        showManualInput = false;
        onClose();
    }

    async function validateEditing() {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        
        if (!transcription || !selectedMove) {
            return;
        }

        const { gameIndex, moveIndex, player } = selectedMove;
        const game = transcription.games[gameIndex];
        
        if (!game || !game.moves[moveIndex]) {
            return;
        }

        const move = game.moves[moveIndex];
        
        // Check if anything changed
        const hasChanges = diceInput !== originalDice || manualMoveInput !== originalMove;

        if (hasChanges) {
            // Update the move
            const playerMove = player === 1 ? move.player1Move : move.player2Move;
            const isIllegal = playerMove?.isIllegal || false;
            const isGala = playerMove?.isGala || false;
            
            // For cube decisions (d/t/p) and resign decisions (r/g/b), pass empty string as move
            const diceStr = diceInput.toLowerCase();
            const isCubeDecision = diceStr === 'd' || diceStr === 't' || diceStr === 'p';
            const isResignDecision = diceStr === 'r' || diceStr === 'g' || diceStr === 'b';
            
            // If manual input is empty but there's an existing move, preserve it (dice was changed without changing move)
            let moveToSave;
            if (isCubeDecision || isResignDecision) {
                moveToSave = '';
            } else if (manualMoveInput) {
                moveToSave = manualMoveInput;
            } else {
                // Preserve existing move if manual input wasn't used
                moveToSave = playerMove?.move || '';
            }
            
            updateMove(gameIndex, move.moveNumber, player, diceInput, moveToSave, isIllegal, isGala);
            
            // Invalidate position cache from this move onwards
            await invalidatePositionsCacheFrom(gameIndex, moveIndex);
            
            // Force recalculation of all positions after this move
            selectedMoveStore.set({ ...selectedMove });
            
            statusBarTextStore.set(`Move updated at game ${gameIndex + 1}, move ${move.moveNumber}`);
        } else {
            statusBarTextStore.set('No changes made');
        }

        // Move to next decision and stay in edit mode
        await moveToNextDecision(gameIndex, moveIndex, player);
        
        // Reset hasAutoFocused so startEditing() will be called again for the next decision
        hasAutoFocused = false;
    }

    async function moveToNextDecision(currentGameIndex, currentMoveIndex, currentPlayer) {
        const transcription = get(transcriptionStore);
        if (!transcription || !transcription.games) return;
        
        const currentGame = transcription.games[currentGameIndex];
        if (!currentGame) return;
        
        // Try to move to next decision
        if (currentPlayer === 1) {
            // Move to player 2 of same move
            const nextSelection = { 
                gameIndex: currentGameIndex, 
                moveIndex: currentMoveIndex, 
                player: 2 
            };
            // Clear position cache for the next decision to force recalculation
            await invalidatePositionsCacheFrom(currentGameIndex, currentMoveIndex);
            selectedMoveStore.set(nextSelection);
        } else if (currentMoveIndex < currentGame.moves.length - 1) {
            // Move to player 1 of next move in same game
            const nextSelection = { 
                gameIndex: currentGameIndex, 
                moveIndex: currentMoveIndex + 1, 
                player: 1 
            };
            // Clear position cache for the next decision to force recalculation
            await invalidatePositionsCacheFrom(currentGameIndex, currentMoveIndex + 1);
            selectedMoveStore.set(nextSelection);
        } else if (currentGameIndex < transcription.games.length - 1) {
            // Move to first move of next game
            const nextSelection = { 
                gameIndex: currentGameIndex + 1, 
                moveIndex: 0, 
                player: 1 
            };
            // Clear position cache for the next game to force recalculation
            await invalidatePositionsCacheFrom(currentGameIndex + 1, 0);
            selectedMoveStore.set(nextSelection);
        } else {
            // We're at the last decision
            // Check if game has ended (drop, resign, or natural bearoff) - if so, don't insert new decision
            const currentMove = currentGame.moves[currentMoveIndex];
            const hasGameEnded = currentGame.naturalBearoffWin ||
                                (currentPlayer === 1 && currentMove.player1Move && 
                                 (currentMove.player1Move.cubeAction === 'drops' || currentMove.player1Move.resignAction)) ||
                                (currentPlayer === 2 && currentMove.player2Move && 
                                 (currentMove.player2Move.cubeAction === 'drops' || currentMove.player2Move.resignAction));
            
            if (hasGameEnded) {
                statusBarTextStore.set('Cannot add decision after game has ended. Tab=exit edit mode');
                return;
            }
            
            // Insert a new empty decision after it
            insertDecisionAfter(currentGameIndex, currentMoveIndex, currentPlayer);
            
            // Calculate the new move index for the inserted decision
            const newMoveIndex = currentMoveIndex + 1;
            
            // Invalidate cache and force recalculation
            await invalidatePositionsCacheFrom(currentGameIndex, newMoveIndex);
            
            // Select the newly inserted decision
            selectedMoveStore.set({ 
                gameIndex: currentGameIndex, 
                moveIndex: newMoveIndex, 
                player: 1 
            });
            
            statusBarTextStore.set('Last decision reached - new empty decision inserted. Tab=exit edit mode');
            // Reset and restart editing for the new decision
            hasAutoFocused = false;
            startEditing();
            return;
        }
        
        // Reset and restart editing for the next decision
        hasAutoFocused = false;
        startEditing();
    }

    function handleKeyDown(event) {
        if (!visible) return;

        // Don't interfere with manual input
        if (showManualInput && document.activeElement === manualInputElement) {
            if (event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                showManualInput = false;
                manualMoveInput = '';
                statusBarTextStore.set('Manual entry cancelled');
            } else if (event.key === 'Enter') {
                event.preventDefault();
                event.stopPropagation();
                showManualInput = false;
                validateEditing();
            }
            return;
        }

        event.stopPropagation();

        if (event.key === 'Escape') {
            event.preventDefault();
            cancelEditing();
        } else if (event.key === 'Backspace') {
            // Backspace behavior depends on edit mode:
            // - Rapid mode (last non-empty decision): Just clear input fields to allow re-entry
            // - Normal mode: Erase dice and move from transcription
            event.preventDefault();
            if (diceInput) {
                const isRapidMode = isEditingLastNonEmptyDecision();
                
                if (isRapidMode) {
                    // Rapid mode: Just reset input fields without updating transcription
                    console.log('[BoardEditPanel] Backspace (rapid mode) - clearing input fields');
                    diceInput = '';
                    manualMoveInput = '';
                    statusBarTextStore.set('Input cleared - type new dice to continue');
                } else {
                    // Normal mode: Erase from transcription
                    console.log('[BoardEditPanel] Backspace (normal mode) - erasing dice and move from transcription');
                    diceInput = '';
                    manualMoveInput = '';
                    
                    // Clear dice and move in transcription
                    const transcription = get(transcriptionStore);
                    const selectedMove = get(selectedMoveStore);
                    if (transcription && selectedMove) {
                        const { gameIndex, moveIndex, player } = selectedMove;
                        const move = transcription.games[gameIndex]?.moves[moveIndex];
                        const playerMove = player === 1 ? move?.player1Move : move?.player2Move;
                        const isIllegal = playerMove?.isIllegal || false;
                        const isGala = playerMove?.isGala || false;
                        
                        // Update with empty dice and move
                        updateMove(gameIndex, move.moveNumber, player, '', '', isIllegal, isGala);
                        
                        // Invalidate position cache to force board to recalculate
                        invalidatePositionsCacheFrom(gameIndex, moveIndex);
                        
                        statusBarTextStore.set('Dice and move erased');
                    }
                }
            }
        } else if (event.key === 'Enter') {
            event.preventDefault();
            validateEditing();
        } else if (event.key === ' ') {
            // Space = open manual move entry
            event.preventDefault();
            showManualInput = true;
            setTimeout(() => {
                manualInputElement?.focus();
            }, 50);
        } else if (event.key === 'f' || event.key === 'F') {
            // f = fail/cannot move - only works if dice are entered
            if (diceInput && validateDiceInput(diceInput) && !isCubeDecision && !isResignDecision) {
                event.preventDefault();
                manualMoveInput = 'f';
                validateEditing();
            }
        } else if (/^[1-6dtprgb]$/i.test(event.key)) {
            // Dice input - only single character at a time
            handleDiceKeyPress(event.key.toLowerCase());
        }
    }

    function handleDiceKeyPress(key) {
        const value = key.toLowerCase();
        const isRapidMode = isEditingLastNonEmptyDecision();
        
        // Check for cube decisions and resign decisions first
        if (value === 'd' || value === 't' || value === 'p' || value === 'r' || value === 'g' || value === 'b') {
            console.log('[BoardEditPanel] Special action detected:', value);
            diceInput = value;
            manualMoveInput = '';
            
            // Immediately update transcription
            updateDiceOnly(value);
            return;
        }
        
        // Handle digit input for dice
        if (/^[1-6]$/.test(value)) {
            if (diceInput.length === 0) {
                // First digit
                diceInput = value;
            } else if (diceInput.length === 1 && /^[1-6]$/.test(diceInput)) {
                // Second digit - complete the dice
                const completedDice = diceInput + value;
                diceInput = completedDice;
                
                // Check if this is the first decision of the game
                const selectedMove = get(selectedMoveStore);
                const isFirstDecision = selectedMove && selectedMove.moveIndex === 0 && selectedMove.player === 1;
                
                if (isFirstDecision && validateDiceInput(completedDice)) {
                    // Opening roll convention: doubles are impossible (players re-roll if same)
                    const die1 = parseInt(completedDice[0]);
                    const die2 = parseInt(completedDice[1]);
                    
                    if (die1 === die2) {
                        // Doubles on opening roll are impossible - reject
                        console.log('[BoardEditPanel] Opening roll error: doubles are impossible');
                        diceInput = '';
                        statusBarTextStore.set('ERROR: Opening roll cannot be doubles - players must re-roll if same');
                        return;
                    } else if (die1 < die2) {
                        // Player2 is on roll - set player1 as empty and move to player2
                        console.log('[BoardEditPanel] Opening roll: player2 starts with', completedDice);
                        
                        // Set player1's decision as empty (no dice, no move)
                        const transcription = get(transcriptionStore);
                        if (transcription && selectedMove) {
                            const { gameIndex, moveIndex } = selectedMove;
                            const move = transcription.games[gameIndex]?.moves[moveIndex];
                            if (move) {
                                // Clear player1's decision
                                updateMove(gameIndex, move.moveNumber, 1, '', '', false, false);
                                
                                // Move to player2's decision
                                invalidatePositionsCacheFrom(gameIndex, moveIndex);
                                selectedMoveStore.set({ 
                                    gameIndex, 
                                    moveIndex, 
                                    player: 2 
                                });
                                
                                // Reset and restart editing for player2 with the dice
                                hasAutoFocused = false;
                                
                                // Wait for selection to update, then set the dice for player2
                                setTimeout(() => {
                                    diceInput = completedDice;
                                    updateDiceOnly(completedDice);
                                    statusBarTextStore.set('Player 2 starts (opening roll: ' + completedDice + ')');
                                }, 50);
                                
                                return;
                            }
                        }
                    }
                }
                
                // Normal case or player1 starts (die1 >= die2)
                if (validateDiceInput(completedDice)) {
                    updateDiceOnly(completedDice);
                }
            } else {
                // Already have 2 digits or special character
                // In rapid mode: this means user is typing new dice for next decision
                // In normal mode: replace current dice with new digit
                if (isRapidMode && diceInput.length === 2 && /^[1-6]{2}$/.test(diceInput) && manualMoveInput) {
                    // Rapid mode: User has completed previous decision (dice + move)
                    // This new digit starts the next decision
                    console.log('[BoardEditPanel] Rapid mode - auto-validating and starting next decision');
                    
                    // First, validate current decision
                    validateEditing();
                    
                    // Wait a bit for validation to complete, then start new dice
                    setTimeout(() => {
                        diceInput = value;
                    }, 50);
                } else {
                    // Normal mode or rapid mode without completed move: replace with new digit
                    diceInput = value;
                }
            }
        }
    }

    async function updateDiceOnly(newDice) {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        
        if (!transcription || !selectedMove) {
            console.log('[BoardEditPanel] No transcription or selectedMove');
            return;
        }

        const { gameIndex, moveIndex, player } = selectedMove;
        const game = transcription.games[gameIndex];
        
        if (!game || !game.moves[moveIndex]) {
            console.log('[BoardEditPanel] No game or move at indices');
            return;
        }

        const move = game.moves[moveIndex];
        const playerMove = player === 1 ? move.player1Move : move.player2Move;
        
        // Keep existing move and flags when dice change
        const existingMove = playerMove?.move || '';
        const isIllegal = playerMove?.isIllegal || false;
        const isGala = playerMove?.isGala || false;
        
        console.log('[BoardEditPanel] Calling updateMove with dice:', newDice, 'preserving move:', existingMove);
        updateMove(gameIndex, move.moveNumber, player, newDice, existingMove, isIllegal, isGala);
        
        // Invalidate position cache to force board to recalculate with new dice
        await invalidatePositionsCacheFrom(gameIndex, moveIndex);
        
        // Trigger CandidateMovesPanel to re-analyze by updating selectedMoveStore
        // This forces the reactive subscription to fire and board to update
        selectedMoveStore.set({ ...selectedMove });
        console.log('[BoardEditPanel] updateMove completed, cache invalidated, triggered candidate analysis');
    }

    /**
     * Check if the current decision is the last non-empty decision in the current game
     * Returns true if we're editing the last decision that has dice or move data
     */
    function isEditingLastNonEmptyDecision() {
        const transcription = get(transcriptionStore);
        const selectedMove = get(selectedMoveStore);
        
        if (!transcription || !selectedMove) return false;
        
        const { gameIndex, moveIndex, player } = selectedMove;
        const game = transcription.games[gameIndex];
        if (!game) return false;
        
        // Find the last non-empty decision in this game
        // Go through moves in reverse to find last decision with dice or move
        for (let i = game.moves.length - 1; i >= 0; i--) {
            const move = game.moves[i];
            
            // Check player 2 first (later in turn order)
            const p2Move = move.player2Move;
            if (p2Move && (p2Move.dice || p2Move.move || p2Move.cubeAction || p2Move.resignAction)) {
                // This is the last non-empty decision
                return moveIndex === i && player === 2;
            }
            
            // Check player 1
            const p1Move = move.player1Move;
            if (p1Move && (p1Move.dice || p1Move.move || p1Move.cubeAction || p1Move.resignAction)) {
                // This is the last non-empty decision
                return moveIndex === i && player === 1;
            }
        }
        
        // No non-empty decision found, so this could be the first one
        return true;
    }

    function validateDiceInput(dice) {
        if (!dice) return false;
        
        const lowerDice = dice.toLowerCase();
        
        // Allow cube decisions
        if (lowerDice === 'd' || lowerDice === 't' || lowerDice === 'p') {
            return true;
        }
        
        // Allow resign decisions
        if (lowerDice === 'r' || lowerDice === 'g' || lowerDice === 'b') {
            return true;
        }
        
        // Allow empty for player1's first decision
        const selectedMove = get(selectedMoveStore);
        if (selectedMove) {
            const { gameIndex, moveIndex, player } = selectedMove;
            if (moveIndex === 0 && player === 1 && dice === '') {
                return true; // Allow empty dice for player1's first decision
            }
        }
        
        if (dice.length !== 2) return false;
        const d1 = parseInt(dice[0]);
        const d2 = parseInt(dice[1]);
        return d1 >= 1 && d1 <= 6 && d2 >= 1 && d2 <= 6;
    }

    onMount(() => {
    });

    onDestroy(() => {
    });
    
    // Check if dice is a cube decision or resign decision
    $: isCubeDecision = diceInput.toLowerCase() === 'd' || diceInput.toLowerCase() === 't' || diceInput.toLowerCase() === 'p';
    $: isResignDecision = diceInput.toLowerCase() === 'r' || diceInput.toLowerCase() === 'g' || diceInput.toLowerCase() === 'b';
    
    // Display formatted dice for user
    $: diceDisplay = (() => {
        if (!diceInput) return '';
        const lower = diceInput.toLowerCase();
        if (lower === 'd') return 'double';
        if (lower === 't') return 'take';
        if (lower === 'p') return 'pass';
        if (lower === 'r') return 'resign';
        if (lower === 'g') return 'resign gammon';
        if (lower === 'b') return 'resign backgammon';
        return diceInput;
    })();
</script>

{#if visible}
    <!-- No visible UI overlay - all interaction is keyboard-driven and reflected on the board -->
    {#if showManualInput}
        <div class="manual-input-bar">
            <input
                bind:this={manualInputElement}
                bind:value={manualMoveInput}
                on:keydown={handleKeyDown}
                type="text"
                placeholder="Enter move (e.g., 24/20 13/8)"
                class="manual-input"
            />
        </div>
    {/if}
{/if}

<svelte:window on:keydown={handleKeyDown} />

<style>
    .manual-input-bar {
        position: fixed;
        top: 350px;
        left: 50%;
        transform: translateX(-50%);
        z-index: 1000;
        width: 70%;
        pointer-events: auto;
    }

    .manual-input {
        width: 100%;
        padding: 8px;
        border: 1px solid rgba(74, 158, 255, 0.5);
        border-radius: 1px;
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
        outline: none;
        background-color: white;
        font-size: 18px;
        font-family: 'Courier New', monospace;
    }

    .manual-input:focus {
        border-color: #4a9eff;
        box-shadow: 0 2px 10px rgba(74, 158, 255, 0.5);
    }
</style>
