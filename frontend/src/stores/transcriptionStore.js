import { writable, derived, get } from 'svelte/store';

// LazyBG file format version
export const LAZYBG_VERSION = '1.0.0';

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
    //     player1Move: { dice: '54', move: '24/20 13/8', isIllegal: false, isGala: false },
    //     player2Move: { dice: '31', move: '8/5* 6/5', isIllegal: false, isGala: false },
    //     cubeAction: null // { player: 1|2, action: 'doubles', value: 2, response: 'takes'|'drops' }
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
            moves: [],
            winner: null
        });
        return t;
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
        
        return t;
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
        
        return t;
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
}

export function insertMoveAfter(gameIndex, moveIndex) {
    insertMoveBefore(gameIndex, moveIndex + 1);
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
        
        return t;
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
}

export function updateMove(gameIndex, moveIndex, player, dice, move, isIllegal = false, isGala = false) {
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        const moveEntry = game.moves.find(m => m.moveNumber === moveIndex);
        if (!moveEntry) return t;
        
        // Check if this is a cube decision (d/t/p)
        const diceStr = dice.toLowerCase();
        if (diceStr === 'd' || diceStr === 't' || diceStr === 'p') {
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
        
        return t;
    });
    
    invalidatePositionsCacheFrom(gameIndex, moveIndex);
}

export function setGameWinner(gameIndex, player, points) {
    transcriptionStore.update(t => {
        const game = t.games[gameIndex];
        if (!game) return t;
        
        game.winner = { player, points };
        return t;
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
            
            return t;
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
        const inconsistencies = validateGamePositions(game, startMoveIndex);
        
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
        t.metadata = { ...t.metadata, ...metadata };
        return t;
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
        
        return t;
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
