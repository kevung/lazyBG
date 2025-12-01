// Version management utilities for lazyBG file format

/**
 * Compare two semantic version strings
 * @param {string} v1 - First version (e.g., "1.0.0")
 * @param {string} v2 - Second version (e.g., "2.0.0")
 * @returns {number} - Returns -1 if v1 < v2, 0 if v1 === v2, 1 if v1 > v2
 */
export function compareVersions(v1, v2) {
    const parts1 = v1.split('.').map(Number);
    const parts2 = v2.split('.').map(Number);
    
    for (let i = 0; i < Math.max(parts1.length, parts2.length); i++) {
        const num1 = parts1[i] || 0;
        const num2 = parts2[i] || 0;
        
        if (num1 < num2) return -1;
        if (num1 > num2) return 1;
    }
    
    return 0;
}

/**
 * Check if a version is compatible with the current version
 * @param {string} fileVersion - Version from the file
 * @param {string} currentVersion - Current application version
 * @returns {object} - { compatible: boolean, needsMigration: boolean, message: string }
 */
export function checkCompatibility(fileVersion, currentVersion) {
    const comparison = compareVersions(fileVersion, currentVersion);
    
    // Future version (file is newer than app)
    if (comparison > 0) {
        return {
            compatible: false,
            needsMigration: false,
            message: `This file was created with a newer version of lazyBG (v${fileVersion}). Please update your application to open this file.`
        };
    }
    
    // Same version
    if (comparison === 0) {
        return {
            compatible: true,
            needsMigration: false,
            message: 'File version matches current version.'
        };
    }
    
    // Older version - check if migration is needed
    const fileParts = fileVersion.split('.').map(Number);
    const currentParts = currentVersion.split('.').map(Number);
    
    // Major version difference requires migration
    if (fileParts[0] < currentParts[0]) {
        return {
            compatible: true,
            needsMigration: true,
            message: `This file will be migrated from v${fileVersion} to v${currentVersion}.`
        };
    }
    
    // Minor version difference may require migration (depending on changes)
    if (fileParts[1] < currentParts[1]) {
        return {
            compatible: true,
            needsMigration: true,
            message: `This file will be updated from v${fileVersion} to v${currentVersion}.`
        };
    }
    
    // Patch version difference doesn't require migration
    return {
        compatible: true,
        needsMigration: false,
        message: 'File is compatible with current version.'
    };
}

/**
 * Migrate transcription data from one version to another
 * @param {object} transcription - The transcription object to migrate
 * @param {string} targetVersion - The version to migrate to
 * @returns {object} - Migrated transcription object
 */
export function migrateTranscription(transcription, targetVersion) {
    const currentVersion = transcription.version || '1.0.0';
    
    // If already at target version, return as-is
    if (currentVersion === targetVersion) {
        return transcription;
    }
    
    let migrated = { ...transcription };
    
    // Migration chain - add future migrations here
    // Example for v1.0.0 -> v1.1.0:
    // if (compareVersions(currentVersion, '1.1.0') < 0 && compareVersions(targetVersion, '1.1.0') >= 0) {
    //     migrated = migrateToV1_1_0(migrated);
    // }
    
    // Update version to target
    migrated.version = targetVersion;
    
    console.log(`Migrated transcription from v${currentVersion} to v${targetVersion}`);
    return migrated;
}

/**
 * Validate transcription structure for a given version
 * @param {object} transcription - The transcription object to validate
 * @returns {object} - { valid: boolean, errors: string[] }
 */
export function validateTranscription(transcription) {
    const errors = [];
    
    // Check required fields
    if (!transcription.version) {
        errors.push('Missing version field');
    }
    
    if (!transcription.metadata) {
        errors.push('Missing metadata field');
    } else {
        // Validate metadata structure
        const requiredMetadata = ['variation', 'crawford', 'matchLength'];
        for (const field of requiredMetadata) {
            if (transcription.metadata[field] === undefined || transcription.metadata[field] === null) {
                errors.push(`Missing required metadata field: ${field}`);
            }
        }
    }
    
    if (!transcription.games || !Array.isArray(transcription.games)) {
        errors.push('Missing or invalid games field');
    }
    
    return {
        valid: errors.length === 0,
        errors
    };
}

// Example migration function for future use
// function migrateToV1_1_0(transcription) {
//     // Add new fields or transform existing ones
//     return {
//         ...transcription,
//         metadata: {
//             ...transcription.metadata,
//             // Add new metadata field example:
//             // newField: 'defaultValue'
//         }
//     };
// }
