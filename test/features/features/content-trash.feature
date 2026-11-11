Feature: The trash
  A trashed item waits to be restored or deleted for good. While it waits it
  takes no edits and no new words, and keeps the words parked before it left.
  Words parked after its last save still count as new once it comes back.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator

  Scenario: A post in the trash takes no autosave
    Given the post "Hello world"
    When the administrator trashes "Hello world"
    And the editor autosaves "Hello world" as "Hello again"
    Then the request is refused with the code "content_trashed"

  Scenario: A post in the trash keeps its title
    Given the post "Hello world"
    When the administrator trashes "Hello world"
    And the editor saves "Hello world" as "Hello again"
    Then the request is refused with the code "content_trashed"

  Scenario: A post in the trash is not published
    Given the post "Hello world"
    When the administrator trashes "Hello world"
    And the administrator publishes "Hello world"
    Then the request is refused with the code "content_trashed"

  Scenario: A post in the trash keeps its address
    Given the post "Hello world"
    When the administrator trashes "Hello world"
    And the administrator renames "Hello world" to "hello-world"
    Then the request is refused with the code "content_trashed"

  Scenario: A post in the trash keeps its values
    Given the "text" field "subtitle" labeled "Subtitle" on "post"
    And the post "Hello world"
    When the administrator trashes "Hello world"
    And the administrator saves "Draft words" into "subtitle" of "Hello world"
    Then the request is refused with the code "content_trashed"

  Scenario: A page in the trash keeps its place
    Given the type "page" labeled "Page" and "Pages" under "pages" that nests
    And the page "About"
    And the page "Team"
    When the administrator trashes "Team"
    And the administrator files "Team" under "About"
    Then the request is refused with the code "content_trashed"

  Scenario: A post sent to the trash keeps its parked words
    Given the published post "Hello world"
    And the editor parked "Hello again" on "Hello world"
    When the administrator trashes "Hello world"
    And the editor opens the autosave of "Hello world"
    Then the autosave holds the title "Hello again"

  Scenario: A post back from the trash keeps its parked words
    Given the published post "Hello world"
    And the editor parked "Hello again" on "Hello world"
    When the administrator trashes "Hello world"
    And the administrator restores "Hello world"
    And the editor opens the autosave of "Hello world"
    Then the autosave holds the title "Hello again"

  Scenario: Words parked before a trash are no older than the post once it comes back
    Given the published post "Hello world"
    And the editor parked "Hello again" on "Hello world"
    When the administrator trashes "Hello world"
    And the administrator restores "Hello world"
    Then the autosave of "Hello world" is no older than the post

  Scenario: Words parked before the last save stay older than the post once it comes back
    Given the published post "Hello world"
    And the editor parked "Hello again" on "Hello world"
    When the editor saves "Hello world" as "Hello there"
    And the administrator trashes "Hello world"
    And the administrator restores "Hello world"
    Then the autosave of "Hello world" is older than the post
