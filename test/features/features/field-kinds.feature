Feature: Field kinds
  A choice field offers a list of value and label pairs and takes only
  what it lists unless custom values are allowed. Text fields carry a
  variant that checks an email or a web address on save. A media field
  can hold many items. The registry alone says which kinds exist.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the group "Extras" placed on "post"

  Scenario: Declaring a choice field serves its choices back
    When the administrator declares the "choice" field "style" in "Extras" with settings:
      """
      {"choices": [{"value": "ipa", "label": "IPA"}, {"value": "stout", "label": "Stout"}]}
      """
    Then the field "style" on "post" carries the setting "choices"

  Scenario: A kind the registry does not hold is refused
    When the administrator declares the "flavor" field "taste" in "Extras" with settings:
      """
      {}
      """
    Then the request is refused

  Scenario: A choice outside the list is refused on save
    Given the "choice" field "style" in "Extras" with settings:
      """
      {"choices": [{"value": "ipa", "label": "IPA"}]}
      """
    And the post "Hello world"
    When the administrator saves "porter" into "style" of "Hello world"
    Then the request is refused

  Scenario: A listed choice is stored
    Given the "choice" field "style" in "Extras" with settings:
      """
      {"choices": [{"value": "ipa", "label": "IPA"}]}
      """
    And the post "Hello world"
    When the administrator saves "ipa" into "style" of "Hello world"
    Then the post "Hello world" holds "ipa" in "style"

  Scenario: A stranger choice is stored when custom values are allowed
    Given the "choice" field "style" in "Extras" with settings:
      """
      {"allow_custom": true, "choices": [{"value": "ipa", "label": "IPA"}]}
      """
    And the post "Hello world"
    When the administrator saves "porter" into "style" of "Hello world"
    Then the post "Hello world" holds "porter" in "style"

  Scenario: A multiple choice stores every listed value it was given
    Given the "choice" field "styles" in "Extras" with settings:
      """
      {"multiple": true, "choices": [{"value": "ipa", "label": "IPA"}, {"value": "stout", "label": "Stout"}]}
      """
    And the post "Hello world"
    When the administrator saves the list ["ipa", "stout"] into "styles" of "Hello world"
    Then the post "Hello world" holds the list ["ipa", "stout"] in "styles"

  Scenario: An email that is not one is refused on save
    Given the "text" field "contact" in "Extras" with settings:
      """
      {"variant": "email"}
      """
    And the post "Hello world"
    When the administrator saves "not-an-email" into "contact" of "Hello world"
    Then the request is refused

  Scenario: An email is stored
    Given the "text" field "contact" in "Extras" with settings:
      """
      {"variant": "email"}
      """
    And the post "Hello world"
    When the administrator saves "maria@example.com" into "contact" of "Hello world"
    Then the post "Hello world" holds "maria@example.com" in "contact"

  Scenario: A media field holding many stores a list
    Given the many media field "gallery" in "Extras"
    And the post "Hello world"
    When the administrator saves the list [1, 2] into "gallery" of "Hello world"
    Then the post "Hello world" holds the list [1, 2] in "gallery"

  Scenario: The autosave buffer parks a choice the list refuses
    Given the "choice" field "style" in "Extras" with settings:
      """
      {"choices": [{"value": "ipa", "label": "IPA"}]}
      """
    And the post "Hello world"
    When the editor autosaves "Hello world" holding "porter" in "style"
    Then the buffer it saved holds "porter" in "style"
    And saving "porter" into "style" of "Hello world" is refused
