Feature: Operators and administrators update the core and see where it stands
  A server install keeps each build of Gophenberg in its own folder under
  an install root, with a current link naming the build that runs and a
  previous link naming the build before it. update:apply checks the
  signed index again, downloads the archive, checks its checksum and its
  signature, keeps a backup of the database and points current at the new
  build, then asks for a restart. At its first start the new build counts
  the attempt and checks itself. A healthy start keeps the update. A build
  that fails goes back to the previous one, and that version is never
  picked again on its own unless a person retries it. An update that
  raised the minimum version never goes back on its own, since the older
  build refuses the database the newer one migrated, so the site stays
  down and shows how to restore the backup. An install with no install
  root, like the Docker image, and a custom build keep the manual steps.
  A step that fails leaves which update failed, when, at which step and
  why. An administrator asks the site for the same: the version that
  runs, whether this install updates itself, the refused versions, the
  last update, the backups kept and when the next check runs. Once the
  site runs the version a check offered, or the version an update
  failed to reach, it names neither.
  An administrator also starts an update from the admin. The site runs
  the checks that need no download first and answers at once when one
  refuses. It runs the rest on its own, one update at a time, and once
  it switched builds it stops and exits with code 75, so the service
  manager starts the new build and the boot guard checks it.

  Background:
    Given the settings point at an empty database
    And the operator ran "migrate"

  Scenario: Applying an update points current at the new build and keeps the old one
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 75
    And the command asks for a restart to start "99.0.0"
    And the current build is "99.0.0"
    And the previous build is this version

  Scenario: Applying an update keeps a backup of the database first
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 75
    And the install root keeps a database backup named after this version

  Scenario Outline: Applying an update refuses <case>
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And <change>
    When the operator runs "update:apply"
    Then the command exits with code 1
    And the error says "<error>"
    And the current build is still this version
    And the install root holds no download and no build of "99.0.0"

    Examples:
      | case                              | change                                    | error                                                                       |
      | an index no listed key signed     | only a stranger signed the index          | refused the index, no release or standby key the list names signed it       |
      | an archive whose checksum differs | the archive changed after it was released | the one the release record names                                            |
      | an archive no listed key signed   | only a stranger signed the archive        | refused the archive, no release or standby key the key list names signed it |
      | an expired timestamp file         | the timestamp file expired yesterday      | refused the timestamp file, which expired at                                |

  Scenario Outline: A version that failed is never picked again on its own
    Given an install root running this version as an official build
    And version "99.0.0" failed its start check after an update and went back
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "<line>"
    Then the command exits with code 1
    And the error says "Gophenberg 99.0.0 failed"
    And the error says "update:apply -to 99.0.0 -retry"
    And the current build is still this version

    Examples:
      | line                    |
      | update:apply            |
      | update:apply -to 99.0.0 |

  Scenario: A person retries a version that failed
    Given an install root running this version as an official build
    And version "99.0.0" failed its start check after an update and went back
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply -to 99.0.0 -retry"
    Then the command exits with code 75
    And the current build is "99.0.0"
    And the update status lists no refused version

  Scenario: An install with no install root, like the Docker image, keeps the manual steps
    Given an official build with no install root, like the Docker image
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 1
    And the error says "GOPHENBERG_INSTALL_ROOT is empty"
    And the error says "update by hand to Gophenberg 99.0.0"

  Scenario: A custom build keeps the manual steps
    Given an install root running this version as a custom build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When the operator runs "update:apply"
    Then the command exits with code 1
    And the error says "custom build"
    And the error says "rebuild it on Gophenberg 99.0.0"
    And the current build is still this version

  Scenario: A healthy first start keeps the update
    Given an install root where an update switched from "0.18.0" to this version
    When the site starts
    Then the update status says the update to this version is kept
    And the current build is still this version
    And the previous build is still "0.18.0"

  Scenario: A build that keeps failing to start goes back to the previous one
    Given an install root where an update switched from "0.18.0" to this version
    And this version already tried to start 3 times
    When the site starts
    Then the command exits with code 76
    And the current build is "0.18.0"
    And the update status lists this version as refused

  Scenario: A failed update that raised the minimum version stays down and shows the restore steps
    Given an install root where an update switched from "0.18.0" to this version
    And the update raised the minimum version, with a backup taken before the switch
    And this version already tried to start 3 times
    When the site starts
    Then the command exits with code 76
    And the current build is still this version
    And the error names the backup taken before the switch
    And the error says "restore the backup taken before the update"
    And the update status names the backup taken before the switch

  Scenario Outline: The service manager takes the site back after <failure>
    Given an install root where an update switched from this version to "99.0.0"
    When the service manager runs "update:rollback -after-failure" with <variable> set to "<value>"
    Then the command succeeds
    And the current build is this version
    And the update status lists version "99.0.0" as refused

    Examples:
      | failure                           | variable               | value           |
      | the new build failed its check    | MONITOR_EXIT_STATUS    | 76              |
      | the new build hit the start limit | MONITOR_SERVICE_RESULT | start-limit-hit |

  Scenario: The service manager leaves an update that passed its check alone
    Given an install root where an update switched from this version to "99.0.0"
    And the new build passed its check
    When the service manager runs "update:rollback -after-failure" with MONITOR_SERVICE_RESULT set to "start-limit-hit"
    Then the command exits with code 1
    And the current build is still "99.0.0"

  Scenario Outline: The service manager does not take back an update that raised the minimum version after <failure>
    Given an install root where an update switched from this version to "99.0.0"
    And the update raised the minimum version, with a backup taken before the switch
    And the new build already tried to start 3 times
    When the service manager runs "update:rollback -after-failure" with <variable> set to "<value>"
    Then the command exits with code 1
    And the current build is still "99.0.0"
    And the error names the backup taken before the switch
    And the error says "restore the backup taken before the update"
    And the update status names the backup taken before the switch

    Examples:
      | failure                           | variable               | value           |
      | the new build failed its check    | MONITOR_EXIT_STATUS    | 76              |
      | the new build hit the start limit | MONITOR_SERVICE_RESULT | start-limit-hit |

  Scenario: An update that fails at a step leaves when, where and why
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And the archive changed after it was released
    When the operator runs "update:apply"
    Then the command exits with code 1
    And the update status names when the update from this version to "99.0.0" failed, at the step "download", and why: "the one the release record names"

  Scenario: The next update that switches clears the failure
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And an earlier update failed at the step "stage"
    When the operator runs "update:apply"
    Then the command exits with code 75
    And the update status names no failure

  Scenario: An official build on an install root updates itself
    Given an install root running this version as an official build
    When an administrator asks the site for its updates
    Then the site names this version as the one that runs
    And the site says this install can update itself
    And the site names no refused version and no update
    And the site counts 0 backups kept out of 3
    And the site says the next check is due now

  Scenario Outline: The site says why <install> cannot update itself
    Given <given>
    When an administrator asks the site for its updates
    Then the site says this install cannot update itself, since "<reason>"

    Examples:
      | install                         | given                                                         | reason          |
      | an install with no install root | an official build with no install root, like the Docker image | no_install_root |
      | a custom build                  | an install root running this version as a custom build        | custom_build    |

  Scenario: The site names a version that failed its start check as refused
    Given an install root running this version as an official build
    And version "99.0.0" failed its start check after an update and went back
    When an administrator asks the site for its updates
    Then the site names "99.0.0" as refused
    And the site names the update from this version to "99.0.0" at the step "rolled-back"

  Scenario: The site names no update once it runs the version the last check offered
    Given an install root running this version as an official build
    And the last check ran on the build "0.18.0" and found this version
    When an administrator asks the site for its updates
    Then the site names no update on offer

  Scenario: The site names no failure once it runs the version the failed update went to
    Given an install root running this version as an official build
    And an earlier update from "0.18.0" to this version failed at the step "download"
    When an administrator asks the site for its updates
    Then the site names no failure

  Scenario: The site names when, where and why the last update failed
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And the archive changed after it was released
    And the operator ran "update:apply" and it failed
    When an administrator asks the site for its updates
    Then the site names when the update from this version to "99.0.0" failed, at the step "download", and why: "the one the release record names"

  Scenario: The site counts the backups kept against their limit
    Given an install root running this version as an official build
    And the install root keeps 2 database backups, the newest taken at "2026-10-21T14:02:00Z"
    And the settings keep 5 backups
    When an administrator asks the site for its updates
    Then the site counts 2 backups kept out of 5, the newest taken at "2026-10-21T14:02:00Z"

  Scenario: The site says when the check for a newer version runs next
    Given an install root running this version as an official build
    And the check for a newer version ran an hour ago
    When an administrator asks the site for its updates
    Then the site says the next check runs in 11 hours

  Scenario: The site names no next check while the checks are off
    Given an install root running this version as an official build
    And the settings turn the checks for a newer version off
    When an administrator asks the site for its updates
    Then the site names no next check

  Scenario: An administrator starts an update from the admin and the site stops to start the new build
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When an administrator starts the update to "99.0.0" from the admin
    Then the site answers that the update started
    And the site stops with code 75 and asks for a restart to start "99.0.0"
    And the current build is "99.0.0"
    And the previous build is this version
    And the install root keeps a database backup named after this version
    And the update status says the update to "99.0.0" waits for its first start

  Scenario: The site names the step of the update it runs and refuses a second start
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And the database backup waits until the scenario lets it end
    When an administrator starts the update to "99.0.0" from the admin
    Then the site names the update from this version to "99.0.0" at the step "backup"
    And a second start of the update to "99.0.0" is refused with the code "update_runs"
    When the database backup ends
    Then the site stops with code 75 and asks for a restart to start "99.0.0"

  Scenario: The site refuses a start while update:apply runs
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And update:apply holds the update lock
    When an administrator starts the update to "99.0.0" from the admin
    Then the site refuses the start with the code "update_runs", saying "another update runs"
    And the site keeps serving the build that runs
    And the current build is still this version

  Scenario Outline: The site refuses at once to start an update on <install>
    Given <given>
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When an administrator starts the update to "99.0.0" from the admin
    Then the site refuses the start with the code "<code>", saying "<reason>"
    And the site keeps serving the build that runs

    Examples:
      | install                         | given                                                         | code            | reason                           |
      | an install with no install root | an official build with no install root, like the Docker image | no_install_root | GOPHENBERG_INSTALL_ROOT is empty |
      | a custom build                  | an install root running this version as a custom build        | custom_build    | rebuild it on Gophenberg 99.0.0  |

  Scenario: The site refuses at once a version that failed its start check
    Given an install root running this version as an official build
    And version "99.0.0" failed its start check after an update and went back
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When an administrator starts the update to "99.0.0" from the admin
    Then the site refuses the start with the code "update_refused", saying "Gophenberg 99.0.0 failed its start check"
    And the site keeps serving the build that runs
    And the current build is still this version

  Scenario: An administrator retries a version that failed its start check
    Given an install root running this version as an official build
    And version "99.0.0" failed its start check after an update and went back
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    When an administrator retries the update to "99.0.0" from the admin
    Then the site answers that the update started
    And the site stops with code 75 and asks for a restart to start "99.0.0"
    And the update status lists no refused version

  Scenario: An update started from the admin that fails at a step leaves the site serving
    Given an install root running this version as an official build
    And an update site offering version "99.0.0" that keeps the minimum version of this binary
    And the archive changed after it was released
    When an administrator starts the update to "99.0.0" from the admin
    Then the site answers that the update started
    And the site names when the update from this version to "99.0.0" failed, at the step "download", and why: "the one the release record names"
    And the site keeps serving the build that runs
    And the current build is still this version

  Scenario Outline: The site tells the Updates screen how often to ask for progress and how long to wait <when>
    Given the Updates screen settings <settings>
    When an administrator asks the site for its settings
    Then the site tells the Updates screen to ask every <ask> milliseconds and to wait <wait> milliseconds for the restart

    Examples:
      | when       | settings              | ask  | wait   |
      | by default | are left out          | 2000 | 300000 |
      | once set   | are "500ms" and "90s" | 500  | 90000  |
