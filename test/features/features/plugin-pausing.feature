Feature: A site keeps running when one plugin fails at start
  The site keeps one record per compiled plugin: its state and the error
  that paused it. The state is not installed, inactive, active or paused.
  The first start that sees a plugin records it as active. Each start
  registers the active and paused plugins one by one. A plugin that fails
  or panics as it registers, migrates, declares its roles or content
  types, or starts is paused with its error, the way WordPress's
  recovery mode pauses a plugin, and the rest of the site starts. A
  paused plugin is left out of the site and tried again at every start.
  An inactive plugin, one turned off on purpose, is never tried. A start
  cut short by a shutdown pauses nothing.

  Background:
    Given the settings point at an empty database
    And the compiled plugins "events" and "notes"

  Scenario: The first start records each compiled plugin as active
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 200
    And the plugin records hold "events" as active with no error
    And the plugin records hold "notes" as active with no error

  Scenario: A plugin whose Register fails is paused and the site serves
    Given the Register of "notes" fails with "the title is too long"
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 404
    And the plugin records hold "notes" as paused, its error saying "pluginkit: notes register: the title is too long"
    And the site logged that "notes" is paused at the step "register"

  Scenario: A plugin whose Register panics is paused
    Given the Register of "notes" panics with "boom"
    When the site serves
    Then the route of "events" answers 200
    And the plugin records hold "notes" as paused, its error saying "pluginkit: notes register panicked: boom"

  Scenario: A plugin whose migration fails is paused and never starts
    Given the migration of "notes" fails with "schema exploded"
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 404
    And the Start of "notes" never ran
    And the plugin records hold "notes" as paused, its error saying "pluginkit: notes migrate: schema exploded"
    And the site logged that "notes" is paused at the step "migrate"

  Scenario: A plugin whose Start fails is paused and its routes are not mounted
    Given the Start of "notes" fails with "no route table"
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 404
    And the plugin records hold "notes" as paused, its error saying "pluginkit: notes start: no route table"
    And the site logged that "notes" is paused at the step "start"

  Scenario: A plugin whose roles fail to declare is paused and its roles are left out
    Given "notes" declares the role "archivist" with the capability "manage_archive"
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 404
    And the plugin records hold "notes" as paused, its error saying "manage_archive"
    And the site logged that "notes" is paused at the step "roles"

  Scenario: A plugin whose content types fail to declare is paused
    Given the content types of "notes" fail with "the field list is missing"
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 404
    And the plugin records hold "notes" as paused, its error saying "the field list is missing"
    And the site logged that "notes" is paused at the step "types"

  Scenario Outline: A plugin whose <method> panics is paused and the site serves
    Given the <method> of "notes" panics with "boom"
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 404
    And the plugin records hold "notes" as paused, its error saying "plugin notes <step> panicked: boom"
    And the site logged that "notes" is paused at the step "<step>"

    Examples:
      | method       | step  |
      | Roles        | roles |
      | DeclareTypes | types |

  Scenario: A paused plugin that starts again is active with no error
    Given the plugin records already hold "notes" as paused with "a setting is missing"
    When the site serves
    Then the route of "notes" answers 200
    And the plugin records hold "notes" as active with no error

  Scenario: A paused plugin that fails again stays paused with its new error
    Given the plugin records already hold "notes" as paused with "a setting is missing"
    And the Start of "notes" fails with "no route table"
    When the site serves
    Then the route of "events" answers 200
    And the plugin records hold "notes" as paused, its error saying "pluginkit: notes start: no route table"

  Scenario Outline: A plugin the records hold as <state> is never registered
    Given the plugin records already hold "notes" as <state>
    When the site serves
    Then the route of "events" answers 200
    And the route of "notes" answers 404
    And the Register of "notes" never ran
    And the plugin records hold "notes" as <state> with no error

    Examples:
      | state         |
      | inactive      |
      | not installed |

  Scenario: A start cut short by a shutdown pauses nothing
    Given the site is told to stop while "notes" starts
    When the site starts
    Then the plugin records hold "events" as active with no error
    And the plugin records hold "notes" as active with no error

  Scenario: A default role that a paused plugin would declare still stops the start
    Given "notes" declares the role "archivist" with the capability "notes.manage"
    And the Register of "notes" fails with "the title is too long"
    And GOPHENBERG_DEFAULT_ROLE is "archivist"
    When the site starts
    Then the command exits with code 1
    And the error says "GOPHENBERG_DEFAULT_ROLE: must name a role the core or a plugin declares"
    And the error names the paused plugin "notes"

  Scenario: A plugin paused at its start leaves its roles out
    Given "notes" declares the role "archivist" with the capability "notes.manage"
    And the Start of "notes" fails with "no route table"
    And GOPHENBERG_DEFAULT_ROLE is "archivist"
    When the site starts
    Then the command exits with code 1
    And the error names the paused plugin "notes"

  Scenario: A plugin that fails at the first start after an update is paused and the update is kept
    Given an install root where an update switched from "0.18.0" to this version
    And the Start of "notes" fails with "no route table"
    When the site starts
    Then the update status says the update to this version is kept
    And the plugin records hold "notes" as paused, its error saying "pluginkit: notes start: no route table"

  Scenario: A plugin whose start runs past the budget is named in a warning
    Given GOPHENBERG_PLUGIN_START_BUDGET is "10ms"
    And the Start of "notes" takes "50ms"
    When the site serves
    Then the site logged a warning that "notes" started slower than its budget
    And the site logged no slow start for "events"

  Scenario: Commands leave the plugin records alone
    When the operator runs "migrate"
    Then the command succeeds
    And the plugin records hold no record
