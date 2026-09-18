Feature: Content hierarchy
  A hierarchical type nests items under a parent and the address is the chain
  of their names. A flat type refuses parents outright. A type may be told to
  nest at any time, and stops nesting only once none of its items sits inside
  another.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the type "page" labeled "Page" and "Pages" under "pages" that nests

  Scenario: A child answers under its parent
    Given the page "About"
    When the administrator files the page "Team" under "About"
    Then the page "Team" answers at "pages/about/team"

  Scenario: A flat type refuses a parent
    Given the post "Hello World"
    When the administrator files a post under "Hello World"
    Then the request is refused

  Scenario: Siblings settle a shared name with a suffix
    Given the page "About"
    And the page "Team" filed under "About"
    When the administrator files the page "Team" under "About"
    Then the page answers at "pages/about/team-2"

  Scenario: The same name under different parents does not collide
    Given the page "About"
    And the page "Careers"
    And the page "Team" filed under "About"
    When the administrator files the page "Team" under "Careers"
    Then the page answers at "pages/careers/team"

  Scenario: Moving a page carries its children
    Given the page "About"
    And the page "Company"
    And the page "Team" filed under "About"
    When the administrator files "About" under "Company"
    Then the page "Team" answers at "pages/company/about/team"

  Scenario: Renaming a page carries its children
    Given the page "About"
    And the page "Team" filed under "About"
    When the administrator renames "About" to "company"
    Then the page "Team" answers at "pages/company/team"

  Scenario: A page cannot nest inside itself
    Given the page "About"
    When the administrator files "About" under "About"
    Then the request is refused

  Scenario: Nesting past ten levels is refused
    Given a chain of ten nested pages
    When the administrator files a page under the deepest one
    Then the request is refused

  Scenario: An address the CMS keeps is refused
    When the administrator creates the post "Admin"
    Then the request is refused

  Scenario: An address another type answers under is refused
    When the administrator creates the post "Pages"
    Then the request is refused

  Scenario: A parent on its way out takes no new children
    Given the page "About"
    And the page "Team"
    When the administrator deletes "About"
    And the administrator files "Team" under "About"
    Then the request is refused

  Scenario: Renaming into an address the CMS keeps is refused
    Given the post "Hello World"
    When the administrator renames "Hello World" to "admin"
    Then the request is refused

  Scenario: The trash is reached by deleting, never by editing
    Given the page "About"
    When the administrator marks "About" as trashed
    Then the request is refused

  Scenario: Renaming the route word of a type carries its content
    Given the page "About"
    And the page "Team" filed under "About"
    When the administrator files the type "page" under "sections"
    Then the page "Team" answers at "sections/about/team"

  Scenario: A flat type is told to nest after it is registered
    Given the post "Hello World"
    When the administrator lets the type "post" nest
    And the administrator files a post under "Hello World"
    Then the post answers at "hello-world/nested"

  Scenario: A type stops nesting while none of its items sits inside another
    Given the page "About"
    When the administrator stops the type "page" nesting
    Then the type "page" no longer nests

  Scenario: A type whose items sit inside one another keeps nesting
    Given the page "About"
    And the page "Team" filed under "About"
    When the administrator stops the type "page" nesting
    Then the request is refused with the code "type_nesting_in_use"
    And the error names "page" under "type"
    And the type "page" still nests

  Scenario: A nested item in the trash still keeps its type nesting
    Given the page "About"
    And the page "Team" filed under "About"
    When the administrator deletes "Team"
    And the administrator stops the type "page" nesting
    Then the request is refused with the code "type_nesting_in_use"

  Scenario: A type stops nesting once its nested items moved to the top
    Given the page "About"
    And the page "Team" filed under "About"
    And the page "Team" moved to the top level
    When the administrator stops the type "page" nesting
    Then the type "page" no longer nests

  Scenario: A plan warns that a type whose items nest keeps nesting
    Given the page "About"
    And the page "Team" filed under "About"
    And the site's file stops the type "page" nesting
    When the administrator plans the file
    Then the plan warns that "page" keeps nesting

  Scenario: An import leaves a type nesting while its items sit inside one another
    Given the page "About"
    And the page "Team" filed under "About"
    And the site's file stops the type "page" nesting
    When the administrator imports the file
    Then the import is applied
    And the type "page" still nests

  Scenario: An import stops a type nesting while none of its items sits inside another
    Given the page "About"
    And the site's file stops the type "page" nesting
    When the administrator imports the file
    Then the import is applied
    And the type "page" no longer nests
