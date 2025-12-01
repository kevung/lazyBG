# LazyBG Version Management - Quick Reference

## For Users

### What is the version field?
Every `.lbg` file now includes a version number (e.g., "1.0.0") that identifies which format the file uses.

### What happens when I open an old file?
LazyBG automatically updates old files to the latest format when you open them. Your data is preserved and enhanced with new features.

### What if I see a version warning?
- **"File from older version"**: Normal - file will be automatically updated
- **"File from newer version"**: Update LazyBG to open this file

### How do I check the file version?
Open the `.lbg` file in a text editor and look for the `"version"` field at the top.

---

## For Developers

### Quick Start - Adding a New Version

1. **Update version constants**
   ```javascript
   // frontend/src/stores/transcriptionStore.js
   export const LAZYBG_VERSION = '1.1.0';
   ```

2. **Create migration function**
   ```javascript
   // frontend/src/utils/versionUtils.js
   function migrateToV1_1_0(transcription) {
       return {
           ...transcription,
           version: '1.1.0',
           // Add new fields with defaults
           metadata: {
               ...transcription.metadata,
               newField: 'defaultValue'
           }
       };
   }
   ```

3. **Add to migration chain**
   ```javascript
   // frontend/src/utils/versionUtils.js
   if (compareVersions(currentVersion, '1.1.0') < 0 && 
       compareVersions(targetVersion, '1.1.0') >= 0) {
       migrated = migrateToV1_1_0(migrated);
   }
   ```

4. **Update stores and models**
   - Add new fields to `transcriptionStore`
   - Update `clearTranscription()`
   - Update Go structs in `model.go`

5. **Test thoroughly**
   - Load old files (should migrate)
   - Save new files (should have new version)
   - Verify data integrity

### Version Naming Convention

**Semantic Versioning: MAJOR.MINOR.PATCH**

- **MAJOR** (X.0.0): Breaking changes, incompatible with previous versions
- **MINOR** (0.X.0): New features, backward compatible, may need migration
- **PATCH** (0.0.X): Bug fixes, fully backward compatible, no migration needed

### Key Files

| File | Purpose |
|------|---------|
| `frontend/src/stores/transcriptionStore.js` | Version constant, store structure |
| `frontend/src/utils/versionUtils.js` | Version utilities and migrations |
| `frontend/src/utils/matchParser.js` | File parsing with version handling |
| `frontend/src/App.svelte` | Loading/saving with version checks |
| `model.go` | Backend data structures |
| `doc/VERSION_MANAGEMENT.md` | Full documentation |
| `doc/MIGRATION_EXAMPLE.js` | Step-by-step migration example |

### API Reference

#### Compare Versions
```javascript
import { compareVersions } from './utils/versionUtils.js';

compareVersions('1.0.0', '1.1.0'); // Returns -1 (first is older)
compareVersions('2.0.0', '1.9.9'); // Returns 1 (first is newer)
compareVersions('1.0.0', '1.0.0'); // Returns 0 (equal)
```

#### Check Compatibility
```javascript
import { checkCompatibility } from './utils/versionUtils.js';

const result = checkCompatibility('1.0.0', '1.1.0');
// {
//   compatible: true,
//   needsMigration: true,
//   message: "This file will be updated from v1.0.0 to v1.1.0."
// }
```

#### Migrate Transcription
```javascript
import { migrateTranscription } from './utils/versionUtils.js';

const migrated = migrateTranscription(oldTranscription, '1.1.0');
// Returns transcription object migrated to version 1.1.0
```

#### Validate Transcription
```javascript
import { validateTranscription } from './utils/versionUtils.js';

const result = validateTranscription(transcription);
// {
//   valid: true,
//   errors: []
// }
```

### Common Patterns

#### Optional Field (Backward Compatible)
```javascript
// Add with default value in migration
function migrateToV1_1_0(transcription) {
    return {
        ...transcription,
        metadata: {
            ...transcription.metadata,
            optionalField: transcription.metadata.optionalField || 'default'
        }
    };
}
```

#### Required Field (Breaking Change)
```javascript
// Increment major version (e.g., 1.x.x -> 2.0.0)
function migrateToV2_0_0(transcription) {
    if (!transcription.metadata.requiredField) {
        throw new Error('Cannot migrate: missing required field');
    }
    return transcription;
}
```

#### Field Rename
```javascript
function migrateToV1_2_0(transcription) {
    const { oldName, ...rest } = transcription.metadata;
    return {
        ...transcription,
        metadata: {
            ...rest,
            newName: oldName // Rename field
        }
    };
}
```

#### Data Transformation
```javascript
function migrateToV1_3_0(transcription) {
    return {
        ...transcription,
        games: transcription.games.map(game => ({
            ...game,
            // Transform data structure
            score: {
                player1: game.player1Score,
                player2: game.player2Score
            }
        }))
    };
}
```

### Testing Checklist

- [ ] Old files load without errors
- [ ] Old files are migrated correctly
- [ ] New files save with current version
- [ ] Validation accepts both old and new formats
- [ ] No data loss during migration
- [ ] Version mismatch warnings display correctly
- [ ] Migration chain works for multiple version jumps (1.0 → 1.2)
- [ ] Backend can serialize/deserialize new format
- [ ] UI displays new fields correctly

### Deployment Checklist

- [ ] Update `LAZYBG_VERSION` constant
- [ ] Implement migration function(s)
- [ ] Update migration chain
- [ ] Update Go models if needed
- [ ] Update stores and initialization functions
- [ ] Update validation logic
- [ ] Update UI components
- [ ] Test migration thoroughly
- [ ] Update `VERSION_MANAGEMENT.md`
- [ ] Update changelog
- [ ] Tag release with version number

### Troubleshooting

**Q: Migration not being applied?**
- Check version comparison logic
- Verify migration function is in the chain
- Check console for migration messages

**Q: Validation failing for old files?**
- Add version-specific validation
- Make new fields optional in validation
- Check for field presence before validating

**Q: Data corrupted after migration?**
- Review migration function logic
- Check for field name typos
- Verify deep copying of nested objects
- Add migration unit tests

### Resources

- [Semantic Versioning](https://semver.org/)
- [Full Documentation](VERSION_MANAGEMENT.md)
- [Migration Example](MIGRATION_EXAMPLE.js)
- [LazyBG Source Code](https://github.com/kevung/lazyBG)

---

**Last Updated:** December 2025  
**Current Version:** 1.0.0
