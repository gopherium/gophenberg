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

  @wip
  Scenario: A plugin command refuses a needed flag left out
    Given a plugin offering "notes:restore", which needs -since
    When the operator runs "notes:restore"
    Then the command exits with code 2
    And the error says "notes:restore wants -since <day>"
    And the plugin command never ran

  @wip
  Scenario: A plugin command refuses a needed flag holding only spaces
    Given a plugin offering "notes:restore", which needs -since
    When the operator runs "notes:restore" with -since holding only spaces
    Then the command exits with code 2
    And the plugin command never ran

  @wip
  Scenario: Check names a plugin command needing a flag it does not declare
    Given a plugin offering "notes:restore", which needs -since but declares no flags
    When the operator runs "check"
    Then the command exits with code 1
    And the error says "needs -since, which it does not declare"
