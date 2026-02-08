---
name: tpg-admin
description: >-
  Maintain, repair, and diagnose the tpg database. Use this skill when dealing
  with database corruption, backup operations, restore operations, diagnostics,
  integrity issues, or general tpg database maintenance. Trigger phrases:
  "backup", "restore", "database corrupt", "diagnostics", "maintain tpg",
  "repair", "clean up". This skill covers all administrative operations for
  the tpg task database including doctor checks, backups, restores, cleanup,
  and configuration management.
---

# tpg-admin

Administrative operations for the tpg database - backup, restore, diagnostics,
repair, and maintenance. Use this skill when the database needs attention or
before/after risky operations.

## ALWAYS use this skill when

- The user mentions "backup" or "restore" for the tpg database.
- The user says "database corrupt", "corruption", or "integrity issues".
- The user asks for "diagnostics" or "doctor" checks on tpg.
- The user wants to "maintain tpg", "repair", or "fix" the database.
- The user asks to "clean up", "vacuum", or "optimize" the database.
- Before or after risky operations that could affect database integrity.
- When recovering from crashes, errors, or unexpected shutdowns.

## When NOT to use

- For regular task operations (use tpg-agent or tpg-orchestrator).
- For creating new tasks or managing work (use tpg-planner).
- For code exploration (use explore-code agent).

## Safety First: ALWAYS Backup Before Risky Operations

**CRITICAL:** Before running `tpg doctor` or `tpg restore`, ALWAYS create a backup:

```bash
# Create backup before any repair or restore operation
tpg backup

# Then proceed with doctor or restore
tpg doctor        # or
tpg restore <path>
```

This ensures you can recover if something goes wrong.

## Commands Reference

### doctor - Check and Fix Integrity Issues

Diagnose and repair database integrity problems.

```bash
tpg doctor              # Check and fix issues
tpg doctor --dry-run    # Show issues without fixing
```

**When to use:**
- After crashes or unexpected shutdowns
- When seeing database errors or corruption warnings
- Periodically as preventive maintenance
- Before major operations (as a check)

**What it does:**
- Validates database schema
- Checks for orphaned records
- Repairs referential integrity
- Reports data inconsistencies

### backup - Create Database Backup

Create a timestamped backup of the tpg database.

```bash
tpg backup              # Create backup with auto-generated name
tpg backup [path]       # Create backup at specific path
tpg backup -q           # Quiet mode (no output)
```

**When to use:**
- **ALWAYS before `tpg doctor`** (safety precaution)
- **ALWAYS before `tpg restore`** (preserve current state)
- Before risky operations (bulk edits, migrations)
- On a schedule for disaster recovery
- Before upgrading tpg versions

**Best practices:**
- Backups are stored in `.tpg/backups/` by default
- Use descriptive names: `tpg backup pre-migration-v7`
- Keep recent backups; clean old ones periodically

### backups - List Available Backups

Show all available backup files.

```bash
tpg backups             # List all backups with timestamps and sizes
```

**When to use:**
- Before restore to see available options
- To check backup health and age
- To identify old backups for cleanup

### restore - Recover from Backup

Restore the database from a backup file.

```bash
tpg restore <path>      # Restore from specific backup file
```

**When to use:**
- After data corruption or loss
- To rollback after failed operations
- To migrate data between environments
- To recover from accidental deletions

**⚠️ WARNING - DESTRUCTIVE OPERATION:**
- **ALWAYS backup first** - restore OVERWRITES current database
- Current database will be replaced entirely
- Any changes since backup will be lost
- Verify backup integrity before restoring

**Safety workflow:**
```bash
# 1. Backup current state (even if corrupted)
tpg backup pre-restore-safety

# 2. List available backups
tpg backups

# 3. Restore from chosen backup
tpg restore .tpg/backups/tpg-20240115-120000.db

# 4. Verify restoration
tpg status
```

### clean - Vacuum and Optimize

Clean up old data and optimize database performance.

```bash
tpg clean --vacuum              # Compact database (reclaim space)
tpg clean --done --days 30      # Remove done tasks older than 30 days
tpg clean --canceled --days 30  # Remove canceled tasks older than 30 days
tpg clean --logs                # Remove orphaned log entries
tpg clean --all --days 30       # Do all cleanup operations
tpg clean --dry-run             # Preview what would be deleted
```

**When to use:**
- Database file is unusually large
- Performance has degraded
- After archiving old projects
- Periodic maintenance (monthly recommended)

**⚠️ WARNING - DESTRUCTIVE OPERATION:**
- `--done` and `--canceled` permanently delete tasks
- Use `--dry-run` first to see what would be removed
- Deleted tasks cannot be recovered (unless from backup)

### config - View and Modify Settings

Manage tpg configuration values.

```bash
tpg config                      # Show all configuration
tpg config <key>                # Show specific config value
tpg config <key> <value>        # Set config value
```

**When to use:**
- Checking current settings
- Modifying behavior (e.g., default project)
- Troubleshooting configuration issues
- Setting up new environments

**Common config keys:**
- `default_project` - Default project for new tasks
- `editor` - Preferred editor for `tpg edit`

## Common Workflows

### Post-Crash Recovery

```bash
# 1. Check for corruption
tpg doctor --dry-run

# 2. Create safety backup
tpg backup pre-repair

# 3. Fix issues
tpg doctor

# 4. Verify fix
tpg status
```

### Pre-Migration Safety

```bash
# 1. Create backup
tpg backup pre-migration-v7

# 2. Check integrity
tpg doctor

# 3. Clean old data
tpg clean --all --days 90 --dry-run  # Preview first
tpg clean --all --days 90            # Then execute

# 4. Verify
tpg status
```

### Database Maintenance Schedule

**Weekly:**
```bash
tpg doctor --dry-run    # Quick health check
```

**Monthly:**
```bash
tpg backup monthly-$(date +%Y%m)
tpg doctor
tpg clean --vacuum
tpg clean --done --days 60
tpg status
```

## Warning Signs That Need Attention

- `tpg` commands fail with database errors
- Tasks appear missing or corrupted
- `tpg doctor` reports issues
- Database file is growing rapidly
- Performance degradation on operations
- Unexpected behavior after crashes

## Emergency Recovery

If the database is severely corrupted:

```bash
# 1. Stop all tpg operations
# 2. Copy the corrupted database for analysis
cp .tpg/tpg.db .tpg/tpg-corrupted-$(date +%Y%m%d).db

# 3. List available backups
tpg backups

# 4. Restore from most recent good backup
tpg restore .tpg/backups/<most-recent-backup>

# 5. If no backups exist, check for auto-backups in .tpg/backups/
ls -la .tpg/backups/

# 6. After restore, run doctor to verify
tpg doctor
```

## Remember

- **Backup before repair** - Always protect current state
- **Use --dry-run first** - Preview destructive operations
- **Regular maintenance** - Prevents issues before they become critical
- **Monitor backup age** - Ensure you have recent recovery points
- **Test restores periodically** - Verify backups are valid
