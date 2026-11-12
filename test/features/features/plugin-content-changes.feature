Feature: Plugins hear when a saved write changes what visitors see
  A plugin that keeps its own data about content, such as event seats, a
  search index or a page cache, needs to hear when content changes, the
  way WordPress plugins use save_post and transition_post_status. A plugin
  that lists the hook content.changed hears each saved write that changes
  what visitors see: an item published, or edited while visitors see it,
  one set back so visitors no longer see it, one moved to the trash and
  one deleted for good. Writes visitors never see call no plugin. The hook
  runs after the write is saved, under its own time budget, so a plugin
  that fails or panics never undoes or blocks the save.

  Background:
    Given the settings point at an empty database
    And the compiled plugins "events" and "notes"
    And "notes" hears content changes
    And an administrator account exists

  @wip
  Scenario: Each change visitors see of a post reaches the plugin in order
    When the site serves
    And the administrator signs in
    And the administrator writes the draft "Hello"
    And the administrator publishes "Hello"
    And the administrator renames "Hello" to "Hello again"
    And the administrator sets "Hello again" back to draft
    And the administrator publishes "Hello again"
    And the administrator moves "Hello again" to the trash
    Then "notes" heard these changes in order:
      | change      | title       |
      | published   | Hello       |
      | published   | Hello again |
      | unpublished | Hello again |
      | published   | Hello again |
      | trashed     | Hello again |

  @wip
  Scenario: Deleting a published post for good tells the plugin it was deleted
    When the site serves
    And the administrator signs in
    And the administrator writes the draft "Hello"
    And the administrator publishes "Hello"
    And the administrator deletes "Hello" for good
    Then "notes" heard these changes in order:
      | change    | title |
      | published | Hello |
      | deleted   | Hello |

  @wip
  Scenario: Writes to a draft tell the plugin nothing
    When the site serves
    And the administrator signs in
    And the administrator writes the draft "Hello"
    And the administrator renames "Hello" to "Hello again"
    And the administrator moves "Hello again" to the trash
    And the administrator restores "Hello again"
    And the administrator moves "Hello again" to the trash
    And the administrator empties the trash of posts
    Then "notes" heard no change

  @wip
  Scenario: A plugin that fails when it hears a change leaves the save done
    Given the content changes "notes" hears fail with "the index is offline"
    When the site serves
    And the administrator signs in
    And the administrator writes the draft "Hello"
    And the administrator publishes "Hello"
    Then the post "Hello" is published
    And the site logged that the hook "content.changed" of "notes" failed with "the index is offline"

  @wip
  Scenario: A plugin that panics when it hears a change leaves the save done
    Given the content changes "notes" hears panic with "boom"
    When the site serves
    And the administrator signs in
    And the administrator writes the draft "Hello"
    And the administrator publishes "Hello"
    Then the post "Hello" is published
    And the site logged that the hook "content.changed" of "notes" panicked with "boom", with its stack
