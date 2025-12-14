// Parser pour les fichiers de match au format texte
import { LAZYBG_VERSION } from '../stores/transcriptionStore.js';

// Helper function to count checkers borne off from move notation
function countCheckersOff(moves) {
    let player1Off = 0;
    let player2Off = 0;
    
    for (const move of moves) {
        // Count checkers borne off by player 1
        if (move.player1Move && move.player1Move.move && !move.player1Move.isGala) {
            const moveText = move.player1Move.move;
            // Match patterns like "5/Off", "2/Off(2)", "6/1 6/Off", etc.
            const offMatches = moveText.match(/\/Off(\(\d+\))?/gi);
            if (offMatches) {
                for (const match of offMatches) {
                    // Check if there's a count like (2) or (3)
                    const countMatch = match.match(/\((\d+)\)/);
                    player1Off += countMatch ? parseInt(countMatch[1]) : 1;
                }
            }
        }
        
        // Count checkers borne off by player 2
        if (move.player2Move && move.player2Move.move && !move.player2Move.isGala) {
            const moveText = move.player2Move.move;
            const offMatches = moveText.match(/\/Off(\(\d+\))?/gi);
            if (offMatches) {
                for (const match of offMatches) {
                    const countMatch = match.match(/\((\d+)\)/);
                    player2Off += countMatch ? parseInt(countMatch[1]) : 1;
                }
            }
        }
    }
    
    return { player1: player1Off, player2: player2Off };
}

export function parseMatchFile(content) {

    const originalLines = content.split('\n');
    const lines = originalLines.map(line => line.trim());
    
    const transcription = {
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
        
        // Check for winner information on lines without move numbers (e.g., "      Wins 1 point")
        if (!line.match(/^\s*(\d+)\)/)) {
            const winMatch = line.match(/Wins\s+(\d+)\s+point/i);
            if (winMatch) {
                const points = parseInt(winMatch[1]);
                // Determine which player won based on context
                // When a player drops, the doubling player wins
                // Check the last move for a drops action
                if (currentGame.moves.length > 0) {
                    const lastMove = currentGame.moves[currentGame.moves.length - 1];
                    if (lastMove.cubeAction) {
                        if (lastMove.cubeAction.action === 'doubles') {
                            // The player who doubled wins
                            currentGame.winner = {
                                player: lastMove.cubeAction.player,
                                points
                            };
                        } else if (lastMove.cubeAction.action === 'drops') {
                            // The player who did NOT drop wins
                            // If player 2 dropped, player 1 wins; if player 1 dropped, player 2 wins
                            // We need to find who doubled (in the previous move or same move)
                            let doublingPlayer = null;
                            // Check if there's a doubles action in an earlier move
                            for (let j = currentGame.moves.length - 1; j >= 0; j--) {
                                const move = currentGame.moves[j];
                                if (move.cubeAction && move.cubeAction.action === 'doubles') {
                                    doublingPlayer = move.cubeAction.player;
                                    break;
                                }
                            }
                            if (doublingPlayer) {
                                currentGame.winner = {
                                    player: doublingPlayer,
                                    points
                                };
                            }
                        }
                    } else {
                        // No cube action - could be resign or natural bearoff completion
                        // Check if last move has an illegal/empty move (resign with ????)
                        if (lastMove.player2Move && (lastMove.player2Move.isIllegal || !lastMove.player2Move.move || lastMove.player2Move.move.trim() === '')) {
                            // Player 2 resigned, so player 1 wins
                            currentGame.winner = { player: 1, points };
                        } else if (lastMove.player1Move && (lastMove.player1Move.isIllegal || !lastMove.player1Move.move || lastMove.player1Move.move.trim() === '')) {
                            // Player 1 resigned, so player 2 wins
                            currentGame.winner = { player: 2, points };
                        } else if (!lastMove.player2Move && lastMove.player1Move) {
                            // Player 1 has a move but player 2 doesn't - player 1 wins by bearoff
                            currentGame.winner = { player: 1, points };
                        } else if (!lastMove.player1Move && lastMove.player2Move) {
                            // Player 2 has a move but player 1 doesn't - player 2 wins by bearoff
                            currentGame.winner = { player: 2, points };
                        } else {
                            // Fallback: determine winner from column position of "Wins" text
                            // "Wins" text position indicates which player won
                            // Left column (before position 39) = player 1, right column (after 39) = player 2
                            const originalLine = originalLines[i].replace(/\r/g, '');
                            const winTextPos = originalLine.indexOf('Wins');
                            if (winTextPos !== -1) {
                                // If "Wins" appears after position 39, it's in the right column (player 2)
                                // Otherwise it's in the left column (player 1)
                                const player = winTextPos >= 39 ? 2 : 1;
                                currentGame.winner = { player, points };
                                console.log(`[matchParser] Game ${currentGame.gameNumber}: Detected winner from column position - player ${player}, points ${points}, winTextPos ${winTextPos}`);
                            }
                        }
                    }
                }
            }
            continue;
        }
        
        // Parse moves - format: "  1) 54: 24/20 13/8                     31: 8/5* 6/5"
        // Use fixed column positions: right column starts at position 39 in original line
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
                
                // Check for cube actions first
                if (trimmed.match(/Doubles\s*=>\s*(\d+)/i)) {
                    const cubeMatch = trimmed.match(/Doubles\s*=>\s*(\d+)/i);
                    const cubeValue = parseInt(cubeMatch[1]);
                    
                    // Store cube action - only set if not already set
                    if (!moveEntry.cubeAction) {
                        const player = isLeftColumn ? 1 : 2;
                        moveEntry.cubeAction = {
                            player,
                            action: 'doubles',
                            value: cubeValue,
                            response: null
                        };
                    }
                } else if (trimmed.match(/Takes/i)) {
                    // Takes is a cube response
                    // Check if there's a doubles action in the current move entry first
                    if (moveEntry.cubeAction && moveEntry.cubeAction.action === 'doubles') {
                        // Same move - store as response
                        moveEntry.cubeAction.response = 'takes';
                    } else if (currentGame.moves.length > 0) {
                        // Previous move - create a new cube action entry
                        const prevMove = currentGame.moves[currentGame.moves.length - 1];
                        if (prevMove.cubeAction && !moveEntry.cubeAction) {
                            const player = isLeftColumn ? 1 : 2;
                            moveEntry.cubeAction = {
                                player,
                                action: 'takes',
                                value: prevMove.cubeAction.value,
                                response: null
                            };
                        }
                    }
                } else if (trimmed.match(/Drops/i)) {
                    // Drops is a cube response
                    // Check if there's a doubles action in the current move entry first
                    if (moveEntry.cubeAction && moveEntry.cubeAction.action === 'doubles') {
                        // Same move - store as response
                        moveEntry.cubeAction.response = 'drops';
                    } else if (currentGame.moves.length > 0) {
                        // Previous move - create a new cube action entry
                        const prevMove = currentGame.moves[currentGame.moves.length - 1];
                        if (prevMove.cubeAction && !moveEntry.cubeAction) {
                            const player = isLeftColumn ? 1 : 2;
                            moveEntry.cubeAction = {
                                player,
                                action: 'drops',
                                value: prevMove.cubeAction.value,
                                response: null
                            };
                        }
                    }
                } else if (trimmed.match(/Wins\s+(\d+)\s+point/i)) {
                    const winMatch = trimmed.match(/Wins\s+(\d+)\s+point/i);
                    const points = parseInt(winMatch[1]);
                    // Left column → player 1 (1st row), Right column → player 2 (2nd row)
                    const player = isLeftColumn ? 1 : 2;
                    currentGame.winner = {
                        player,
                        points
                    };
                } else {
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
            }
            
            if (moveEntry.player1Move || moveEntry.player2Move || moveEntry.cubeAction) {
                currentGame.moves.push(moveEntry);
                
                // After adding the move, check if the game ended naturally
                const checkersOff = countCheckersOff(currentGame.moves);
                
                if (checkersOff.player1 >= 15) {
                    // Player 1 bore off all 15 checkers - mark game as naturally completed
                    if (!currentGame.winner) {
                        currentGame.winner = { player: 1, points: 1 }; // Points will be updated by "Wins" line
                    }
                    currentGame.naturalBearoffWin = true;
                    currentGame.bearoffWinner = 1;
                } else if (checkersOff.player2 >= 15) {
                    // Player 2 bore off all 15 checkers - mark game as naturally completed
                    if (!currentGame.winner) {
                        currentGame.winner = { player: 2, points: 1 }; // Points will be updated by "Wins" line
                    }
                    currentGame.naturalBearoffWin = true;
                    currentGame.bearoffWinner = 2;
                }
            }
        }
    }
    

    
    // Post-process: detect resign decisions
    // If a game has a winner with points won, and the last decision has no checker movement,
    // add the appropriate resign action
    console.log(`[matchParser] Post-processing ${transcription.games.length} games for resign detection`);
    for (const game of transcription.games) {

        
        // Process games with moves, regardless of whether winner is set
        if (game.moves.length > 0) {
            const lastMove = game.moves[game.moves.length - 1];
            console.log(`[matchParser] Game ${game.gameNumber} last move:`, JSON.stringify({
                player1: lastMove.player1Move ? { dice: lastMove.player1Move.dice, move: lastMove.player1Move.move, isIllegal: lastMove.player1Move.isIllegal, cubeAction: lastMove.player1Move.cubeAction } : null,
                player2: lastMove.player2Move ? { dice: lastMove.player2Move.dice, move: lastMove.player2Move.move, isIllegal: lastMove.player2Move.isIllegal, cubeAction: lastMove.player2Move.cubeAction } : null
            }));
            
            // Find the last player decision (could be player 1 or player 2)
            let lastPlayerMove = null;
            let lastPlayer = null;
            
            // Check player 2 first (later in turn order)
            if (lastMove.player2Move && !lastMove.player2Move.cubeAction && !lastMove.player2Move.resignAction) {
                // Check if it's an empty move or just dice with no checker movement
                // isIllegal means ???? was in the text, which gets stored as empty move
                if (lastMove.player2Move.isIllegal || 
                    !lastMove.player2Move.move || 
                    lastMove.player2Move.move.trim() === '') {
                    lastPlayerMove = lastMove.player2Move;
                    lastPlayer = 2;
                }
            }
            
            // If player 2 didn't have a valid decision, check player 1
            if (!lastPlayerMove && lastMove.player1Move && !lastMove.player1Move.cubeAction && !lastMove.player1Move.resignAction) {
                // Check if it's an empty move or just dice with no checker movement
                // isIllegal means ???? was in the text, which gets stored as empty move
                if (lastMove.player1Move.isIllegal || 
                    !lastMove.player1Move.move || 
                    lastMove.player1Move.move.trim() === '') {
                    lastPlayerMove = lastMove.player1Move;
                    lastPlayer = 1;
                }
            }
            
            // If we found an empty last decision, convert it to a resign
            // But only if the game has a winner (completed game)
            if (lastPlayerMove && lastPlayer && game.winner) {
                // Track cube value through the game
                // Note: At this point cube actions are still in move.cubeAction format,
                // not yet migrated to player1Move.cubeAction/player2Move.cubeAction
                let cubeValue = 1;
                for (let j = 0; j < game.moves.length - 1; j++) {
                    const move = game.moves[j];
                    // Check move-level cubeAction (before migration)
                    if (move.cubeAction) {
                        if (move.cubeAction.action === 'doubles' || move.cubeAction.action === 'takes') {
                            cubeValue = move.cubeAction.value;
                        }
                    }
                }
                
                // Determine resign type based on points won
                // NOTE: We cannot accurately determine gammon/backgammon from text alone
                // without analyzing board state (whether checkers were borne off).
                // We'll make an educated guess based on the score multiplier.
                let resignType = 'normal';
                const pointsWon = game.winner.points;
                
                // Check if any bearing off moves are present to help determine resign type
                let hasBearoffMoves = false;
                for (let i = game.moves.length - 1; i >= Math.max(0, game.moves.length - 10); i--) {
                    const move = game.moves[i];
                    // Check if moves contain "off" or "/Off" indicating bearing off
                    if (move.player1Move?.move && /\/off/i.test(move.player1Move.move)) {
                        if (lastPlayer === 1) {
                            hasBearoffMoves = true;
                        }
                    }
                    if (move.player2Move?.move && /\/off/i.test(move.player2Move.move)) {
                        if (lastPlayer === 2) {
                            hasBearoffMoves = true;
                        }
                    }
                }
                
                // Determine resign type
                if (pointsWon === cubeValue * 3) {
                    resignType = 'backgammon';
                } else if (pointsWon === cubeValue * 2) {
                    // If losing player has borne off checkers, it's likely a normal resign with doubled cube
                    // Otherwise it's a gammon resign
                    resignType = hasBearoffMoves ? 'normal' : 'gammon';
                } else {
                    resignType = 'normal';
                }
                
                // Only set resign if the resigning player is the one who lost
                if (game.winner.player !== lastPlayer) {
                    lastPlayerMove.resignAction = resignType;
                    lastPlayerMove.dice = '';
                    lastPlayerMove.move = '';
                    lastPlayerMove.isIllegal = false;  // Clear illegal flag for valid resign
                } else {
                    console.error(`[matchParser] ✗ SKIPPED: Game ${game.gameNumber}, winner=${game.winner.player}, lastPlayer=${lastPlayer} (same player)`);
                }
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
        const transcription = JSON.parse(jsonString);
        
        // Add version field if missing (backward compatibility)
        if (!transcription.version) {
            console.warn('Loading file without version field. Assuming version 1.0.0');
            transcription.version = '1.0.0';
        }
        
        // Future version migration logic can go here
        // Example:
        // if (compareVersions(transcription.version, '2.0.0') < 0) {
        //     transcription = migrateToV2(transcription);
        // }
        
        return transcription;
    } catch (e) {
        console.error('Error parsing transcription:', e);
        return null;
    }
}
