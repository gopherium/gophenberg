Feature: A SQLite site keeps backups of its file
  A site on SQLite keeps its whole database in one file. Before a new
  version of Gophenberg migrates that file, it copies the file into the
  backups folder, and database:backup copies it on demand. The folder
  sits in GOPHENBERG_DATA_DIR or GOPHENBERG_INSTALL_ROOT. When neither
  is set, it sits beside the file and is named for it, such as
  site.db.backups beside site.db, so each file keeps its own backups.
  Before it copies, each backup checks that the backups folder has free
  space for the copy and GOPHENBERG_UPDATE_DISK_MARGIN_MB, and with less
  it stops and copies nothing. A backups folder that is already there
  keeps its mode, and each backup in it is readable and writable by its
  owner alone.
  GOPHENBERG_SQLITE_BACKUPS_KEEP keeps the newest update and scheduled
  backups together, and a backup taken on demand stays until the owner
  deletes it. A copy on the server is a convenience, not a backup, so
  the owner moves each copy off the server.

  Scenario: A new version backs up the file before it migrates it
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    When the operator runs "migrate"
    Then the command succeeds
    And the backups folder holds 1 update backup
    And the update backup records Gophenberg "0.0.1" as the version that last migrated it
    And the file records this binary as the version that last migrated it

  Scenario: A command that migrates first backs up the file too
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    When the operator creates the administrator "admin@example.com" with a password on standard input
    Then the command succeeds
    And the backups folder holds 1 update backup

  Scenario: A second run of one version takes no more backup
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    And the operator ran "migrate"
    When the operator runs "migrate"
    Then the command succeeds
    And the backups folder holds 1 update backup

  Scenario: A new empty file takes no backup
    Given the settings point at an empty SQLite file
    When the operator runs "migrate"
    Then the command succeeds
    And the backups folder holds no backup

  Scenario: A file a newer Gophenberg migrated stops before any backup
    Given the settings point at an empty SQLite file
    And a Gophenberg migrated the database and left the minimum version "99.0.0"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    When the operator runs "migrate"
    Then the command exits with code 1
    And the error says "this database needs Gophenberg 99.0.0 or newer"
    And the backups folder holds no backup

  Scenario: One count keeps the newest update and scheduled backups together
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    And GOPHENBERG_SQLITE_BACKUPS_KEEP is "2"
    And a backup named "0.0.1-20200101T031500Z-scheduled.sqlite" waits in the backups folder
    And a backup named "0.0.1-20200102T031500Z-scheduled.sqlite" waits in the backups folder
    And a backup named "0.0.1-20200103T101500Z-manual.sqlite" waits in the backups folder
    When the operator runs "migrate"
    Then the command succeeds
    And the backups folder holds 1 update backup
    And the backups folder still holds "0.0.1-20200102T031500Z-scheduled.sqlite"
    And the backups folder still holds "0.0.1-20200103T101500Z-manual.sqlite"
    And the backups folder no longer holds "0.0.1-20200101T031500Z-scheduled.sqlite"

  Scenario: The skip setting lets its version migrate with no backup
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    And GOPHENBERG_SQLITE_SKIP_BACKUP_FOR names the version of this binary
    When the operator runs "migrate"
    Then the command succeeds
    And the backups folder holds no backup
    And the file records this binary as the version that last migrated it

  Scenario: The skip setting never skips a rising minimum version
    Given the settings point at an empty SQLite file
    And a Gophenberg migrated the database and left the minimum version "0.0.1"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    And GOPHENBERG_SQLITE_SKIP_BACKUP_FOR names the version of this binary
    When the operator runs "migrate"
    Then the command succeeds
    And the backups folder holds 1 update backup

  Scenario: database:backup copies a SQLite file on demand
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    When the operator runs "database:backup"
    Then the command succeeds
    And the backups folder holds 1 manual backup
    And the answer names where it put the manual backup

  Scenario: A backup on demand deletes no other backup
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And GOPHENBERG_SQLITE_BACKUPS_KEEP is "1"
    And a backup named "0.0.1-20200101T031500Z-update.sqlite" waits in the backups folder
    And a backup named "0.0.1-20200102T031500Z-scheduled.sqlite" waits in the backups folder
    When the operator runs "database:backup"
    Then the command succeeds
    And the backups folder holds 1 manual backup
    And the backups folder still holds "0.0.1-20200101T031500Z-update.sqlite"
    And the backups folder still holds "0.0.1-20200102T031500Z-scheduled.sqlite"

  Scenario: database:backup stops when the backups folder lacks free space for the copy and the margin
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And GOPHENBERG_UPDATE_DISK_MARGIN_MB is "1073741824"
    When the operator runs "database:backup"
    Then the command exits with code 1
    And the error says "MB free"
    And the error says "GOPHENBERG_UPDATE_DISK_MARGIN_MB"
    And the error says "free some disk space"
    And the backups folder holds no backup

  Scenario: A new version stops before it migrates when the backups folder lacks free space
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And the file records Gophenberg "0.0.1" as the version that last migrated it
    And GOPHENBERG_UPDATE_DISK_MARGIN_MB is "1073741824"
    When the operator runs "migrate"
    Then the command exits with code 1
    And the error says "take the backup before migrating"
    And the error says "GOPHENBERG_UPDATE_DISK_MARGIN_MB"
    And the backups folder holds no backup
    And the file still records Gophenberg "0.0.1" as the version that last migrated it

  Scenario Outline: A margin past its bound stops a command that backs up the file
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And GOPHENBERG_UPDATE_DISK_MARGIN_MB is "1073741825"
    When the operator runs "<command>"
    Then the command exits with code 1
    And the error says "GOPHENBERG_UPDATE_DISK_MARGIN_MB: must stand at or below 1073741824"
    And the backups folder holds no backup

    Examples:
      | command         |
      | migrate         |
      | database:backup |

  Scenario: database:backup leaves a backups folder that is already there as it is
    Given the settings point at an empty SQLite file
    And the operator ran "migrate"
    And the SQLite file is set to the mode "0644"
    And the backups folder is set to the mode "0755"
    When the operator runs "database:backup"
    Then the command succeeds
    And the backups folder has the mode "0755"
    And the manual backup has the mode "0600"

  Scenario: database:backup on PostgreSQL names pg_dump
    Given the settings point at an empty PostgreSQL database
    When the operator runs "database:backup"
    Then the command exits with code 1
    And the error says "pg_dump"
    And the error says "#what-to-back-up"

  Scenario Outline: Check names a bad backup setting
    Given the settings point at an empty SQLite file
    And <setting> is "<value>"
    When the operator runs "check"
    Then the command exits with code 1
    And the error says "<setting>"
    And the error says "<error>"

    Examples:
      | setting                             | value | error                       |
      | GOPHENBERG_SQLITE_BACKUP_INTERVAL   | 10m   | must be 0 or at least 1h    |
      | GOPHENBERG_SQLITE_BACKUP_INTERVAL   | 745h  | must stand at or below 744h |
      | GOPHENBERG_SQLITE_BACKUPS_KEEP      | 0     | must stand above zero       |
      | GOPHENBERG_SQLITE_BACKUPS_KEEP      | 101   | must stand at or below 100  |
      | GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT | 25h   | must stand at or below 24h  |
      | GOPHENBERG_SQLITE_SKIP_BACKUP_FOR   | 0.21  | must name a version         |

  Scenario: update:status lists the SQLite backups beside the dumps
    Given the settings point at an empty SQLite file
    And the settings name an install root
    And a backup named "0.19.0-20200101T091500Z.dump" waits in the backups folder
    And a backup named "0.19.0-20200102T091500Z-update.sqlite" waits in the backups folder
    And a backup named "0.19.0-20200103T091500Z-scheduled.sqlite" waits in the backups folder
    And a backup named "0.19.0-20200104T091500Z-manual.sqlite" waits in the backups folder
    And a file named "0.19.0-20200105T091500Z-manual.sqlite.dbkit-snapshot.partial" waits in the backups folder
    And a file named ".dbkit-snapshot.lock" waits in the backups folder
    When the operator runs "update:status"
    Then the command succeeds
    And the backups kept are "0.19.0-20200104T091500Z-manual.sqlite", "0.19.0-20200103T091500Z-scheduled.sqlite", "0.19.0-20200102T091500Z-update.sqlite" and "0.19.0-20200101T091500Z.dump", the newest first
