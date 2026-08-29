Feature: Field settings
  A field carries settings: a default the editor pre-fills, help text,
  and bounds a value has to respect. Bounds gate every write of a new
  value, stored values stand still, and the autosave buffer stays open
  so an author never loses work over a half filled box.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the group "Extras" placed on "post"

  Scenario: Declaring a field with settings serves them back
    When the administrator declares the "number" field "rating" in "Extras" with settings:
      """
      {"min": 1, "max": 10, "instructions": "One to ten."}
      """
    Then the field "rating" on "post" carries the setting "min"

  Scenario: A setting the kind does not take is refused
    When the administrator declares the "number" field "rating" in "Extras" with settings:
      """
      {"maxlength": 80}
      """
    Then the request is refused

  Scenario: Settings that disagree are refused
    When the administrator declares the "number" field "rating" in "Extras" with settings:
      """
      {"min": 10, "max": 5}
      """
    Then the request is refused

  Scenario: Patching settings stores them
    Given the "number" field "rating" in "Extras" with settings:
      """
      {"min": 1}
      """
    When the administrator patches the settings of "rating" in "Extras" to:
      """
      {"min": 2, "max": 8}
      """
    Then the field "rating" on "post" carries the setting "max"

  Scenario: Patching the label leaves the settings standing
    Given the "number" field "rating" in "Extras" with settings:
      """
      {"min": 1}
      """
    When the administrator relabels the field "rating" in "Extras" as "Stars"
    Then the field "rating" on "post" carries the setting "min"

  Scenario: A value below the minimum is refused on save
    Given the "number" field "rating" in "Extras" with settings:
      """
      {"min": 10}
      """
    And the post "Hello world"
    When the administrator saves the number 5 into "rating" of "Hello world"
    Then the request is refused

  Scenario: A value above the maximum is refused on save
    Given the "number" field "rating" in "Extras" with settings:
      """
      {"max": 10}
      """
    And the post "Hello world"
    When the administrator saves the number 50 into "rating" of "Hello world"
    Then the request is refused

  Scenario: A text longer than maxlength is refused on save
    Given the "text" field "subtitle" in "Extras" with settings:
      """
      {"maxlength": 3}
      """
    And the post "Hello world"
    When the administrator saves "much too long" into "subtitle" of "Hello world"
    Then the request is refused

  Scenario: A value inside the bounds is stored
    Given the "number" field "rating" in "Extras" with settings:
      """
      {"min": 1, "max": 10}
      """
    And the post "Hello world"
    When the administrator saves the number 5 into "rating" of "Hello world"
    Then the post "Hello world" holds the number 5 in "rating"

  Scenario: Tightening a bound leaves a stored value standing
    Given the "number" field "rating" in "Extras" with settings:
      """
      {"min": 1}
      """
    And the post "Hello world"
    And the administrator saves the number 500 into "rating" of "Hello world"
    When the administrator patches the settings of "rating" in "Extras" to:
      """
      {"min": 1, "max": 100}
      """
    And the administrator retitles "Hello world" as "Still here"
    Then the post "Hello world" holds the number 500 in "rating"

  Scenario: The autosave buffer parks a value the bounds refuse
    Given the "number" field "rating" in "Extras" with settings:
      """
      {"min": 10}
      """
    And the post "Hello world"
    When the editor autosaves "Hello world" holding the number 5 in "rating"
    Then the buffer it saved holds the number 5 in "rating"
    And saving the number 5 into "rating" of "Hello world" is refused
