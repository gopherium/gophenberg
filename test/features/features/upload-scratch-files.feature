Feature: Uploads leave no scratch files behind
  A large upload is held in a scratch file while the server reads it.
  Every upload, accepted or refused, removes its scratch files once the
  server has answered it.

  Background:
    Given uploads may reach 64 MB
    And a running Gophenberg with an empty media directory
    And a signed in administrator
    And the server writes its scratch files to a watched folder

  Scenario: A large accepted upload removes its scratch file
    When the administrator uploads a 33 MB PDF named "manual.pdf"
    Then the library lists one file named "manual"
    And the watched folder holds no scratch file

  Scenario: A large upload under the wrong field removes its scratch file
    When the administrator uploads a 33 MB PDF named "manual.pdf" under the field "attachment"
    Then the upload is refused explaining the upload carries no file
    And the watched folder holds no scratch file

  Scenario: A large theme archive the server refuses removes its scratch file
    When the administrator uploads a 33 MB theme archive named "aurora" that is not a zip
    Then the upload is refused explaining the archive could not be read
    And the watched folder holds no scratch file
