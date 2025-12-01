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
 * @returns {Object} - New position after applying move
 */
function applyMoveSegment(position, moveSegment, isPlayer = true) {
  const newPos = clonePosition(position);
  let { from, to } = moveSegment;
  
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
        return newPos;
      }
      newPos.bar--;  // Remove from player bar (decrease positive count)
    } else {
      if (newPos.opponentBar >= 0) {
        console.warn('Cannot move opponent from bar: no checkers on bar');
        return newPos;
      }
      newPos.opponentBar++;  // Remove from opponent bar (increase towards 0 from negative)
    }

    // Place checker on destination point
    if (typeof to === 'number' && to >= 1 && to <= 24) {
      // Check if we hit an opponent checker
      if (isPlayer && newPos.points[to] === -1) {
        newPos.points[to] = 1;
        newPos.opponentBar--;  // Opponent bar becomes more negative
      } else if (!isPlayer && newPos.points[to] === 1) {
        newPos.points[to] = -1;
        newPos.bar++;  // Player bar increases
      } else {
        newPos.points[to] += multiplier;
      }
    }
    return newPos;
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
    return newPos;
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
      } else if (!isPlayer && newPos.points[to] === 1) {
        console.log(`Player 2 hits opponent at point ${to}, bar: ${newPos.bar} -> ${newPos.bar + 1}`);
        newPos.points[to] = -1;
        newPos.bar++;  // Player bar increases (0 -> 1)
      } else {
        newPos.points[to] += multiplier;
      }
    }
  }

  return newPos;
}

/**
 * Apply a complete move (potentially multiple segments) to a position
 * @param {Object} position - Starting position
 * @param {string} moveText - Move notation (e.g., "24/20 13/8")
 * @param {boolean} isPlayer - true for current player, false for opponent
 * @returns {Object} - New position after all move segments applied
 */
export function applyMove(position, moveText, isPlayer = true) {
  const segments = parseMoveNotation(moveText);
  
  let currentPos = position;
  for (const segment of segments) {
    currentPos = applyMoveSegment(currentPos, segment, isPlayer);
  }
  
  return currentPos;
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
        position = applyMove(position, move.player1Move.move, true);
        break;
      }
    }

    // Apply both players' moves for completed moves
    if (move.player1Move) {
      position = applyMove(position, move.player1Move.move, true);
    }
    if (move.player2Move) {
      position = applyMove(position, move.player2Move.move, false);
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
