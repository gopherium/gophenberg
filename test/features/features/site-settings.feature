Feature: The settings the site chose for itself
  An administrator writes what the site does once, and every listing
  and every upload that follows obeys it.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator

  Scenario: A listing carries the number of items the site chose
    Given the published post "First"
    And the published post "Second"
    And the published post "Third"
    When the administrator sets the page size to 2
    And a visitor lists the published content
    Then the listing carries 2 items out of 3

  Scenario: A visitor naming a page size outranks the site's choice
    Given the published post "First"
    And the published post "Second"
    And the published post "Third"
    When the administrator sets the page size to 2
    And a visitor lists the published content one item at a time
    Then the listing carries 1 item out of 3

  Scenario: The built in site paginates at the size the site chose
    Given the published post "First"
    And the published post "Second"
    And the published post "Third"
    When the administrator sets the page size to 2
    Then "/" carries 2 summaries
    And "/page/2" carries 1 summary

  Scenario: A listing carries twenty items when the site chose nothing
    When a visitor lists the published content
    Then the listing offers pages of 20

  Scenario Outline: A page size the site cannot use is refused
    When the administrator sets the page size to <size>
    Then the request is refused with the code "per_page_invalid"

    Examples:
      | size |
      | 0    |
      | 101  |

  Scenario Outline: A picture quality the site cannot use is refused
    When the administrator sets the picture quality to <quality>
    Then the request is refused with the code "jpeg_quality_invalid"

    Examples:
      | quality |
      | 0       |
      | 101     |

  Scenario: A lower picture quality stores lighter copies
    When the administrator uploads a 2400 by 1600 pixel JPEG named "harbor.jpg"
    And the administrator sets the picture quality to 30
    And the administrator uploads a 2400 by 1600 pixel JPEG named "harbor.jpg"
    Then the copies of the second picture weigh less
    And both pictures keep the same original
