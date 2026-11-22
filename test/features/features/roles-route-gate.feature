Feature: A role decides which routes an account reaches
  A site owner hands out accounts that write without handing over the
  keys to the site, and accounts for members who never enter the admin.
  Each administration area answers only a role carrying its capability,
  the content routes answer every role carrying access_admin, and a
  member keeps only its own session and language.

  Background:
    Given a running Gophenberg with an empty managed themes directory

  Scenario: An author is refused the account list
    Given a signed in author
    When the account asks for the user list
    Then the request is refused with the code "role_insufficient"

  Scenario: An editor is refused the account list
    Given a signed in editor
    When the account asks for the user list
    Then the request is refused with the code "role_insufficient"

  Scenario: An editor is refused the post counts
    Given a signed in editor
    When the account asks for the post counts
    Then the request is refused with the code "role_insufficient"

  Scenario: An author is refused the themes
    Given a signed in author
    When the account asks for the theme list
    Then the request is refused with the code "capability_missing"

  Scenario: An editor is refused the themes
    Given a signed in editor
    When the account asks for the theme list
    Then the request is refused with the code "capability_missing"

  Scenario: An editor cannot send an invitation
    Given a signed in editor
    When the account invites "maria@example.com" as "Maria Perez" with the role "member"
    Then the request is refused with the code "role_insufficient"

  Scenario: An editor cannot resend an invitation
    Given a signed in editor
    When the account resends the invitation to "maria@example.com"
    Then the request is refused with the code "role_insufficient"

  Scenario: The activation link route answers without a session
    When a visitor opens a dead link
    Then the request is refused with the code "token_invalid"

  Scenario: An administrator reaches the account list
    Given a signed in administrator
    When the account asks for the user list
    Then the request is answered

  Scenario: An author reaches the content list
    Given a signed in author
    When the account asks for the content list
    Then the request is answered

  Scenario: An editor reaches the content list
    Given a signed in editor
    When the account asks for the content list
    Then the request is answered

  Scenario: An author reaches the type registry it needs to read a screen
    Given a signed in author
    When the account asks for the content types
    Then the request is answered

  Scenario: A member reads its own session, which holds no way into the admin
    Given a signed in "member"
    When the account reads its session
    Then the request is answered
    And the session lacks the capability "access_admin"

  Scenario: A member keeps its own language
    Given a signed in "member"
    When the account sets its locale to "es-ES"
    Then the request is answered

  Scenario: A member is refused the content list
    Given a signed in "member"
    When the account asks for the content list
    Then the request is refused with the code "capability_missing"
    And the error names "access_admin" under "capability"

  Scenario: A member is refused the content types
    Given a signed in "member"
    When the account asks for the content types
    Then the request is refused with the code "capability_missing"
    And the error names "access_admin" under "capability"

  Scenario: A member is refused the themes
    Given a signed in "member"
    When the account asks for the theme list
    Then the request is refused with the code "capability_missing"

  Scenario: A member is refused the account list
    Given a signed in "member"
    When the account asks for the user list
    Then the request is refused with the code "role_insufficient"

  Scenario: The administrator's session opens the admin and may give the member role
    Given a signed in administrator
    When the account reads its session
    Then the session carries the capability "access_admin"
    And the session may give the role "member"

  Scenario: An answer to a signed in account is kept out of shared caches
    Given a signed in author
    When the account asks for the content list
    Then the request is answered
    And the answer is kept out of shared caches

  Scenario: A plugin role carrying the themes capability reaches the themes
    Given the plugin "studio" declaring the role "designer" with the capability "manage_themes"
    And a signed in "designer"
    When the account asks for the theme list
    Then the request is answered

  Scenario: A plugin role carrying the themes capability is refused the account list
    Given the plugin "studio" declaring the role "designer" with the capability "manage_themes"
    And a signed in "designer"
    When the account asks for the user list
    Then the request is refused with the code "role_insufficient"

  Scenario: The administrator's session names what it may do and give
    Given the plugin "events" declaring the role "moderator" with the capability "events.manage"
    And a signed in administrator
    When the account reads its session
    Then the session carries the capability "events.manage"
    And the session may give the role "moderator"

  Scenario: An administrator cannot give a role nobody declares
    Given the account "author@example.com" holding the role "author"
    And a signed in administrator
    When the account gives "author@example.com" the role "archivist"
    Then the request is refused with the code "role_unknown"

  Scenario: A role carrying only the account capability cannot make an administrator
    Given the plugin "teams" declaring the role "team_lead" with the capability "manage_users"
    And the account "member@example.com" holding the role "member"
    And a signed in "team_lead"
    When the account gives "member@example.com" the role "admin"
    Then the request is refused with the code "role_not_grantable"

  Scenario: A role carrying only the account capability cannot change an author
    Given the plugin "teams" declaring the role "team_lead" with the capability "manage_users"
    And the account "author@example.com" holding the role "author"
    And a signed in "team_lead"
    When the account gives "author@example.com" the role "member"
    Then the request is refused with the code "user_not_editable"

  Scenario: A role carrying only the account capability cannot disable an administrator
    Given the plugin "teams" declaring the role "team_lead" with the capability "manage_users"
    And the account "owner@example.com" holding the role "admin"
    And a signed in "team_lead"
    When the account disables "owner@example.com"
    Then the request is refused with the code "user_not_editable"
