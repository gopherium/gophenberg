Feature: Serving media files
  Uploads are part of the public site. They are served to every
  visitor, cache well and never expose anything beyond the library.

  Background:
    Given a running Gophenberg holding a seeded media library

  Scenario: A visitor sees an uploaded image without signing in
    When a visitor requests the stored image "harbor"
    Then the file is served with the content type "image/jpeg"

  Scenario: A rendition is served like its original
    When a visitor requests the thumbnail of "harbor"
    Then the file is served with the content type "image/jpeg"

  Scenario: A served file may be cached
    When a visitor requests the stored image "harbor"
    Then the response allows public caching

  Scenario: A deleted file stops being served
    Given a signed in administrator
    And the administrator permanently deletes the image "harbor"
    When a visitor requests that image again
    Then the request reports the file does not exist

  Scenario: The media directory is never listed
    When a visitor requests the media directory itself
    Then the request reports the file does not exist

  Scenario: A path that climbs out of the media directory is refused
    When a visitor requests a media path that escapes the directory
    Then the request is refused

  Scenario: A hidden file is never served
    Given a hidden file rests in the media directory
    When a visitor requests that hidden file
    Then the request reports the file does not exist

  Scenario: The media prefix never falls through to a theme
    Given "driftwood" is installed and active
    When a visitor requests an unknown media path
    Then the request reports the file does not exist
    And the answer does not carry the theme's page
