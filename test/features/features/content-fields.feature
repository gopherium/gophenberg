Feature: Managing content fields
  A type declares typed fields. The definitions drive the editor, the
  write path and the public payloads, so changing them must stay safe
  while values exist.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator

  Scenario: A type starts with no fields
    When the administrator lists the fields of "post"
    Then no fields are listed

  Scenario: Adding a text field
    When the administrator adds the "text" field "color" labeled "Color" to "post"
    Then the field "color" is listed on "post"

  Scenario: A taken field key is refused
    Given the "text" field "color" labeled "Color" on "post"
    When the administrator adds the "number" field "color" labeled "Colour" to "post"
    Then the request is refused

  Scenario: An unknown field kind is refused
    When the administrator adds the "flavor" field "taste" labeled "Taste" to "post"
    Then the request is refused

  Scenario: A relation field needs a target type
    When the administrator adds the "relation" field "engine" labeled "Engine" to "post" without a target
    Then the request is refused

  Scenario: Deleting a type a field targets is refused
    Given the type "category" labeled "Category" and "Categories" under "categories"
    And the "relation" field "categories" on "post" targeting "category"
    When the administrator deletes the type "category"
    Then the request is refused

  Scenario: A stored value survives a relabel
    Given the "text" field "color" labeled "Color" on "post"
    And a published post "Hello world" holding "red" in "color"
    When the administrator relabels the field "color" on "post" as "Paint"
    Then the post "Hello world" holds "red" in "color"

  Scenario: A value of the wrong shape is refused
    Given the "number" field "doors" labeled "Doors" on "post"
    And the post "Hello world"
    When the administrator saves "many" into "doors" of "Hello world"
    Then the request is refused

  Scenario: An unknown field key is refused
    Given the post "Hello world"
    When the administrator saves "red" into "finish" of "Hello world"
    Then the request is refused

  Scenario: A draft lives without a required field
    Given the required "text" field "color" labeled "Color" on "post"
    When the administrator creates the post "Hello world"
    Then the post "Hello world" holds no field "color"

  Scenario: Publishing without a required field is refused
    Given the required "text" field "color" labeled "Color" on "post"
    And the post "Hello world"
    When the administrator publishes "Hello world"
    Then the request is refused

  Scenario: Emptying a required field of a published post is refused
    Given the required "text" field "color" labeled "Color" on "post"
    And a published post "Hello world" holding "red" in "color"
    When the administrator clears "color" of "Hello world"
    Then the request is refused

  Scenario: An autosave carries field values
    Given the "text" field "color" labeled "Color" on "post"
    And the post "Hello world"
    When the editor autosaves "Hello world" holding "red" in "color"
    Then the buffer it saved holds "red" in "color"
    And the post "Hello world" holds "red" in "color"

  Scenario: Deleting a field removes its values
    Given the "text" field "color" labeled "Color" on "post"
    And a published post "Hello world" holding "red" in "color"
    When the administrator deletes the field "color" on "post"
    Then the post "Hello world" holds no field "color"

  Scenario: A restored revision brings its values back
    Given the "text" field "color" labeled "Color" on "post"
    And a published post "Hello world" holding "red" in "color"
    When the administrator saves "blue" into "color" of "Hello world"
    And the administrator restores the previous revision of "Hello world"
    Then the post "Hello world" holds "red" in "color"

  Scenario: A restored revision holds no deleted field
    Given the "text" field "color" labeled "Color" on "post"
    And a published post "Hello world" holding "red" in "color"
    When the administrator saves "blue" into "color" of "Hello world"
    And the administrator deletes the field "color" on "post"
    And the administrator restores the previous revision of "Hello world"
    Then the post "Hello world" holds no field "color"

  Scenario: Fields keep the order they are given
    Given the "text" field "color" labeled "Color" on "post"
    And the "text" field "engine" labeled "Engine" on "post"
    And the "text" field "doors" labeled "Doors" on "post"
    When the administrator reorders the fields of "post" as "doors, color, engine"
    Then the fields of "post" are listed as "doors, color, engine"

  Scenario: A reorder naming a stranger field is refused
    Given the "text" field "color" labeled "Color" on "post"
    When the administrator reorders the fields of "post" as "color, finish"
    Then the request is refused

  Scenario: A reorder leaving a field out is refused
    Given the "text" field "color" labeled "Color" on "post"
    And the "text" field "engine" labeled "Engine" on "post"
    When the administrator reorders the fields of "post" as "color"
    Then the request is refused

  @wip
  Scenario: A field edit naming an unknown attribute is refused
    Given the "text" field "color" labeled "Color" on "post"
    When the administrator edits the field "color" on "post" with the unknown attribute "kind"
    Then the request is refused

  @wip
  Scenario: A field made required later gates the next publish
    Given the "text" field "color" labeled "Color" on "post"
    And the post "Hello world"
    When the administrator marks the field "color" on "post" required
    And the administrator publishes "Hello world"
    Then the request is refused
