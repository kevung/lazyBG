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
    
    // Parse metadata - temporarily store as filePlayer1 and filePlayer2
    let filePlayer1 = '';
    let filePlayer2 = '';
    
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
                    case 'Player 1': filePlayer1 = value; break;
                    case 'Player 2': filePlayer2 = value; break;
                    case 'EventDate': 
                        // Convert from "2025.10.24" to "2025-10-24"
                        transcription.metadata.eventDate = value.replace(/\./g, '-');
                        break;
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
    
    // Find first score line to determine column layout
    // Format: "Left Player : 0                       Right Player : 0"
    let leftColumnPlayer = '';
    let rightColumnPlayer = '';
    
    for (const line of lines) {
        const scoreMatch = line.match(/^\s*([^:]+):\s*\d+\s+([^:]+):\s*\d+/);
        if (scoreMatch) {
            leftColumnPlayer = scoreMatch[1].trim();
            rightColumnPlayer = scoreMatch[2].trim();
            break;
        }
    }
    
    // Assign player1/player2 based on column position
    // lazyBG convention: player1 = left column (bottom), player2 = right column (top)
    if (leftColumnPlayer && rightColumnPlayer) {
        transcription.metadata.player1 = leftColumnPlayer;
        transcription.metadata.player2 = rightColumnPlayer;
    } else {
        // Fallback to file metadata order if no score line found
        transcription.metadata.player1 = filePlayer1;
        transcription.metadata.player2 = filePlayer2;
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
            
            // Parse scores from line like "Kévin Unger : 0                       Jacques Ravier : 0"
            // Simple mapping: left column → player1, right column → player2
            const scoreLineMatch = scoreLine.match(/[^:]+:\s*(\d+)\s+[^:]+:\s*(\d+)/);
            let player1Score = 0;
            let player2Score = 0;
            
            if (scoreLineMatch) {
                player1Score = parseInt(scoreLineMatch[1]); // Left column score
                player2Score = parseInt(scoreLineMatch[2]); // Right column score
            }
            
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
        // Use fixed column positions: right column starts at character 39 in original line
        const moveMatch = line.match(/^\s*(\d+)\)(.*)/);  // Capture everything after )
        if (moveMatch) {
            const moveNumber = parseInt(moveMatch[1]);
            const afterParen = moveMatch[2]; // Everything after ), preserving all spacing
            
            const moveEntry = {
                moveNumber,
                player1Move: null,
                player2Move: null,
                cubeAction: null
            };
            
            // The match gives us the string starting right after )
            // We need to know how many characters were before the ) in the original line
            // Move numbers are right-aligned, so ) is always at position 3 (0-indexed)
            // Format: "  1)" = positions 0,1,2,3 or " 10)" = positions 0,1,2,3
            // After the ), position 4 starts the content with a space
            // So content actually starts at position 5 in the original line
            const offsetInOriginalLine = 5; // Position where actual move content starts after ") "
            
            // Right column starts at position 39 in original line
            // In afterParen (which starts at offsetInOriginalLine), right column is at: 39 - offsetInOriginalLine
            const leftBoundary = Math.max(0, 39 - offsetInOriginalLine);
            
            const items = [];
            const leftContent = afterParen.substring(0, leftBoundary).trim();
            const rightContent = afterParen.substring(leftBoundary).trim();
            
            if (leftContent) {
                items.push({
                    text: leftContent,
                    isLeft: true
                });
            }
            if (rightContent) {
                items.push({
                    text: rightContent,
                    isLeft: false
                });
            }
            
            for (const item of items) {
                const trimmed = item.text;
                const isLeftColumn = item.isLeft;
                
                // Check for cube actions
                if (trimmed.match(/Doubles\s*=>\s*(\d+)/i)) {
                    const cubeMatch = trimmed.match(/Doubles\s*=>\s*(\d+)/i);
                    const cubeValue = parseInt(cubeMatch[1]);
                    
                    // Store cube action metadata only
                    const player = isLeftColumn ? 1 : 2;
                    moveEntry.cubeAction = {
                        player,
                        action: 'doubles',
                        value: cubeValue,
                        response: null
                    };
                    continue;
                }
                
                if (trimmed.match(/Takes/i)) {
                    // Takes is a cube response - store as a cube action on this move
                    if (currentGame.moves.length > 0) {
                        const prevMove = currentGame.moves[currentGame.moves.length - 1];
                        if (prevMove.cubeAction) {
                            // Create cube action showing the take response
                            const player = isLeftColumn ? 1 : 2;
                            moveEntry.cubeAction = {
                                player,
                                action: 'takes',
                                value: prevMove.cubeAction.value,
                                response: null
                            };
                        }
                    }
                    continue;
                }
                
                if (trimmed.match(/Drops/i)) {
                    // Drops is a cube response - store as a cube action on this move
                    if (currentGame.moves.length > 0) {
                        const prevMove = currentGame.moves[currentGame.moves.length - 1];
                        if (prevMove.cubeAction) {
                            // Create cube action showing the drop response
                            const player = isLeftColumn ? 1 : 2;
                            moveEntry.cubeAction = {
                                player,
                                action: 'drops',
                                value: prevMove.cubeAction.value,
                                response: null
                            };
                        }
                    }
                    continue;
                }
                
                if (trimmed.match(/Wins\s+(\d+)\s+point/i)) {
                    const winMatch = trimmed.match(/Wins\s+(\d+)\s+point/i);
                    const points = parseInt(winMatch[1]);
                    // Left column → player 1 (1st row), Right column → player 2 (2nd row)
                    const player = isLeftColumn ? 1 : 2;
                    currentGame.winner = {
                        player,
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
                    
                    // Assign based on column position in text file
                    // Left column in text file → 1st row in UI (player1Move)
                    // Right column in text file → 2nd row in UI (player2Move)
                    if (isLeftColumn) {
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
