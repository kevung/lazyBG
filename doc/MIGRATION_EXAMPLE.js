// Example Migration: Adding Rating Information in v1.1.0
// This file demonstrates how to implement a version migration
// DO NOT include this file in production - it's for reference only

/**
 * STEP 1: Update the version constant
 * File: frontend/src/stores/transcriptionStore.js
 */

// Change from:
export const LAZYBG_VERSION = '1.0.0';

// To:
export const LAZYBG_VERSION = '1.1.0';

/**
 * STEP 2: Update the Go model (if backend changes needed)
 * File: model.go
 */

// Update TranscriptionVersion constant:
const (
    DatabaseVersion      = "1.2.0"
    TranscriptionVersion = "1.1.0"  // Changed from 1.0.0
)

// Add new optional fields to metadata:
type TranscriptionMetadata struct {
    Site        string  `json:"site"`
    MatchID     string  `json:"matchId"`
    Event       string  `json:"event"`
    Round       string  `json:"round"`
    Player1     string  `json:"player1"`
    Player2     string  `json:"player2"`
    Player1Elo  float64 `json:"player1Elo,omitempty"`   // NEW FIELD
    Player2Elo  float64 `json:"player2Elo,omitempty"`   // NEW FIELD
    EventDate   string  `json:"eventDate"`
    // ... rest of fields
}

/**
 * STEP 3: Update the transcription store structure
 * File: frontend/src/stores/transcriptionStore.js
 */

export const transcriptionStore = writable({
    version: LAZYBG_VERSION,
    metadata: {
        site: '',
        matchId: '',
        event: '',
        round: '',
        player1: '',
        player2: '',
        player1Elo: 0,      // NEW FIELD
        player2Elo: 0,      // NEW FIELD
        eventDate: '',
        // ... rest of fields
    },
    games: []
});

/**
 * STEP 4: Create migration function
 * File: frontend/src/utils/versionUtils.js
 */

// Add this function before migrateTranscription:
function migrateToV1_1_0(transcription) {
    console.log('Migrating to v1.1.0: Adding player rating fields');
    
    return {
        ...transcription,
        version: '1.1.0',
        metadata: {
            ...transcription.metadata,
            // Add new fields with default values
            player1Elo: transcription.metadata.player1Elo || 0,
            player2Elo: transcription.metadata.player2Elo || 0
        }
    };
}

/**
 * STEP 5: Update migration chain
 * File: frontend/src/utils/versionUtils.js
 */

export function migrateTranscription(transcription, targetVersion) {
    const currentVersion = transcription.version || '1.0.0';
    
    if (currentVersion === targetVersion) {
        return transcription;
    }
    
    let migrated = { ...transcription };
    
    // Migration chain - add new migrations in order
    if (compareVersions(currentVersion, '1.1.0') < 0 && 
        compareVersions(targetVersion, '1.1.0') >= 0) {
        migrated = migrateToV1_1_0(migrated);
    }
    
    // Future migrations go here:
    // if (compareVersions(migrated.version, '1.2.0') < 0 && 
    //     compareVersions(targetVersion, '1.2.0') >= 0) {
    //     migrated = migrateToV1_2_0(migrated);
    // }
    
    migrated.version = targetVersion;
    console.log(`Migrated transcription from v${currentVersion} to v${targetVersion}`);
    return migrated;
}

/**
 * STEP 6: Update validation (optional)
 * File: frontend/src/utils/versionUtils.js
 */

export function validateTranscription(transcription) {
    const errors = [];
    
    // Existing validation...
    if (!transcription.version) {
        errors.push('Missing version field');
    }
    
    // Version-specific validation
    if (transcription.version === '1.1.0') {
        // Validate new fields if they are required
        if (transcription.metadata.player1Elo !== undefined && 
            typeof transcription.metadata.player1Elo !== 'number') {
            errors.push('player1Elo must be a number');
        }
        if (transcription.metadata.player2Elo !== undefined && 
            typeof transcription.metadata.player2Elo !== 'number') {
            errors.push('player2Elo must be a number');
        }
    }
    
    return {
        valid: errors.length === 0,
        errors
    };
}

/**
 * STEP 7: Update clearTranscription function
 * File: frontend/src/stores/transcriptionStore.js
 */

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
            player1Elo: 0,      // NEW FIELD
            player2Elo: 0,      // NEW FIELD
            eventDate: '',
            // ... rest of fields
        },
        games: []
    });
    // ... rest of function
}

/**
 * STEP 8: Update matchParser to handle new fields
 * File: frontend/src/utils/matchParser.js
 */

export function parseMatchFile(content) {
    // ... existing parsing code
    
    const transcription = {
        version: LAZYBG_VERSION,
        metadata: {
            site: '',
            matchId: '',
            event: '',
            round: '',
            player1: '',
            player2: '',
            player1Elo: 0,      // NEW FIELD
            player2Elo: 0,      // NEW FIELD
            // ... rest of fields
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
                    // ... existing cases
                    case 'Player 1 Elo': 
                        transcription.metadata.player1Elo = parseFloat(value) || 0;
                        break;
                    case 'Player 2 Elo': 
                        transcription.metadata.player2Elo = parseFloat(value) || 0;
                        break;
                }
            }
        }
    }
    
    return transcription;
}

/**
 * STEP 9: Update UI components to display new fields
 * File: frontend/src/components/MetadataModal.svelte (or similar)
 */

// Add input fields for player ratings in the metadata form

/**
 * STEP 10: Test the migration
 */

// Test cases to verify:
// 1. Load a v1.0.0 file - should migrate to v1.1.0 automatically
// 2. Save a new file - should have v1.1.0 and include rating fields
// 3. Load a v1.1.0 file - should load without migration
// 4. Verify validation works for both versions
// 5. Verify no data loss during migration

/**
 * STEP 11: Update VERSION_MANAGEMENT.md
 */

// Add to version history:
/*
### Version 1.1.0
**Release Date:** [DATE]

**Features:**
- Added player rating (Elo) fields to metadata
- player1Elo: Optional field for player 1's rating
- player2Elo: Optional field for player 2's rating

**Migration:**
- Files from v1.0.0 are automatically migrated
- Rating fields default to 0 if not present
- Fully backward compatible

**Breaking Changes:**
- None
*/

/**
 * COMPLETE EXAMPLE: Version 1.1.0 Test
 */

// Old file (v1.0.0):
const oldFile = {
    "version": "1.0.0",
    "metadata": {
        "site": "Paris",
        "player1": "Alice",
        "player2": "Bob",
        // ... other fields
    },
    "games": []
};

// After loading and migration:
const migratedFile = {
    "version": "1.1.0",
    "metadata": {
        "site": "Paris",
        "player1": "Alice",
        "player2": "Bob",
        "player1Elo": 0,     // Added during migration
        "player2Elo": 0,     // Added during migration
        // ... other fields
    },
    "games": []
};

// New file (v1.1.0):
const newFile = {
    "version": "1.1.0",
    "metadata": {
        "site": "Paris",
        "player1": "Alice",
        "player2": "Bob",
        "player1Elo": 1850,  // User-provided rating
        "player2Elo": 1720,  // User-provided rating
        // ... other fields
    },
    "games": []
};
