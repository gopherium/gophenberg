Feature: The admin app comes inside the binary
  A release build carries the admin app inside the gophenberg binary, so a
  site needs no admin folder beside it. An owner may still name a built
  admin folder, which then wins. A named folder that holds no admin page
  stops the command before it serves, so the admin never answers a quiet
  404. The hashed files of the admin may be kept for a long window, while
  the admin page is checked on every load.

  Scenario: The admin answers with no folder named
    Given the settings name no admin folder
    When the site serves
    Then "/admin/" answers the admin page built into this binary

  Scenario: A named admin folder wins over the built-in admin
    Given the settings name an admin folder whose page says "Maria Perez built this admin"
    When the site serves
    Then "/admin/" answers a page saying "Maria Perez built this admin"

  Scenario Outline: A named folder with no admin page stops the command
    Given the settings name an admin folder that holds no index.html
    When the operator runs "<command>"
    Then the command fails, naming the folder and the admin built into this binary

    Examples:
      | command |
      | serve   |
      | check   |

  Scenario: The hashed admin files may be kept a year
    Given the settings name an admin folder holding the script "assets/index-DWLUpakq.js"
    When the site serves
    Then "/admin/assets/index-DWLUpakq.js" may be kept for 31536000 seconds and never changes
    And "/admin/" is checked again on every load
    And "/admin/posts" is checked again on every load

  Scenario: The window of the hashed admin files is a setting
    Given the settings name an admin folder holding the script "assets/index-DWLUpakq.js"
    And the settings keep the hashed admin files for "720h"
    When the site serves
    Then "/admin/assets/index-DWLUpakq.js" may be kept for 2592000 seconds and never changes
