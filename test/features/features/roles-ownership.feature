Feature: An author changes only its own work
  An author writes and publishes freely, but the work of other accounts
  is theirs to read and not to change. An editor is held to no such
  limit, since the whole content side is its job.

  Background:
    Given a running Gophenberg with the default content types

  Scenario: An author cannot edit a post another account wrote
    Given a signed in administrator
    And the post "Written by the owner"
    When a signed in author edits "Written by the owner"
    Then the request is refused with the code "not_the_author"

  Scenario: An author cannot trash a post another account wrote
    Given a signed in administrator
    And the post "Written by the owner"
    When a signed in author trashes "Written by the owner"
    Then the request is refused with the code "not_the_author"

  Scenario: An editor edits a post another account wrote
    Given a signed in administrator
    And the post "Written by the owner"
    When a signed in editor edits "Written by the owner"
    Then the request is answered

  Scenario: An author edits the post it wrote itself
    Given a signed in author
    And the post "Written by the author"
    When the account edits "Written by the author"
    Then the request is answered

  Scenario: An author still reads a post another account wrote
    Given a signed in administrator
    And the post "Written by the owner"
    When a signed in author reads "Written by the owner"
    Then the request is answered
