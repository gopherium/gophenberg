Feature: A role decides which routes an account reaches
  A site owner hands out accounts that write without handing over the
  keys to the site. The administration routes answer only an admin,
  while the content routes stay open to every role that can sign in.

  Background:
    Given a running Gophenberg with an empty managed themes directory

  Scenario: An author is refused the account list
    Given a signed in author
    When the account asks for the user list
    Then the request is refused with the code "role_insufficient"

  Scenario: An editor is refused the account list
    Given a signed in editor
    When the account asks for the user list
    Then the request is refused with the code "role_insufficient"

  Scenario: An author is refused the themes
    Given a signed in author
    When the account asks for the theme list
    Then the request is refused with the code "role_insufficient"

  Scenario: An editor is refused the themes
    Given a signed in editor
    When the account asks for the theme list
    Then the request is refused with the code "role_insufficient"

  Scenario: An administrator reaches the account list
    Given a signed in administrator
    When the account asks for the user list
    Then the request is answered

  Scenario: An author reaches the content list
    Given a signed in author
    When the account asks for the content list
    Then the request is answered

  Scenario: An editor reaches the content list
    Given a signed in editor
    When the account asks for the content list
    Then the request is answered

  Scenario: An author reaches the type registry it needs to read a screen
    Given a signed in author
    When the account asks for the content types
    Then the request is answered
