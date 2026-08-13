Feature: Serving content by address
  Every public address resolves through one lookup against the stored path,
  and the registry decides what renders. Content always beats pagination.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the type "page" labeled "Page" and "Pages" under "pages" that nests
    And the published post "Hello World"
    And the published page "About"
    And the published page "Team" filed under "About"

  Scenario: A post of the default type answers at the root
    Then "/hello-world" answers with "Hello World"

  Scenario: A nested page answers along its chain
    Then "/pages/about/team" answers with "Team"

  Scenario: The front page lists the default type
    Then "/" lists "Hello World"

  Scenario: An archive root lists its own type
    Then "/pages" lists "About"

  Scenario: The front page paginates behind the page word
    Then "/page/1" lists "Hello World"

  Scenario: An archive paginates behind the page word
    Then "/pages/page/1" lists "About"

  Scenario: Exact content beats pagination
    Given the published page "Page"
    And the published page "2" filed under "Page"
    Then "/pages/page/2" answers with "2"

  Scenario: A draft answers nowhere
    Given the page "Not Yet"
    Then "/pages/not-yet" answers not found

  Scenario: An unknown address answers not found
    Then "/nowhere/at/all" answers not found

  Scenario: A page number below the first answers not found
    Then "/page/0" answers not found

  Scenario: An archive of an inactive type answers not found
    Given the type "page" is deactivated
    Then "/pages" answers not found
