Feature: Serving content by address
  Every public address resolves through one lookup against the stored path,
  and the registry decides what renders. Content beats pagination under a
  route word, and the front page always keeps its numbered pages. A listing
  page answers at one address, so page 1, a padded number and a trailing
  slash move for good to it, unless content already holds that address.

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

  Scenario: The first page of the front page answers at the front page
    Then "/page/1" moves for good to "/"

  Scenario: The first page of an archive answers at the archive root
    Then "/pages/page/1" moves for good to "/pages"

  Scenario: A padded page number answers at the plain number
    Then "/page/02" moves for good to "/page/2"

  Scenario: A listing with a trailing slash moves to the address without it
    Then "/pages/" moves for good to "/pages"

  Scenario: A listing that moves keeps its query
    Then "/page/1?ref=feed" moves for good to "/?ref=feed"

  Scenario: Exact content beats pagination
    Given the published page "Page"
    And the published page "2" filed under "Page"
    Then "/pages/page/2" answers with "2"

  Scenario: Exact content beats a number that is no page number
    Given the published page "Page"
    And the published page "٢" filed under "Page"
    Then "/pages/page/٢" answers with "٢"

  Scenario: A padded page number stays where content holds the plain number
    Given the published page "Page"
    And the published page "2" filed under "Page"
    Then "/pages/page/02" answers with an archive page, not a redirect

  Scenario: The front page keeps its numbered pages
    Given the administrator lets the type "post" nest
    And the published post "Page"
    When the administrator files the post "2" under "Page"
    Then the request is refused with the code "address_reserved"
    When a visitor resolves "/page/2"
    Then the answer is an archive of "post"

  Scenario: A draft answers nowhere
    Given the page "Not Yet"
    Then "/pages/not-yet" answers not found

  Scenario: An unknown address answers not found
    Then "/nowhere/at/all" answers not found

  Scenario: An answer that failed is never cached
    When a visitor resolves "/nowhere/at/all" and finds nothing
    Then the answer is never cached

  Scenario: A page number below the first answers not found
    Then "/page/0" answers not found

  Scenario: A signed page number answers not found
    Then "/page/+2" answers not found
    And "/pages/page/+2" answers not found

  Scenario: An archive of an inactive type answers not found
    Given the type "page" is deactivated
    Then "/pages" answers not found

  Scenario: The handshake advertises the types
    When a visitor reads the content handshake
    Then it carries api 0
    And it serves kit "0.9.0"
    And it lists "post" as the default type at the root
    And it lists "page" under "pages" as hierarchical

  Scenario: The resolver names what an address is
    When a visitor resolves "/pages/about"
    Then the answer is a single "page"
    When a visitor resolves "/pages"
    Then the answer is an archive of "page"
