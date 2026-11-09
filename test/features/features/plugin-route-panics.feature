Feature: A panic in a plugin's route costs only that request
  A plugin's routes run inside the site. When a route panics before it
  answered, the visitor gets the site's own internal error and never the
  plugin's name, the way WordPress shows its critical error page. When a
  route panics after it began its answer, the connection is cut, so a
  half answer never looks complete. The site logs each panic as an error
  with the plugin, the method, the path and the stack. The plugin stays
  active, and its other routes and the other plugins keep answering.

  Background:
    Given the settings point at an empty database
    And the compiled plugins "events" and "notes"

  @wip
  Scenario: A route that panics answers the site's internal error and the plugin stays active
    Given the route "/boom" of "notes" panics with "boom"
    When the site serves
    Then the route "/boom" of "notes" answers 500 with the code "internal", kept out of every cache
    And the route of "notes" answers 200
    And the route of "events" answers 200
    And the site logged once that GET "/api/plugins/notes/boom" of "notes" panicked with "boom", answered
    And the plugin records hold "notes" as active with no error

  @wip
  Scenario: A route that panics after it began its answer is cut
    Given the route "/boom" of "notes" answers 200, then panics with "boom"
    When the site serves
    Then the answer of the route "/boom" of "notes" is cut
    And the route of "notes" answers 200
    And the site logged once that GET "/api/plugins/notes/boom" of "notes" panicked with "boom", not answered

  @wip
  Scenario: A route behind a session that panics answers the site's internal error
    Given the route "/private" of "notes" needs a session and panics with "boom"
    And an administrator account exists
    When the site serves
    And the administrator signs in
    Then the route "/private" of "notes" answers 500 with the code "internal", kept out of every cache
    And the site logged once that GET "/api/plugins/notes/private" of "notes" panicked with "boom", answered
