import { writable, derived, get } from 'svelte/store';
import { transcriptionStore, selectedMoveStore } from './transcriptionStore';

// Store for showing the search modal
export const showMoveSearchModalStore = writable(false);

// Store for search query and type
export const searchQueryStore = writable('');
export const searchTypeStore = writable('dice'); // 'dice', 'double', 'take', 'pass'

// Store for search results (array of {gameIndex, moveIndex, player})
export const searchResultsStore = writable([]);

// Store for current result index
export const currentSearchResultIndexStore = writable(-1);

// Derived store for current result
export const currentSearchResultStore = derived(
    [searchResultsStore, currentSearchResultIndexStore],
    ([$results, $index]) => {
        if ($index >= 0 && $index < $results.length) {
            return $results[$index];
        }
        return null;
    }
);

/**
 * Search for moves matching dice roll (e.g., '32' or '23')
 * @param {string} diceQuery - Two digits representing dice roll
 * @returns {Array} Array of matching moves with {gameIndex, moveIndex, player}
 */
export function searchByDice(diceQuery) {
    const transcription = get(transcriptionStore);
    const results = [];
    
    if (!diceQuery || diceQuery.length !== 2) {
        return results;
    }
    
    // Normalize dice query (both orders)
    const dice1 = diceQuery;
    const dice2 = diceQuery.split('').reverse().join('');
    
    transcription.games.forEach((game, gameIndex) => {
        game.moves.forEach((move, moveIndex) => {
            // Check player 1
            if (move.player1Move && move.player1Move.dice) {
                const moveDice = move.player1Move.dice.replace(/[^0-9]/g, '');
                if (moveDice === dice1 || moveDice === dice2) {
                    results.push({ gameIndex, moveIndex, player: 1 });
                }
            }
            
            // Check player 2
            if (move.player2Move && move.player2Move.dice) {
                const moveDice = move.player2Move.dice.replace(/[^0-9]/g, '');
                if (moveDice === dice1 || moveDice === dice2) {
                    results.push({ gameIndex, moveIndex, player: 2 });
                }
            }
        });
    });
    
    return results;
}

/**
 * Search for cube actions (double, take, pass)
 * @param {string} action - 'd' for double, 't' for take, 'p' for pass
 * @returns {Array} Array of matching moves with {gameIndex, moveIndex, player}
 */
export function searchByCubeAction(action) {
    const transcription = get(transcriptionStore);
    const results = [];
    
    const actionMap = {
        'd': 'doubles',
        't': 'takes',
        'p': 'drops'
    };
    
    const searchAction = actionMap[action.toLowerCase()];
    if (!searchAction) {
        return results;
    }
    
    transcription.games.forEach((game, gameIndex) => {
        game.moves.forEach((move, moveIndex) => {
            if (move.cubeAction) {
                // For 'doubles', check the action field
                if (searchAction === 'doubles' && move.cubeAction.action === 'doubles') {
                    results.push({ 
                        gameIndex, 
                        moveIndex, 
                        player: move.cubeAction.player 
                    });
                }
                // For 'takes' or 'drops', check the response field
                else if (searchAction !== 'doubles' && move.cubeAction.response === searchAction) {
                    // The player who responds is opposite to the one who doubled
                    const respondingPlayer = move.cubeAction.player === 1 ? 2 : 1;
                    results.push({ 
                        gameIndex, 
                        moveIndex, 
                        player: respondingPlayer
                    });
                }
            }
        });
    });
    
    return results;
}

/**
 * Execute a search based on query
 * @param {string} query - Search query (e.g., '32', 'd', 't', 'p')
 */
export function executeSearch(query) {
    if (!query) {
        searchResultsStore.set([]);
        currentSearchResultIndexStore.set(-1);
        return;
    }
    
    const normalizedQuery = query.trim().toLowerCase();
    let results = [];
    
    // Check if it's a dice query (2 digits)
    if (/^\d{2}$/.test(normalizedQuery)) {
        results = searchByDice(normalizedQuery);
        searchTypeStore.set('dice');
    }
    // Check if it's a cube action (d, t, or p)
    else if (['d', 't', 'p'].includes(normalizedQuery)) {
        results = searchByCubeAction(normalizedQuery);
        const typeMap = { d: 'double', t: 'take', p: 'pass' };
        searchTypeStore.set(typeMap[normalizedQuery]);
    }
    
    searchResultsStore.set(results);
    searchQueryStore.set(query);
    
    // If results found, go to first result
    if (results.length > 0) {
        currentSearchResultIndexStore.set(0);
        selectedMoveStore.set(results[0]);
    } else {
        currentSearchResultIndexStore.set(-1);
    }
}

/**
 * Navigate to next search result
 */
export function nextSearchResult() {
    const results = get(searchResultsStore);
    const currentIndex = get(currentSearchResultIndexStore);
    
    if (results.length === 0) return;
    
    // Prevent going beyond the last result
    if (currentIndex >= results.length - 1) return;
    
    const nextIndex = currentIndex + 1;
    currentSearchResultIndexStore.set(nextIndex);
    selectedMoveStore.set(results[nextIndex]);
}

/**
 * Navigate to previous search result
 */
export function previousSearchResult() {
    const results = get(searchResultsStore);
    const currentIndex = get(currentSearchResultIndexStore);
    
    if (results.length === 0) return;
    
    // Prevent going before the first result
    if (currentIndex <= 0) return;
    
    const prevIndex = currentIndex - 1;
    currentSearchResultIndexStore.set(prevIndex);
    selectedMoveStore.set(results[prevIndex]);
}

/**
 * Clear search results
 */
export function clearSearch() {
    searchResultsStore.set([]);
    currentSearchResultIndexStore.set(-1);
    searchQueryStore.set('');
}
