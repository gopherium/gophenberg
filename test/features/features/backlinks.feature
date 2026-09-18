Feature: Linked from
  A Backlinks field lists the items pointing at this one through a relation
  field it names. The list is read at request time and never written, so the
  two sides of a link can never disagree. An import judges each relation it
  takes away by what the whole file leaves behind.

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

  Scenario: A Linked from field cannot move inside a container
    Given the "section" field "filing" in "category fields"
    When the administrator moves the field "linked-from" inside "filing"
    Then the request is refused with the code "field_backlinks_inside"

  Scenario: A relation a Linked from reads cannot move into a container
    Given the "section" field "filing" in "post fields"
    When the administrator moves the field "categories" inside "filing"
    Then the request is refused with the code "field_referenced"

  Scenario: An import giving up a relation and the backlinks reading it takes both away
    Given the site's file leaves out "categories" in "post fields"
    And the site's file leaves out "linked-from" in "category fields"
    And the administrator confirms the loss of "categories" in "post fields"
    And the administrator confirms the loss of "linked-from" in "category fields"
    When the administrator imports the file
    Then the import is applied
    And the field "categories" is gone from "post"
    And the field "linked-from" is gone from "category"

  Scenario: An import keeping the backlinks of a relation it gives up writes nothing
    Given the site's file retitles "post fields" as "Post extras"
    And the site's file leaves out "categories" in "post fields"
    And the site's file leaves out "linked-from" in "category fields"
    And the administrator confirms the loss of "categories" in "post fields"
    When the administrator imports the file
    Then the request is refused with the code "field_referenced"
    And no group is titled "Post extras"
    And the group "post fields" holds "categories"

  Scenario: An import moving a relation points its backlinks at the new place
    Given the site's file moves "categories" from "post fields" into a new group "Filing"
    And the site's file points "linked-from" in "category fields" at "categories" in "Filing"
    And the administrator confirms the loss of "categories" in "post fields"
    When the administrator imports the file
    Then the import is applied
    And the group "Filing" holds "categories"
    And the field "linked-from" on "category" reads "categories" in "Filing"

  Scenario: An import reshaping a relation its backlinks reads keeps the backlinks
    Given the site's file makes "categories" in "post fields" hold one
    And the administrator confirms the loss of "categories" in "post fields"
    When the administrator imports the file
    Then the import is applied
    And the group "category fields" holds "linked-from"
