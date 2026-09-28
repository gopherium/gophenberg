Feature: The trash
  A trashed item waits to be restored or deleted for good. While it waits it
  takes no new words, so nothing is parked on a post nobody is editing.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator

  @wip
  Scenario: A post in the trash takes no autosave
    Given the post "Hello world"
    When the administrator trashes "Hello world"
    And the editor autosaves "Hello world" as "Hello again"
    Then the request is refused with the code "content_trashed"
