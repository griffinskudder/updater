# Database Schema Management

This directory contains database schema files organized by engine type, designed for easy migration management.

## Structure

```
schema/
├── postgres/           # PostgreSQL-specific schemas
│   ├── 001_initial.sql
│   ├── 002_add_indexes.sql
│   └── ...
├── sqlite/             # SQLite-specific schemas
│   ├── 001_initial.sql
│   ├── 002_add_indexes.sql
│   └── ...
└── README.md
```

## Migration Naming Convention

- **Format**: `{number}_{description}.sql`
- **Number**: 3-digit zero-padded sequential number (001, 002, 003...)
- **Description**: Brief snake_case description of the change

Examples:
- `001_initial.sql` - Initial schema creation
- `002_add_indexes.sql` - Performance index additions
- `003_add_user_table.sql` - New feature additions
- `004_alter_releases_table.sql` - Schema modifications

## Migration Guidelines

### PostgreSQL-Specific Features
- Use JSONB for JSON columns
- Utilize advanced indexing (GIN, partial indexes)
- Leverage PostgreSQL-specific functions
- Use proper timestamp types (TIMESTAMPTZ)

### SQLite-Specific Considerations
- Use TEXT for JSON storage
- Simpler indexing strategy
- TEXT for timestamp storage
- Consider SQLite limitations (no ALTER COLUMN, etc.)

### Best Practices
1. **Additive Changes**: Prefer adding new columns/tables over modifying existing ones
2. **Backward Compatibility**: Ensure changes don't break existing code
3. **Index Strategy**: Add indexes for query performance, but consider write performance impact
4. **Data Types**: Use appropriate types for each database engine
5. **Comments**: Document complex changes and business logic

## sqlc Integration

The sqlc tool reads all `.sql` files in each directory to generate the complete schema context for code generation. This allows:

- **Incremental Development**: Add new migrations without regenerating everything
- **Engine-Specific Optimizations**: Different schemas for different databases
- **Version Control Friendly**: Each migration is a separate file
- **Team Collaboration**: Clear migration history and conflict resolution

## Usage with sqlc

```bash
# Generate code for all schemas (recommended)
sqlc generate

# The tool automatically processes all .sql files in:
# - internal/storage/sqlc/schema/postgres/
# - internal/storage/sqlc/schema/sqlite/
```

## Migration Tools Integration

This structure is compatible with popular Go migration tools:

- **golang-migrate/migrate**: Can read files in sequence
- **pressly/goose**: Supports numbered migration files
- **rubenv/sql-migrate**: Works with directory-based migrations

## Future Considerations

- Consider adding a `migrations` table to track applied migrations
- Implement rollback scripts (down migrations) if needed
- Add validation scripts to ensure schema consistency
- Consider automated migration testing in CI/CD