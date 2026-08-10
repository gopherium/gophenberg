Feature: Activating a theme
  Activation switches the public site to an installed theme without a
  restart and without a moment where the site stops answering.

  Background:
    Given a running Gophenberg with a managed themes directory
    And a signed in administrator

  Scenario: Activating an installed theme
    Given a valid theme "aurora" is installed
    When the administrator activates "aurora"
    Then the public site is served through "aurora" once it reports ready
    And the theme list shows "aurora" as active
    And the theme list shows "aurora" as serving

  Scenario: The site keeps answering while a theme starts
    Given a valid theme "aurora" is installed
    When the administrator activates "aurora"
    Then requests before "aurora" is ready are answered by the built-in renderer

  Scenario: Switching themes keeps the old one serving until the new one is ready
    Given "driftwood" is installed and active
    And a valid theme "aurora" is installed
    When the administrator activates "aurora"
    Then requests before "aurora" is ready are served through "driftwood"
    And the public site is served through "aurora" once it reports ready

  Scenario: A theme that never becomes ready does not take over
    Given "driftwood" is installed and active
    And an installed theme "brokenboot" that passes validation but never starts
    When the administrator activates "brokenboot"
    Then the activation fails explaining the theme did not start
    And the public site is still served through "driftwood"
    And the theme list still shows "driftwood" as active

  Scenario: Deactivating returns the site to the built-in renderer
    Given "aurora" is installed and active
    When the administrator deactivates the theme
    Then no theme is active
    And the public site is served by the built-in renderer

  Scenario: The active theme survives a restart
    Given "aurora" is installed and active
    When the server restarts
    Then the public site is served through "aurora" once it reports ready

  Scenario: A stored theme that breaks on disk does not stop the server
    Given "aurora" is installed and active
    And the files of "aurora" become invalid while the server is stopped
    When the server restarts
    Then the server starts and the public site is served by the built-in renderer
    And the theme list shows "aurora" as broken

  Scenario: A theme that dies after starting is not reported as serving
    Given "aurora" is installed and active
    And the files of "aurora" are replaced by a theme that never starts
    When the server restarts
    Then the server starts and the public site is served by the built-in renderer
    And the theme list shows "aurora" as active but not serving

  Scenario: An operator pinned theme disables admin activation
    Given the server was started with an operator pinned theme "driftwood"
    And a valid theme "aurora" is installed
    When the administrator tries to activate "aurora"
    Then the request is refused explaining the theme is pinned by the operator
    And the public site is still served through "driftwood"