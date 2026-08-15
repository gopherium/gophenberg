Feature: Relating content
  One field kind points at another type. The rows carry the term page, so
  what an item points at must stay true as the item is published or trashed.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the type "category" labeled "Category" and "Categories" under "categories"
    And the "relation" field "categories" on "post" targeting "category" holding many

  Scenario: Filing a post under a category
    Given the category "News"
    And the post "Hello world"
    When the administrator files "Hello world" under "News"
    Then "Hello world" lists "News" in "categories"

  Scenario: A single target field refuses a second
    Given the "relation" field "series" on "post" targeting "category" holding one
    And the categories "News" and "Guides"
    And the post "Hello world"
    When the administrator files "Hello world" in "series" under "News" and "Guides"
    Then the request is refused because the field holds one target

  Scenario: A single target is replaced, not appended
    Given the "relation" field "series" on "post" targeting "category" holding one
    And the categories "News" and "Guides"
    And the post "Hello world" filed in "series" under "News"
    When the administrator files "Hello world" in "series" under "Guides"
    Then "Hello world" lists "Guides" in "series"

  Scenario: A target of the wrong type is refused
    Given the post "Hello world"
    And the post "Second post"
    When the administrator files "Hello world" under "Second post"
    Then the request is refused because the target is the wrong type

  Scenario: A target that does not exist is refused
    Given the post "Hello world"
    When the administrator files "Hello world" under a category that was never stored
    Then the request is refused as unfindable

  Scenario: The author's order is kept
    Given the categories "Guides" and "News"
    And the post "Hello world"
    When the administrator files "Hello world" under "Guides" and "News"
    Then "Hello world" lists "Guides" then "News" in "categories"

  Scenario: Clearing a relation unfiles the post
    Given the category "News"
    And the post "Hello world" filed under "News"
    When the administrator clears "categories" of "Hello world"
    Then "Hello world" lists nothing in "categories"

  Scenario: Permanently deleting a category unfiles its posts
    Given the category "News"
    And the post "Hello world" filed under "News"
    When the administrator permanently deletes the category "News"
    Then "Hello world" lists nothing in "categories"

  Scenario: Deleting the field unfiles every post
    Given the category "News"
    And the post "Hello world" filed under "News"
    When the administrator deletes the field "categories" on "post"
    Then "Hello world" lists no field "categories"

  Scenario: Registering a cars site needs no new machinery
    Given the type "car" labeled "Car" and "Cars" under "cars"
    And the type "engine-type" labeled "Engine Type" and "Engine Types" under "engine-types"
    And the required "relation" field "engine" on "car" targeting "engine-type" holding one
    And the engine-type "V8"
    When the administrator files the car "Ford Focus" under "V8"
    Then the car "Ford Focus" lists "V8" in "engine"
    And publishing a car holding no engine is refused
