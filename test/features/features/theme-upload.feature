@wip
Feature: Installing a theme from the admin
  A signed in administrator installs themes by uploading a packaged
  theme archive. Installing never changes what the public site
  serves. Activation is a separate, deliberate step.

  Background:
    Given a running Gophenberg with an empty managed themes directory
    And a signed in administrator

  Scenario: Uploading a valid theme installs it
    When the administrator uploads a valid theme archive named "aurora"
    Then the theme list shows "aurora" as installed
    And no theme is active
    And the public site is served by the built-in renderer

  Scenario: Installing does not touch the active theme
    Given "driftwood" is installed and active
    When the administrator uploads a valid theme archive named "aurora"
    Then the public site is still served through "driftwood"

  Scenario Outline: A broken archive is refused whole
    When the administrator uploads a theme archive named "aurora" that <flaw>
    Then the upload is refused explaining <reason>
    And the theme list does not show "aurora"
    And the managed themes directory holds no trace of the upload

    Examples:
      | flaw                                     | reason                        |
      | carries no theme.json                    | the manifest is missing       |
      | declares the name "other" in theme.json  | the name does not match       |
      | holds no server entry                    | the server entry is missing   |
      | holds no client directory                | the client assets are missing |
      | contains a symbolic link                 | symlinks are not allowed      |
      | unpacks to more than the size cap        | the theme is too large        |
      | contains an entry escaping its directory | the archive is unsafe         |

  Scenario: Replacing the active theme is refused
    Given "aurora" is installed and active
    When the administrator uploads a valid theme archive named "aurora"
    Then the upload is refused explaining the theme is active
    And the public site is still served through "aurora"

  Scenario: Replacing an inactive theme updates it
    Given "aurora" is installed and not active
    When the administrator uploads a newer valid theme archive named "aurora"
    Then the theme list shows "aurora" as installed once