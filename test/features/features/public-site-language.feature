Feature: The public site speaks the site's language
  The public pages follow the language the site was set to, since a
  reader arriving at a published page has no admin session to ask.

  Background:
    Given a running Gophenberg with the default content types
    And the supported locales are "en-US" and "es-ES"

  Scenario: The public pages follow the site default
    Given the site default locale is "es-ES"
    When a reader opens a page that does not exist
    Then the public page reads "No existe ninguna página en esta dirección."
    And the public page declares the language "es-ES"

  Scenario: The public pages read English when no default is set
    Given no site default locale
    When a reader opens a page that does not exist
    Then the public page reads "No page lives at this address."
    And the public page declares the language "en-US"

  Scenario: A publication date follows the site default
    Given the site default locale is "es-ES"
    And a post published on "2026-08-16"
    When a reader opens the latest posts
    Then the public page reads "16 de agosto de 2026"

  Scenario: A publication date reads English when no default is set
    Given no site default locale
    And a post published on "2026-08-16"
    When a reader opens the latest posts
    Then the public page reads "16 August 2026"

  Scenario: The reader's own browser language is not negotiated
    Given the site default locale is "en-US"
    When a reader opens a page that does not exist preferring "es-ES"
    Then the public page declares the language "en-US"
