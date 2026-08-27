Feature: Grouping fields and placing them by rule
  Fields live in groups, and a group's location rules decide which
  content types it appears on. Two groups may hold the same field key
  only while they never meet on one type, so a group that is edited
  into another's company is refused before it can shadow a value.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator

  Scenario: A site starts with no field groups
    When the administrator lists the field groups
    Then no field groups are listed

  Scenario: Creating a group that names one content type
    When the administrator creates the group "Article details" for "post"
    Then the group "Article details" is listed
    And the group "Article details" appears on "post"

  Scenario: A group needs a title
    When the administrator creates the group " " for "post"
    Then the request is refused with the code "group_title_required"

  Scenario: A rule naming a source nothing declares is refused
    When the administrator creates the group "Extras" reading the source "vanished"
    Then the request is refused with the code "rule_source_unknown"

  Scenario: The any rule places a group on every content type
    Given the type "recipe" labeled "Recipe" and "Recipes" under "recipes"
    When the administrator creates the group "Everywhere" for any content type
    Then the group "Everywhere" appears on "post"
    And the group "Everywhere" appears on "recipe"

  Scenario: Excluding every content type is refused
    When the administrator creates the group "Nowhere" excluding any content type
    Then the request is refused with the code "rule_any_negated"

  Scenario: Renaming a group
    Given the group "Article details" for "post"
    When the administrator renames the group "Article details" to "Article extras"
    Then the group "Article extras" is listed

  Scenario: A resting group serves its fields nowhere
    Given the group "Article details" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Article details"
    When the administrator rests the group "Article details"
    Then the field "subtitle" is not served on "post"

  Scenario: Declaring a field inside a group
    Given the group "Article details" for "post"
    When the administrator adds the "text" field "subtitle" labeled "Subtitle" to "Article details"
    Then the field "subtitle" is served on "post"

  Scenario: A key another group already places on the type is refused
    Given the group "Article details" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Article details"
    And the group "Extras" for "post"
    When the administrator adds the "text" field "subtitle" labeled "Subtitle" to "Extras"
    Then the request is refused with the code "field_taken"

  Scenario: The same key stands on groups that never meet
    Given the type "recipe" labeled "Recipe" and "Recipes" under "recipes"
    And the group "Article details" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Article details"
    And the group "Recipe details" for "recipe"
    When the administrator adds the "text" field "subtitle" labeled "Subtitle" to "Recipe details"
    Then the field "subtitle" is served on "post"
    And the field "subtitle" is served on "recipe"

  Scenario: Waking a group into a collision is refused
    Given the group "Article details" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Article details"
    And the resting group "Extras" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Extras"
    When the administrator wakes the group "Extras"
    Then the request is refused with the code "field_taken"

  Scenario: Moving a field to another group
    Given the group "Article details" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Article details"
    And the group "Extras" for "post"
    When the administrator moves the field "subtitle" from "Article details" to "Extras"
    Then the field "subtitle" is served on "post"

  Scenario: Deleting a group takes its fields with it
    Given the group "Article details" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Article details"
    When the administrator deletes the group "Article details"
    Then no field groups are listed
    And the field "subtitle" is not served on "post"

  Scenario: The order the groups are read in decides which field comes first
    Given the group "Article details" for "post"
    And the "text" field "subtitle" labeled "Subtitle" in "Article details"
    And the group "Extras" for "post"
    And the "text" field "footnote" labeled "Footnote" in "Extras"
    When the administrator orders the groups "Extras" then "Article details"
    Then "post" serves the fields "footnote" then "subtitle"

  Scenario: An order leaving a group out is refused
    Given the group "Article details" for "post"
    And the group "Extras" for "post"
    When the administrator orders the groups "Extras"
    Then the request is refused with the code "group_order_incomplete"

  Scenario: The rule sources a location may read are offered
    When the administrator lists the rule sources
    Then the source "content_type" is offered with a choice for "post"
