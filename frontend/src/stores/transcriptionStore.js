import { writable, derived, get } from 'svelte/store';

// LazyBG file format version
export const LAZYBG_VERSION = '1.0.0';

/**
 * Debug helper to dump current transcription state to console
 */
export function dumpTranscriptionState() {
    const t = get(transcriptionStore);
    console.log('=== TRANSCRIPTION STATE DUMP ===');
    console.log(`Match length: ${t.metadata?.matchLength}`);
    console.log(`Total games: ${t.games?.length || 0}`);
    
    if (t.games) {
        t.games.forEach((game, idx) => {
            console.log(`\nGame ${idx} (number ${game.gameNumber}):`);
            console.log(`  Scores: ${game.player1Score}-${game.player2Score}`);
            console.log(`  Moves: ${game.moves?.length || 0}`);
            console.log(`  Winner: ${game.winner ? `player ${game.winner.player}, ${game.winner.points} pts` : 'none'}`);
            
            // Show first few moves with cube actions
            if (game.moves) {
                game.moves.slice(0, 3).forEach((move, mIdx) => {
                    const p1cube = move.player1Move?.cubeAction ? ` [${move.player1Move.cubeAction}@${move.player1Move.cubeValue}]` : '';
                    const p2cube = move.player2Move?.cubeAction ? ` [${move.player2Move.cubeAction}@${move.player2Move.cubeValue}]` : '';
                    if (p1cube || p2cube) {
                        console.log(`    Move ${mIdx}: p1${p1cube} p2${p2cube}`);
                    }
                });
            }
        });
    }
    console.log('=== END DUMP ===\n');
}

/**
 * Migrates old cube action structure to new format
 * Old: move.cubeAction = { player, action, value, response }
 * New: player1Move.cubeAction = 'doubles'|'takes'|'drops', player1Move.cubeValue = number
 */
export function migrateCubeActions(transcription) {
    if (!transcription || !transcription.games) return transcription;
    
    for (const game of transcription.games) {
        if (!game.moves) continue;
        
        for (const move of game.moves) {
            if (!move.cubeAction) continue;
            
            const { player, action, value, response } = move.cubeAction;
            
            // Migrate the action to the appropriate player's move
            const playerMove = player === 1 ? 'player1Move' : 'player2Move';
            if (!move[playerMove]) {
                move[playerMove] = { dice: '', move: '', isIllegal: false, isGala: false };
            }
            move[playerMove].cubeAction = action;
            move[playerMove].cubeValue = value;
            
            // If there's a response, migrate it to the other player's move
            if (response) {
                const otherPlayerMove = player === 1 ? 'player2Move' : 'player1Move';
                if (!move[otherPlayerMove]) {
                    move[otherPlayerMove] = { dice: '', move: '', isIllegal: false, isGala: false };
                }
                move[otherPlayerMove].cubeAction = response;
                move[otherPlayerMove].cubeValue = value;
            }
            
            // Clear the old structure
            move.cubeAction = null;
        }
    }
    
    return transcription;
}

// Structure d'une transcription complète
export const transcriptionStore = writable({
    version: LAZYBG_VERSION,
    metadata: {
        site: '',
        matchId: '',
        event: '',
        round: '',
        player1: '',
        player2: '',
        eventDate: '',
        eventTime: '',
        variation: 'Backgammon',
        unrated: 'Off',
        crawford: 'On',
        cubeLimit: '1024',
        transcriber: '',
        matchLength: 0
    },
    games: []
    // games structure: [{
    //   gameNumber: 1,
    //   player1Score: 0,
    //   player2Score: 0,
    //   moves: [{
    //     moveNumber: 1,
    //     player1Move: { dice: '54', move: '24/20 13/8', cubeAction: 'doubles'|'takes'|'drops', cubeValue: 2, isIllegal: false, isGala: false },
    //     player2Move: { dice: '31', move: '8/5* 6/5', cubeAction: 'doubles'|'takes'|'drops', cubeValue: 2, isIllegal: false, isGala: false }
    //   }],
    //   winner: null, // { player: 1|2, points: 1|2 }
    // }]
});

// Index du coup sélectionné: {gameIndex, moveIndex, player: 1|2}
export const selectedMoveStore = writable({ gameIndex: 0, moveIndex: 0, player: 1 });

// Chemin du fichier courant
export const transcriptionFilePathStore = writable('');

// Inversion des couleurs des checkers
export const invertColorsStore = writable(false);

// Cache des positions calculées pour optimisation
export const positionsCacheStore = writable({});

// Store pour les incohérences détectées dans les coups
// Format: { gameIndex: { moveIndex: { player: 1|2, reason: string } } }
export const moveInconsistenciesStore = writable({});

// Derived store: validation du match
export const matchValidationStore = derived(
    transcriptionStore,
    $transcription => {
        if (!$transcription.metadata.matchLength) {
            return { valid: false, reason: 'Match length not defined' };
        }
        
        let player1TotalScore = 0;
        let player2TotalScore = 0;
        
        for (const game of $transcription.games) {
            if (game.winner) {
                if (game.winner.player === 1) {
                    player1TotalScore += game.winner.points;
                } else if (game.winner.player === 2) {
                    player2TotalScore += game.winner.points;
                }
            }
        }
        
        const targetScore = $transcription.metadata.matchLength;
        const isComplete = player1TotalScore >= targetScore || player2TotalScore >= targetScore;
        
        return {
            valid: isComplete,
            reason: isComplete ? 'Match complete' : `Match in progress (${player1TotalScore}-${player2TotalScore})`,
            player1Score: player1TotalScore,
            player2Score: player2TotalScore,
            targetScore: targetScore
        };
    }
);

// Helper functions
export function addGame(gameNumber, player1Score, player2Score) {
    transcriptionStore.update(t => {
        t.games.push({
            gameNumber,
            player1Score,
            player2Score,
            moves: [{
                moveNumber: 1,
                player1Move: { dice: '', move: '', isIllegal: false, isGala: false },
                player2Move: null,
                cubeAction: null
            }],
            winner: null
        });
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
}

export function addMove(gameIndex, moveNumber, player, dice, move, isIllegal = false, isGala = false) {
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        // Find or create move entry
        let moveEntry = game.moves.find(m => m.moveNumber === moveNumber);
        if (!moveEntry) {
            moveEntry = {
                moveNumber,
                player1Move: null,
                player2Move: null,
                cubeAction: null
            };
            game.moves.push(moveEntry);
            game.moves.sort((a, b) => a.moveNumber - b.moveNumber);
        }
        
        const moveData = { dice, move, isIllegal, isGala };
        if (player === 1) {
            moveEntry.player1Move = moveData;
        } else {
            moveEntry.player2Move = moveData;
        }
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
    
    // Invalider le cache des positions après ce coup
    invalidatePositionsCacheFrom(gameIndex, moveNumber);
}

export function insertMoveBefore(gameIndex, moveIndex) {
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        // Incrémenter tous les numéros de coups à partir de moveIndex
        game.moves.forEach(move => {
            if (move.moveNumber >= moveIndex) {
                move.moveNumber++;
            }
        });
        
        // Insérer le nouveau coup
        game.moves.push({
            moveNumber: moveIndex,
            player1Move: null,
            player2Move: null,
            cubeAction: null
        });
        game.moves.sort((a, b) => a.moveNumber - b.moveNumber);
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
}

export function insertMoveAfter(gameIndex, moveIndex) {
    insertMoveBefore(gameIndex, moveIndex + 1);
}

/**
 * Insert a decision (player-specific move) before the current selected decision
 * @param {number} gameIndex - Game index
 * @param {number} moveIndex - Move index
 * @param {number} player - Player (1 or 2)
 */
export function insertDecisionBefore(gameIndex, moveIndex, player) {
    console.log(`[insertDecisionBefore] gameIndex=${gameIndex}, moveIndex=${moveIndex}, player=${player}`);
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        console.log(`[insertDecisionBefore] Before insertion:`, JSON.stringify(game.moves.slice(0, 5), null, 2));
        
        const currentMove = game.moves[moveIndex];
        if (!currentMove) return t;
        
        const currentMoveNumber = currentMove.moveNumber;
        
        // Collect all decisions from the insertion point onwards
        // Use deep copies to avoid reference sharing issues
        const decisionsToShift = [];
        
        for (let i = moveIndex; i < game.moves.length; i++) {
            const move = game.moves[i];
            
            // For the current move, collect decisions from insertion point onwards
            if (i === moveIndex) {
                if (player === 1) {
                    // Collect player1Move (even if null/empty)
                    decisionsToShift.push({ decision: move.player1Move ? JSON.parse(JSON.stringify(move.player1Move)) : null });
                }
                // Always collect player2Move (even if null/empty)
                decisionsToShift.push({ decision: move.player2Move ? JSON.parse(JSON.stringify(move.player2Move)) : null });
            } else {
                // All decisions from subsequent moves (even if null/empty)
                decisionsToShift.push({ decision: move.player1Move ? JSON.parse(JSON.stringify(move.player1Move)) : null });
                decisionsToShift.push({ decision: move.player2Move ? JSON.parse(JSON.stringify(move.player2Move)) : null });
            }
        }
        
        console.log(`[insertDecisionBefore] Decisions to shift:`, decisionsToShift.length);
        
        // Determine where empty slot goes and where redistribution starts
        let emptyMoveIndex, emptyPlayer;
        let redistributeStartMoveIndex, redistributeStartPlayer;
        
        if (player === 1) {
            // Inserting before player1 of current move
            // Empty slot goes to player1 of current move
            emptyMoveIndex = moveIndex;
            emptyPlayer = 1;
            // Redistribution starts at player2 of current move
            redistributeStartMoveIndex = moveIndex;
            redistributeStartPlayer = 2;
        } else {
            // Inserting before player2 of current move
            // Empty slot goes to player2 of current move
            emptyMoveIndex = moveIndex;
            emptyPlayer = 2;
            // Redistribution starts at player1 of next move
            redistributeStartMoveIndex = moveIndex + 1;
            redistributeStartPlayer = 1;
        }
        
        // Clear all affected slots from the empty slot position onward
        for (let i = emptyMoveIndex; i < game.moves.length; i++) {
            if (i === emptyMoveIndex && emptyPlayer === 1) {
                game.moves[i].player1Move = null;
                game.moves[i].player2Move = null;
            } else if (i === emptyMoveIndex && emptyPlayer === 2) {
                game.moves[i].player2Move = null;
            } else {
                game.moves[i].player1Move = null;
                game.moves[i].player2Move = null;
            }
        }
        
        // Place shifted decisions starting from redistribution point
        let decisionIndex = 0;
        let targetMoveIndex = redistributeStartMoveIndex;
        let targetPlayer = redistributeStartPlayer;
        
        while (decisionIndex < decisionsToShift.length) {
            // Ensure target move exists
            if (targetMoveIndex >= game.moves.length) {
                const newMoveNumber = game.moves.length > 0 ? game.moves[game.moves.length - 1].moveNumber + 1 : 1;
                game.moves.push({ moveNumber: newMoveNumber, player1Move: null, player2Move: null, cubeAction: null });
            }
            
            const targetMove = game.moves[targetMoveIndex];
            const decisionToPlace = decisionsToShift[decisionIndex].decision;
            console.log(`[insertDecisionBefore] Placing decision ${decisionIndex} at move ${targetMoveIndex} player ${targetPlayer}: dice="${decisionToPlace?.dice}", move="${decisionToPlace?.move?.substring(0, 20)}"`);
            
            if (targetPlayer === 1) {
                targetMove.player1Move = decisionToPlace;
                targetPlayer = 2;
            } else {
                targetMove.player2Move = decisionToPlace;
                targetPlayer = 1;
                targetMoveIndex++;
            }
            
            decisionIndex++;
        }
        
        game.moves.sort((a, b) => a.moveNumber - b.moveNumber);
        console.log(`[insertDecisionBefore] After insertion:`, JSON.stringify(game.moves.slice(0, 5), null, 2));
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
}

/**
 * Insert a decision (player-specific move) after the current selected decision
 * @param {number} gameIndex - Game index
 * @param {number} moveIndex - Move index
 * @param {number} player - Player (1 or 2)
 */
export function insertDecisionAfter(gameIndex, moveIndex, player) {
    console.log(`[insertDecisionAfter] gameIndex=${gameIndex}, moveIndex=${moveIndex}, player=${player}`);
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        console.log(`[insertDecisionAfter] Before insertion:`, JSON.stringify(game.moves.slice(0, 5), null, 2));
        
        const currentMove = game.moves[moveIndex];
        if (!currentMove) return t;
        
        const currentMoveNumber = currentMove.moveNumber;
        
        // Collect all decisions after the insertion point in sequence
        // Use deep copies to avoid reference sharing issues
        const decisionsToShift = [];
        
        for (let i = moveIndex; i < game.moves.length; i++) {
            const move = game.moves[i];
            
            // For the current move, only collect decisions after the insertion point
            if (i === moveIndex) {
                if (player === 1) {
                    // Collect player2Move (even if null/empty)
                    decisionsToShift.push({ decision: move.player2Move ? JSON.parse(JSON.stringify(move.player2Move)) : null });
                }
                // If player === 2, we don't collect anything from current move
            } else {
                // All decisions from subsequent moves (even if null/empty)
                decisionsToShift.push({ decision: move.player1Move ? JSON.parse(JSON.stringify(move.player1Move)) : null });
                decisionsToShift.push({ decision: move.player2Move ? JSON.parse(JSON.stringify(move.player2Move)) : null });
            }
        }
        
        console.log(`[insertDecisionAfter] Decisions to shift:`, decisionsToShift.length);
        console.log(`[insertDecisionAfter] Collected decisions:`, JSON.stringify(decisionsToShift.map(d => ({dice: d.decision?.dice || null, move: d.decision?.move?.substring(0, 20) || null})), null, 2));
        
        // Determine where the empty slot goes and where redistribution starts
        let emptyMoveIndex, emptyPlayer;
        let redistributeStartMoveIndex, redistributeStartPlayer;
        
        if (player === 1) {
            // Inserting after player1 of current move
            // Empty slot goes to player2 of current move
            emptyMoveIndex = moveIndex;
            emptyPlayer = 2;
            // Redistribution starts at player1 of next move
            redistributeStartMoveIndex = moveIndex + 1;
            redistributeStartPlayer = 1;
        } else {
            // Inserting after player2 of current move
            // Empty slot goes to player1 of next move
            emptyMoveIndex = moveIndex + 1;
            emptyPlayer = 1;
            // Redistribution starts at player2 of next move (after the empty slot)
            redistributeStartMoveIndex = moveIndex + 1;
            redistributeStartPlayer = 2;
            
            // Ensure next move exists for the empty slot
            if (moveIndex + 1 >= game.moves.length) {
                const newMoveNumber = game.moves.length > 0 ? game.moves[game.moves.length - 1].moveNumber + 1 : 1;
                game.moves.push({ moveNumber: newMoveNumber, player1Move: null, player2Move: null, cubeAction: null });
            }
        }
        
        // Clear all affected slots (from where empty slot goes onward)
        for (let i = emptyMoveIndex; i < game.moves.length; i++) {
            if (i === emptyMoveIndex && emptyPlayer === 2) {
                game.moves[i].player2Move = null;
            } else if (i === emptyMoveIndex && emptyPlayer === 1) {
                game.moves[i].player1Move = null;
                game.moves[i].player2Move = null;
            } else {
                game.moves[i].player1Move = null;
                game.moves[i].player2Move = null;
            }
        }
        
        // Place shifted decisions starting from the redistribution point
        let decisionIndex = 0;
        let targetMoveIndex = redistributeStartMoveIndex;
        let targetPlayer = redistributeStartPlayer;
        
        while (decisionIndex < decisionsToShift.length) {
            // Ensure target move exists
            if (targetMoveIndex >= game.moves.length) {
                const newMoveNumber = game.moves.length > 0 ? game.moves[game.moves.length - 1].moveNumber + 1 : 1;
                game.moves.push({ moveNumber: newMoveNumber, player1Move: null, player2Move: null, cubeAction: null });
            }
            
            const targetMove = game.moves[targetMoveIndex];
            const decisionToPlace = decisionsToShift[decisionIndex].decision;
            console.log(`[insertDecisionAfter] Placing decision ${decisionIndex} at move ${targetMoveIndex} player ${targetPlayer}: dice="${decisionToPlace?.dice}", move="${decisionToPlace?.move?.substring(0, 20)}"`);
            
            if (targetPlayer === 1) {
                targetMove.player1Move = decisionToPlace;
                targetPlayer = 2;
            } else {
                targetMove.player2Move = decisionToPlace;
                targetPlayer = 1;
                targetMoveIndex++;
            }
            
            decisionIndex++;
        }
        
        game.moves.sort((a, b) => a.moveNumber - b.moveNumber);
        console.log(`[insertDecisionAfter] After insertion:`, JSON.stringify(game.moves.slice(0, 5), null, 2));
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
}

/**
 * Delete a decision (player-specific move) and shift subsequent decisions
 * @param {number} gameIndex - Game index
 * @param {number} moveIndex - Move index
 * @param {number} player - Player (1 or 2)
 */
export function deleteDecision(gameIndex, moveIndex, player) {
    console.log(`[deleteDecision] gameIndex=${gameIndex}, moveIndex=${moveIndex}, player=${player}`);
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        console.log(`[deleteDecision] Before deletion:`, JSON.stringify(game.moves.slice(0, 5), null, 2));
        
        const currentMove = game.moves[moveIndex];
        if (!currentMove) return t;
        
        // Collect all decisions after the deletion point in sequence
        const decisionsToShift = [];
        
        for (let i = moveIndex; i < game.moves.length; i++) {
            const move = game.moves[i];
            
            // For the current move, skip the decision being deleted
            if (i === moveIndex) {
                if (player === 1) {
                    // Skip player1Move, collect player2Move
                    if (move.player2Move) {
                        decisionsToShift.push({ decision: JSON.parse(JSON.stringify(move.player2Move)) });
                    }
                }
                // If player === 2, skip player2Move but there's nothing after in this move
            } else {
                // Collect all decisions from subsequent moves
                if (move.player1Move) {
                    decisionsToShift.push({ decision: JSON.parse(JSON.stringify(move.player1Move)) });
                }
                if (move.player2Move) {
                    decisionsToShift.push({ decision: JSON.parse(JSON.stringify(move.player2Move)) });
                }
            }
        }
        
        console.log(`[deleteDecision] Decisions to shift:`, decisionsToShift.length);
        
        // Clear all affected slots starting from the deletion point
        for (let i = moveIndex; i < game.moves.length; i++) {
            if (i === moveIndex) {
                if (player === 1) {
                    game.moves[i].player1Move = null;
                    game.moves[i].player2Move = null;
                } else {
                    game.moves[i].player2Move = null;
                }
            } else {
                game.moves[i].player1Move = null;
                game.moves[i].player2Move = null;
            }
        }
        
        // Redistribute shifted decisions starting from the deletion point
        let decisionIndex = 0;
        let targetMoveIndex = moveIndex;
        let targetPlayer = player;
        
        while (decisionIndex < decisionsToShift.length) {
            // Ensure target move exists
            if (targetMoveIndex >= game.moves.length) {
                break; // No more moves to fill
            }
            
            const targetMove = game.moves[targetMoveIndex];
            const decisionToPlace = decisionsToShift[decisionIndex].decision;
            console.log(`[deleteDecision] Placing decision ${decisionIndex} at move ${targetMoveIndex} player ${targetPlayer}`);
            
            if (targetPlayer === 1) {
                targetMove.player1Move = decisionToPlace;
                targetPlayer = 2;
            } else {
                targetMove.player2Move = decisionToPlace;
                targetPlayer = 1;
                targetMoveIndex++;
            }
            
            decisionIndex++;
        }
        
        game.moves.sort((a, b) => a.moveNumber - b.moveNumber);
        
        // Remove trailing empty moves (moves where both players are null)
        while (game.moves.length > 0) {
            const lastMove = game.moves[game.moves.length - 1];
            if (!lastMove.player1Move && !lastMove.player2Move && !lastMove.cubeAction) {
                console.log(`[deleteDecision] Removing empty trailing move ${lastMove.moveNumber}`);
                game.moves.pop();
            } else {
                break;
            }
        }
        
        console.log(`[deleteDecision] After deletion:`, JSON.stringify(game.moves.slice(0, 5), null, 2));
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
    
    // If this was a cube action deletion, recalculate scores
    recalculateScoresFromGame(gameIndex);
}

/**
 * Delete multiple decisions at once
 * @param {Array} decisions - Array of {gameIndex, moveIndex, player} objects
 */
export function deleteDecisions(decisions) {
    if (!decisions || decisions.length === 0) return;
    
    console.log(`[deleteDecisions] Deleting ${decisions.length} decisions`);
    
    // Sort decisions in reverse order (from last to first) to avoid index shifting issues
    const sortedDecisions = [...decisions].sort((a, b) => {
        if (a.gameIndex !== b.gameIndex) return b.gameIndex - a.gameIndex;
        if (a.moveIndex !== b.moveIndex) return b.moveIndex - a.moveIndex;
        return b.player - a.player; // player 2 before player 1
    });
    
    // Delete each decision
    sortedDecisions.forEach(({ gameIndex, moveIndex, player }) => {
        deleteDecision(gameIndex, moveIndex, player);
    });
    
    // Invalidate cache from the earliest affected position
    const earliestDecision = decisions.reduce((earliest, current) => {
        if (!earliest) return current;
        if (current.gameIndex < earliest.gameIndex) return current;
        if (current.gameIndex === earliest.gameIndex && current.moveIndex < earliest.moveIndex) return current;
        return earliest;
    }, null);
    
    if (earliestDecision) {
        invalidatePositionsCacheFrom(earliestDecision.gameIndex, earliestDecision.moveIndex);
    }
}

export function deleteMove(gameIndex, moveIndex) {
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        game.moves = game.moves.filter(m => m.moveNumber !== moveIndex);
        
        // Renuméroter les coups suivants
        game.moves.forEach(move => {
            if (move.moveNumber > moveIndex) {
                move.moveNumber--;
            }
        });
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
}

export function updateMove(gameIndex, moveIndex, player, dice, move, isIllegal = false, isGala = false) {
    let isCubeAction = false;
    let isResignAction = false;
    
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        const moveEntry = game.moves.find(m => m.moveNumber === moveIndex);
        if (!moveEntry) return t;
        
        // Check if this is a resign decision (r/g/b)
        const diceStr = dice.toLowerCase();
        if (diceStr === 'r' || diceStr === 'g' || diceStr === 'b') {
            isResignAction = true;
            
            // Determine resign type
            let resignType;
            if (diceStr === 'r') {
                resignType = 'normal';
            } else if (diceStr === 'g') {
                resignType = 'gammon';
            } else { // 'b'
                resignType = 'backgammon';
            }
            
            // Store resign action in the player's move data
            const moveData = { 
                dice: '', 
                move: '', 
                isIllegal: false, 
                isGala: false,
                resignAction: resignType
            };
            
            if (player === 1) {
                moveEntry.player1Move = moveData;
            } else {
                moveEntry.player2Move = moveData;
            }
        } else if (diceStr === 'd' || diceStr === 't' || diceStr === 'p') {
            // Check if this is a cube decision (d/t/p)
            isCubeAction = true;
            
            // Handle cube decision independently for this player
            // Calculate cube value by looking at previous cube actions
            let cubeValue = 1;
            
            // Look through all previous moves and both players' actions
            for (let i = 0; i < game.moves.length; i++) {
                const mv = game.moves[i];
                if (!mv || mv.moveNumber >= moveIndex) break;
                
                // Check player1's cube action
                if (mv.player1Move?.cubeAction === 'doubles') {
                    cubeValue = mv.player1Move.cubeValue || (cubeValue * 2);
                }
                // Check player2's cube action
                if (mv.player2Move?.cubeAction === 'doubles') {
                    cubeValue = mv.player2Move.cubeValue || (cubeValue * 2);
                }
            }
            
            // Determine action and cube value
            let action, value;
            if (diceStr === 'd') {
                action = 'doubles';
                value = cubeValue * 2;
            } else if (diceStr === 't') {
                action = 'takes';
                value = cubeValue;
            } else { // 'p'
                action = 'drops';
                value = cubeValue;
            }
            
            // Store cube action in the player's move data
            const moveData = { 
                dice: '', 
                move: '', 
                isIllegal: false, 
                isGala: false,
                cubeAction: action,
                cubeValue: value
            };
            
            if (player === 1) {
                moveEntry.player1Move = moveData;
            } else {
                moveEntry.player2Move = moveData;
            }
            
            // Clear old cubeAction structure if it exists for this player
            if (moveEntry.cubeAction && moveEntry.cubeAction.player === player) {
                moveEntry.cubeAction = null;
            }
        } else {
            // Regular move with dice
            const moveData = { dice, move, isIllegal, isGala };
            if (player === 1) {
                moveEntry.player1Move = moveData;
            } else {
                moveEntry.player2Move = moveData;
            }
        }
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
    
    // If this was a cube action or resign action, recalculate scores for this game and all subsequent games
    if (isCubeAction || isResignAction) {
        console.log(`[updateMove] ${isResignAction ? 'Resign' : 'Cube'} action detected, recalculating scores from game ${gameIndex}`);
        recalculateScoresFromGame(gameIndex);
    }
}

export function setGameWinner(gameIndex, player, points) {
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        game.winner = { player, points };
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
}

export async function invalidatePositionsCacheFrom(gameIndex, moveIndex) {
    positionsCacheStore.update(cache => {
        const newCache = {};
        for (const key in cache) {
            const [gIdx, mIdx] = key.split('-').map(Number);
            if (gIdx < gameIndex || (gIdx === gameIndex && mIdx < moveIndex)) {
                newCache[key] = cache[key];
            }
        }
        return newCache;
    });
    
    // Auto-correct hit markers in subsequent moves
    await autoCorrectHitMarkers(gameIndex, moveIndex);
    
    // Recalculate inconsistencies for affected game starting from moveIndex
    validateGameInconsistencies(gameIndex, moveIndex);
}

/**
 * Auto-correct hit markers (*) in moves based on actual position
 * Recalculates which moves actually cause hits and updates notation
 */
export async function autoCorrectHitMarkers(gameIndex, startMoveIndex = 0) {
    try {
        const { 
            createInitialPosition, 
            applyMove, 
            removeHitMarker, 
            addHitMarker 
        } = await import('../utils/positionCalculator.js');
        
        const transcription = get(transcriptionStore);
        
        if (!transcription || !transcription.games || !transcription.games[gameIndex]) {
            return;
        }
        
        const game = transcription.games[gameIndex];
        let position = createInitialPosition();
        
        // Build position up to startMoveIndex
        for (let i = 0; i < startMoveIndex && i < game.moves.length; i++) {
            const move = game.moves[i];
            
            // Skip cube decisions (they don't have move property or have empty move)
            if (move.player1Move && !move.player1Move.cubeAction && move.player1Move.move && 
                move.player1Move.move !== 'Cannot Move' && 
                move.player1Move.move !== '????') {
                const cleanMove = removeHitMarker(move.player1Move.move);
                const result = applyMove(position, cleanMove, true);
                position = result.position;
            }
            
            if (move.player2Move && !move.player2Move.cubeAction && move.player2Move.move &&
                move.player2Move.move !== 'Cannot Move' && 
                move.player2Move.move !== '????') {
                const cleanMove = removeHitMarker(move.player2Move.move);
                const result = applyMove(position, cleanMove, false);
                position = result.position;
            }
        }
        
        // Auto-correct moves from startMoveIndex onwards
        transcriptionStore.update(t => {
            const currentGame = t.games[gameIndex];
            
            for (let i = startMoveIndex; i < currentGame.moves.length; i++) {
                const move = currentGame.moves[i];
                
                // Check and correct player1's move (skip cube decisions)
                if (move.player1Move && !move.player1Move.cubeAction && move.player1Move.move && 
                    move.player1Move.move !== 'Cannot Move' && 
                    move.player1Move.move !== '????') {
                    
                    const cleanMove = removeHitMarker(move.player1Move.move);
                    const result = applyMove(position, cleanMove, true);
                    
                    // Update notation based on actual hit segments
                    const correctNotation = result.hasHit 
                        ? addHitMarker(cleanMove, result.hitSegments) 
                        : cleanMove;
                    if (correctNotation !== move.player1Move.move) {
                        move.player1Move.move = correctNotation;
                    }
                    
                    position = result.position;
                }
                
                // Check and correct player2's move (skip cube decisions)
                if (move.player2Move && !move.player2Move.cubeAction && move.player2Move.move &&
                    move.player2Move.move !== 'Cannot Move' && 
                    move.player2Move.move !== '????') {
                    
                    const cleanMove = removeHitMarker(move.player2Move.move);
                    const result = applyMove(position, cleanMove, false);
                    
                    // Update notation based on actual hit segments
                    const correctNotation = result.hasHit 
                        ? addHitMarker(cleanMove, result.hitSegments) 
                        : cleanMove;
                    if (correctNotation !== move.player2Move.move) {
                        move.player2Move.move = correctNotation;
                    }
                    
                    position = result.position;
                }
            }
            
            // Return a deep copy to prevent reference sharing issues with subscribers
            return JSON.parse(JSON.stringify(t));
        });
    } catch (error) {
        console.error('Error auto-correcting hit markers:', error);
    }
}

export async function validateGameInconsistencies(gameIndex, startMoveIndex = 0) {
    try {
        const { validateGamePositions } = await import('../utils/positionCalculator.js');
        const transcription = get(transcriptionStore);
        
        if (!transcription || !transcription.games || !transcription.games[gameIndex]) {
            return;
        }
        
        const game = transcription.games[gameIndex];
        const inconsistencies = validateGamePositions(game, startMoveIndex, transcription, gameIndex);
        
        // Add score validation for first move (moveIndex 0) if starting from beginning
        if (startMoveIndex === 0 && transcription.metadata.matchLength) {
            const scoreValidation = validateGameScores(
                game.player1Score,
                game.player2Score,
                transcription.metadata.matchLength
            );
            
            if (!scoreValidation.valid) {
                // If the game itself is impossible (starting scores already win/exceed match length),
                // mark ALL decisions in the game as inconsistent
                if (scoreValidation.isImpossibleGame) {
                    // Mark all moves in the game as inconsistent
                    for (let mIdx = 0; mIdx < game.moves.length; mIdx++) {
                        const move = game.moves[mIdx];
                        if (move.player1Move || (mIdx === 0 && !move.player2Move)) {
                            inconsistencies.push({
                                moveIndex: mIdx,
                                player: 1,
                                reason: scoreValidation.reason,
                                type: 'impossible-game'
                            });
                        }
                        if (move.player2Move) {
                            inconsistencies.push({
                                moveIndex: mIdx,
                                player: 2,
                                reason: scoreValidation.reason,
                                type: 'impossible-game'
                            });
                        }
                    }
                } else {
                    // Just a score inconsistency at the start
                    inconsistencies.push({
                        moveIndex: 0,
                        player: 1,
                        reason: `Score inconsistency: ${scoreValidation.reason}`,
                        type: 'score'
                    });
                }
            }
        }
        
        moveInconsistenciesStore.update(store => {
            // Keep existing inconsistencies before startMoveIndex
            const existingInconsistencies = store[gameIndex] || {};
            const preservedInconsistencies = {};
            
            // Preserve inconsistencies from before startMoveIndex
            for (const key in existingInconsistencies) {
                const [moveIdx] = key.split('-').map(Number);
                if (moveIdx < startMoveIndex) {
                    preservedInconsistencies[key] = existingInconsistencies[key];
                }
            }
            
            // Add new inconsistencies from startMoveIndex onwards
            for (const inc of inconsistencies) {
                const key = `${inc.moveIndex}-${inc.player}`;
                preservedInconsistencies[key] = inc;
            }
            
            if (Object.keys(preservedInconsistencies).length === 0) {
                // Remove all inconsistencies for this game if none left
                delete store[gameIndex];
            } else {
                store[gameIndex] = preservedInconsistencies;
            }
            
            return store;
        });
    } catch (error) {
        console.error('Error validating game inconsistencies:', error);
    }
}

export function updateMetadata(metadata) {
    transcriptionStore.update(t => {
        const oldMatchLength = t.metadata.matchLength;
        t.metadata = { ...t.metadata, ...metadata };
        
        // If matchLength changed, clear position cache to force recalculation of away scores
        if (metadata.matchLength !== undefined && oldMatchLength !== metadata.matchLength) {
            positionsCacheStore.set({});
        }
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
}

export function swapPlayers() {
    transcriptionStore.update(t => {
        // Swap player names in metadata
        const temp = t.metadata.player1;
        t.metadata.player1 = t.metadata.player2;
        t.metadata.player2 = temp;
        
        // Swap all moves in all games
        for (const game of t.games) {
            // Swap scores
            const tempScore = game.player1Score;
            game.player1Score = game.player2Score;
            game.player2Score = tempScore;
            
            // Swap winner
            if (game.winner) {
                game.winner.player = game.winner.player === 1 ? 2 : 1;
            }
            
            // Swap moves
            for (const move of game.moves) {
                const tempMove = move.player1Move;
                move.player1Move = move.player2Move;
                move.player2Move = tempMove;
            }
        }
        
        // Return a deep copy to prevent reference sharing issues with subscribers
        return JSON.parse(JSON.stringify(t));
    });
}

export function clearTranscription() {
    transcriptionStore.set({
        version: LAZYBG_VERSION,
        metadata: {
            site: '',
            matchId: '',
            event: '',
            round: '',
            player1: '',
            player2: '',
            eventDate: '',
            eventTime: '',
            variation: 'Backgammon',
            unrated: 'Off',
            crawford: 'On',
            cubeLimit: '1024',
            transcriber: '',
            matchLength: 0
        },
        games: []
    });
    selectedMoveStore.set({ gameIndex: 0, moveIndex: 0, player: 1 });
    transcriptionFilePathStore.set('');
    positionsCacheStore.set({});
    moveInconsistenciesStore.set({});
}

/**
 * Calculate scores after a game based on winner and cube value
 * @param {Object} game - The game object
 * @param {number} matchLength - Match length from metadata
 * @param {number} startingPlayer1Score - Optional starting score for player 1 (if not provided, uses game.player1Score)
 * @param {number} startingPlayer2Score - Optional starting score for player 2 (if not provided, uses game.player2Score)
 * @returns {Object} - { player1Score, player2Score }
 */
function calculateScoresAfterGame(game, matchLength, startingPlayer1Score = null, startingPlayer2Score = null) {
    // Use provided starting scores or fall back to game's scores
    let player1Score = startingPlayer1Score !== null ? startingPlayer1Score : (game.player1Score || 0);
    let player2Score = startingPlayer2Score !== null ? startingPlayer2Score : (game.player2Score || 0);
    
    console.log(`[calculateScoresAfterGame] Game ${game.gameNumber}, starting scores: p1=${player1Score}, p2=${player2Score}`);
    
    // Try to determine winner and points from game data
    let winner = null;
    let pointsWon = 1;
    let cubeValue = 1; // Track cube value through the game
    
    // First, check if winner is explicitly set
    if (game.winner && game.winner.player && game.winner.points) {
        winner = game.winner.player;
        pointsWon = game.winner.points;
        console.log(`[calculateScoresAfterGame] Winner explicitly set: player ${winner}, points ${pointsWon}`);
    } else if (game.moves && game.moves.length > 0) {
        console.log(`[calculateScoresAfterGame] Analyzing ${game.moves.length} moves...`);
        // Try to detect winner from cube actions (drops) or resignations
        // Need to track cube value as we go through moves
        for (let i = 0; i < game.moves.length; i++) {
            const move = game.moves[i];
            
            console.log(`[calculateScoresAfterGame] Move ${i} (moveNumber ${move.moveNumber}): p1cube=${move.player1Move?.cubeAction}, p2cube=${move.player2Move?.cubeAction}`);
            
            // Check for player 1's cube action first
            if (move.player1Move?.cubeAction === 'doubles') {
                // Player 1 doubles - check if player 2 drops in response
                if (move.player2Move?.cubeAction === 'drops') {
                    // Player 2 dropped, player 1 wins the cube value BEFORE the double
                    winner = 1;
                    pointsWon = cubeValue;
                    console.log(`[calculateScoresAfterGame] Move ${i}: Player 1 doubles, player 2 drops, player 1 wins ${pointsWon} points (cube before double)`);
                    break;
                }
                // Player 2 took or no response yet, update cube value
                cubeValue = move.player1Move.cubeValue || (cubeValue * 2);
                console.log(`[calculateScoresAfterGame] Move ${i}: Player 1 doubles, cube now ${cubeValue}`);
            } else if (move.player1Move?.cubeAction === 'drops') {
                // Player 1 dropped (responding to player 2's double), player 2 wins
                winner = 2;
                pointsWon = cubeValue;
                console.log(`[calculateScoresAfterGame] Move ${i}: Player 1 drops, player 2 wins ${pointsWon} points (cubeValue=${cubeValue})`);
                break;
            }
            
            // Check for player 2's cube action
            if (move.player2Move?.cubeAction === 'doubles') {
                // Player 2 doubles - check if player 1 drops in the NEXT move
                // We need to look ahead to see if player 1 drops
                if (i + 1 < game.moves.length) {
                    const nextMove = game.moves[i + 1];
                    if (nextMove.player1Move?.cubeAction === 'drops') {
                        // Player 1 will drop, player 2 wins the cube value BEFORE the double
                        winner = 2;
                        pointsWon = cubeValue;
                        console.log(`[calculateScoresAfterGame] Move ${i}: Player 2 doubles, player 1 drops in next move, player 2 wins ${pointsWon} points (cube before double)`);
                        break;
                    }
                }
                // Player 1 took or no response yet, update cube value
                cubeValue = move.player2Move.cubeValue || (cubeValue * 2);
                console.log(`[calculateScoresAfterGame] Move ${i}: Player 2 doubles, cube now ${cubeValue}`);
            } else if (move.player2Move?.cubeAction === 'drops') {
                // Player 2 dropped (responding to player 1's double), player 1 wins
                // This should already be caught above when we see player 1's double
                if (winner === null) {
                    winner = 1;
                    pointsWon = cubeValue;
                    console.log(`[calculateScoresAfterGame] Move ${i}: Player 2 drops (delayed check), player 1 wins ${pointsWon} points`);
                    break;
                }
            }
            
            // Check for resign actions
            if (move.player1Move?.resignAction) {
                // Player 1 resigned, player 2 wins
                winner = 2;
                // Calculate points based on resign type and cube value
                if (move.player1Move.resignAction === 'normal') {
                    pointsWon = cubeValue; // 1x cube
                } else if (move.player1Move.resignAction === 'gammon') {
                    pointsWon = cubeValue * 2; // 2x cube
                } else if (move.player1Move.resignAction === 'backgammon') {
                    pointsWon = cubeValue * 3; // 3x cube
                }
                console.log(`[calculateScoresAfterGame] Move ${i}: Player 1 resigned ${move.player1Move.resignAction}, player 2 wins ${pointsWon} points`);
                break;
            }
            if (move.player2Move?.resignAction) {
                // Player 2 resigned, player 1 wins
                winner = 1;
                // Calculate points based on resign type and cube value
                if (move.player2Move.resignAction === 'normal') {
                    pointsWon = cubeValue; // 1x cube
                } else if (move.player2Move.resignAction === 'gammon') {
                    pointsWon = cubeValue * 2; // 2x cube
                } else if (move.player2Move.resignAction === 'backgammon') {
                    pointsWon = cubeValue * 3; // 3x cube
                }
                console.log(`[calculateScoresAfterGame] Move ${i}: Player 2 resigned ${move.player2Move.resignAction}, player 1 wins ${pointsWon} points`);
                break;
            }
            
            // Check for resignations (moves ending with '-') - legacy support
            if (move.player1Move?.move && move.player1Move.move.trim().endsWith('-')) {
                // Player 1 resigned, player 2 wins
                winner = 2;
                pointsWon = cubeValue; // Use current cube value
                console.log(`[calculateScoresAfterGame] Move ${i}: Player 1 resigned (legacy), player 2 wins ${pointsWon} points`);
                break;
            }
            if (move.player2Move?.move && move.player2Move.move.trim().endsWith('-')) {
                // Player 2 resigned, player 1 wins
                winner = 1;
                pointsWon = cubeValue; // Use current cube value
                console.log(`[calculateScoresAfterGame] Move ${i}: Player 2 resigned (legacy), player 1 wins ${pointsWon} points`);
                break;
            }
        }
        
        // If no winner detected but game has moves, assume implicit resignation
        // The last player who made a real move (not cube action only) wins
        if (winner === null && game.moves.length > 0) {
            console.log(`[calculateScoresAfterGame] No explicit winner found, checking for implicit resignation...`);
            
            // Find the last move with actual play (not just cube actions)
            for (let i = game.moves.length - 1; i >= 0; i--) {
                const move = game.moves[i];
                
                // Check if player 2 made the last real move
                if (move.player2Move && move.player2Move.move && move.player2Move.move.trim() !== '') {
                    // Player 2 made last move, player 2 wins (player 1 implicitly resigned)
                    winner = 2;
                    pointsWon = cubeValue; // Single game with current cube value
                    console.log(`[calculateScoresAfterGame] Implicit resignation detected: Player 2 made last move, player 2 wins ${pointsWon} points`);
                    break;
                }
                
                // Check if player 1 made the last real move
                if (move.player1Move && move.player1Move.move && move.player1Move.move.trim() !== '') {
                    // Player 1 made last move, player 1 wins (player 2 implicitly resigned)
                    winner = 1;
                    pointsWon = cubeValue; // Single game with current cube value
                    console.log(`[calculateScoresAfterGame] Implicit resignation detected: Player 1 made last move, player 1 wins ${pointsWon} points`);
                    break;
                }
            }
        }
    }
    
    // Apply the winner's points to the appropriate score
    if (winner === 1) {
        player1Score += pointsWon;
        console.log(`[calculateScoresAfterGame] Player 1 wins, score: ${player1Score}-${player2Score}`);
    } else if (winner === 2) {
        player2Score += pointsWon;
        console.log(`[calculateScoresAfterGame] Player 2 wins, score: ${player1Score}-${player2Score}`);
    } else {
        console.log(`[calculateScoresAfterGame] No winner detected, score unchanged: ${player1Score}-${player2Score}`);
    }
    
    return { player1Score, player2Score };
}

/**
 * Validate if scores are possible given match length and previous score
 * @param {number} player1Score - Current player 1 score
 * @param {number} player2Score - Current player 2 score
 * @param {number} matchLength - Match length
 * @returns {Object} - { valid: boolean, reason: string }
 */
function validateGameScores(player1Score, player2Score, matchLength) {
    // If no match length set, scores can't be validated
    if (!matchLength || matchLength === 0) {
        return { valid: true, reason: '', isImpossibleGame: false };
    }
    
    // Check if match should already be over (impossible game state)
    if (player1Score >= matchLength) {
        return { 
            valid: false, 
            reason: `Game impossible: Player 1 already won the match (${player1Score}/${matchLength})`,
            isImpossibleGame: true
        };
    }
    
    if (player2Score >= matchLength) {
        return { 
            valid: false, 
            reason: `Game impossible: Player 2 already won the match (${player2Score}/${matchLength})`,
            isImpossibleGame: true
        };
    }
    
    return { valid: true, reason: '', isImpossibleGame: false };
}

/**
 * Recalculate scores from the specified game onwards
 * @param {number} startGameIndex - Index of the first game to recalculate
 */
function recalculateScoresFromGame(startGameIndex) {
    console.log(`[recalculateScoresFromGame] Starting from game ${startGameIndex}`);
    
    transcriptionStore.update(t => {
        if (!t.games || startGameIndex < 0 || startGameIndex >= t.games.length) return t;
        
        // We need to recalculate ALL games from game 0 to ensure scores propagate correctly
        let currentPlayer1Score = 0;
        let currentPlayer2Score = 0;
        
        // Calculate scores for ALL games starting from game 0
        for (let i = 0; i < t.games.length; i++) {
            const currentGame = t.games[i];
            
            console.log(`[recalculateScoresFromGame] Game ${i}: starting scores ${currentPlayer1Score}-${currentPlayer2Score}`);
            
            // Update current game's starting scores
            currentGame.player1Score = currentPlayer1Score;
            currentGame.player2Score = currentPlayer2Score;
            
            // Calculate scores after this game (which will be used as starting scores for next game)
            const newScores = calculateScoresAfterGame(currentGame, t.metadata.matchLength, currentPlayer1Score, currentPlayer2Score);
            console.log(`[recalculateScoresFromGame] Game ${i}: after game scores ${newScores.player1Score}-${newScores.player2Score}`);
            
            // Update current scores for next iteration
            currentPlayer1Score = newScores.player1Score;
            currentPlayer2Score = newScores.player2Score;
        }
        
        return JSON.parse(JSON.stringify(t));
    });
}

/**
 * Insert a new empty game before the specified game index
 * @param {number} gameIndex - Index of the game to insert before
 */
export function insertGameBefore(gameIndex) {
    console.log(`[insertGameBefore] gameIndex=${gameIndex}`);
    console.log('[insertGameBefore] BEFORE:');
    dumpTranscriptionState();
    
    transcriptionStore.update(t => {
        if (!t.games || gameIndex < 0 || gameIndex > t.games.length) return t;
        
        // Calculate scores from previous game if it exists and is valid
        let player1Score = 0;
        let player2Score = 0;
        
        if (gameIndex > 0) {
            const prevGame = t.games[gameIndex - 1];
            const scores = calculateScoresAfterGame(prevGame, t.metadata.matchLength);
            player1Score = scores.player1Score;
            player2Score = scores.player2Score;
        } else {
            // If inserting before first game, use current game's scores
            const currentGame = t.games[gameIndex];
            player1Score = currentGame ? currentGame.player1Score : 0;
            player2Score = currentGame ? currentGame.player2Score : 0;
        }
        
        // Create new game with empty first move for player1
        const newGame = {
            gameNumber: gameIndex + 1,
            player1Score: player1Score,
            player2Score: player2Score,
            moves: [{
                moveNumber: 0,
                player1Move: { dice: '', move: '', isIllegal: false, isGala: false },
                player2Move: null,
                cubeAction: null
            }],
            winner: null
        };
        
        // Insert the new game at the specified index
        t.games.splice(gameIndex, 0, newGame);
        
        // Update game numbers and recalculate scores for all subsequent games
        console.log(`[insertGameBefore] Recalculating scores for games ${gameIndex + 1} to ${t.games.length - 1}`);
        for (let i = gameIndex + 1; i < t.games.length; i++) {
            t.games[i].gameNumber = i + 1;
            
            // Recalculate scores based on previous game
            const prevGame = t.games[i - 1];
            console.log(`[insertGameBefore] Recalculating game ${i} (number ${i + 1}) from previous game ${i - 1}`);
            const scores = calculateScoresAfterGame(prevGame, t.metadata.matchLength);
            console.log(`[insertGameBefore] Game ${i}: old scores ${t.games[i].player1Score}-${t.games[i].player2Score}, new scores ${scores.player1Score}-${scores.player2Score}`);
            t.games[i].player1Score = scores.player1Score;
            t.games[i].player2Score = scores.player2Score;
        }
        
        // Return a deep copy to prevent reference sharing issues
        return JSON.parse(JSON.stringify(t));
    });
    
    console.log('[insertGameBefore] AFTER:');
    dumpTranscriptionState();
    
    // Invalidate positions cache from the insertion point
    invalidatePositionsCacheFrom(gameIndex, 0);
}

/**
 * Insert a new empty game after the specified game index
 * @param {number} gameIndex - Index of the game to insert after
 */
export function insertGameAfter(gameIndex) {
    console.log(`[insertGameAfter] gameIndex=${gameIndex}`);
    console.log('[insertGameAfter] BEFORE:');
    dumpTranscriptionState();
    
    transcriptionStore.update(t => {
        if (!t.games || gameIndex < 0 || gameIndex >= t.games.length) return t;
        
        // Calculate scores from current game if it's valid
        const currentGame = t.games[gameIndex];
        const scores = calculateScoresAfterGame(currentGame, t.metadata.matchLength);
        
        // Create new game with empty first move for player1
        const newGame = {
            gameNumber: gameIndex + 2,
            player1Score: scores.player1Score,
            player2Score: scores.player2Score,
            moves: [{
                moveNumber: 0,
                player1Move: { dice: '', move: '', isIllegal: false, isGala: false },
                player2Move: null,
                cubeAction: null
            }],
            winner: null
        };
        
        // Insert the new game after the specified index
        t.games.splice(gameIndex + 1, 0, newGame);
        
        // Update game numbers and recalculate scores for all subsequent games
        console.log(`[insertGameAfter] Recalculating scores for games ${gameIndex + 2} to ${t.games.length - 1}`);
        for (let i = gameIndex + 2; i < t.games.length; i++) {
            t.games[i].gameNumber = i + 1;
            
            // Recalculate scores based on previous game
            const prevGame = t.games[i - 1];
            console.log(`[insertGameAfter] Recalculating game ${i} (number ${i + 1}) from previous game ${i - 1}`);
            const recalcScores = calculateScoresAfterGame(prevGame, t.metadata.matchLength);
            console.log(`[insertGameAfter] Game ${i}: old scores ${t.games[i].player1Score}-${t.games[i].player2Score}, new scores ${recalcScores.player1Score}-${recalcScores.player2Score}`);
            t.games[i].player1Score = recalcScores.player1Score;
            t.games[i].player2Score = recalcScores.player2Score;
        }
        
        // Return a deep copy to prevent reference sharing issues
        return JSON.parse(JSON.stringify(t));
    });
    
    console.log('[insertGameAfter] AFTER:');
    dumpTranscriptionState();
    
    // Invalidate positions cache from the insertion point
    invalidatePositionsCacheFrom(gameIndex + 1, 0);
}

/**
 * Delete a game at the specified game index
 * @param {number} gameIndex - Index of the game to delete
 * @returns {boolean} - Returns true if deletion was successful, false otherwise
 */
export function deleteGame(gameIndex) {
    console.log(`[deleteGame] gameIndex=${gameIndex}`);
    let deletionSuccessful = false;
    
    transcriptionStore.update(t => {
        if (!t.games || gameIndex < 0 || gameIndex >= t.games.length) return t;
        
        // Don't allow deleting the last game
        if (t.games.length === 1) {
            console.log('[deleteGame] Cannot delete the last game');
            return t;
        }
        
        // Remove the game at the specified index
        t.games.splice(gameIndex, 1);
        
        // Update game numbers for all subsequent games
        for (let i = gameIndex; i < t.games.length; i++) {
            t.games[i].gameNumber = i + 1;
        }
        
        deletionSuccessful = true;
        
        // Return a deep copy to prevent reference sharing issues
        return JSON.parse(JSON.stringify(t));
    });
    
    if (deletionSuccessful) {
        // Invalidate positions cache from the deletion point
        invalidatePositionsCacheFrom(gameIndex, 0);
    }
    
    return deletionSuccessful;
}
