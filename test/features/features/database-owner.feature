Feature: A site logs in as its database's own owner role
  A site that logs in to PostgreSQL as a superuser can run programs on the
  database server, so a site logs in as a plain role that owns only its
  own database. The database:create-owner command sets that role up.
  GOPHENBERG_DATABASE_URL names the role and its password, and
  GOPHENBERG_DATABASE_SUPERUSER_URL names the superuser that makes the
  change. Only this command reads the second address. A run is a preview
  until -yes, and it is safe to run again.

  Background:
    Given a database whose tables a superuser owns
    And the settings name a role the server does not have yet
    And the settings name the superuser's address

  Scenario: A preview lists the steps and changes nothing
    When the operator runs "database:create-owner"
    Then the command succeeds
    And the answer says nothing changed until it is confirmed
    And the answer lists the step that creates the role
    And the server has no such role

  Scenario: The role gets the database and everything inside it
    When the operator runs "database:create-owner -yes"
    Then the command succeeds
    And the role owns the database and everything inside it
    And the role logs in with the password the settings name

  Scenario: A site starts on the role it was given
    Given the operator ran "database:create-owner -yes"
    When the site starts and listens
    Then the command succeeds

  Scenario: A second run finds nothing to move and sets a new password
    Given the operator ran "database:create-owner -yes"
    And the settings name a new password for the role
    When the operator runs "database:create-owner -yes"
    Then the command succeeds
    And the answer says there was nothing to move
    And the role logs in with the password the settings name
    And the first password no longer logs in

  Scenario Outline: No run prints a password
    When the operator runs "<line>"
    Then the command succeeds
    And neither answer holds a password

    Examples:
      | line                       |
      | database:create-owner      |
      | database:create-owner -yes |

  Scenario: A run with no superuser address changes nothing
    Given the settings name no superuser address
    When the operator runs "database:create-owner -yes"
    Then the command exits with code 1
    And the error says "GOPHENBERG_DATABASE_SUPERUSER_URL"
    And the server has no such role

  Scenario: A superuser address that is no superuser changes nothing
    Given the superuser's address names a plain role
    When the operator runs "database:create-owner -yes"
    Then the command exits with code 1
    And the error says "GOPHENBERG_DATABASE_SUPERUSER_URL"
    And the error says "takes a superuser connection"
    And the server has no such role

  Scenario: Two addresses that name different databases change nothing
    Given the superuser's address names another database
    When the operator runs "database:create-owner -yes"
    Then the command exits with code 1
    And the error says "name the same database"
    And the server has no such role

  Scenario: Two addresses on different servers change nothing
    Given the superuser's address names another server
    When the operator runs "database:create-owner -yes"
    Then the command exits with code 1
    And the error says "name the same database"
    And the server has no such role

  Scenario: A site role that is a superuser stays as it is
    Given the settings name the superuser as the site's login
    When the operator runs "database:create-owner -yes"
    Then the command exits with code 1
    And the error says "GOPHENBERG_DATABASE_URL"
    And the error says "plain role"

  Scenario: A site role that can create roles stays as it is
    Given the server has the role, which can create roles
    When the operator runs "database:create-owner -yes"
    Then the command exits with code 1
    And the error says "GOPHENBERG_DATABASE_URL"
    And the error says "plain role"

  Scenario: A SQLite site has no roles
    Given the settings point at a SQLite database
    When the operator runs "database:create-owner -yes"
    Then the command exits with code 1
    And the error says "database:create-owner"
    And the error says "PostgreSQL database"

  Scenario: A site that logs in as a superuser stops by default
    Given the settings name the superuser as the site's login
    When the site starts
    Then the command exits with code 1
    And the error says "GOPHENBERG_SUPERUSER_POLICY is refuse"

  Scenario: A site can still choose to log in as a superuser with a warning
    Given the settings name the superuser as the site's login
    And GOPHENBERG_SUPERUSER_POLICY is "warn"
    When the site starts and listens
    Then the command succeeds

  Scenario Outline: A site role that can reach the server stops by default
    Given the server has the role, which <reach>
    And the role can log in
    When the site starts
    Then the command exits with code 1
    And the error says "can reach the database server"
    And the error says "GOPHENBERG_SUPERUSER_POLICY is refuse"

    Examples:
      | reach                       |
      | can create roles            |
      | runs programs on the server |
      | belongs to a superuser      |
