Feature: A write from another site is refused
  A page on another site can make a visitor's browser send a form here,
  from the visitor's own address and with the visitor's own session. The
  server refuses such a write before any route sees it, while its own
  pages and other servers keep writing, and reads stay open.

  Background:
    Given a running Gophenberg serving the plugin "signup" with the public form "/subscribe"

  Scenario: A browser post from another site to a public plugin form is refused
    When a page on another site posts a form to "/api/plugins/signup/subscribe" through the visitor's browser
    Then the request is refused with the code "request_cross_origin"
    And the plugin "signup" received 0 forms

  Scenario: A browser post from a sibling site to a public plugin form is refused
    When a page on a sibling site posts a form to "/api/plugins/signup/subscribe" through the visitor's browser
    Then the request is refused with the code "request_cross_origin"
    And the plugin "signup" received 0 forms

  Scenario: An older browser posting from another site is refused
    When an older browser posts a form to "/api/plugins/signup/subscribe" from a page on another site
    Then the request is refused with the code "request_cross_origin"
    And the plugin "signup" received 0 forms

  Scenario: A browser post from the site's own page to a public plugin form goes through
    When a page on this site posts a form to "/api/plugins/signup/subscribe" through the visitor's browser
    Then the request is answered
    And the plugin "signup" received 1 form

  Scenario: A server to server post to a public plugin form goes through
    When another server posts a form to "/api/plugins/signup/subscribe"
    Then the request is answered
    And the plugin "signup" received 1 form

  Scenario: A read from another site still goes through
    When a page on another site reads "/api/content/v1/items" through the visitor's browser
    Then the request is answered

  Scenario: A browser write from another site to an admin route is refused
    Given a signed in administrator
    When a page on another site sets the page size to 2 through the administrator's browser
    Then the request is refused with the code "request_cross_origin"
    When a visitor lists the published content
    Then the listing offers pages of 20

  Scenario: A browser write from a sibling site to an admin route is refused
    Given a signed in administrator
    When a page on a sibling site sets the page size to 2 through the administrator's browser
    Then the request is refused with the code "request_cross_origin"
    When a visitor lists the published content
    Then the listing offers pages of 20

  Scenario: The administrator's own page still writes to an admin route
    Given a signed in administrator
    When a page on this site sets the page size to 2 through the administrator's browser
    Then the request is answered

  Scenario: A sign in posted from another site is refused
    Given the administrator account exists
    When a page on another site signs in as the administrator through the visitor's browser
    Then the request is refused with the code "request_cross_origin"

  Scenario: A sign out posted from another site is refused
    Given a signed in administrator
    When a page on another site signs the administrator out through the visitor's browser
    Then the request is refused with the code "request_cross_origin"
    And the administrator is still signed in
