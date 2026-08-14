Feature: Serving term pages
  An item whose type serves term pages answers with itself and the published
  content pointing at it, so a category carries a body and its posts at once.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the type "category" labeled "Category" and "Categories" under "categories" serving term pages
    And the "relation" field "categories" on "post" targeting "category" holding many
    And the published category "News"

  Scenario: A term page carries the item and what points at it
    Given the published post "Hello world" filed under "News"
    When a visitor resolves "/categories/news"
    Then the answer is a term of "category"
    And the answer carries the item "News"
    And "Hello world" is among the related content

  Scenario: The archive lists the terms themselves
    When a visitor resolves "/categories"
    Then the answer is an archive of "category"

  Scenario: A draft does not reach the term page
    Given the post "Not yet" filed under "News"
    When a visitor resolves "/categories/news"
    Then "Not yet" is not among the related content

  Scenario: Trashing a post takes it off the term page
    Given the published post "Old news" filed under "News"
    When the administrator trashes "Old news"
    And a visitor resolves "/categories/news"
    Then "Old news" is not among the related content

  Scenario: Deactivating the pointing type empties the term page
    Given the type "guide" labeled "Guide" and "Guides" under "guides"
    And the "relation" field "categories" on "guide" targeting "category" holding many
    And the published guide "Getting started" filed under "News"
    When the administrator deactivates the type "guide"
    And a visitor resolves "/categories/news"
    Then no related content is listed

  Scenario: A post filed twice is listed once
    Given the "relation" field "series" on "post" targeting "category" holding one
    And the published post "Hello world" filed under "News" through both fields
    When a visitor resolves "/categories/news"
    Then "Hello world" is among the related content
    And one item is related

  Scenario: A term page paginates behind the page word
    Given three published posts filed under "News"
    When a visitor resolves "/categories/news/page/2" one item at a time
    Then the answer is a term of "category"
    And it holds "Second filed" alone of 3 related items

  Scenario: A page suffix on a single item answers nothing
    Given the published post "Hello world"
    Then "/hello-world/page/2" answers not found

  Scenario: A term page holds no validator of its own
    Given the published post "Hello world" filed under "News"
    When a visitor resolves "/categories/news"
    Then the answer carries no entity tag
