Feature: The admin speaks the reader's language
  The server resolves one locale per request so the admin, the login
  screen and the API agree on it.

  Background:
    Given a running Gophenberg with the default content types
    And the supported locales are "en-US" and "es-ES"

  Scenario: The browser's language is followed when nothing is set
    Given no site default locale
    When a visitor asks for the locale preferring "es-ES"
    Then the locale answered is "es-ES"

  Scenario: An unsupported browser language falls back
    Given no site default locale
    When a visitor asks for the locale preferring "fr-FR"
    Then the locale answered is "en-US"

  Scenario: The site default wins over the browser
    Given the site default locale is "es-ES"
    When a visitor asks for the locale preferring "en-US"
    Then the locale answered is "es-ES"

  Scenario: A signed in user's own choice wins over everything
    Given the site default locale is "en-US"
    And a signed in administrator whose locale is "es-ES"
    When the administrator asks for the locale
    Then the locale answered is "es-ES"

  Scenario: The choice survives a fresh session
    Given a signed in administrator whose locale is "es-ES"
    When the administrator signs in again
    And the administrator asks for the locale
    Then the locale answered is "es-ES"

  Scenario: An unknown locale is refused with a code
    Given a signed in administrator
    When the administrator sets their locale to "xx-XX"
    Then the request is refused with the code "locale_unknown"

  Scenario: The login screen is answered without a session
    When a visitor asks for the locale without signing in
    Then the locale is answered without refusal

  Scenario: An administrator sets the language the site answers in
    Given a signed in administrator
    When the administrator sets the site locale to "es-ES"
    And a visitor asks for the locale preferring "en-US"
    Then the locale answered is "es-ES"

  Scenario: An unknown site locale is refused with a code
    Given a signed in administrator
    When the administrator sets the site locale to "xx-XX"
    Then the request is refused with the code "locale_unknown"
