// Parser pour les fichiers de match au format texte

export function parseMatchFile(content) {
    const lines = content.split('\n').map(line => line.trim());
    
    const transcription = {
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
    };
    
    // Parse metadata
    for (const line of lines) {
        if (line.startsWith('; [')) {
            const match = line.match(/; \[([^\]]+)\s+"([^"]+)"\]/);
            if (match) {
                const [, key, value] = match;
                switch (key) {
                    case 'Site': transcription.metadata.site = value; break;
                    case 'Match ID': transcription.metadata.matchId = value; break;
                    case 'Event': transcription.metadata.event = value; break;
                    case 'Round': transcription.metadata.round = value; break;
                    case 'Player 1': transcription.metadata.player1 = value; break;
                    case 'Player 2': transcription.metadata.player2 = value; break;
                    case 'EventDate': transcription.metadata.eventDate = value; break;
                    case 'EventTime': transcription.metadata.eventTime = value; break;
                    case 'Variation': transcription.metadata.variation = value; break;
                    case 'Unrated': transcription.metadata.unrated = value; break;
                    case 'Crawford': transcription.metadata.crawford = value; break;
                    case 'CubeLimit': transcription.metadata.cubeLimit = value; break;
                    case 'Transcriber': transcription.metadata.transcriber = value; break;
                }
            }
        }
    }
    
    // Find match length
    for (const line of lines) {
        const matchLengthMatch = line.match(/^(\d+)\s+point\s+match/i);
        if (matchLengthMatch) {
            transcription.metadata.matchLength = parseInt(matchLengthMatch[1]);
            break;
        }
    }
    
    // Parse games
    let currentGame = null;
    let currentGameIndex = -1;
    
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        
        // Detect new game
        const gameMatch = line.match(/^\s*Game\s+(\d+)/i);
        if (gameMatch) {
            const gameNumber = parseInt(gameMatch[1]);
            currentGameIndex++;
            
            // Find score line (next non-empty line)
            let scoreLine = '';
            for (let j = i + 1; j < lines.length; j++) {
                if (lines[j].trim()) {
                    scoreLine = lines[j];
                    break;
                }
            }
            
            // Parse scores from line like "marcow777 : 0     postmanpat : 1"
            const scoreMatch = scoreLine.match(/:\s*(\d+).*:\s*(\d+)/);
            const player1Score = scoreMatch ? parseInt(scoreMatch[1]) : 0;
            const player2Score = scoreMatch ? parseInt(scoreMatch[2]) : 0;
            
            currentGame = {
                gameNumber,
                player1Score,
                player2Score,
                moves: [],
                winner: null
            };
            transcription.games.push(currentGame);
            continue;
        }
        
        if (!currentGame) continue;
        
        // Parse moves - format: "  1) 54: 24/20 13/8                     31: 8/5* 6/5"
        const moveMatch = line.match(/^\s*(\d+)\)\s+(.*)/);
        if (moveMatch) {
            const moveNumber = parseInt(moveMatch[1]);
            const movesText = moveMatch[2];
            
            const moveEntry = {
                moveNumber,
                player1Move: null,
                player2Move: null,
                cubeAction: null
            };
            
            // Split by multiple spaces to separate player 1 and player 2 moves
            const parts = movesText.split(/\s{2,}/);
            
            for (const part of parts) {
                const trimmed = part.trim();
                if (!trimmed) continue;
                
                // Check for cube actions
                if (trimmed.match(/Doubles\s*=>\s*(\d+)/i)) {
                    const cubeMatch = trimmed.match(/Doubles\s*=>\s*(\d+)/i);
                    moveEntry.cubeAction = {
                        player: parts.indexOf(part) === 0 ? 1 : 2,
                        action: 'doubles',
                        value: parseInt(cubeMatch[1]),
                        response: null
                    };
                    continue;
                }
                
                if (trimmed.match(/Takes/i)) {
                    if (moveEntry.cubeAction) {
                        moveEntry.cubeAction.response = 'takes';
                    }
                    continue;
                }
                
                if (trimmed.match(/Drops/i)) {
                    if (moveEntry.cubeAction) {
                        moveEntry.cubeAction.response = 'drops';
                    }
                    continue;
                }
                
                if (trimmed.match(/Wins\s+(\d+)\s+point/i)) {
                    const winMatch = trimmed.match(/Wins\s+(\d+)\s+point/i);
                    const points = parseInt(winMatch[1]);
                    currentGame.winner = {
                        player: parts.indexOf(part) === 0 ? 1 : 2,
                        points
                    };
                    continue;
                }
                
                // Parse regular move: "54: 24/20 13/8" or "Cannot Move" or "????"
                const diceMatch = trimmed.match(/^(\d{2}):\s*(.+)/);
                if (diceMatch) {
                    const dice = diceMatch[1];
                    const move = diceMatch[2].trim();
                    const isGala = move.match(/Cannot Move/i) !== null;
                    const isIllegal = move.match(/\?{2,}/) !== null;
                    
                    const moveData = {
                        dice,
                        move: isGala ? 'Cannot Move' : (isIllegal ? '' : move),
                        isIllegal,
                        isGala
                    };
                    
                    if (parts.indexOf(part) === 0 || !moveEntry.player1Move) {
                        moveEntry.player1Move = moveData;
                    } else {
                        moveEntry.player2Move = moveData;
                    }
                }
            }
            
            if (moveEntry.player1Move || moveEntry.player2Move || moveEntry.cubeAction) {
                currentGame.moves.push(moveEntry);
            }
        }
    }
    
    return transcription;
}

export function serializeTranscription(transcription) {
    // Convert transcription to .lbg JSON format
    return JSON.stringify(transcription, null, 2);
}

export function deserializeTranscription(jsonString) {
    try {
        return JSON.parse(jsonString);
    } catch (e) {
        console.error('Error parsing transcription:', e);
        return null;
    }
}
