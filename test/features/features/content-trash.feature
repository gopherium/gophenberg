Feature: The trash
  A trashed item waits to be restored or deleted for good. While it waits it
  takes no edits and no new words, and holds no words parked before it left.

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

  Scenario: A post sent to the trash holds no parked words
    Given the published post "Hello world"
    And the editor parked "Hello again" on "Hello world"
    When the administrator trashes "Hello world"
    And the editor opens the autosave of "Hello world"
    Then the request is refused with the code "revision_not_found"
