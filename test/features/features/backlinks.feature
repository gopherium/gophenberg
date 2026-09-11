Feature: Linked from
  A Backlinks field lists the items pointing at this one through a relation
  field it names. The list is read at request time and never written, so the
  two sides of a link can never disagree.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the type "category" labeled "Category" and "Categories" under "categories"
    And the "relation" field "categories" on "post" targeting "category" holding many
    And the "backlinks" field "linked-from" on "category" reading "categories" on "post"

  Scenario: A category lists the posts filed under it
    Given the published category "News"
    And the published post "Hello world" filed under "News"
    Then the category "News" is pointed at by "Hello world"

  Scenario: A post nobody published is left out
    Given the published category "News"
    And the post "Hello world" filed under "News"
    Then the category "News" is pointed at by nobody

  Scenario: A category nothing points at lists nobody
    Given the published category "News"
    Then the category "News" is pointed at by nobody

  Scenario: A value sent back for a backlinks field is refused
    Given the published category "News"
    When the administrator saves "linked-from" of "News" as a list of its own
    Then the request is refused with the code "field_shape_value"

  Scenario: A backlinks naming a relation nobody declared is refused
    When the administrator declares a backlinks on "post" reading "nothing" in "loose-ends"
    Then the request is refused with the code "backlinks_source_unknown"

  Scenario: Taking away the relation a backlinks reads is refused
    When the administrator deletes the field "categories" on "post"
    Then the request is refused with the code "field_referenced"

  Scenario: A backlinks is never required
    When the administrator requires "linked-from"
    Then the request is refused with the code "field_never_required"
