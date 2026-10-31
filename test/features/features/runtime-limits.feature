@wip
Feature: The Go runtime takes the site's processor and memory limits
  A small hosting plan caps the memory of a site and how many processes
  and threads it may start. Go reads GOMAXPROCS and GOMEMLIMIT before
  Gophenberg loads .env, so a site names GOPHENBERG_MAX_PROCS and
  GOPHENBERG_MEMORY_LIMIT_MB instead. Every command hands them to Go,
  after .env for the commands that read it, and serve logs the limits
  Go holds. check and serve stop on a value out of range, and every
  other command runs without it.

  Background:
    Given the database refuses every connection
    And serve waits "0" for the database

  Scenario: Limits in .env reach the Go runtime
    Given the site's .env holds the line "GOPHENBERG_MAX_PROCS=1"
    And the site's .env holds the line "GOPHENBERG_MEMORY_LIMIT_MB=256"
    When the site starts
    Then the site logged that Go holds 1 processor and a memory limit of 256 MB

  Scenario: A site that names no limit leaves both to Go
    When the site starts
    Then the site logged no runtime limits

  Scenario Outline: A limit out of range stops check
    Given the site's .env holds the line "<line>"
    When the operator runs "check"
    Then the command exits with code 1
    And the error says "<error>"

    Examples:
      | line                                  | error                                                         |
      | GOPHENBERG_MAX_PROCS=0                | GOPHENBERG_MAX_PROCS: must stand above zero                   |
      | GOPHENBERG_MAX_PROCS=two              | GOPHENBERG_MAX_PROCS: must be a whole number                  |
      | GOPHENBERG_MAX_PROCS=2147483648       | GOPHENBERG_MAX_PROCS: must stand at or below                  |
      | GOPHENBERG_MEMORY_LIMIT_MB=0          | GOPHENBERG_MEMORY_LIMIT_MB: must stand above zero             |
      | GOPHENBERG_MEMORY_LIMIT_MB=1073741825 | GOPHENBERG_MEMORY_LIMIT_MB: must stand at or below 1073741824 |

  Scenario: A limit out of range stops serve
    Given the site's .env holds the line "GOPHENBERG_MEMORY_LIMIT_MB=0"
    When the site starts
    Then the command exits with code 1
    And the error says "GOPHENBERG_MEMORY_LIMIT_MB: must stand above zero"

  Scenario: A command that checks no setting runs past a limit out of range
    Given the site's .env holds the line "GOPHENBERG_MAX_PROCS=0"
    When the operator runs "version"
    Then the command succeeds
