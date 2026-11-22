Feature: A site mails its invitations through the relay its settings name
  A site names its mail relay with GOPHENBERG_SMTP_HOST and the settings
  beside it. Without a host every invitation answers its link instead, so
  a relay setting set without one is a mistake that stops check and serve.
  A relay needs the address mail is sent from and the site's public
  address, and serve logs which relay it mails through.

  Background:
    Given the database refuses every connection
    And serve waits "0" for the database

  Scenario: A site without a relay says its invitations answer their link
    When the site starts
    Then the site logged that invitations answer their link

  Scenario: A site with a relay logs the relay it mails through
    Given the site's .env holds the line "GOPHENBERG_SMTP_HOST=smtp.example.com"
    And the site's .env holds the line "GOPHENBERG_SMTP_FROM=site@example.com"
    And the site's .env holds the line "GOPHENBERG_PUBLIC_URL=https://cms.example.com"
    When the site starts
    Then the site logged the relay "smtp.example.com" on the port 587

  Scenario Outline: A relay setting without a host stops check
    Given the site's .env holds the line "<line>"
    When the operator runs "check"
    Then the command exits with code 1
    And the error says "<error>"

    Examples:
      | line                                   | error                                                               |
      | GOPHENBERG_SMTP_FROM=site@example.com  | GOPHENBERG_SMTP_FROM is set but GOPHENBERG_SMTP_HOST is not         |
      | GOPHENBERG_MAIL_TEMPLATE_DIR=/srv/mail | GOPHENBERG_MAIL_TEMPLATE_DIR is set but GOPHENBERG_SMTP_HOST is not |

  Scenario Outline: A relay without what it needs stops check
    Given the site's .env holds the line "GOPHENBERG_SMTP_HOST=smtp.example.com"
    And the site's .env holds the line "<line>"
    When the operator runs "check"
    Then the command exits with code 1
    And the error says "<error>"

    Examples:
      | line                                          | error                                                              |
      | GOPHENBERG_PUBLIC_URL=https://cms.example.com | GOPHENBERG_SMTP_FROM is required when GOPHENBERG_SMTP_HOST is set  |
      | GOPHENBERG_SMTP_FROM=site@example.com         | GOPHENBERG_PUBLIC_URL is required when GOPHENBERG_SMTP_HOST is set |

  Scenario: A sender address the relay cannot use stops serve
    Given the site's .env holds the line "GOPHENBERG_SMTP_HOST=smtp.example.com"
    And the site's .env holds the line "GOPHENBERG_SMTP_FROM=not-an-address"
    And the site's .env holds the line "GOPHENBERG_PUBLIC_URL=https://cms.example.com"
    When the site starts
    Then the command exits with code 1
    And the error says "mail relay: mailkit/smtp: from address"
