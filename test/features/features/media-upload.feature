Feature: Uploading media files
  A signed in administrator builds the site's media library by
  uploading files. Every accepted image is stored ready to embed,
  with upright pixels and the renditions the editor offers.

  Background:
    Given a running Gophenberg with an empty media directory
    And a signed in administrator

  Scenario: Uploading a photo derives the editor renditions
    When the administrator uploads a 2400 by 1600 pixel JPEG named "harbor.jpg"
    Then the library lists one image named "harbor"
    And the image offers the renditions "thumbnail", "medium", "large" and "full"
    And every rendition fits its bounding box

  Scenario: A sideways phone photo is stored upright
    When the administrator uploads a JPEG carrying orientation tag 6
    Then the stored image is taller than it is wide
    And the stored image carries no orientation tag

  Scenario: An oversized photo gains a scaled display copy
    When the administrator uploads a 3000 by 2000 pixel JPEG named "cliff.jpg"
    Then the full rendition is at most 2560 pixels wide
    And the original upload is kept on disk

  Scenario: An animated GIF keeps its animation
    When the administrator uploads an animated GIF named "loader.gif"
    Then the library lists one image named "loader"
    And the image offers no renditions
    And the stored file is byte for byte the upload

  Scenario: A plain file is stored without renditions
    When the administrator uploads a PDF named "manual.pdf"
    Then the library lists one file named "manual"
    And the stored file is byte for byte the upload

  Scenario: Two uploads may carry the same name
    When the administrator uploads a valid JPEG named "team.jpg" twice
    Then the library lists two images named "team"
    And their stored files differ

  Scenario Outline: An upload that cannot be trusted is refused whole
    When the administrator uploads a file that <flaw>
    Then the upload is refused explaining <reason>
    And the library holds nothing
    And the media directory holds no trace of the upload

    Examples:
      | flaw                                        | reason                       |
      | carries a type outside the allowed set      | the file type is not allowed |
      | is an SVG document                          | the file type is not allowed |
      | names a JPEG but carries executable content | the content does not match   |
      | exceeds the upload size cap                 | the file is too large        |
      | decodes to more pixels than the budget      | the image is too large       |
      | is a corrupt image                          | the image cannot be read     |

  Scenario: Uploading without a session is refused
    Given no administrator is signed in
    When a visitor posts a file to the media endpoint
    Then the request is refused as unauthenticated
