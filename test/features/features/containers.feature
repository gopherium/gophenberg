Feature: Container fields
  A Section bundles sub fields under one key. A Repeater holds rows of
  them. Both nest as deep as an operator builds them, values sit nested
  in the item's own document, and every rule a field carries at the top
  holds just as well inside a row.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the group "Extras" placed on "post"

  Scenario: A section serves its sub fields inside it
    Given the "section" field "author" in "Extras"
    When the administrator declares the "text" field "name" inside "author"
    Then the field "author" on "post" holds the sub field "name"

  Scenario: Two sections each hold a sub field of the same name
    Given the "section" field "author" in "Extras"
    And the "section" field "editor" in "Extras"
    And the "text" field "name" inside "author"
    When the administrator declares the "text" field "name" inside "editor"
    Then the field "editor" on "post" holds the sub field "name"

  Scenario: A sub field the section does not declare is refused
    Given the "section" field "author" in "Extras"
    And the "text" field "name" inside "author"
    And the post "Hello world"
    When the administrator saves the section "author" of "Hello world" as:
      """
      {"nickname": "Kip"}
      """
    Then the request is refused

  Scenario: A section keeps the bounds its sub fields carry
    Given the "section" field "author" in "Extras"
    And the "number" field "rating" inside "author" with settings:
      """
      {"min": 1, "max": 10}
      """
    And the post "Hello world"
    When the administrator saves the section "author" of "Hello world" as:
      """
      {"rating": 50}
      """
    Then the request is refused

  Scenario: A repeater stores the rows an author writes
    Given the "repeater" field "team" in "Extras"
    And the "text" field "name" inside "team"
    And the post "Hello world"
    When the administrator saves the rows of "team" of "Hello world" as:
      """
      [{"name": "Maria Perez"}, {"name": "Kip"}]
      """
    Then the post "Hello world" holds 2 rows in "team"

  Scenario: A repeater refuses fewer rows than it asks for
    Given the "repeater" field "team" in "Extras" with settings:
      """
      {"min": 2}
      """
    And the "text" field "name" inside "team"
    And the post "Hello world"
    When the administrator saves the rows of "team" of "Hello world" as:
      """
      [{"name": "Maria Perez"}]
      """
    Then the request is refused

  Scenario: A repeater row holds a section of its own
    Given the "repeater" field "team" in "Extras"
    And the "section" field "contact" inside "team"
    And the "text" field "phone" inside "contact"
    And the post "Hello world"
    When the administrator saves the rows of "team" of "Hello world" as:
      """
      [{"contact": {"phone": "184467235"}}]
      """
    Then the post "Hello world" holds 1 rows in "team"

  Scenario: A relation inside a container is refused
    Given the "section" field "author" in "Extras"
    When the administrator declares the "relation" field "wrote" inside "author"
    Then the request is refused
