# lazyBG

lazyBG is a backgammon match transcription assistant. It uses [GNU
Backgammon](https://www.gnu.org/software/gnubg), a sophisticated neural net
based multi-ply evalution engine to get candidate checker moves during the
transcription process.

## File Format

LazyBG uses a versioned JSON file format (`.lbg` extension) to store match transcriptions. The current format version is **1.0.0**.

### Version Management

All `.lbg` files include a `version` field to track the file format and ensure compatibility:

```json
{
  "version": "1.0.0",
  "metadata": { ... },
  "games": [ ... ]
}
```

**Key Features:**
- **Backward compatibility**: Older files are automatically migrated when opened
- **Forward compatibility check**: Warns if file was created with newer version
- **Validation**: Ensures file structure integrity
- **Semantic versioning**: Clear version progression (major.minor.patch)

For detailed information about version management, migrations, and file format specifications, see:
- [Version Management Documentation](doc/VERSION_MANAGEMENT.md)
- [Migration Example](doc/MIGRATION_EXAMPLE.js)

# Acknowledgements

I cheerfully thank Rami Keränen alias [foochu](https://github.com/foochu) for
porting GNU Backgammon engine checker evaluation to Go.

