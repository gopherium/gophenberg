Feature: Managing content types
  A site defines its own kinds of content. The registry drives what the admin
  serves, so changing it must stay safe while content exists.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator

  Scenario: The install knows the post type
    When the administrator lists the content types
    Then the types are "post"
    And "post" is the default type

  Scenario: Creating a content type
    When the administrator creates the type "car" labeled "Car" and "Cars" under "cars"
    Then the type "car" is listed as active
    And a car named "Ford Focus" can be created

  Scenario: A taken type key is refused
    When the administrator creates the type "post" labeled "Story" and "Stories" under "stories"
    Then the request is refused

  Scenario: A reserved route word is refused
    When the administrator creates the type "car" labeled "Car" and "Cars" under "admin"
    Then the request is refused

  Scenario: A route word in use is refused
    Given the type "car" labeled "Car" and "Cars" under "cars"
    When the administrator creates the type "van" labeled "Van" and "Vans" under "cars"
    Then the request is refused

  Scenario: A second type cannot take the root
    Given the type "page" labeled "Page" and "Pages" under "pages"
    When the administrator sends "page" to the root
    Then the request is refused

  Scenario: Relabeling a type leaves its key and address alone
    When the administrator relabels "post" as "Story" and "Stories"
    Then the type "post" carries the labels "Story" and "Stories"
    And the type "post" answers under ""

  Scenario: The default type cannot be deactivated
    When the administrator deactivates the type "post"
    Then the request is refused

  Scenario: Deactivating a type refuses edits to its content
    Given the type "car" labeled "Car" and "Cars" under "cars"
    And a car named "Ford Focus"
    When the administrator deactivates the type "car"
    Then editing the car "Ford Focus" is refused
    And reactivating the type "car" allows the edit again

  Scenario: Deleting a type that holds content is refused
    Given the type "car" labeled "Car" and "Cars" under "cars"
    And a car named "Ford Focus"
    When the administrator deletes the type "car"
    Then the request is refused

  Scenario: Deleting an empty type removes it
    Given the type "car" labeled "Car" and "Cars" under "cars"
    When the administrator deletes the type "car"
    Then the type "car" is not listed
