Feature: The content list
  The admin lists the items of one type a page at a time, narrowed by
  status, author and date and sorted by any column the list shows. It
  names the accounts that may write, and empties the trash of one type
  in one call.

  Background:
    Given a running Gophenberg with the default content types

  Scenario: The list keeps only the statuses it is given
    Given a signed in administrator
    And the published post "Morning Notes"
    And the post "Evening Draft"
    And the post "Old Idea"
    And the administrator trashes "Old Idea"
    When the administrator lists the posts with the status "draft,published"
    Then the list holds "Morning Notes" and "Evening Draft"

  Scenario: A list naming no status keeps every status
    Given a signed in administrator
    And the published post "Morning Notes"
    And the post "Old Idea"
    And the administrator trashes "Old Idea"
    When the administrator lists the posts
    Then the list holds "Morning Notes" and "Old Idea"

  Scenario: The list refuses a status it does not know
    Given a signed in administrator
    When the administrator lists the posts with the status "draft,lost"
    Then the request is refused with the code "list_parameters_invalid"

  Scenario: The list keeps only the authors it is given
    Given a signed in author
    And the post "Written by the author"
    And a signed in administrator
    And the post "Written by the administrator"
    When the administrator lists the posts the author wrote
    Then the list holds "Written by the author"

  Scenario: The list leaves out the authors it is told to
    Given a signed in author
    And the post "Written by the author"
    And a signed in administrator
    And the post "Written by the administrator"
    When the administrator lists the posts the author did not write
    Then the list holds "Written by the administrator"

  Scenario: The list refuses an author that names no account
    Given a signed in administrator
    When the administrator lists the posts written by "nobody"
    Then the request is refused with the code "list_parameters_invalid"

  Scenario: Each listed item names its author
    Given a signed in author
    And the post "Written by the author"
    And a signed in administrator
    When the administrator lists the posts
    Then "Written by the author" is credited to "Ada Lovelace"

  Scenario: The list keeps the items dated before a day
    Given a signed in administrator
    And the published post "Spring Notes"
    And "Spring Notes" was published on "2026-03-10"
    And the published post "Summer Notes"
    And "Summer Notes" was published on "2026-07-10"
    When the administrator lists the posts dated before "2026-06-01"
    Then the list holds "Spring Notes"

  Scenario: The list keeps the items dated after a day
    Given a signed in administrator
    And the published post "Spring Notes"
    And "Spring Notes" was published on "2026-03-10"
    And the published post "Summer Notes"
    And "Summer Notes" was published on "2026-07-10"
    When the administrator lists the posts dated after "2026-06-01"
    Then the list holds "Summer Notes"

  Scenario: The list sorts by the name of the author
    Given a signed in administrator
    And the post "Written by the administrator"
    And a signed in author
    And the post "Written by the author"
    When the account lists the posts sorted by "author" "asc"
    Then the list reads "Written by the author" and "Written by the administrator" in that order

  Scenario: The list sorts by slug
    Given a signed in administrator
    And the post "Banana Bread"
    And the post "Apple Pie"
    And the post "Cherry Tart"
    When the administrator lists the posts sorted by "slug" "desc"
    Then the list reads "Cherry Tart", "Banana Bread" and "Apple Pie" in that order

  Scenario: The list sorts the items with no parent first
    Given a signed in administrator
    And the type "page" labeled "Page" and "Pages" under "pages" that nests
    And the page "About"
    And the page "Team" filed under "About"
    And the page "Contact"
    When the administrator lists the pages sorted by "parent" "asc"
    Then the list ends with "Team"

  Scenario: A nested list keeps each child under its parent
    Given a signed in administrator
    And the type "page" labeled "Page" and "Pages" under "pages" that nests
    And the page "About"
    And the page "Team" filed under "About"
    And the page "Contact"
    When the administrator lists the pages by title nested under their parents
    Then the list reads "About", "Team" and "Contact" in that order

  Scenario: A flat list sorts every item by itself
    Given a signed in administrator
    And the type "page" labeled "Page" and "Pages" under "pages" that nests
    And the page "About"
    And the page "Team" filed under "About"
    And the page "Contact"
    When the administrator lists the pages sorted by "title" "asc"
    Then the list reads "About", "Contact" and "Team" in that order

  Scenario: A listed child carries its parent's title
    Given a signed in administrator
    And the type "page" labeled "Page" and "Pages" under "pages" that nests
    And the page "About"
    And the page "Team" filed under "About"
    When the administrator lists the pages sorted by "title" "asc"
    Then "Team" is listed under "About"
    And "About" is listed under no parent

  Scenario: A page larger than the cap is cut to the cap
    Given a signed in administrator
    When the administrator lists the posts 500 at a time
    Then the list was paged 100 at a time

  Scenario: A list naming no page size opens at the configured one
    Given a signed in administrator
    When the administrator lists the posts
    Then the list was paged 20 at a time

  Scenario: The authors list names every account that may write
    Given a signed in author
    And a signed in administrator
    When the account lists the authors
    Then the authors are "Ada Lovelace" and "Maria Perez"

  Scenario: An author reads the authors list too
    Given a signed in administrator
    And a signed in author
    When the account lists the authors
    Then the authors are "Ada Lovelace" and "Maria Perez"

  Scenario: A new item keeps the content and the excerpt it was created with
    Given a signed in administrator
    When the administrator creates the post "Second Copy" with the excerpt "A short summary" and the content "<p>The body</p>"
    Then the post "Second Copy" holds the excerpt "A short summary" and the content "<p>The body</p>"

  Scenario: Emptying the trash of one type keeps the trash of another
    Given a signed in administrator
    And the type "page" labeled "Page" and "Pages" under "pages" that nests
    And the page "Old Page"
    And the administrator trashes "Old Page"
    And the post "Old Post"
    And the administrator trashes "Old Post"
    When the account empties the trash of "page"
    Then the trash answer counts 1 deleted and 0 kept
    And "Old Page" is gone
    And "Old Post" is still in the trash

  Scenario: An author empties only the trash it may change
    Given a signed in administrator
    And the post "Trashed by the administrator"
    And the administrator trashes "Trashed by the administrator"
    And a signed in author
    And the post "Trashed by the author"
    And the account trashes "Trashed by the author"
    When the account empties the trash of "post"
    Then the trash answer counts 1 deleted and 1 kept
    And "Trashed by the author" is gone
    And "Trashed by the administrator" is still in the trash

  Scenario: Emptying the trash names the type it empties
    Given a signed in administrator
    When the account empties the trash of ""
    Then the request is refused with the code "type_unknown"

  Scenario: The settings carry what the lists read
    Given a signed in administrator
    When the account reads the settings
    Then the settings offer the page sizes 10, 20, 50 and 100 opening at 20
    And the settings keep a toast 6000 milliseconds naming at most 45 characters
    And the settings write dates and numbers in "es-ES"
