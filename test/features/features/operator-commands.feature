Feature: Operators run Gophenberg from one command line
  An operator runs every task through the gophenberg command. A run with
  no command lists what the program offers, a change is only a preview
  until the operator confirms it, and a change to an account names the
  administrator who makes it and is kept on record.

  Background:
    Given the settings point at an empty database

  Scenario: A run with no command lists every command
    When the operator runs gophenberg with no command
    Then the command succeeds
    And the answer lists the commands "serve", "migrate", "seed" and "account:create-admin"
    And the database holds no schema

  Scenario Outline: A command name from before the command line is refused
    When the operator runs "<name>"
    Then the command exits with code 2
    And the answer names the unknown command "<name>"

    Examples:
      | name        |
      | createadmin |
      | grantrole   |

  Scenario: A help page answers without a database
    Given the settings name no database
    When the operator asks for the help page of "account:role"
    Then the command succeeds
    And the answer describes "account:role"

  Scenario: About tells what this binary is without a database
    Given the settings name no database
    When the operator runs "about -json"
    Then the command succeeds
    And the answer names the version, the build stamp, the minimum version and the theme kits of this binary

  Scenario: About names the lowest Node.js major
    Given the settings name no database
    When the operator runs "about -json"
    Then the command succeeds
    And the answer names Node.js 24 as the lowest major a theme runs on

  Scenario: Migrating stores the minimum version of this binary
    When the operator runs "migrate"
    Then the command succeeds
    And the database holds the minimum version of this binary

  Scenario: Migrating raises an older minimum version to the one of this binary
    Given a Gophenberg migrated the database and left the minimum version "0.0.1"
    When the operator runs "migrate"
    Then the command succeeds
    And the database holds the minimum version of this binary

  Scenario: A database a newer Gophenberg migrated stops the migration
    Given a Gophenberg migrated the database and left the minimum version "99.0.0"
    When the operator runs "migrate"
    Then the command exits with code 1
    And the error says "this database needs Gophenberg 99.0.0 or newer"
    And the database still holds the minimum version "99.0.0"

  Scenario: A check finds a newer version in the signed index
    Given an update site whose stable index offers version "99.0.0"
    When the operator runs "update:check -json"
    Then the command succeeds
    And the answer offers version "99.0.0"

  Scenario: A check finds nothing newer than this binary
    Given an update site whose stable index offers version "0.0.1"
    When the operator runs "update:check -json"
    Then the command succeeds
    And the answer offers no version

  Scenario Outline: A key list signed by <root> alone is taken
    Given an update site whose stable index offers version "99.0.0"
    And only <root> signed the key list
    When the operator runs "update:check -json"
    Then the command succeeds
    And the answer offers version "99.0.0"

    Examples:
      | root            |
      | the first root  |
      | the second root |

  Scenario Outline: A check refuses <case>
    Given an update site whose stable index offers version "99.0.0"
    And <change>
    When the operator runs "update:check"
    Then the command exits with code 1
    And the error says "<error>"

    Examples:
      | case                                          | change                                                   | error                                                                          |
      | a key list no trusted root signed             | only a stranger signed the key list                      | no trusted root signed it                                                      |
      | an index no listed key signed                 | only a stranger signed the index                         | no release or standby key the list names                                       |
      | an index that expired                         | the index expired yesterday                              | which expired                                                                  |
      | an index for another channel                  | the index names the channel "beta"                       | the site asks for                                                              |
      | a key list with no timestamp key              | the key list names no timestamp key                      | which names no timestamp key                                                   |
      | a key list naming one key under two roles     | the key list names the release key as its timestamp key  | names one key as its release key and its timestamp key                         |
      | a timestamp file that expired                 | the timestamp file expired yesterday                     | refused the timestamp file, which expired at                                   |
      | a missing timestamp file                      | the update site serves no timestamp file                 | timestamp.json: the site answered 404                                          |
      | a timestamp file naming a newer index         | the timestamp file names a newer index                   | refused the index of serial 2, older than serial 3 the timestamp file names    |
      | a timestamp file naming a newer key list      | the timestamp file names a newer key list                | refused the key list of serial 1, older than serial 2 the timestamp file names |
      | a timestamp file the release key signed       | only the release key signed the timestamp file           | no timestamp key the list names signed it                                      |
      | an index the timestamp key signed             | only the timestamp key signed the index                  | no release or standby key the list names                                       |
      | a timestamp file with no line for the channel | the timestamp file names no index of the channel         | which names no index of channel                                                |

  Scenario Outline: A check takes <case>
    Given an update site whose stable index offers version "99.0.0"
    And <change>
    When the operator runs "update:check -json"
    Then the command succeeds
    And the answer offers version "99.0.0"

    Examples:
      | case                                                          | change                                                  |
      | an index older than 30 days that a fresh timestamp file names | the index was signed 40 days ago and expires in 50 days |
      | an index newer than the one the timestamp file names          | the timestamp file names an older index                 |

  Scenario: A check refuses an index older than the one it took before
    Given an update site whose stable index offers version "99.0.0"
    And the operator ran "update:check"
    And the update site went back to an older index
    When the operator runs "update:check"
    Then the command exits with code 1
    And the error says "older than serial"

  Scenario: A database a newer Gophenberg migrated stops an account command
    Given a Gophenberg migrated the database and left the minimum version "99.0.0"
    When the operator creates the administrator "admin@example.com" with a password on standard input
    Then the command exits with code 1
    And the error says "restore the backup taken before the update to run an older version"
    And the database still holds the minimum version "99.0.0"

  Scenario: Seeding only previews until it is confirmed
    When the operator runs "seed"
    Then the command succeeds
    And the answer says nothing changed until it is confirmed
    And the database holds no demo content

  Scenario: The first administrator is created from the command line
    When the operator creates the administrator "admin@example.com" with a password on standard input
    Then the command succeeds
    And the account "admin@example.com" holds the role "admin"

  Scenario Outline: An account change is refused when <case>
    Given the administrator "admin@example.com"
    And the author "author@example.com"
    And the editor "editor@example.com"
    When the operator gives "author@example.com" the role "editor" acting as "<actor>"
    Then the command exits with code <code>
    And the error says "<error>"
    And the account "author@example.com" still holds the role "author"
    And no account change is on record

    Examples:
      | case                              | actor              | code | error                                    |
      | it names no acting account        |                    | 2    | account:role wants -as <email>           |
      | the acting account is an editor   | editor@example.com | 1    | which lacks manage_users                 |
      | no account answers to the address | nobody@example.com | 1    | no account answers to nobody@example.com |

  Scenario: An applied account change is kept on record
    Given the administrator "admin@example.com"
    And the author "author@example.com"
    When the operator gives "author@example.com" the role "editor" acting as "admin@example.com"
    Then the command succeeds
    And the account "author@example.com" holds the role "editor"
    And the records list "account:role" applied by "admin@example.com"

  Scenario: A preview of an account change records nothing
    Given the administrator "admin@example.com"
    And the author "author@example.com"
    When the operator previews giving "author@example.com" the role "editor" acting as "admin@example.com"
    Then the command succeeds
    And the answer says nothing changed until it is confirmed
    And the account "author@example.com" still holds the role "author"
    And no account change is on record

  Scenario: A blank acting account is refused like a missing one
    Given the administrator "admin@example.com"
    And the author "author@example.com"
    When the operator gives "author@example.com" the role "editor" with a blank -as
    Then the command exits with code 2
    And the error says "account:role wants -as <email>"
    And the account "author@example.com" still holds the role "author"
    And no account change is on record

  Scenario Outline: An acting account cannot <case>
    Given the administrator "admin@example.com"
    And the administrator "maria@example.com"
    When the operator runs "<line> -as admin@example.com"
    Then the command exits with code 1
    And the error says "the account admin@example.com cannot <error>"
    And the account "admin@example.com" still holds the role "admin"
    And the account "admin@example.com" is still enabled
    And no account change is on record

    Examples:
      | case                             | line                                       | error               |
      | change its own role              | account:role admin@example.com editor -yes | change its own role |
      | preview a change of its own role | account:role admin@example.com editor      | change its own role |
      | disable itself                   | account:disable admin@example.com -yes     | disable itself      |
      | preview disabling itself         | account:disable admin@example.com          | disable itself      |

  Scenario Outline: An account command refuses a missing flag without a database
    Given the settings name no database
    When the operator runs "<line>"
    Then the command exits with code 2
    And the error says "<error>"

    Examples:
      | line                                                       | error                                   |
      | account:create-admin -email admin@example.com -name Holder | account:create-admin wants -role <role> |
      | account:grant-role -as admin@example.com -yes              | account:grant-role wants -role <role>   |

  Scenario: A plugin command refuses a needed flag left out
    Given a plugin offering "notes:restore", which needs -since
    When the operator runs "notes:restore"
    Then the command exits with code 2
    And the error says "notes:restore wants -since <day>"
    And the plugin command never ran

  Scenario: A plugin command refuses a needed flag holding only spaces
    Given a plugin offering "notes:restore", which needs -since
    When the operator runs "notes:restore" with -since holding only spaces
    Then the command exits with code 2
    And the plugin command never ran

  Scenario: Check names a plugin command needing a flag it does not declare
    Given a plugin offering "notes:restore", which needs -since but declares no flags
    When the operator runs "check"
    Then the command exits with code 1
    And the error says "needs -since, which it does not declare"

  @wip
  Scenario: Check names the variable of a plugin value its type rejects
    Given the plugin "notes" declares the count setting "per_page" with the default "10"
    And GOPHENBERG_PLUGIN_NOTES__PER_PAGE is "banana"
    When the operator runs "check"
    Then the command exits with code 1
    And the error says "GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must be a whole number"

  @wip
  Scenario: List shows a plugin whose value its type rejects under Not loaded
    Given the plugin "notes" declares the count setting "per_page" with the default "10"
    And GOPHENBERG_PLUGIN_NOTES__PER_PAGE is "banana"
    When the operator runs "list"
    Then the command succeeds
    And the answer lists "notes" under "Not loaded", naming "GOPHENBERG_PLUGIN_NOTES__PER_PAGE"

  Scenario: An account becomes a member that never enters the admin
    Given the administrator "admin@example.com"
    And the author "author@example.com"
    When the operator gives "author@example.com" the role "member" acting as "admin@example.com"
    Then the command succeeds
    And the account "author@example.com" holds the role "member"

  Scenario: An account takes a role a plugin declares
    Given a plugin declaring the role "archivist" with the capability "notes.manage"
    And the administrator "admin@example.com"
    And the author "author@example.com"
    When the operator gives "author@example.com" the role "archivist" acting as "admin@example.com"
    Then the command succeeds
    And the account "author@example.com" holds the role "archivist"

  Scenario: An administrator runs a plugin command needing a capability its plugin declares
    Given a plugin declaring the role "archivist" with the capability "notes.manage"
    And a plugin offering "notes:purge", which asks for "notes.manage"
    And the administrator "admin@example.com"
    When the operator runs "notes:purge -as admin@example.com -yes"
    Then the command succeeds
    And the records list "notes:purge" applied by "admin@example.com"

  Scenario: An editor lacks a capability a plugin declares for another role
    Given a plugin declaring the role "archivist" with the capability "notes.manage"
    And a plugin offering "notes:purge", which asks for "notes.manage"
    And the editor "editor@example.com"
    When the operator runs "notes:purge -as editor@example.com -yes"
    Then the command exits with code 1
    And the error says "which lacks notes.manage"
    And the plugin command never ran

  Scenario: A role carrying only the account capability cannot make an administrator
    Given a plugin declaring the role "team_lead" with the capability "manage_users"
    And the administrator "admin@example.com"
    And the author "lead@example.com"
    And the author "author@example.com"
    And the administrator "admin@example.com" gave "lead@example.com" the role "team_lead"
    When the operator gives "author@example.com" the role "admin" acting as "lead@example.com"
    Then the command exits with code 1
    And the error says "the role admin carries access_admin, which the account lead@example.com lacks"
    And the account "author@example.com" still holds the role "author"

  Scenario: A plugin declaring a capability outside its prefix stops the account commands
    Given a plugin declaring the role "archivist" with the capability "manage_archive"
    When the operator creates the administrator "admin@example.com" with a password on standard input
    Then the command exits with code 1
    And the error says "plugin notes"
    And the error says "outside the source's prefix"
