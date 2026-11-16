Feature: Plugins read the settings their manifest declares
  A plugin lists its settings in its plugin.json, each with a key, a type
  and, for most types, a default. When the site starts, it works out each
  value from the environment, then from the default, and lends each plugin
  a reader of its own settings. A value or a declaration the rules reject
  pauses that plugin at the step register with the reason, and every other
  plugin starts. A secret comes from the environment only and never reaches
  the log.

  Background:
    Given the settings point at an empty database
    And the plugin "notes" declares these settings:
      | key      | type     | default | min | max | options        |
      | title    | text     | Notes   |     |     |                |
      | per_page | count    | 10      | 1   | 50  |                |
      | open     | flag     | false   |     |     |                |
      | keep_for | duration | 720h    |     |     |                |
      | order    | choice   | newest  |     |     | newest, oldest |
      | tags     | lines    | news    |     |     |                |
      | api_key  | secret   |         |     |     |                |
    And the plugin "gallery" declares no settings

  @wip
  Scenario: Each setting reads its default when the environment sets none
    When the site serves
    Then the plugin "notes" reads these settings:
      | key      | value  |
      | title    | Notes  |
      | per_page | 10     |
      | open     | false  |
      | keep_for | 720h   |
      | order    | newest |
      | tags     | news   |
      | api_key  |        |

  @wip
  Scenario Outline: A value in the environment wins over the default
    Given <variable> is "<value>"
    When the site serves
    Then the plugin "notes" reads "<read>" for "<key>"

    Examples:
      | key      | variable                          | value           | read            |
      | title    | GOPHENBERG_PLUGIN_NOTES__TITLE    | Field Notes     | Field Notes     |
      | per_page | GOPHENBERG_PLUGIN_NOTES__PER_PAGE | 25              | 25              |
      | open     | GOPHENBERG_PLUGIN_NOTES__OPEN     | true            | true            |
      | keep_for | GOPHENBERG_PLUGIN_NOTES__KEEP_FOR | 48h             | 48h             |
      | order    | GOPHENBERG_PLUGIN_NOTES__ORDER    | oldest          | oldest          |
      | tags     | GOPHENBERG_PLUGIN_NOTES__TAGS     | news,, events   | news, events    |
      | api_key  | GOPHENBERG_PLUGIN_NOTES__API_KEY  | key-for-example | key-for-example |

  @wip
  Scenario: A plugin whose id holds a hyphen reads its settings under underscores
    Given the plugin "notes-pro" declares the count setting "per_page" with the default "10"
    And GOPHENBERG_PLUGIN_NOTES_PRO__PER_PAGE is "30"
    When the site serves
    Then the plugin "notes-pro" reads "30" for "per_page"

  @wip
  Scenario: An empty variable leaves the default in place
    Given GOPHENBERG_PLUGIN_NOTES__TITLE is ""
    When the site serves
    Then the plugin "notes" reads "Notes" for "title"

  @wip
  Scenario Outline: A value its type rejects pauses the plugin and names the variable
    Given <variable> is "<value>"
    When the site serves
    Then the plugin records hold "notes" as paused, its error saying "<reason>"
    And the site logged that "notes" is paused at the step "register"
    And the plugin records hold "gallery" as active with no error

    Examples:
      | variable                          | value  | reason                                                                      |
      | GOPHENBERG_PLUGIN_NOTES__PER_PAGE | banana | GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must be a whole number, got "banana"     |
      | GOPHENBERG_PLUGIN_NOTES__PER_PAGE | 60     | GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must stand at or below 50, got "60"      |
      | GOPHENBERG_PLUGIN_NOTES__PER_PAGE | 0      | GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must stand at or above 1, got "0"        |
      | GOPHENBERG_PLUGIN_NOTES__KEEP_FOR | soon   | GOPHENBERG_PLUGIN_NOTES__KEEP_FOR: must be a duration like 30s, got "soon"  |
      | GOPHENBERG_PLUGIN_NOTES__OPEN     | maybe  | GOPHENBERG_PLUGIN_NOTES__OPEN: must be true or false, got "maybe"           |
      | GOPHENBERG_PLUGIN_NOTES__ORDER    | random | GOPHENBERG_PLUGIN_NOTES__ORDER: must be one of newest, oldest, got "random" |

  @wip
  Scenario Outline: A declaration outside the rules pauses the plugin at the step register
    Given the plugin "drafts" declares <declaration>
    When the site serves
    Then the plugin records hold "drafts" as paused, its error saying "<reason>"
    And the site logged that "drafts" is paused at the step "register"
    And the plugin records hold "notes" as active with no error

    Examples:
      | declaration                                           | reason                                                                                           |
      | a setting "mood" of the type "colour"                 | settings[0] mood: type "colour" is not one of text, count, flag, duration, choice, secret, lines |
      | a text setting "job_every"                            | settings[0] job_every: a key must not start with job_ or db_                                     |
      | a text setting "db_conns"                             | settings[0] db_conns: a key must not start with job_ or db_                                      |
      | a count "max_seats" with no default                   | settings[0] max_seats: a count needs a default                                                   |
      | a secret "api_key" with the default "key-for-example" | settings[0] api_key: a secret takes no default, because plugin.json is public                    |

  @wip
  Scenario: A plugin whose id holds two hyphens in a row cannot declare settings
    Given the plugin "drafts--old" declares the text setting "title"
    When the site serves
    Then the plugin records hold "drafts--old" as paused, its error saying "the id drafts--old must not hold two hyphens in a row or end with one when the plugin has settings"
    And the site logged that "drafts--old" is paused at the step "register"

  @wip
  Scenario: A plugin paused for a rejected value never shows its secret
    Given GOPHENBERG_PLUGIN_NOTES__API_KEY is "key-for-example-184467235"
    And GOPHENBERG_PLUGIN_NOTES__PER_PAGE is "banana"
    When the site serves
    Then the plugin records hold "notes" as paused, its error saying "GOPHENBERG_PLUGIN_NOTES__PER_PAGE: must be a whole number, got "banana""
    And no line of the site's log holds "key-for-example-184467235"

  @wip
  Scenario: A secret read as text answers nothing and the log names only the plugin and the key
    Given GOPHENBERG_PLUGIN_NOTES__API_KEY is "key-for-example-184467235"
    When the site serves
    And the plugin "notes" reads "api_key" as text twice
    Then the plugin "notes" read "" for "api_key"
    And the site's log holds one line "plugin read a setting as another type" for the plugin "notes" and the key "api_key"
    And no line of the site's log holds "key-for-example-184467235"

  @wip
  Scenario: A plugin that reads a key its manifest does not declare reads nothing
    When the site serves
    And the plugin "notes" reads the count "per_pages" twice
    Then the plugin "notes" read "0" for "per_pages"
    And the site's log holds one line "plugin read an undeclared setting" for the plugin "notes" and the key "per_pages"

  @wip
  Scenario: A plugin that reads a setting with another type's reader reads nothing
    When the site serves
    And the plugin "notes" reads "title" as a count
    Then the plugin "notes" read "0" for "title"
    And the site's log holds one line "plugin read a setting as another type" for the plugin "notes" and the key "title"

  @wip
  Scenario: A plugin reads the trusted proxies the site serves under
    Given GOPHENBERG_TRUSTED_PROXIES is "192.0.2.0/24"
    When the site serves
    Then the plugin "notes" reads the core setting "TRUSTED_PROXIES" as "192.0.2.0/24"

  @wip
  Scenario: A core setting off the published list answers nothing
    When the site serves
    And the plugin "notes" reads the core setting "DATABASE_URL"
    Then the plugin "notes" read "" for the core setting "DATABASE_URL"
    And the site's log holds one line "plugin read a core setting off the list" for the plugin "notes" and the name "DATABASE_URL"

  @wip
  Scenario: The feed reads its title and its post count as plugin settings
    Given the feed plugin is compiled into the site
    And three published posts
    And GOPHENBERG_PLUGIN_FEED__TITLE is "Field Notes"
    And GOPHENBERG_PLUGIN_FEED__ITEMS is "2"
    When the site serves
    Then the feed carries 2 posts under the title "Field Notes"

  @wip
  Scenario: The feed's old variable names no longer apply
    Given the feed plugin is compiled into the site
    And three published posts
    And GOPHENBERG_FEED_TITLE is "Old Title"
    And GOPHENBERG_FEED_ITEMS is "banana"
    When the site serves
    Then the plugin records hold "feed" as active with no error
    And the feed carries 3 posts under the title "Gophenberg"
