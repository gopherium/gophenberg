Feature: An error names a code
  Machine readable codes let every client translate an error, while
  the English message stays beside them for logs and for a client
  carrying no catalog.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator

  Scenario: A registry error carries its code
    Given the "text" field "color" labeled "Color" on "post"
    When the administrator declares a second field "color" on "post"
    Then the request is refused with the code "field_taken"

  Scenario: The dynamic detail rides beside the code as data
    Given the post "Hello world"
    When the administrator saves a value under the undeclared field "finish"
    Then the request is refused with the code "field_unknown"
    And the error names "finish" under "field"

  Scenario: A relation field given a value names the shape it wanted
    Given the type "category" labeled "Category" and "Categories" under "categories"
    And the "relation" field "categories" on "post" targeting "category" holding many
    And the post "Hello world"
    When the administrator saves a value rather than targets under "categories"
    Then the request is refused with the code "field_shape_list"

  Scenario: A value field given the wrong shape names the kind it wanted
    Given the "text" field "color" labeled "Color" on "post"
    And the post "Hello world"
    When the administrator saves targets rather than a value under "color"
    Then the request is refused with the code "field_shape_kind"
    And the error names "color" under "field"
    And the error names "text" under "kind"

  Scenario: A malformed identity carries the resource it names
    When the administrator asks for the content item "not-an-id"
    Then the request is refused with the code "content_id_malformed"

  Scenario: The message survives beside the code
    When the administrator asks for the content item "not-an-id"
    Then the error still carries a readable message

  Scenario: An error with no dynamic part carries no data
    Given the post "Hello world"
    When the administrator trashes "Hello world" twice
    Then the request is refused with the code "content_already_trashed"
    And the error carries no data
