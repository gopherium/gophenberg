Feature: Flexible content
  A Flexible content field holds rows like a Repeater, but each row takes
  one of the layouts the field declares and carries only that layout's
  fields. Two layouts may declare a field of the same name. Each layout
  bounds how many rows may take it, and taking a layout away takes its
  rows with it.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the group "Extras" placed on "post"
    And the "flexible" field "features" in "Extras"

  Scenario: A flexible serves the layouts it declares
    When the administrator declares the "layout" field "hero" inside "features"
    Then the field "features" on "post" holds the sub field "hero"

  Scenario: A layout serves the fields it holds
    Given the "layout" field "hero" inside "features"
    When the administrator declares the "text" field "title" inside "hero"
    Then the layout "hero" holds the field "title"

  Scenario: Two layouts each hold a field of the same name
    Given the "layout" field "hero" inside "features"
    And the "layout" field "quote" inside "features"
    And the "text" field "title" inside "hero"
    When the administrator declares the "text" field "title" inside "quote"
    Then the layout "quote" holds the field "title"
    And the layout "hero" holds the field "title"

  Scenario: A layout standing outside a flexible is refused
    Given the "section" field "author" in "Extras"
    When the administrator declares the "layout" field "hero" inside "author"
    Then the request is refused

  Scenario: A field standing directly under a flexible is refused
    When the administrator declares the "text" field "title" inside "features"
    Then the request is refused

  Scenario: A layout is never required
    Given the "layout" field "hero" inside "features"
    When the administrator requires "hero"
    Then the request is refused with the code "field_never_required"

  Scenario: Rows of different layouts are stored together
    Given the "layout" field "hero" inside "features"
    And the "layout" field "quote" inside "features"
    And the "text" field "title" inside "hero"
    And the "text" field "author" inside "quote"
    And the post "Hello world"
    When the administrator saves the rows of "features" of "Hello world" as:
      """
      [{"hero": {"title": "Welcome"}}, {"quote": {"author": "Maria Perez"}}]
      """
    Then the post "Hello world" holds 2 rows in "features"

  Scenario: A row naming a layout nobody declared is refused
    Given the "layout" field "hero" inside "features"
    And the "text" field "title" inside "hero"
    And the post "Hello world"
    When the administrator saves the rows of "features" of "Hello world" as:
      """
      [{"banner": {"title": "Welcome"}}]
      """
    Then the request is refused with the code "field_unknown"

  Scenario: A row naming two layouts is refused
    Given the "layout" field "hero" inside "features"
    And the "layout" field "quote" inside "features"
    And the post "Hello world"
    When the administrator saves the rows of "features" of "Hello world" as:
      """
      [{"hero": {}, "quote": {}}]
      """
    Then the request is refused with the code "field_layout_several"

  Scenario: A row carrying a field its layout does not declare is refused
    Given the "layout" field "hero" inside "features"
    And the "layout" field "quote" inside "features"
    And the "text" field "title" inside "hero"
    And the post "Hello world"
    When the administrator saves the rows of "features" of "Hello world" as:
      """
      [{"quote": {"title": "Welcome"}}]
      """
    Then the request is refused with the code "field_unknown"

  Scenario: More rows of a layout than it takes are refused
    Given the "layout" field "hero" inside "features" with settings:
      """
      {"max": 1}
      """
    And the "text" field "title" inside "hero"
    And the post "Hello world"
    When the administrator saves the rows of "features" of "Hello world" as:
      """
      [{"hero": {"title": "One"}}, {"hero": {"title": "Two"}}]
      """
    Then the request is refused with the code "field_rows_max"

  Scenario: A layout taken away leaves the flexible without it
    Given the "layout" field "hero" inside "features"
    And the "layout" field "quote" inside "features"
    And the "text" field "title" inside "hero"
    When the administrator deletes the field "hero" inside "features"
    Then the layout "hero" is gone from "features"
