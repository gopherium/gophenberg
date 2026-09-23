Feature: A site that names its public address refuses writes sent anywhere else
  A site can name the address people reach it at. A write then counts as
  the site's own only when it was sent to that address, and a browser's
  write only when its page stands there too, so another domain pointed at
  the server gains nothing. Reads stay open, and every refused write is
  logged with its reason.

  Background:
    Given a running Gophenberg at the public address "https://cms.example" serving the plugin "signup" with the public form "/subscribe"

  Scenario: A browser post from a page at the public address goes through
    When a page on this site posts a form to "/api/plugins/signup/subscribe" through the visitor's browser
    Then the request is answered
    And the plugin "signup" received 1 form

  Scenario: An older browser posting from a page at the public address goes through
    When an older browser posts a form to "/api/plugins/signup/subscribe" from a page on this site
    Then the request is answered
    And the plugin "signup" received 1 form

  Scenario: A page on another domain pointed at the server cannot post
    When a page on a domain pointed at this server posts a form to "/api/plugins/signup/subscribe" through the visitor's browser
    Then the request is refused with the code "request_cross_origin"
    And the plugin "signup" received 0 forms
    And the server logged a write refused for the reason "host"

  Scenario: An older browser posting from another site is refused against the public address
    When an older browser posts a form to "/api/plugins/signup/subscribe" from a page on another site
    Then the request is refused with the code "request_cross_origin"
    And the plugin "signup" received 0 forms
    And the server logged a write refused for the reason "origin"

  Scenario: Another server posting through the public address goes through
    When another server posts a form to "/api/plugins/signup/subscribe"
    Then the request is answered
    And the plugin "signup" received 1 form

  Scenario: A write sent to another address of the server is refused
    When another server posts a form to "/api/plugins/signup/subscribe" at the address "gophenberg.internal:8081"
    Then the request is refused with the code "request_cross_origin"
    And the plugin "signup" received 0 forms
    And the server logged a write refused for the reason "host"

  Scenario: A read sent to another domain pointed at the server still goes through
    When a page on a domain pointed at this server reads "/api/content/v1/items" through the visitor's browser
    Then the request is answered

  Scenario: The administrator's own page at the public address still writes
    Given a signed in administrator
    When a page on this site sets the page size to 2 through the administrator's browser
    Then the request is answered
