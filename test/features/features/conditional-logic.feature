Feature: Showing a field by rule
  A field carries conditions naming sibling fields of its own group, and
  the site shows it only while they hold. A rule may name a field that
  carries rules of its own, but never itself, never a field of another
  group, and never a loop. A field a sibling reads stays where it is
  until those rules change. An import judges each field it takes away by
  what the whole file leaves behind.

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

  Scenario: A value under a hidden field is refused
    Given the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the post "Winter sale"
    When the administrator saves into "Winter sale":
      """
      {"on-sale": false, "sale-price": 20}
      """
    Then the request is refused with the code "field_hidden"

  Scenario: A value under a shown field is stored
    Given the "text" field "sale-note" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the post "Winter sale"
    When the administrator saves into "Winter sale":
      """
      {"on-sale": true, "sale-note": "half price"}
      """
    Then the post "Winter sale" holds "half price" in "sale-note"

  Scenario: A value its field later hides stays where it stood
    Given the "text" field "sale-note" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the post "Winter sale"
    And the post "Winter sale" holding:
      """
      {"on-sale": true, "sale-note": "half price"}
      """
    When the administrator saves into "Winter sale":
      """
      {"on-sale": false}
      """
    Then the post "Winter sale" holds "half price" in "sale-note"

  Scenario: An autosave keeps a value the buffer left out
    Given the "text" field "sale-note" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the post "Winter sale"
    And the post "Winter sale" holding:
      """
      {"on-sale": true, "sale-note": "half price"}
      """
    When the editor autosaves "Winter sale" holding:
      """
      {"on-sale": false}
      """
    Then the post "Winter sale" holds "half price" in "sale-note"

  Scenario: An autosave carrying a hidden value is refused
    Given the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the post "Winter sale"
    When the editor autosaves "Winter sale" holding:
      """
      {"on-sale": false, "sale-price": 20}
      """
    Then the request is refused with the code "field_hidden"

  Scenario: A reader never sees a value the rules hide
    Given the "text" field "sale-note" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the post "Winter sale"
    And the post "Winter sale" holding:
      """
      {"on-sale": true, "sale-note": "half price"}
      """
    And the post "Winter sale" holding:
      """
      {"on-sale": false}
      """
    When the administrator publishes "Winter sale"
    And a visitor resolves "winter-sale"
    Then the served fields carry no "sale-note"

  Scenario: An import giving up a field and the sibling reading it takes both away
    Given the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the site's file leaves out "on-sale" in "Extras"
    And the site's file leaves out "sale-price" in "Extras"
    And the administrator confirms the loss of "on-sale" in "Extras"
    And the administrator confirms the loss of "sale-price" in "Extras"
    When the administrator imports the file
    Then the import is applied
    And the field "on-sale" is gone from "post"
    And the field "sale-price" is gone from "post"

  Scenario: An import keeping the reader of a field it gives up writes nothing
    Given the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the site's file retitles "Extras" as "Offers"
    And the site's file leaves out "on-sale" in "Extras"
    And the site's file leaves out "sale-price" in "Extras"
    And the administrator confirms the loss of "on-sale" in "Extras"
    When the administrator imports the file
    Then the request is refused with the code "field_referenced"
    And no group is titled "Offers"
    And the group "Extras" holds "on-sale"

  Scenario: An import moving a field its sibling stops reading takes it along
    Given the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the site's file moves "on-sale" from "Extras" into a new group "Offers"
    And the site's file clears the conditions of "sale-price" in "Extras"
    And the administrator confirms the loss of "on-sale" in "Extras"
    When the administrator imports the file
    Then the import is applied
    And the group "Offers" holds "on-sale"

  Scenario: An import changing the kind of a field its sibling reads keeps the reader
    Given the "number" field "sale-price" in "Extras" with settings:
      """
      {"conditions": [[{"source": "on-sale", "operator": "==", "value": "true"}]]}
      """
    And the site's file turns "on-sale" in "Extras" into a "text" field
    And the administrator confirms the loss of "on-sale" in "Extras"
    When the administrator imports the file
    Then the import is applied
    And the field "sale-price" on "post" carries the setting "conditions"
