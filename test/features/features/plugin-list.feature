Feature: An administrator reads which plugins the site holds and why one paused
  The admin lists every plugin compiled into the site, the way WordPress
  lists its installed plugins. Each plugin shows the name, description and
  version its plugin.json gives, and the state its record holds: active,
  inactive or paused. A paused plugin also shows the step it failed at and
  its own error, kept apart, so the screen can put the error in a
  translated sentence for that step. A plugin the records hold as not
  installed is left out. Only an account that may activate plugins reads
  the list.

  Background:
    Given the settings point at an empty database
    And an administrator account exists

  Scenario: An administrator reads the feed as active with its name, description and version
    Given the feed plugin is compiled into the site
    When the site serves
    And the administrator signs in
    And the administrator reads the plugin list
    Then the plugin list holds "feed" as "active"
    And the plugin list names "feed" as "Feed"
    And the plugin list describes "feed" as "Serves your published posts as an RSS feed." at the version "0.0.1"

  Scenario: The feed paused by a post count that is not a whole number shows its step and its own error
    Given the feed plugin is compiled into the site
    And GOPHENBERG_PLUGIN_FEED__ITEMS is "banana"
    When the site serves
    And the administrator signs in
    And the administrator reads the plugin list
    Then the plugin list holds "feed" as paused at the step "register", its reason reading "GOPHENBERG_PLUGIN_FEED__ITEMS: must be a whole number, got "banana""

  Scenario: A paused plugin that starts again is listed as active with no step and no reason
    Given the compiled plugins "events" and "notes"
    And the manifest of "notes" names it "Notes" and describes it as "Keeps short notes beside each post."
    And the plugin records already hold "notes" as paused with "a setting is missing"
    When the site serves
    And the administrator signs in
    And the administrator reads the plugin list
    Then the plugin list holds "notes" as "active"

  Scenario: A plugin whose Start fails is listed as paused at the step start with its own error
    Given the compiled plugins "events" and "notes"
    And the manifest of "notes" names it "Notes" and describes it as "Keeps short notes beside each post."
    And the Start of "notes" fails with "no route table"
    When the site serves
    And the administrator signs in
    And the administrator reads the plugin list
    Then the plugin list holds "notes" as paused at the step "start", its reason reading "no route table"
    And the plugin list holds "events" as "active"

  Scenario: A plugin the records hold as inactive is listed as inactive
    Given the compiled plugins "events" and "notes"
    And the manifest of "notes" names it "Notes" and describes it as "Keeps short notes beside each post."
    And the plugin records already hold "notes" as inactive
    When the site serves
    And the administrator signs in
    And the administrator reads the plugin list
    Then the plugin list holds "notes" as "inactive"
    And the plugin list holds "events" as "active"

  Scenario: A plugin the records hold as not installed is left out
    Given the compiled plugins "events" and "notes"
    And the manifest of "notes" names it "Notes" and describes it as "Keeps short notes beside each post."
    And the plugin records already hold "notes" as not installed
    When the site serves
    And the administrator signs in
    And the administrator reads the plugin list
    Then the plugin list holds "events" as "active"
    And the plugin list leaves out "notes"

  Scenario: The plugins are listed by name, not by id
    Given the compiled plugins "a" and "b"
    And the manifest of "a" names it "Beta" and describes it as "Comes second by name."
    And the manifest of "b" names it "Alpha" and describes it as "Comes first by name."
    When the site serves
    And the administrator signs in
    And the administrator reads the plugin list
    Then the plugin list names "b" and "a", in that order

  Scenario: An editor may not read the plugin list
    Given the compiled plugins "events" and "notes"
    And an editor account exists
    When the site serves
    And the editor signs in
    And the editor reads the plugin list
    Then the request is rejected with the code "capability_missing"
