// Exporter to convert LBG transcription back to match text format
// This must preserve exact format, case, and spacing of the original text files

export function exportToMatchText(transcription) {
    const lines = [];
    
    // Export metadata
    const meta = transcription.metadata;
    if (meta.site) lines.push(`; [Site "${meta.site}"]`);
    if (meta.matchId) lines.push(`; [Match ID "${meta.matchId}"]`);
    if (meta.event) lines.push(`; [Event "${meta.event}"]`);
    if (meta.round) lines.push(`; [Round "${meta.round}"]`);
    if (meta.player1) lines.push(`; [Player 1 "${meta.player1}"]`);
    if (meta.player2) lines.push(`; [Player 2 "${meta.player2}"]`);
    
    // Export Elo (optional, but include placeholder if present in metadata)
    if (meta.player1Elo !== undefined) {
        lines.push(`; [Player 1 Elo "${meta.player1Elo}"]`);
    }
    if (meta.player2Elo !== undefined) {
        lines.push(`; [Player 2 Elo "${meta.player2Elo}"]`);
    }
    
    // Convert date format from "2025-10-24" to "2025.10.24"
    if (meta.eventDate) {
        const formattedDate = meta.eventDate.replace(/-/g, '.');
        lines.push(`; [EventDate "${formattedDate}"]`);
    }
    if (meta.eventTime) lines.push(`; [EventTime "${meta.eventTime}"]`);
    if (meta.variation) lines.push(`; [Variation "${meta.variation}"]`);
    if (meta.unrated) lines.push(`; [Unrated "${meta.unrated}"]`);
    if (meta.crawford) lines.push(`; [Crawford "${meta.crawford}"]`);
    if (meta.cubeLimit) lines.push(`; [CubeLimit "${meta.cubeLimit}"]`);
    if (meta.transcriber) lines.push(`; [Transcriber "${meta.transcriber}"]`);
    
    // Add ClockType if present
    if (meta.clockType) {
        lines.push(`; [ClockType "${meta.clockType}"]`);
    }
    
    lines.push('');
    
    // Match length
    if (meta.matchLength) {
        lines.push(`${meta.matchLength} point match`);
    }
    
    // Export games
    for (const game of transcription.games) {
        lines.push('');
        lines.push('');
        lines.push(` Game ${game.gameNumber}`);
        
        // Score line - left column is player1, right column is player2
        const player1Name = meta.player1 || 'Player 1';
        const player2Name = meta.player2 || 'Player 2';
        
        // Calculate spacing: target is 39 characters from start of line to start of right column
        // Format: " Player1Name : score                       Player2Name : score"
        const leftPart = ` ${player1Name} : ${game.player1Score}`;
        const rightPart = `${player2Name} : ${game.player2Score}`;
        
        // Total target width is approximately 39 chars to right column start
        // Adjust spacing to match original format
        const spacingNeeded = Math.max(1, 39 - leftPart.length);
        const scoreLine = leftPart + ' '.repeat(spacingNeeded) + rightPart;
        lines.push(scoreLine);
        
        // Export moves
        for (const move of game.moves) {
            const moveNum = move.moveNumber.toString().padStart(3, ' ');
            let leftContent = '';
            let rightContent = '';
            
            // Build left column content (player1)
            if (move.player1Move) {
                leftContent = formatMoveData(move.player1Move);
            }
            
            // Build right column content (player2)
            if (move.player2Move) {
                rightContent = formatMoveData(move.player2Move);
            }
            
            // Handle cube actions - check both old format (move.cubeAction) and new format (in player moves)
            // Old format: move.cubeAction
            if (move.cubeAction) {
                const cubeText = formatCubeAction(move.cubeAction);
                if (move.cubeAction.player === 1) {
                    leftContent = cubeText;
                } else {
                    rightContent = cubeText;
                }
                
                // If there's a response in the same move
                if (move.cubeAction.response) {
                    const responsePlayer = move.cubeAction.player === 1 ? 2 : 1;
                    const responseText = formatCubeResponse(move.cubeAction.response);
                    if (responsePlayer === 1) {
                        leftContent = responseText;
                    } else {
                        rightContent = responseText;
                    }
                }
            }
            
            // Format the line with proper spacing
            // Left content starts at position 5 (after ") ")
            // Right content starts at position 39
            const leftSection = leftContent.padEnd(34, ' '); // 39 - 5 = 34
            const moveLine = `${moveNum}) ${leftSection}${rightContent}`;
            lines.push(moveLine);
        }
        
        // Add winner information if present
        if (game.winner) {
            // Always use "point" (singular) to match original format
            const winText = `Wins ${game.winner.points} point`;
            if (game.winner.player === 1) {
                // Winner is player 1 (left column)
                const leftSection = winText.padEnd(34, ' ');
                lines.push(`      ${leftSection}`);
            } else {
                // Winner is player 2 (right column)
                const leftSection = ''.padEnd(34, ' ');
                lines.push(`      ${leftSection}${winText}`);
            }
        }
        
        // Check if this is the match winner
        if (game.winner && meta.matchLength) {
            const totalScore1 = game.player1Score + (game.winner.player === 1 ? game.winner.points : 0);
            const totalScore2 = game.player2Score + (game.winner.player === 2 ? game.winner.points : 0);
            
            if (totalScore1 >= meta.matchLength || totalScore2 >= meta.matchLength) {
                // Add "and the match" to the last line
                if (lines[lines.length - 1].includes('Wins')) {
                    lines[lines.length - 1] = lines[lines.length - 1].replace(/point(s?)$/, 'point$1 and the match');
                }
            }
        }
    }
    
    lines.push('');
    lines.push('');
    
    return lines.join('\n');
}

function formatMoveData(moveData) {
    // Handle cube actions stored in player move (new migrated format)
    if (moveData.cubeAction) {
        // Format: cubeAction is a string like "doubles", "takes", "drops"
        // cubeValue contains the cube value
        if (moveData.cubeAction === 'doubles') {
            return ` Doubles => ${moveData.cubeValue || 2}`;
        } else if (moveData.cubeAction === 'takes') {
            return ' Takes';
        } else if (moveData.cubeAction === 'drops') {
            return ' Drops';
        }
    }
    
    // Handle resign actions - show as ???? (with dice if available)
    if (moveData.resignAction) {
        // Resign actions are indicated by ????
        // If there's dice info (shouldn't be after parsing, but check anyway)
        if (moveData.dice) {
            return `${moveData.dice}: ????`;
        }
        return '????';
    }
    
    // Handle regular moves
    if (moveData.isGala) {
        return `${moveData.dice}: Cannot Move`;
    }
    
    if (moveData.isIllegal || !moveData.move || moveData.move.trim() === '') {
        // Empty or illegal moves show as ????
        if (moveData.dice) {
            return `${moveData.dice}: ????`;
        }
        return '????';
    }
    
    return `${moveData.dice}: ${moveData.move}`;
}

function formatCubeAction(cubeAction) {
    if (cubeAction.action === 'doubles') {
        return ` Doubles => ${cubeAction.value}`;
    }
    if (cubeAction.action === 'takes') {
        return ' Takes';
    }
    if (cubeAction.action === 'drops') {
        return ' Drops';
    }
    return '';
}

function formatCubeResponse(response) {
    if (response === 'takes') {
        return ' Takes';
    }
    if (response === 'drops') {
        return ' Drops';
    }
    return '';
}
