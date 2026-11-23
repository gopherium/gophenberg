Feature: An administrator invites a person, who then chooses a password
  Add User sends an invitation instead of asking for a password. The site
  mails the person a link through its relay, or hands the link back to the
  administrator when it sends no mail. The link opens the activation page
  under the site's public address, works once, and signs the person in. The
  answer never tells whether an address already held an account, and a
  relay that rejects the mail leaves the account invited and its reason in
  the log.

  Scenario: An invitation mails its link under the public address
    Given a running Gophenberg at the public address "https://cms.example.com" that mails its invitations
    And a signed in administrator
    When the administrator invites "maria@example.com" as "Maria Perez" with the role "member"
    Then the invitation is answered as sent
    And "maria@example.com" received an invitation to "A Test Site" greeting "Maria Perez"
    And the latest link opens "https://cms.example.com/admin/activate"

  Scenario: Without mail the invitation answers its link to the administrator
    Given a running Gophenberg at the public address "https://cms.example.com" that sends no mail
    And a signed in administrator
    When the administrator invites "maria@example.com" as "Maria Perez" with the role "member"
    Then the invitation answers its link
    And the latest link opens "https://cms.example.com/admin/activate"

  Scenario: Without a public address the link is a path on the site
    Given a running Gophenberg that sends no mail
    And a signed in administrator
    When the administrator invites "maria@example.com" as "Maria Perez" with the role "member"
    Then the invitation answers its link
    And the latest link opens "/admin/activate"

  Scenario: An invited account is listed as not confirmed yet
    Given a running Gophenberg that sends no mail
    And a signed in administrator
    When the administrator invites "maria@example.com" as "Maria Perez" with the role "member"
    Then the user list shows "maria@example.com" as a "member" not confirmed yet

  Scenario: The link sets the password and signs the person in
    Given a running Gophenberg that mails its invitations
    And a signed in administrator
    And the administrator invited "maria@example.com" as "Maria Perez" with the role "member"
    When the invited person opens the latest link and chooses the password "a password Maria chose"
    Then the invited person is signed in as "maria@example.com"
    And "maria@example.com" signs in with the password "a password Maria chose"
    And the user list shows "maria@example.com" as a confirmed "member"

  Scenario: A password too short is rejected and the link still works
    Given a running Gophenberg that sends no mail
    And a signed in administrator
    And the administrator invited "maria@example.com" as "Maria Perez" with the role "member"
    When the invited person opens the latest link and chooses the password "short"
    Then the request is refused with the code "password_too_short"
    When the invited person opens the latest link and chooses the password "a password Maria chose"
    Then the invited person is signed in as "maria@example.com"

  Scenario: A spent link is rejected
    Given a running Gophenberg that sends no mail
    And a signed in administrator
    And the administrator invited "maria@example.com" as "Maria Perez" with the role "member"
    And the invited person opened the latest link and chose the password "a password Maria chose"
    When the invited person opens the latest link and chooses the password "another password entirely"
    Then the request is refused with the code "token_invalid"
    And "maria@example.com" signs in with the password "a password Maria chose"

  Scenario: A resend mails a fresh link and the first one is rejected
    Given a running Gophenberg that mails its invitations
    And a signed in administrator
    And the administrator invited "maria@example.com" as "Maria Perez" with the role "member"
    When the administrator resends the invitation to "maria@example.com"
    Then the invitation is answered as sent
    And "maria@example.com" received 2 invitations
    When the invited person opens the first link and chooses the password "a password Maria chose"
    Then the request is refused with the code "token_invalid"
    When the invited person opens the latest link and chooses the password "a password Maria chose"
    Then the invited person is signed in as "maria@example.com"

  Scenario: An invitation to an address that holds an account answers as sent and mails nothing
    Given a running Gophenberg that mails its invitations
    And a signed in administrator
    When the administrator invites "admin@example.com" as "Someone Else" with the role "member"
    Then the invitation is answered as sent
    And no invitation was mailed
    And the user list shows "admin@example.com" as a confirmed "admin"

  Scenario: A resend to an address with nothing to renew answers as sent and mails nothing
    Given a running Gophenberg that mails its invitations
    And a signed in administrator
    When the administrator resends the invitation to "nobody@example.com"
    Then the invitation is answered as sent
    And no invitation was mailed

  Scenario: A relay that rejects the mail leaves the account invited and logs why
    Given a running Gophenberg that mails its invitations
    And the mail relay rejects every message
    And a signed in administrator
    When the administrator invites "maria@example.com" as "Maria Perez" with the role "member"
    Then the request is refused with the code "delivery_failed"
    And the user list shows "maria@example.com" as a "member" not confirmed yet
    And the server logged the undelivered invitation with the relay's reason

  Scenario: A role the reader may not give is rejected and nothing is mailed
    Given a running Gophenberg that mails its invitations
    And the plugin "teams" declaring the role "team_lead" with the capability "manage_users"
    And a signed in "team_lead"
    When the account invites "maria@example.com" as "Maria Perez" with the role "admin"
    Then the request is refused with the code "role_not_grantable"
    And no invitation was mailed

  Scenario: A client that keeps opening dead links is slowed down
    Given a running Gophenberg that sends no mail
    And a visitor opened 10 dead links
    When a visitor opens a dead link
    Then the request is refused with the code "link_rate_limited"
