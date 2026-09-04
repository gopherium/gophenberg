Feature: Showing a field by rule
  A field carries conditions naming sibling fields of its own group, and
  the site shows it only while they hold. A rule may name a field that
  carries rules of its own, but never itself, never a field of another
  group, and never a loop. A field a sibling reads stays where it is
  until those rules change.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the group "Extras" placed on "post"
    And the "boolean" field "on-sale" in "Extras" with settings:
      """
      {}
      """

  Scenario: A field shown by a sibling is declared
    When the administrator declares the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    Then the field "sale-price" on "post" carries the setting "conditions"

  Scenario: A rule naming no sibling is refused
    When the administrator declares the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "vanished", "operator": "==", "value": "true"}]]}
      """
    Then the request is refused with the code "rule_source_unknown"

  Scenario: A rule naming the field itself is refused
    When the administrator declares the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "sale-price", "operator": "not_empty", "value": ""}]]}
      """
    Then the request is refused with the code "rule_cycle"

  Scenario: A value the source cannot hold is refused
    When the administrator declares the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "yes"}]]}
      """
    Then the request is refused with the code "rule_value_shape"

  Scenario: A rule may name a field carrying rules of its own
    Given the "text" field "sale-kind" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    When the administrator declares the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "sale-kind", "operator": "==", "value": "percent"}]]}
      """
    Then the field "sale-price" on "post" carries the setting "conditions"

  Scenario: A loop between two fields is refused
    Given the "text" field "sale-kind" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    When the administrator patches the settings of "on-sale" in "Extras" to:
      """
      {"conditions": [[{"source": "sale-kind", "operator": "not_empty", "value": ""}]]}
      """
    Then the request is refused with the code "rule_cycle"

  Scenario: A field a sibling reads cannot be taken away
    Given the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    When the administrator deletes the field "on-sale" from "Extras"
    Then the request is refused with the code "field_referenced"

  Scenario: A field nobody reads is taken away
    When the administrator deletes the field "on-sale" from "Extras"
    Then the field "on-sale" is gone from "post"
