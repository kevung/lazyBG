# LazyBG File Format Version Management

## Overview

LazyBG uses a versioned file format (`.lbg` files) to ensure compatibility between different versions of the application. This document describes the version management system and how to handle version migrations.

## Current Version

**Current Format Version:** `1.0.0`

## File Structure

All `.lbg` files are JSON files with the following top-level structure:

```json
{
  "version": "1.0.0",
  "metadata": {
    "site": "",
    "matchId": "",
    "event": "",
    "round": "",
    "player1": "",
    "player2": "",
    "eventDate": "",
    "eventTime": "",
    "variation": "Backgammon",
    "unrated": "Off",
    "crawford": "On",
    "cubeLimit": "1024",
    "transcriber": "",
    "matchLength": 0
  },
  "games": []
}
```

### Version Field

The `version` field is **required** and follows [Semantic Versioning](https://semver.org/):
- **Major version** (X.0.0): Breaking changes that require migration
- **Minor version** (0.X.0): New features that may require migration
- **Patch version** (0.0.X): Bug fixes, backward compatible

## Version Compatibility

### Loading Files

When loading a `.lbg` file, LazyBG performs the following checks:

1. **Version Detection**: Checks for the `version` field
   - If missing, assumes version `1.0.0` (backward compatibility)
   
2. **Compatibility Check**: Compares file version with application version
   - **Same version**: Load directly
   - **Older version**: Load with optional migration
   - **Newer version**: Display error (requires application update)

3. **Validation**: Ensures file structure is valid for the detected version

4. **Migration**: If needed, automatically migrates data to current version

### Saving Files

When saving a `.lbg` file, LazyBG always:
- Includes the current `version` field
- Uses the latest file format structure
- Validates data before writing

## Implementation Details

### Frontend Components

#### `transcriptionStore.js`
- Exports `LAZYBG_VERSION` constant
- Includes version in all transcription objects
- Used by all save operations

#### `versionUtils.js`
Provides utilities for version management:
- `compareVersions(v1, v2)`: Compare two version strings
- `checkCompatibility(fileVersion, currentVersion)`: Check if file can be loaded
- `migrateTranscription(transcription, targetVersion)`: Migrate data between versions
- `validateTranscription(transcription)`: Validate file structure

#### `matchParser.js`
- Adds version field when parsing `.txt` files
- Handles missing version field (backward compatibility)
- Applies migrations during deserialization

#### `App.svelte`
- Validates version when loading files
- Shows appropriate error/warning messages
- Saves files with current version

### Backend Components

#### `model.go`
Defines the Go struct for transcriptions:
```go
type Transcription struct {
    Version  string                `json:"version"`
    Metadata TranscriptionMetadata `json:"metadata"`
    Games    []TranscriptionGame   `json:"games"`
}
```

## Version History

### Version 1.0.0 (Current)
**Release Date:** December 2025

**Features:**
- Initial version field implementation
- Core transcription structure (metadata + games)
- Move tracking with dice, moves, illegal/gala flags
- Cube action support
- Game winner tracking
- Player score tracking

**Structure:**
- Top-level: `version`, `metadata`, `games`
- Metadata: Site, event, players, date/time, match settings
- Games: Array of game objects with moves and winner
- Moves: Player moves, cube actions, move validation

## Adding New Versions

When creating a new version of the file format:

### 1. Update Version Constants

Update `LAZYBG_VERSION` in:
- `frontend/src/stores/transcriptionStore.js`
- `model.go` (if backend changes)

### 2. Implement Migration Function

Add a migration function in `versionUtils.js`:

```javascript
function migrateToV1_1_0(transcription) {
    return {
        ...transcription,
        version: '1.1.0',
        // Add new fields or transform existing ones
        metadata: {
            ...transcription.metadata,
            newField: 'defaultValue'
        }
    };
}
```

### 3. Update Migration Chain

Add the migration to the `migrateTranscription` function:

```javascript
export function migrateTranscription(transcription, targetVersion) {
    const currentVersion = transcription.version || '1.0.0';
    let migrated = { ...transcription };
    
    // Migration chain
    if (compareVersions(currentVersion, '1.1.0') < 0 && 
        compareVersions(targetVersion, '1.1.0') >= 0) {
        migrated = migrateToV1_1_0(migrated);
    }
    
    // Add future migrations here...
    
    return migrated;
}
```

### 4. Update Validation

Update `validateTranscription` if new required fields are added:

```javascript
export function validateTranscription(transcription) {
    const errors = [];
    
    // Check for new required fields based on version
    if (compareVersions(transcription.version, '1.1.0') >= 0) {
        if (!transcription.metadata.newField) {
            errors.push('Missing newField in metadata');
        }
    }
    
    return { valid: errors.length === 0, errors };
}
```

### 5. Update Backend Models

Update Go structs in `model.go` if needed:

```go
type TranscriptionMetadata struct {
    // ... existing fields
    NewField string `json:"newField,omitempty"` // Add omitempty for backward compat
}
```

### 6. Update Documentation

Update this file with:
- New version number and release date
- Description of changes
- Migration notes
- Breaking changes (if any)

### 7. Test Migration

Create test cases to verify:
- Old files load correctly with migration
- New files save with correct version
- Validation works for both old and new formats
- No data loss during migration

## Best Practices

1. **Always use semantic versioning**
   - Increment major version for breaking changes
   - Increment minor version for new features
   - Increment patch version for bug fixes

2. **Maintain backward compatibility when possible**
   - Use optional fields (`omitempty` in Go)
   - Provide default values in migrations
   - Never remove required fields without major version bump

3. **Test thoroughly**
   - Test loading old files
   - Test saving new files
   - Test migration paths
   - Test validation logic

4. **Document changes**
   - Update version history
   - Document breaking changes
   - Provide migration examples
   - Update API documentation

5. **User communication**
   - Show clear messages during migration
   - Warn about incompatible versions
   - Provide upgrade instructions

## Troubleshooting

### File Won't Load

**Symptom:** "This file was created with a newer version" error

**Solution:** Update your LazyBG application to the latest version

---

**Symptom:** "Invalid file structure" error

**Solution:** 
1. Check if file is corrupted
2. Try opening in text editor to verify JSON structure
3. Check for missing required fields
4. Restore from backup if available

### Migration Issues

**Symptom:** Data looks incorrect after loading old file

**Solution:**
1. Check migration function for the specific version
2. Verify field mappings are correct
3. Report bug with example file if issue persists

### Version Mismatch

**Symptom:** Warning about file being from older version

**Solution:** This is normal. LazyBG will automatically migrate the file to the current version when you save it.

## Related Files

- `frontend/src/stores/transcriptionStore.js` - Store with LAZYBG_VERSION constant
- `frontend/src/utils/versionUtils.js` - Version management utilities
- `frontend/src/utils/matchParser.js` - File parsing with version handling
- `frontend/src/App.svelte` - File loading/saving with version checks
- `model.go` - Backend data structures
- Test files: `test/match*.lbg` - Example files with version field

## Future Considerations

Potential features for future versions:

- **v1.1.0**: Enhanced metadata (ratings, time controls)
- **v1.2.0**: Analysis annotations
- **v2.0.0**: New game variants support
- **v2.1.0**: Multi-language support
- **v3.0.0**: Cloud sync format

---

**Note:** This version management system was implemented in December 2025 to ensure long-term compatibility and smooth upgrades for LazyBG users.
