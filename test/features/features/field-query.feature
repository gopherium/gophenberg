Feature: Finding content by the values it holds
  A listing narrows to the items holding a value under a field, named as a
  field[key] parameter and read by the kind the field declares. A key the
  type does not declare, a kind no filter reads, a value the kind cannot
  take and a key named twice are all refused. Term pages and archives
  ignore the parameter the way they ignore any other they do not read.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the group "Extras" placed on "post"
    And the "number" field "price" in "Extras" with settings:
      """
      {"listed": true}
      """
    And the "boolean" field "on-sale" in "Extras" with settings:
      """
      {}
      """
    And the "media" field "cover" in "Extras" with settings:
      """
      {}
      """
    And the post "Winter sale"
    And the post "Winter sale" holding:
      """
      {"price": 10, "on-sale": true}
      """
    And the post "Summer sale"
    And the post "Summer sale" holding:
      """
      {"price": 20, "on-sale": false}
      """

  Scenario: A listing narrows to the items holding a number
    When the administrator lists content where "price" is "10"
    Then the listing holds only "Winter sale"

  Scenario: A listing narrows to the items holding a boolean
    When the administrator lists content where "on-sale" is "false"
    Then the listing holds only "Summer sale"

  Scenario: A listing nobody narrows holds every item
    When the administrator lists content where "" is ""
    Then the listing holds 2 items

  Scenario: A listing narrowed to a value nobody holds is empty
    When the administrator lists content where "price" is "99"
    Then the listing holds 0 items

  Scenario: A listing carries the values the type marks for the list
    When the administrator lists content where "price" is "10"
    Then the listed item "Winter sale" carries "10" under "price"

  Scenario: A listing carries no value the type leaves off the list
    When the administrator lists content where "price" is "10"
    Then the listed item "Winter sale" carries nothing under "on-sale"

  Scenario: A key the type does not declare is refused
    When the administrator lists content where "vanished" is "10"
    Then the request is refused with the code "list_parameters_invalid"

  Scenario: A kind no filter reads is refused
    When the administrator lists content where "cover" is "10"
    Then the request is refused with the code "list_parameters_invalid"

  Scenario: A value the kind cannot take is refused
    When the administrator lists content where "price" is "ten"
    Then the request is refused with the code "list_parameters_invalid"

  Scenario: A key named twice is refused
    When the administrator lists content where "price" is "10" and "10" again
    Then the request is refused with the code "list_parameters_invalid"

  Scenario: A reader narrows the published items by a value
    Given the administrator publishes "Winter sale"
    And the administrator publishes "Summer sale"
    When a visitor lists published "post" where "price" is "10"
    Then the listing holds only "Winter sale"

  Scenario: A reader naming a key the type lacks is refused
    When a visitor lists published "post" where "vanished" is "10"
    Then the request is refused with the code "list_parameters_invalid"
