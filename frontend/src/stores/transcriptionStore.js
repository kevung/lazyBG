import { writable, derived } from 'svelte/store';

// Structure d'une transcription complète
export const transcriptionStore = writable({
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
        
        const moveData = { dice, move, isIllegal, isGala };
        if (player === 1) {
            moveEntry.player1Move = moveData;
        } else {
            moveEntry.player2Move = moveData;
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

export function invalidatePositionsCacheFrom(gameIndex, moveIndex) {
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
                
                // Swap cube action player
                if (move.cubeAction) {
                    move.cubeAction.player = move.cubeAction.player === 1 ? 2 : 1;
                }
            }
        }
        
        return t;
    });
}

export function clearTranscription() {
    transcriptionStore.set({
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
}
