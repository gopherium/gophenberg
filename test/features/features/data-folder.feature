Feature: The site keeps its data in a folder of its own
  A redeploy on a small hosting plan may rebuild the whole app folder, so
  GOPHENBERG_DATA_DIR names a folder outside it. That folder keeps the
  same backups and state folders an install root keeps, and a data folder
  for what the site writes itself. Only the site's own user can open it,
  since a backup holds every account. A media folder nobody names goes
  inside it. With no data folder named, every folder stays where it was.

  Scenario: A site makes a private data folder when it starts
    Given the settings name a free address
    And the settings name a data folder that is not there yet
    When the site serves
    Then only the site's own user can open the data folder

  Scenario: A site leaves a data folder that is already there as it stands
    Given the settings name a free address
    And the settings name a data folder with the mode 750
    When the site serves
    Then the data folder has the mode 750

  Scenario: A site that cannot make its data folder stops
    Given the settings name a free address
    And the settings name a data folder inside a file
    When the site starts
    Then the site exits with code 1 by itself

  Scenario Outline: A data folder that is not absolute stops check
    Given the settings hold "<settings>"
    When the operator runs "check"
    Then the command fails, saying "<error>"

    Examples:
      | settings                 | error                                                   |
      | GOPHENBERG_DATA_DIR=data | GOPHENBERG_DATA_DIR: must be an absolute path, got "data" |
