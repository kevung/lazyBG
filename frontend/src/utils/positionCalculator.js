/**
 * Position Calculator for Backgammon
 * Handles parsing move notation and calculating board positions
 */

/**
 * Creates an initial backgammon position
 * Point numbering: 1-24 from perspective of Player 1 (black checkers)
 * Player 1 moves from 24 to 1 (bearing off at home board 1-6)
 * Positive numbers = Player 1's checkers (black), Negative = Player 2's checkers (white)
 */
export function createInitialPosition() {
  return {
    points: [
      0,   // Point 0 (not used)
      -2,  // Point 1: 2 opponent (white) checkers
      0,   // Point 2
      0,   // Point 3
      0,   // Point 4
      0,   // Point 5
      5,   // Point 6: 5 Player 1 (black) checkers
      0,   // Point 7
      3,   // Point 8: 3 Player 1 (black) checkers
      0,   // Point 9
      0,   // Point 10
      0,   // Point 11
      -5,  // Point 12: 5 opponent (white) checkers
      5,   // Point 13: 5 Player 1 (black) checkers (midpoint)
      0,   // Point 14
      0,   // Point 15
      0,   // Point 16
      -3,  // Point 17: 3 opponent (white) checkers
      0,   // Point 18
      -5,  // Point 19: 5 opponent (white) checkers
      0,   // Point 20
      0,   // Point 21
      0,   // Point 22
      0,   // Point 23
      2,   // Point 24: 2 Player 1 (black) back checkers
    ],
    bar: 0,        // Checkers on bar (positive = player, negative = opponent)
    off: 0,        // Checkers borne off
    opponentBar: 0,
    opponentOff: 0,
  };
}

/**
 * Clone a position object
 */
export function clonePosition(position) {
  return {
    points: [...position.points],
    bar: position.bar,
    off: position.off,
    opponentBar: position.opponentBar,
    opponentOff: position.opponentOff,
  };
}

/**
 * Parse a single move segment (e.g., "24/20", "bar/21", "6/off")
 * Returns { from, to } where from/to can be numbers, 'bar', or 'off'
 */
function parseMoveSegment(segment) {
  const parts = segment.toLowerCase().split('/');
  if (parts.length !== 2) {
    return null;
  }

  let from = parts[0].trim();
  let to = parts[1].trim();

  // Convert "bar" to bar indicator
  if (from === 'bar') {
    from = 'bar';
  } else {
    from = parseInt(from, 10);
    if (isNaN(from)) return null;
  }

  // Convert "off" to off indicator
  if (to === 'off') {
    to = 'off';
  } else {
    to = parseInt(to, 10);
    if (isNaN(to)) return null;
  }

  return { from, to };
}

/**
 * Parse move notation into array of move segments
 * Examples:
 *   "24/20 13/8" -> [{ from: 24, to: 20 }, { from: 13, to: 8 }]
 *   "bar/21" -> [{ from: 'bar', to: 21 }]
 *   "6/off 4/off" -> [{ from: 6, to: 'off' }, { from: 4, to: 'off' }]
 *   "Cannot Move" -> []
 *   "????" -> []
 */
export function parseMoveNotation(moveText) {
  if (!moveText || moveText.trim() === '') {
    return [];
  }

  const normalized = moveText.trim();

  // Handle special cases
  if (normalized === 'Cannot Move' || normalized === '????') {
    return [];
  }

  // Handle moves with repetition notation: "8/5(2)" or "13/7(4)"
  // This means the same move repeated multiple times
  const segments = [];
  const parts = normalized.split(/\s+/);

  for (const part of parts) {
    // Check for repetition notation: "8/5(2)"
    const repeatMatch = part.match(/^(.+?)\((\d+)\)$/);
    if (repeatMatch) {
      const baseMove = repeatMatch[1];
      const count = parseInt(repeatMatch[2], 10);
      const segment = parseMoveSegment(baseMove);
      if (segment) {
        for (let i = 0; i < count; i++) {
          segments.push(segment);
        }
      }
    } else {
      const segment = parseMoveSegment(part);
      if (segment) {
        segments.push(segment);
      }
    }
  }

  return segments;
}

/**
 * Convert point number from Player 2's perspective to board perspective
 * Player 2's point 24 = Board point 1, Player 2's point 1 = Board point 24
 * @param {number|string} point - Point number from player's perspective, or 'bar'/'off'
 * @returns {number|string} - Point number in board perspective, or 'bar'/'off'
 */
function convertPlayer2Point(point) {
  if (point === 'bar' || point === 'off' || typeof point !== 'number') {
    return point;
  }
  return 25 - point;
}

/**
 * Apply a single move segment to a position
 * @param {Object} position - Current position
 * @param {Object} moveSegment - { from, to } where from/to are numbers, 'bar', or 'off'
 * @param {boolean} isPlayer - true for Player 1, false for Player 2
 * @returns {Object} - { position: new position, hit: true if opponent was hit }
 */
function applyMoveSegment(position, moveSegment, isPlayer = true) {
  const newPos = clonePosition(position);
  let { from, to } = moveSegment;
  let hit = false;
  
  // Convert Player 2's perspective to board perspective
  if (!isPlayer) {
    from = convertPlayer2Point(from);
    to = convertPlayer2Point(to);
  }
  
  const multiplier = isPlayer ? 1 : -1;

  // Handle moving from bar
  if (from === 'bar') {
    if (isPlayer) {
      if (newPos.bar <= 0) {
        console.warn('Cannot move from bar: no checkers on bar');
        return { position: newPos, hit };
      }
      newPos.bar--;  // Remove from player bar (decrease positive count)
    } else {
      if (newPos.opponentBar >= 0) {
        console.warn('Cannot move opponent from bar: no checkers on bar');
        return { position: newPos, hit };
      }
      newPos.opponentBar++;  // Remove from opponent bar (increase towards 0 from negative)
    }

    // Place checker on destination point
    if (typeof to === 'number' && to >= 1 && to <= 24) {
      // Check if we hit an opponent checker
      if (isPlayer && newPos.points[to] === -1) {
        newPos.points[to] = 1;
        newPos.opponentBar--;  // Opponent bar becomes more negative
        hit = true;
      } else if (!isPlayer && newPos.points[to] === 1) {
        newPos.points[to] = -1;
        newPos.bar++;  // Player bar increases
        hit = true;
      } else {
        newPos.points[to] += multiplier;
      }
    }
    return { position: newPos, hit };
  }

  // Handle bearing off
  if (to === 'off') {
    if (typeof from === 'number' && from >= 1 && from <= 24) {
      if (isPlayer) {
        if (newPos.points[from] > 0) {
          newPos.points[from]--;
          newPos.off++;
        }
      } else {
        if (newPos.points[from] < 0) {
          newPos.points[from]++;
          newPos.opponentOff++;
        }
      }
    }
    return { position: newPos, hit: false };
  }

  // Regular move from point to point
  if (typeof from === 'number' && typeof to === 'number') {
    if (from >= 1 && from <= 24 && to >= 1 && to <= 24) {
      // Remove checker from source point
      if (isPlayer) {
        if (newPos.points[from] > 0) {
          newPos.points[from]--;
        }
      } else {
        if (newPos.points[from] < 0) {
          newPos.points[from]++;
        }
      }

      // Check if we hit an opponent checker at destination
      if (isPlayer && newPos.points[to] === -1) {
        console.log(`Player 1 hits opponent at point ${to}, opponentBar: ${newPos.opponentBar} -> ${newPos.opponentBar - 1}`);
        newPos.points[to] = 1;
        newPos.opponentBar--;  // Opponent bar becomes more negative (0 -> -1)
        hit = true;
      } else if (!isPlayer && newPos.points[to] === 1) {
        console.log(`Player 2 hits opponent at point ${to}, bar: ${newPos.bar} -> ${newPos.bar + 1}`);
        newPos.points[to] = -1;
        newPos.bar++;  // Player bar increases (0 -> 1)
        hit = true;
      } else {
        newPos.points[to] += multiplier;
      }
    }
  }

  return { position: newPos, hit };
}

/**
 * Apply a complete move (potentially multiple segments) to a position
 * @param {Object} position - Starting position
 * @param {string} moveText - Move notation (e.g., "24/20 13/8")
 * @param {boolean} isPlayer - true for current player, false for opponent
 * @returns {Object} - { position: new position, hasHit: true if any hit occurred }
 */
export function applyMove(position, moveText, isPlayer = true) {
  const segments = parseMoveNotation(moveText);
  
  let currentPos = position;
  let hasHit = false;
  const hitSegments = []; // Track which segments caused hits
  
  for (let i = 0; i < segments.length; i++) {
    const result = applyMoveSegment(currentPos, segments[i], isPlayer);
    currentPos = result.position;
    if (result.hit) {
      hasHit = true;
      hitSegments.push(i);
    }
  }
  
  return { position: currentPos, hasHit, hitSegments };
}

/**
 * Calculate position after a sequence of moves from a transcription
 * @param {Object} transcription - Full transcription object
 * @param {number} targetGameNumber - Which game (1-based)
 * @param {number} targetMoveNumber - Which move number (1-based)
 * @param {string} targetPlayer - Which player's turn: 'player1' or 'player2'
 * @returns {Object} - Position at that point in the game
 */
export function calculatePositionAtMove(transcription, targetGameNumber, targetMoveNumber, targetPlayer = 'player1') {
  if (!transcription || !transcription.games) {
    return createInitialPosition();
  }

  const game = transcription.games.find(g => g.gameNumber === targetGameNumber);
  if (!game) {
    return createInitialPosition();
  }

  let position = createInitialPosition();

  // Apply all moves up to (but not including) the target move
  for (const move of game.moves) {
    if (move.moveNumber > targetMoveNumber) {
      break;
    }

    if (move.moveNumber === targetMoveNumber) {
      // Apply only the moves before the target player's turn
      if (targetPlayer === 'player1') {
        // Don't apply player1's move yet
        break;
      } else if (targetPlayer === 'player2' && move.player1Move) {
        // Apply player1's move, but not player2's
        const result = applyMove(position, move.player1Move.move, true);
        position = result.position;
        break;
      }
    }

    // Apply both players' moves for completed moves
    if (move.player1Move) {
      const result = applyMove(position, move.player1Move.move, true);
      position = result.position;
    }
    if (move.player2Move) {
      const result = applyMove(position, move.player2Move.move, false);
      position = result.position;
    }
  }

  return position;
}

/**
 * Convert position to GNUBG position ID format (for future compatibility)
 * This is a placeholder for now
 */
export function positionToId(position) {
  // TODO: Implement GNUBG position ID encoding
  return 'position-id-placeholder';
}

/**
 * Validate that a position is legal (15 checkers per side)
 */
export function validatePosition(position) {
  let playerCount = position.bar + position.off;
  let opponentCount = Math.abs(position.opponentBar) + position.opponentOff;

  for (let i = 1; i <= 24; i++) {
    if (position.points[i] > 0) {
      playerCount += position.points[i];
    } else if (position.points[i] < 0) {
      opponentCount += Math.abs(position.points[i]);
    }
  }

  return {
    valid: playerCount === 15 && opponentCount === 15,
    playerCount,
    opponentCount,
  };
}

/**
 * Check if a move violates the "bar-first" rule
 * If a player has checkers on the bar, they must enter before making any other moves
 * The rule is: you must enter ALL checkers from bar before moving other checkers
 * @param {Object} position - Position before the move
 * @param {string} moveText - Move notation
 * @param {boolean} isPlayer - true for Player 1, false for Player 2
 * @returns {boolean} - true if move is valid, false if it violates bar-first rule
 */
export function validateBarFirstRule(position, moveText, isPlayer = true) {
  const checkersOnBar = isPlayer ? position.bar : Math.abs(position.opponentBar);
  
  if (checkersOnBar === 0) {
    return true; // No checkers on bar, any move is fine
  }
  
  // Parse the move to check segments
  const segments = parseMoveNotation(moveText);
  
  if (segments.length === 0) {
    return true; // Cannot Move or ???? are acceptable
  }
  
  // Track how many checkers we've entered from bar
  let checkersEnteredFromBar = 0;
  
  // Check each segment in order
  for (let i = 0; i < segments.length; i++) {
    const segment = segments[i];
    
    if (segment.from === 'bar') {
      checkersEnteredFromBar++;
    } else {
      // Moving from a point - check if all bar checkers have been entered
      if (checkersEnteredFromBar < checkersOnBar) {
        return false; // Invalid: still have checkers on bar that weren't entered
      }
    }
  }
  
  return true;
}

/**
 * Validate bar entry with dice information
 * For doubles: if multiple checkers on bar and not all dice used, it's suspicious
 * For non-doubles: cannot validate reliably (player might not be able to play both dice)
 * @param {Object} positionBefore - Position before the move
 * @param {Object} positionAfter - Position after applying the move
 * @param {string} moveText - The move notation
 * @param {string} dice - The dice rolled (e.g., "22", "54", "66")
 * @param {boolean} isPlayer - true for Player 1, false for Player 2
 * @returns {Object} - { valid: boolean, reason: string }
 */
export function validateBarWithDice(positionBefore, positionAfter, moveText, dice, isPlayer = true) {
  if (!dice || moveText === 'Cannot Move' || moveText === '????') {
    return { valid: true, reason: '' };
  }
  
  const checkersOnBarBefore = isPlayer ? positionBefore.bar : Math.abs(positionBefore.opponentBar);
  const checkersOnBarAfter = isPlayer ? positionAfter.bar : Math.abs(positionAfter.opponentBar);
  
  if (checkersOnBarBefore === 0) {
    return { valid: true, reason: '' };
  }
  
  // Parse dice to get number of moves available
  const die1 = parseInt(dice[0], 10);
  const die2 = parseInt(dice[1], 10);
  const isDoubles = die1 === die2;
  const numMoves = isDoubles ? 4 : 2;
  
  // Count segments in the move
  const segments = parseMoveNotation(moveText);
  
  // Only validate for DOUBLES where all dice should enter the same point
  // For non-doubles, we can't know if both dice can enter (different entry points)
  if (isDoubles && checkersOnBarBefore >= 3 && checkersOnBarAfter > 0 && segments.length < numMoves) {
    // With doubles and 3+ checkers on bar, if not all dice used and checkers remain, likely invalid
    return {
      valid: false,
      reason: `${checkersOnBarAfter} checker(s) remain on bar, only ${segments.length} of ${numMoves} dice used (doubles)`
    };
  }
  
  return { valid: true, reason: '' };
}

/**
 * Add hit marker (*) to move notation if a hit occurred
 * @param {string} moveText - Original move notation
 * @returns {string} - Move notation with * added to the last segment if hit occurred
 */
export function addHitMarker(moveText, hitSegments = null) {
  if (!moveText || moveText === 'Cannot Move' || moveText === '????') {
    return moveText;
  }
  
  // If no specific segments provided, add * at the end (legacy behavior)
  if (!hitSegments || hitSegments.length === 0) {
    if (moveText.includes('*')) {
      return moveText;
    }
    return moveText + '*';
  }
  
  // Split move into segments (e.g., "Bar/24 21/16" -> ["Bar/24", "21/16"])
  const segments = moveText.split(' ').filter(s => s.length > 0);
  
  // Add * to specific segments that caused hits
  const markedSegments = segments.map((seg, idx) => {
    // Remove existing * if present
    const cleanSeg = seg.replace(/\*/g, '');
    // Add * if this segment caused a hit
    return hitSegments.includes(idx) ? cleanSeg + '*' : cleanSeg;
  });
  
  return markedSegments.join(' ');
}

/**
 * Remove hit marker (*) from move notation
 * @param {string} moveText - Move notation possibly with *
 * @returns {string} - Move notation without *
 */
export function removeHitMarker(moveText) {
  if (!moveText) {
    return moveText;
  }
  return moveText.replace(/\*/g, '').trim();
}

/**
 * Check if a move notation matches what actually happens in the position
 * For example, if move notation has *, verify a hit actually occurred
 * @param {Object} position - Position before the move
 * @param {string} moveText - Move notation (possibly with *)
 * @param {boolean} isPlayer - true for Player 1, false for Player 2
 * @returns {Object} - { valid: boolean, reason: string }
 */
export function validateMoveNotation(position, moveText, isPlayer = true) {
  if (!moveText || moveText === 'Cannot Move' || moveText === '????') {
    return { valid: true };
  }
  
  const hasHitMarker = moveText.includes('*');
  const cleanMove = removeHitMarker(moveText);
  
  try {
    const result = applyMove(position, cleanMove, isPlayer);
    const actualHit = result.hasHit;
    
    // Check if notation matches reality
    if (hasHitMarker && !actualHit) {
      return { valid: false, reason: 'Move has * but no hit occurred' };
    }
    
    // Note: We don't require * to be present if hit occurred, as it might be missing
    // The EditMovePanel will add it, but existing moves might not have it
    
    return { valid: true };
  } catch (error) {
    return { valid: false, reason: `Cannot apply move: ${error.message}` };
  }
}

/**
 * Validate all positions in a game and detect inconsistencies
 * Returns an array of move indices that have position errors
 * @param {Object} game - Game object with moves
 * @param {number} startMoveIndex - Index to start validation from (default 0)
 * @param {Object} transcription - Full transcription object (optional, for Crawford detection)
 * @param {number} gameIndex - Index of this game in transcription (optional, for Crawford detection)
 */
export function validateGamePositions(game, startMoveIndex = 0, transcription = null, gameIndex = 0) {
  const inconsistentMoves = [];
  
  if (!game || !game.moves || game.moves.length === 0) {
    return inconsistentMoves;
  }

  // Determine if this is a Crawford game
  let isCrawfordGame = false;
  if (transcription && transcription.metadata) {
    const matchLength = transcription.metadata.matchLength || 0;
    const crawfordOn = transcription.metadata.crawford === 'On';
    
    if (matchLength > 0 && crawfordOn) {
      const player1Score = game.player1Score || 0;
      const player2Score = game.player2Score || 0;
      const awayScore1 = matchLength - player1Score;
      const awayScore2 = matchLength - player2Score;
      
      // Check if one player is 1-away AND this is the first such game
      if (awayScore1 === 1 || awayScore2 === 1) {
        const firstCrawfordIndex = transcription.games.findIndex(g => 
          (matchLength - (g.player1Score || 0) === 1 || matchLength - (g.player2Score || 0) === 1)
        );
        isCrawfordGame = (gameIndex === firstCrawfordIndex);
      }
    }
  }

  let position = createInitialPosition();
  let firstError = null;
  
  // Build position up to startMoveIndex without validation
  // Also check for drops and resignations in earlier moves
  let gameEndedAtMove = null;
  for (let i = 0; i < startMoveIndex && i < game.moves.length; i++) {
    const move = game.moves[i];
    
    // Check for drops (game ends)
    if (move.player1Move?.cubeAction === 'drops' || move.player2Move?.cubeAction === 'drops') {
      gameEndedAtMove = i;
      break;
    }
    
    // Check for resignations (move ends with '-')
    // Only treat as resignation if move is not empty and ends with '-'
    if ((move.player1Move?.move && move.player1Move.move.trim() && move.player1Move.move.trim().endsWith('-')) ||
        (move.player2Move?.move && move.player2Move.move.trim() && move.player2Move.move.trim().endsWith('-'))) {
      gameEndedAtMove = i;
      break;
    }
    
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
  
  // Track cube ownership: -1 = center, 0 = player 1, 1 = player 2
  let cubeOwner = -1; // Start in center
  let cubeValue = 1;
  
  // Track if there's an unanswered double (blocks all future cube actions)
  let unansweredDoubleAt = null; // move index where unanswered double occurred
  
  // Build cube state up to startMoveIndex
  for (let i = 0; i < startMoveIndex && i < game.moves.length; i++) {
    const move = game.moves[i];
    
    // Process player 1's cube actions
    if (move.player1Move?.cubeAction) {
      if (move.player1Move.cubeAction === 'doubles') {
        cubeValue = move.player1Move.cubeValue || (cubeValue * 2);
        cubeOwner = -1; // Center when doubled
      } else if (move.player1Move.cubeAction === 'takes') {
        cubeOwner = 0; // Player 1 owns after taking
      }
    }
    
    // Process player 2's cube actions
    if (move.player2Move?.cubeAction) {
      if (move.player2Move.cubeAction === 'doubles') {
        cubeValue = move.player2Move.cubeValue || (cubeValue * 2);
        cubeOwner = -1; // Center when doubled
      } else if (move.player2Move.cubeAction === 'takes') {
        cubeOwner = 1; // Player 2 owns after taking
      }
    }
  }
  
  // Helper function to find next cube action after a given player's cube action
  // skipPlayer1AtIndex: if true, skip player1's action at fromIndex (because we already saw it)
  // skipPlayer2AtIndex: if true, skip player2's action at fromIndex (because we already saw it)
  function findNextCubeAction(fromIndex, skipPlayer1AtIndex = false, skipPlayer2AtIndex = false) {
    for (let j = fromIndex; j < game.moves.length; j++) {
      const m = game.moves[j];
      
      // Check player 1's action (skip if we're told to)
      if (!(j === fromIndex && skipPlayer1AtIndex) && m.player1Move?.cubeAction) {
        return { moveIndex: j, player: 1, action: m.player1Move.cubeAction };
      }
      
      // Check player 2's action (skip if we're told to)
      if (!(j === fromIndex && skipPlayer2AtIndex) && m.player2Move?.cubeAction) {
        return { moveIndex: j, player: 2, action: m.player2Move.cubeAction };
      }
    }
    return null;
  }
  
  // Validate from startMoveIndex onwards
  for (let i = startMoveIndex; i < game.moves.length; i++) {
    const move = game.moves[i];
    
    // NEW: Check if cube decision is the first decision of the game
    // The first decision of a game cannot be a cube decision (d/t/p)
    if (i === 0) {
      // Check if player 1 has a cube action as first decision
      if (move.player1Move?.cubeAction) {
        const action = move.player1Move.cubeAction === 'doubles' ? 'double' 
                     : move.player1Move.cubeAction === 'takes' ? 'take' 
                     : 'pass';
        console.log(`[Move 1] Player 1 cannot ${action} - first decision of game must be a checker move`);
        inconsistentMoves.push({ 
          moveIndex: 0, 
          player: 1, 
          reason: `Cannot ${action}: first decision of game must be a checker move` 
        });
        if (firstError === null) firstError = 0;
      }
      
      // Check if player 2 has a cube action as first decision
      if (move.player2Move?.cubeAction) {
        const action = move.player2Move.cubeAction === 'doubles' ? 'double' 
                     : move.player2Move.cubeAction === 'takes' ? 'take' 
                     : 'pass';
        console.log(`[Move 1] Player 2 cannot ${action} - first decision of game must be a checker move`);
        inconsistentMoves.push({ 
          moveIndex: 0, 
          player: 2, 
          reason: `Cannot ${action}: first decision of game must be a checker move` 
        });
        if (firstError === null) firstError = 0;
      }
      
      // NEW: Check if the first decision is a dice double (forbidden in opening roll)
      // The opening roll cannot be a double - if both players roll the same, they re-roll
      // However, this only applies to the ACTUAL opening roll (first player to move)
      // If player1 goes first, player2's response can be doubles
      // If player2 goes first (player1 has no move), player2's opening roll cannot be doubles
      if (move.player1Move?.dice) {
        const dice = move.player1Move.dice;
        if (dice.length === 2 && dice[0] === dice[1]) {
          console.log(`[Move 1] Player 1 cannot have dice double ${dice} as opening roll`);
          inconsistentMoves.push({ 
            moveIndex: 0, 
            player: 1, 
            reason: `Opening roll cannot be a double (${dice})` 
          });
          if (firstError === null) firstError = 0;
        }
      } else if (move.player2Move?.dice) {
        // Only check player2's doubles if player1 did NOT go first (player2 is opening)
        // If player1 went first (has a move), player2's response can be doubles
        const dice = move.player2Move.dice;
        if (dice.length === 2 && dice[0] === dice[1]) {
          console.log(`[Move 1] Player 2 cannot have dice double ${dice} as opening roll (player2 starts)`);
          inconsistentMoves.push({ 
            moveIndex: 0, 
            player: 2, 
            reason: `Opening roll cannot be a double (${dice})` 
          });
          if (firstError === null) firstError = 0;
        }
      }
    }
    
    // NEW: Check if player 1 takes/drops without a preceding double
    // Player 1 takes/passes should only occur after player 2 doubles in the previous move
    if (move.player1Move?.cubeAction === 'takes' || move.player1Move?.cubeAction === 'drops') {
      // First check Crawford game
      if (isCrawfordGame) {
        const action = move.player1Move.cubeAction === 'takes' ? 'take' : 'pass';
        console.log(`[Move ${i+1}] Player 1 cannot ${action} - Crawford game`);
        inconsistentMoves.push({ 
          moveIndex: i, 
          player: 1, 
          reason: `Cannot ${action}: cube is dead in Crawford game` 
        });
        if (firstError === null) firstError = i;
      } else {
        let hasValidPrecedingDouble = false;
        
        // Check if previous move (i-1) has player 2 doubling
        if (i > 0) {
          const prevMove = game.moves[i - 1];
          if (prevMove.player2Move?.cubeAction === 'doubles') {
            hasValidPrecedingDouble = true;
          }
        }
        
        if (!hasValidPrecedingDouble) {
          const action = move.player1Move.cubeAction === 'takes' ? 'take' : 'pass';
          console.log(`[Move ${i+1}] Player 1 cannot ${action} - no preceding double from player 2`);
          inconsistentMoves.push({ 
            moveIndex: i, 
            player: 1, 
            reason: `Cannot ${action}: no preceding double from opponent` 
          });
          if (firstError === null) firstError = i;
        }
      }
    }
    
    // NEW: Check if player 2 takes/drops without a preceding double
    // Player 2 takes/passes should only occur after player 1 doubles in the same move
    if (move.player2Move?.cubeAction === 'takes' || move.player2Move?.cubeAction === 'drops') {
      // First check Crawford game
      if (isCrawfordGame) {
        const action = move.player2Move.cubeAction === 'takes' ? 'take' : 'pass';
        console.log(`[Move ${i+1}] Player 2 cannot ${action} - Crawford game`);
        inconsistentMoves.push({ 
          moveIndex: i, 
          player: 2, 
          reason: `Cannot ${action}: cube is dead in Crawford game` 
        });
        if (firstError === null) firstError = i;
      } else {
        let hasValidPrecedingDouble = false;
        
        // Check if player 1 doubled in the same move entry
        if (move.player1Move?.cubeAction === 'doubles') {
          hasValidPrecedingDouble = true;
        }
        
        if (!hasValidPrecedingDouble) {
          const action = move.player2Move.cubeAction === 'takes' ? 'take' : 'pass';
          console.log(`[Move ${i+1}] Player 2 cannot ${action} - no preceding double from player 1`);
          inconsistentMoves.push({ 
            moveIndex: i, 
            player: 2, 
            reason: `Cannot ${action}: no preceding double from opponent` 
          });
          if (firstError === null) firstError = i;
        }
      }
    }
    
    // If there's an unanswered double, mark ALL actions (cube and regular moves) as invalid
    // and skip further processing of this move
    if (unansweredDoubleAt !== null && unansweredDoubleAt < i) {
      console.log(`[Move ${i+1}] Unanswered double at move ${unansweredDoubleAt + 1} blocks this move`);
      if (move.player1Move && (move.player1Move.cubeAction || move.player1Move.move)) {
        const reason = move.player1Move.cubeAction === 'doubles' 
          ? `Cannot double: must first respond to opponent's unanswered double at move ${unansweredDoubleAt + 1}`
          : `Must respond to opponent's double at move ${unansweredDoubleAt + 1}`;
        console.log(`[Move ${i+1}] Marking player 1 invalid: ${reason}`);
        inconsistentMoves.push({ moveIndex: i, player: 1, reason });
        if (firstError === null) firstError = i;
      }
      if (move.player2Move && (move.player2Move.cubeAction || move.player2Move.move)) {
        const reason = move.player2Move.cubeAction === 'doubles' 
          ? `Cannot double: must first respond to opponent's unanswered double at move ${unansweredDoubleAt + 1}`
          : `Must respond to opponent's double at move ${unansweredDoubleAt + 1}`;
        console.log(`[Move ${i+1}] Marking player 2 invalid: ${reason}`);
        inconsistentMoves.push({ moveIndex: i, player: 2, reason });
        if (firstError === null) firstError = i;
      }
      // Skip further processing since everything is invalid
      continue;
    }
    
    // Player 1 doubles - check if player 2 responds properly
    if (move.player1Move?.cubeAction === 'doubles') {
      
      console.log(`[Move ${i+1}] Player 1 doubles, cubeOwner=${cubeOwner}`);
      
      // Validate: cannot double in Crawford game
      if (isCrawfordGame) {
        console.log(`[Move ${i+1}] Player 1 cannot double - Crawford game`);
        inconsistentMoves.push({ moveIndex: i, player: 1, reason: `Cannot double: cube is dead in Crawford game` });
        if (firstError === null) firstError = i;
      }
      // Validate: can only double if cube is in center or on their side
      else if (cubeOwner !== -1 && cubeOwner !== 0) {
        console.log(`[Move ${i+1}] Player 1 cannot double - cube owned by opponent`);
        inconsistentMoves.push({ moveIndex: i, player: 1, reason: `Cannot double: cube is owned by opponent` });
        if (firstError === null) firstError = i;
      } else {
        // Update cube state
        cubeValue = move.player1Move?.cubeValue || (cubeValue * 2);
        cubeOwner = -1; // Center when doubled
        
        // Check if player 2 responds with take/pass at the right position
        const nextCube = findNextCubeAction(i, true, false);
        console.log(`[Move ${i+1}] Looking for player 2 response, found:`, nextCube);
        
        // Check if player 2 responds with another double (invalid)
        if (nextCube && 
            nextCube.player === 2 && 
            nextCube.action === 'doubles' &&
            nextCube.moveIndex <= i + 1) {
          // Invalid: cannot double in response to a double
          console.log(`[Move ${i+1}] Player 2 cannot double in response to player 1's double`);
          inconsistentMoves.push({ 
            moveIndex: nextCube.moveIndex, 
            player: 2, 
            reason: `Cannot double: must respond to player 1's double with take or pass`
          });
          if (firstError === null) firstError = nextCube.moveIndex;
          unansweredDoubleAt = i;
        } else if (nextCube && 
            nextCube.player === 2 && 
            (nextCube.action === 'takes' || nextCube.action === 'drops') &&
            nextCube.moveIndex <= i + 1) {
          console.log(`[Move ${i+1}] Valid response found at move ${nextCube.moveIndex + 1}`);
          // Valid response found
          if (nextCube.action === 'drops') {
            gameEndedAtMove = nextCube.moveIndex;
          } else if (nextCube.action === 'takes') {
            cubeOwner = 1; // Player 2 owns after taking
          }
        } else if (nextCube && nextCube.player === 2 && nextCube.moveIndex > i + 1) {
          // Player 2 has a cube action but it's too far away (more than next move)
          // This means they didn't respond immediately - mark as unanswered
          console.log(`[Move ${i+1}] Player 2's cube action at move ${nextCube.moveIndex + 1} is too late`);
          unansweredDoubleAt = i;
          // Don't mark the double as invalid - just note it's unanswered
        } else {
          // No immediate cube response found
          // Check if player 2 has a regular move at the same move entry
          if (move.player2Move && move.player2Move.move && 
              move.player2Move.move !== 'Cannot Move' && 
              move.player2Move.move !== '????' &&
              !move.player2Move.cubeAction) {
            // Player 2 has a regular move but should respond to the double
            console.log(`[Move ${i+1}] Player 2 has a regular move but should respond to player 1's double`);
            inconsistentMoves.push({ 
              moveIndex: i, 
              player: 2, 
              reason: `Must respond to player 1's double with take or pass`
            });
            if (firstError === null) firstError = i;
            unansweredDoubleAt = i;
          } else {
            // No response found yet - this is OK, just note it as unanswered
            // Don't mark the double as invalid - it's just waiting for a response
            console.log(`[Move ${i+1}] No response yet - setting unansweredDoubleAt = ${i}`);
            unansweredDoubleAt = i;
            // Don't add to inconsistentMoves - the double itself is not invalid
          }
        }
      }
    }
    
    // Player 2 doubles - check if player 1 responds properly
    if (move.player2Move?.cubeAction === 'doubles') {
      
      console.log(`[Move ${i+1}] Player 2 doubles, cubeOwner=${cubeOwner}`);
      
      // Check if player 1 also doubled at the same move (invalid - both can't double)
      if (move.player1Move?.cubeAction === 'doubles') {
        console.log(`[Move ${i+1}] Both players doubled at same move - marking player 2 invalid`);
        inconsistentMoves.push({ 
          moveIndex: i, 
          player: 2, 
          reason: `Cannot double: player 1 already doubled at this move`
        });
        if (firstError === null) firstError = i;
        continue; // Skip further processing
      }
      
      // Validate: cannot double in Crawford game
      if (isCrawfordGame) {
        console.log(`[Move ${i+1}] Player 2 cannot double - Crawford game`);
        inconsistentMoves.push({ moveIndex: i, player: 2, reason: `Cannot double: cube is dead in Crawford game` });
        if (firstError === null) firstError = i;
      }
      // Validate: can only double if cube is in center or on their side
      else if (cubeOwner !== -1 && cubeOwner !== 1) {
        console.log(`[Move ${i+1}] Player 2 cannot double - cube owned by opponent`);
        inconsistentMoves.push({ moveIndex: i, player: 2, reason: `Cannot double: cube is owned by opponent` });
        if (firstError === null) firstError = i;
      } else {
        // Update cube state
        cubeValue = move.player2Move?.cubeValue || (cubeValue * 2);
        cubeOwner = -1; // Center when doubled
        
        // Check if player 1 responds with take/pass at the right position
        // Response can be at next move (i+1) where player 1 acts
        const nextCube = findNextCubeAction(i, false, true);
        console.log(`[Move ${i+1}] Looking for player 1 response to player 2's double, found:`, nextCube);
        
        // Check if player 1 responds with another double (invalid)
        // Allow response at i+1 only (player 1 acts at next move after player 2's double)
        if (nextCube && 
            nextCube.player === 1 && 
            nextCube.action === 'doubles' &&
            (nextCube.moveIndex === i + 1 || nextCube.moveIndex === i)) {
          // Invalid: cannot double in response to a double
          console.log(`[Move ${i+1}] Player 1 cannot double in response to player 2's double`);
          inconsistentMoves.push({ 
            moveIndex: nextCube.moveIndex, 
            player: 1, 
            reason: `Cannot double: must respond to player 2's double with take or pass`
          });
          if (firstError === null) firstError = nextCube.moveIndex;
          unansweredDoubleAt = i;
        } else if (nextCube && 
            nextCube.player === 1 && 
            (nextCube.action === 'takes' || nextCube.action === 'drops') &&
            (nextCube.moveIndex === i + 1 || nextCube.moveIndex === i)) {
          // Valid response found
          console.log(`[Move ${i+1}] Valid take/pass response found at move ${nextCube.moveIndex + 1}`);
          if (nextCube.action === 'drops') {
            gameEndedAtMove = nextCube.moveIndex;
          } else if (nextCube.action === 'takes') {
            cubeOwner = 0; // Player 1 owns after taking
          }
        } else if (nextCube && nextCube.player === 1 && nextCube.moveIndex > i + 1) {
          // Player 1 has a cube action but it's too far away (more than next move)
          // This means they didn't respond immediately - mark as unanswered
          console.log(`[Move ${i+1}] Player 1's cube action at move ${nextCube.moveIndex + 1} is too late`);
          unansweredDoubleAt = i;
          // Don't mark the double as invalid - just note it's unanswered
        } else {
          // No immediate cube response found
          // Player 1's regular move at move i is OK (it happened before player 2's double)
          // Check if there's a regular move at i+1 (which would be invalid - should respond to double)
          const nextMoveEntry = i + 1 < game.moves.length ? game.moves[i + 1] : null;
          if (nextMoveEntry && nextMoveEntry.player1Move && 
              nextMoveEntry.player1Move.move && 
              nextMoveEntry.player1Move.move !== 'Cannot Move' && 
              nextMoveEntry.player1Move.move !== '????' &&
              !nextMoveEntry.player1Move.cubeAction) {
            // Player 1 made a regular move at i+1 without responding to the double
            console.log(`[Move ${i+2}] Player 1 has a regular move but should respond to player 2's double`);
            inconsistentMoves.push({ 
              moveIndex: i + 1, 
              player: 1, 
              reason: `Must respond to player 2's double with take or pass`
            });
            if (firstError === null) firstError = i + 1;
            unansweredDoubleAt = i;
          } else {
            // No response found yet - this is OK, just note it as unanswered
            // Don't mark the double as invalid - it's just waiting for a response
            console.log(`[Move ${i+1}] No response yet - setting unansweredDoubleAt = ${i}`);
            unansweredDoubleAt = i;
            // Don't add to inconsistentMoves - the double itself is not invalid
          }
        }
      }
    }
    
    // If we've already encountered an error or drop from previous move, mark all moves as inconsistent
    if (firstError !== null && i > firstError) {
      if (move.player1Move && (move.player1Move.move || move.player1Move.cubeAction)) {
        inconsistentMoves.push({ moveIndex: i, player: 1, reason: gameEndedAtMove !== null ? 'Game already ended' : 'Previous move caused inconsistency' });
      }
      if (move.player2Move && (move.player2Move.move || move.player2Move.cubeAction)) {
        inconsistentMoves.push({ moveIndex: i, player: 2, reason: gameEndedAtMove !== null ? 'Game already ended' : 'Previous move caused inconsistency' });
      }
      continue;
    }
    
    // Check for drops (game ends after a drop)
    if (move.player1Move?.cubeAction === 'drops' || move.player2Move?.cubeAction === 'drops') {
      
      // Only set gameEndedAtMove if not already set (from double handling above)
      if (gameEndedAtMove === null) {
        gameEndedAtMove = i;
      }
      
      // In backgammon, player 1 acts first in each move entry
      // If player 1 drops, player 2 should NOT have a move in the same entry
      // If player 2 drops, it's valid for player 1 to have moved first (they act chronologically first)
      
      if (move.player1Move?.cubeAction === 'drops') {
        // Player 1 dropped - mark player 2's move in same entry as inconsistent
        if (move.player2Move && (move.player2Move.move || move.player2Move.cubeAction)) {
          inconsistentMoves.push({ 
            moveIndex: i, 
            player: 2, 
            reason: 'Game ended after player 1 dropped' 
          });
        }
      }
      // If player 2 drops, player 1's move in same entry is valid (it happened first chronologically)
      
      // Set firstError to mark all subsequent moves in next iterations
      firstError = i;
      continue; // Skip further processing of this move
    }
    
    try {
      // Try to apply player1's move (skip cube decisions)
      if (move.player1Move && !move.player1Move.cubeAction && move.player1Move.move && 
          move.player1Move.move !== 'Cannot Move' && 
          move.player1Move.move !== '????') {
        
        // Validate move notation (e.g., check if * is correct)
        const notationCheck = validateMoveNotation(position, move.player1Move.move, true);
        if (!notationCheck.valid) {
          inconsistentMoves.push({ moveIndex: i, player: 1, reason: notationCheck.reason });
          firstError = i;
        } else {
          // Remove hit marker for validation
          const cleanMove = removeHitMarker(move.player1Move.move);
          
          // Check bar-first rule
          if (!validateBarFirstRule(position, cleanMove, true)) {
            const reason = 'Must enter from bar before moving other checkers';
            inconsistentMoves.push({ moveIndex: i, player: 1, reason });
            firstError = i;
          } else {
            const result = applyMove(position, cleanMove, true);
            const newPosition = result.position;
            
            // Check bar entry with dice if available
            const dice = move.player1Move.dice;
            if (dice) {
              const barDiceValidation = validateBarWithDice(position, newPosition, cleanMove, dice, true);
              if (!barDiceValidation.valid) {
                inconsistentMoves.push({ moveIndex: i, player: 1, reason: barDiceValidation.reason });
                firstError = i;
                // Don't update position if validation failed
                continue;
              }
            }
            
            const validation = validatePosition(newPosition);
            
            if (!validation.valid) {
              const reason = `Invalid checker count: Player1=${validation.playerCount}, Player2=${validation.opponentCount}`;
              inconsistentMoves.push({ moveIndex: i, player: 1, reason });
              firstError = i;
            } else {
              position = newPosition;
            }
          }
        }
      }
      
      // Only try player2's move if player1's move was valid (or not present) (skip cube decisions)
      if (firstError === null && move.player2Move && !move.player2Move.cubeAction && move.player2Move.move &&
          move.player2Move.move !== 'Cannot Move' && 
          move.player2Move.move !== '????') {
        
        // Validate move notation (e.g., check if * is correct)
        const notationCheck = validateMoveNotation(position, move.player2Move.move, false);
        if (!notationCheck.valid) {
          inconsistentMoves.push({ moveIndex: i, player: 2, reason: notationCheck.reason });
          firstError = i;
        } else {
          // Remove hit marker for validation
          const cleanMove = removeHitMarker(move.player2Move.move);
          
          // Check bar-first rule
          if (!validateBarFirstRule(position, cleanMove, false)) {
            const reason = 'Must enter from bar before moving other checkers';
            inconsistentMoves.push({ moveIndex: i, player: 2, reason });
            firstError = i;
          } else {
            const result = applyMove(position, cleanMove, false);
            const newPosition = result.position;
            
            // Check bar entry with dice if available
            const dice = move.player2Move.dice;
            if (dice) {
              const barDiceValidation = validateBarWithDice(position, newPosition, cleanMove, dice, false);
              if (!barDiceValidation.valid) {
                inconsistentMoves.push({ moveIndex: i, player: 2, reason: barDiceValidation.reason });
                firstError = i;
                // Don't update position if validation failed
                continue;
              }
            }
            
            const validation = validatePosition(newPosition);
            
            if (!validation.valid) {
              const reason = `Invalid checker count: Player1=${validation.playerCount}, Player2=${validation.opponentCount}`;
              inconsistentMoves.push({ moveIndex: i, player: 2, reason });
              firstError = i;
            } else {
              position = newPosition;
            }
          }
        }
      }
    } catch (error) {
      // If we can't apply a move, it's inconsistent
      const player = move.player1Move && move.player1Move.move ? 1 : 2;
      inconsistentMoves.push({ moveIndex: i, player, reason: `Error: ${error.message}` });
      firstError = i;
    }
  }
  
  return inconsistentMoves;
}

