Feature: Rolling back a theme
  Activation remembers what served before, so one action returns the
  site to it.

  Background:
    Given a running Gophenberg with a managed themes directory
    And a signed in administrator

  Scenario: Rolling back returns to the previous theme
    Given "driftwood" was active and the administrator activated "aurora"
    When the administrator rolls back
    Then the public site is served through "driftwood" once it reports ready
    And the theme list shows "driftwood" as active

  Scenario: Rolling back after the first activation returns to the renderer
    Given no theme was active and the administrator activated "aurora"
    When the administrator rolls back
    Then no theme is active
    And the public site is served by the built-in renderer

  Scenario: Nothing to roll back to
    Given no theme has ever been activated
    Then rolling back is not offered

  Scenario: A failed activation needs no rollback
    Given "driftwood" is installed and active
    And an installed theme "brokenboot" that passes validation but never starts
    When the administrator activates "brokenboot"
    Then the public site is still served through "driftwood"
    And the theme list still shows "driftwood" as active