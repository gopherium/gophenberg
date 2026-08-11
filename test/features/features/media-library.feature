Feature: Managing the media library
  The library is where an administrator finds, describes and removes
  uploads. Descriptions feed the editor and the public site, so edits
  must be safe under concurrent use.

  Background:
    Given a running Gophenberg holding a seeded media library
    And a signed in administrator

  Scenario: The library lists newest uploads first
    When the administrator uploads a new image named "jetty.jpg"
    Then the library lists "jetty" first

  Scenario: Searching the library by name
    When the administrator searches the library for "harbor"
    Then the library lists only media matching "harbor"

  Scenario: Filtering the library to images
    When the administrator filters the library to images
    Then the library lists no plain files
    And the library still lists every image

  Scenario: Filtering the library to the kinds a block accepts
    When the administrator filters the library to images and video
    Then the library lists no plain files
    And the library still lists every image

  Scenario: Paging through the library
    When the administrator opens the second page of two per page
    Then the library reports the total while listing the remainder

  Scenario: Describing an image
    When the administrator describes the image "harbor"
    Then reading "harbor" back returns every saved description

  Scenario: An edit that changes nothing is not a conflict
    When the administrator saves the image "harbor" unchanged
    Then the request is accepted

  Scenario: A stale description does not overwrite a newer one
    Given two administrators read the image "harbor"
    When both save a description in turn
    Then the second save is refused as a conflict
    And the first description is untouched

  Scenario: Deleting an image removes every trace
    When the administrator permanently deletes the image "harbor"
    Then the library does not list "harbor"
    And the files of "harbor" are gone from the media directory

  Scenario: Deleting an image twice reports it missing
    When the administrator permanently deletes the image "harbor"
    And the administrator deletes "harbor" again
    Then the request reports the media does not exist

  Scenario: Reading media that was never stored reports it missing
    When the administrator asks for media that was never stored
    Then the request reports the media does not exist

  Scenario Outline: A listing the library cannot make sense of is refused
    When the administrator lists the library with <parameters>
    Then the request is refused as a bad listing

    Examples:
      | parameters          |
      | the type "audio"    |
      | the page "0"        |
      | the page size "500" |
