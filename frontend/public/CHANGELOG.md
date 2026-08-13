## [0.9.101] - 2026-08-08

### Fixed

- Fixed issue where Changelog would appear on every page visit when cookies were cleared. Now, the Changelog will only appear if a previous version is detected in the cookies and it is different from the current version. This will prevent the Changelog from appearing on every page visit when cookies are cleared.

---

## [0.9.101] - 2026-08-08

### Added

- Added new Authorization header for Jellyfin media servers. This is to fix an issue where some Jellyfin servers were not accepting the X-Emby-Token header for authentication. Now, if the media server type is set to Jellyfin, it will use the new Authorization header as well. (Thanks to degradedcode for the PR!)
- Added new Accordion component to the Changelog page to allow for better organization of the changelog entries. This will allow users to expand/collapse each version's changes for better readability.

### Fixed

- [#142](https://github.com/mediux-team/AURA/issues/142) Fixed issue where user page sometimes crashes when viewing or filtering Movie Sets.
- [#138](https://github.com/mediux-team/AURA/issues/138) Fixed issue where the navigation dropdown menu would sometimes only show "Logs" and "Jobs" after login or logout, hiding "Saved Sets", "Collections", "Download Queue", "Settings", and "View Density" until the page was refreshed multiple times.

---

## [0.9.100] - 2026-04-01

### Fixed

- Fixed issue where Save Image Locally option was not getting correct Season/Episode number when episode number was greater than 99.

---

## [0.9.99] - 2026-03-31

### Added

- Added option for users to ignore a Media Item until new sets are available for it. This is to help with items that have sets that you are not interested in downloading, but you still want to keep track of when new sets become available for it.
- Added more logging for the Autodownload process to help diagnose issues with it not working properly for some sets.
- Added support for multiple paths for library sections.

### Fixed

- Fixed issue where Shows with 0 sets would cause a panic error on the Media Item page.
- Fixed issue where items with more than 1 set would not get correct information from MediUX, causing them to not show up in the UI properly.
- Fixed issue where getting Library Section Options for the Settings page would fail after the onbaording process was completed.

---

## [0.9.98] - 2026-03-28

### Added

- Added new MediUX endpoint to fetch items with sets for more accurate results.
- Added set creator to Download Queue entries for better tracking of where sets are coming from.

### Fixed

- Fixed issue where multiple sets being sent to Download Queue at the same time would cause some sets to be missed.
- Fixed issue where User page would not properly update when selecting different library sections that don't have any sets available for them.
- Fixed issue where Show Sets that were empty would throw an error (different than handling of movie sets).

---

## [0.9.97] - 2026-03-24

### Fixed

- Fixed issue where Plex WebSocket client would not properly stop when changed from the Settings Page.
- Fixed issue where migration v1 to v2 was taking too long.
- Fixed panic error if Plex Server returned a non-200 response.
- Fixed issue where user could not delete items from the download queue.

---

## [0.9.96] - 2026-03-22

### Fixed

- Fixed issue where duplicate Media Items were showing on the Home Page.
- Fixed issue where Rate Media Item modal would not show the current user rating for the media item properly.
- Fixed issue where "Clear All Filters" button on Saved Sets page would not clear the "On Server" filter.

---

## [0.9.95] - 2026-03-20

### Added

- Added support for Auto Download for movies.
- [#131](https://github.com/mediux-team/AURA/issues/131) Added support for downloading future movies that are added to a collection.
- Added fallback to "Original Title" for Plex items when Title is empty.
- [#130](https://github.com/mediux-team/AURA/issues/130) Added support for future image types per set. This allows you to select any image type to be automatically downloaded if they are ever added to a set.

### Fixed

- [#136](https://github.com/mediux-team/AURA/issues/136) Fixed issue where "Future Updates Only" was still applying Labels/Tags for items that were added to the database.
- [#135](https://github.com/mediux-team/AURA/issues/135) Fixed issue where if Title is blank, it would cause issues with displaying the media item in the UI.
- Fixed issue where Sonarr/Radarr items would switch to monitored when they were updated from the Sonarr/Radarr Apply Tags feature.
- Fixed issue where image count was showing up as multiple even though there was only one image in the set for that item.

### UI Tweaks

- Made the Saved Sets page bulk edit mode more visually distinct by adding a border to selected items and changing the cursor to default when hovering over non-selected items.

---

## [0.9.94] - 2026-03-18

### Added

- Added new "On Server" filter option on the Saved Sets page to allow filtering of sets based on whether the media item exists on the media server. This required a new database field to track whether the media item exists on the media server, which is updated during the fetching of media items from the media server and during the check for media item changes job.

---

## [0.9.93] - 2026-03-17

### Added

- New floating "Edit" and "Save" buttons for the Settings Page to replace the previous Edit button that was hard to notice.
- Changed input for Notification message to Textarea to allow for better formatting of messages.
- Added support for HTML in Pushover notifications to allow for better formatting of messages.

### Fixed

- Fixed issue where Jellyfin/Emby Mixed library type would not show up on list of options during onboarding/settings.
- Fixed issue where enabling "Remove Overlay Label" for Plex would not be detected properly during saving.
- Fixed issue on Download Queue page where error would showup if no Sets were available.
- Fixed issue where Current Images for Shows were not showing up in order.
- Fixed issue where default notification template had a bad variable.

### UI Tweaks

- Fixed issue where "Jump to Top" button was spread out too far.
- Fixed alignment of "Refresh" button.

---

## [0.9.92] - 2026-03-15

### Added

- [#133](https://github.com/mediux-team/AURA/issues/133) Added option to remove "Overlay" label from Plex media items only when a poster image is downloaded. This allows Kometa to reprocess the image and apply the overlays. This is a Plex exclusive feature and can only be used if you have "Overlay" in your "Remove" list for the Plex application under "LabelsAndTags".

### Fixed

- Fixed issue where Plex labels were not being applied for each selected image type during downloads.

---

## [0.9.91] - 2026-03-14

### Added

- Added option to test notification templates in the Settings/Notifications section. This will send a test notification with the current template to the configured notification providers to allow you to see how the notification will look.
- Added option to customize the "Auto-Download" Notification title and message.
- Added option to customize the "Download Queue" Notification title and message.
- Added option to customize the "New Sets Available for Ignored Items" Notification title and message.
- Added option to customize the "Check For Media Item Changes Job" Notification title and message.
- Added option to customize the "Sonarr" Notification title and message.

### Fixed

- Fixed issue where Clear Temp Images button was pointing to an old route that was removed in the backend rewrite, now it points to the correct route for clearing temp images.

---

## [0.9.90] - 2026-03-13

### Added

- Added support for Mixed library type for Jellyfin/Emby media servers.

### Fixed

- Fixed issue where Library Titles with leading/trailing whitespace would not be filtered correctly on the Saved Sets page.

---

## [0.9.89] - 2026-03-12

### Added

- Added option to reapply images to Plex Items when they are refreshed in Plex. Plex exclusive feature. Shoutout to Geralt for the suggestion and most of the implementation!
- Added option to change "Items Per Page" on all pages. Shoutout to Geralt for the implementation!

### Fixed

- Reduced database file size by improving automatic SQLite vacuuming.
- Fixed issue where Media Items that no longer exist on the Media Server and had no sets available for them would still show up in Rating Key Change Job (now named: Media Item Change Job).
- Fixed issue where Duration/Size changes were not being detected properly for Autodownload checks. This causes images to be redownloaded when there are minor system-level changes that occur to the media files (e.g. metadata changes, container changes, etc.) that do not actually indicate a real change to the media item that would require new images. This is fixed by adding a threshold for size and duration changes to be considered a real change that requires new images.

---

## [0.9.88] - 2026-03-11

### Fixed

- Fixed issue where episode size changes were not being detected properly for Plex media servers.
- Fixed issue where titlecards for Special Seasons had blank season numbers, causing issues with autodownload and viewing titlecards on the media item page.

---

## [0.9.87] - 2026-03-10

### Fixed

- Fixed issue where backdrop and poster images were not being downloaded because of the batch processing changes. This was due to the download happening too quickly and the media server not having enough time to update the images before the next download was attempted. This has been fixed by splitting the batch processing into 3 steps: first it will download the posters, then the backdrops, and finally the season posters and titlecards. This allows the media server to update the images properly before the next download is attempted.
- Fixed issue where Jellyfin/Emby Boxsets/Collections that contained shows were being duplicated on the Home Page. This was due to the way the media server was returning the items in the set. This has been fixed by removing any shows/seasons/episodes from the Boxset/Collection item.

---

## [0.9.86] - 2026-03-09

### Added

- New batch processing for downloads to improve performance when downloading large sets of images.
- Added new Filter option for Home page that allows you to filter out Media Items that have no sets available for them. This is to help users who have large libraries and want to focus on items that have sets available.

---

## [0.9.85] - 2026-03-09

### Added

- Added auto-refresh to the App Loading page every 3 seconds to check if the backend API is ready, improving user experience during startup.

### Fixed

- Fixed issue where App Loading page was not working properly when the backend was still booting up.

---

## [0.9.84] - 2026-03-09

### Added

- Added new optional limit parameter to the API route for fetching media server items to allow limiting the number of items returned for better performance on large libraries.
- New shared http client for faster requests to external services.

### Fixed

- Fixed issue where Autodownload was not respecting the "Autodownload Off" for Saved Sets.
- Fixed issue where Autodownload would not properly display reason for redownload for episode changes.
- Readded Fetch Episode Last Updated Date logic as an optional part of the startup and home page fetching. This was because it slows down significantly when fetching large libraries, but it is needed for the recently added sorting by recently added episodes on the Home Page. This is now an optional part of the startup and home page fetching that can be enabled with the `EnableSortByEpisodeAddedDate` config option under the MediaServer section.
- Fixed Panic error for Jellyfin/Emby users when Media Item is not found.
- Fixed issue where size of episode for Jellyfin/Emby media servers was not being set. Since this is now fixed, it may cause autodownload to redownload episodes for these media servers since the size will now be different than what is currently in the database.
- Removed redundant code for refreshing metadata for Plex, increasing performance when applying images to Plex media items.

---

## [0.9.83] - 2026-03-07

### Added

- Added number of images in each download type to the download modal for better context when downloading images.

### Fixed

- Standardized the way filters use types for safety.

---

## [0.9.82] - 2026-03-07

### Fixed

- Fixed issue where logging was not occurring for scheduled jobs. This was due to a missing call to log the logger after the job function was executed. This has been fixed by adding a call to `ld.Log()` at the end of each job function.
- Fixed issue where User Page "In DB" filter would not work correctly.
- Fixed issue where some downloads were not reaching 100% in the Download Modal due to a mismatch in the way progress was being calculated.

---

## [0.9.81] - 2026-03-06

### Added

- Added sorting and tracking of episode last added date for better user experience when sorting by recently added episodes on the Home Page.

### Fixed

- Fixed issue where sorting was not working on the User Page.
- Fixed issue where Sonarr webhook would not find Media Item in Media Server after 10 second wait, increased retries and added more logging for better debugging of this process.
- Fixed issue where Boxsets would query Media Server for each item in the set, causing performance issues for large sets. Now it will query for the by set when accordion is opened and cache the results for future use.
- Fixed issue where Showsets would query Media Server for each item in the sets, causing performance issues for large sets. Now it will query for the by set when scrolled into view and cache the results for future use.

---

## [0.9.80] - 2026-02-16

### Breaking

- This is a massive rewrite of the backend to organize the codebase better and prepare for future features. Because of this, there are likely to be some bugs that slipped through testing. Please report any issues that you find to the Github issue page.
- There is a new config structure that is used internally. Please make a backup of your DB file and your config.yaml file just in case.

### Added

- New database schema. This is a breaking change, but all existing data should be migrated automatically. The new schema is more organized and will allow for better performance and future features.
- New App Loading page so that you can see the current status of the backend API.
- New ignore feature will allow you to mark an item as ignored until a set is available for it. This option is only available for items with no sets.
- New User Preference option to allow viewing of the MediUX image date last modified.
- New Custom Notifications feature to allow users to create their own custom notifications for different events in the app.

---

## [0.9.77] - 2026-01-15

### Fixed

- Fixed issue where Series didn't have any Seasons/Episodes, causing panic error during AutoDownload.
- Fixed issue where Manual Import from MediUX would not properly handle missing information.
- Reordered startup process to ensure database is initialized before attempting to connect to external services.
- Fixed issue where database had inflated size due to migration. Added automatic vacuuming after migration to reduce database size.

---

## [0.9.76] - 2026-01-09

### Added

- Added new Login With Plex button for easier authentication with Plex Media Servers.
- Added multiple retry logic for connecting with Sonarr/Radarr during startup for better reliability.

### Fixed

- Fixed default logging level to INFO when onboarding.
- Fixed issue where if no Season Posters are found during AutoDownload, it would cause panic error.
- Fixed issue where Sonarr Webhook integration would update database even if no images were downloaded.

---

## [0.9.75] - 2026-01-05

### Added

- Added new clear search button to Search Bar in Navbar for better user experience.

---

## [0.9.74] - 2025-12-31

### Added

- Added new "Manual YAML Import" action. This will allow you to use the YAML from MediUX to manually import poster sets to your media server. Keep in mind that downloads through this method will not be tracked in the aura database, so auto-downloads and rechecks will not apply to these sets.

### Fixed

- Fixed issue where rating modal was being cut off on smaller screens.

---

## [0.9.73] - 2025-12-30

### Added

- Revamp Sonarr Webhook integration so that it is more robust and better at handling errors. Will now handle Season Poster downloads too.

---

## [0.9.72] - 2025-12-30

### Added

- Added better colors to "New Version Available" footer for better visibility.
- Added "New Version Available" to Settings Icon for better visibility.
- Added option to add labels/tags for each selected type in the Settings/Onboarding UI.
- Moved "Refresh Metadata", "Rate", "Ignore Set" and "View Saved Sets" option to new dropdown menu on Media Item page.

---

## [0.9.71] - 2025-12-29

### Added

- Added new option to refresh metadata for Media Items from Media Item Page. For Shows, this will allow you to refresh metadata for specific seasons as well.
- Added option to rate items in your Plex library from the Media Item Page.

---

## [0.9.70] - 2025-12-27

### Added

- Added better logic for checking Plex images when saving locally.

---

## [0.9.69] - 2025-12-26

### Added

- Added better UI for Recheck Status output on the Saved Sets Page.
- Added 'Clear All' button to the Recheck Status output on the Saved Sets Page for better user experience.

### Fixed

- Fixed issue where aura would select already selected image when downloading new images.
- Fixed issue where "-thumb" suffix was not being added to titlecard images. This is to standardize naming conventions between Plex/Emby/Jellyfin.
- Fixed issue where notifications would be sent even when the Notifications section was disabled in the config.

---

## [0.9.68] - 2025-12-19

### Added

- Added list of image types present in the database to the Database icon popover in the MediUX Carousel for better context.

### Fixed

- Fixed issue where using "static" episode naming convention would cause errors when downloading Titlecards with non-standard episode numbers (e.g. 1x03).
- Fixed typo in Download Modal error message for Titlecards and Special Season Posters.
- Fixed issue where Download Modal popover for existing database items would be cut off on smaller screens.
- Fixed issue where Download Modal item would not show green border when all tasks were successful.
- Fixed issue where Gotify token would be masked after changing it in the Settings Page.

---

## [0.9.67] - 2025-12-17

### Added

- Revamped Download Modal logic to allow for better handling for failed downloads.
- Added more icons to Download Modal buttons for better visual context.

### Fixed

- Fixed issue where Download Modal would not properly update button text when toggling between "Add to Database Only" and normal download options.
- Fixed hydration issue in Download Modal and Saved Sets Page.
- Fixed issue where Download Modal would not show cursor pointer on Database icon hover.
- Fixed issue where Download Modal button icon would not update properly when toggling between options.

---

## [0.9.66] - 2025-12-16

### Added

- Changed Download Modal to show progress inline with image type for each item.
- Changed Download Modal progress bar to show current step in download process.
- Changed Download Modal to show currently applied sets for better context when downloading images.

### Fixed

- Fixed issue where Download Modal would not show proper error message when download failed.

---

## [0.9.65] - 2025-12-12

### Fixed

- Fixed issue where refreshing on the User Page would cause library section options to not load correctly.

---

## [0.9.64] - 2025-12-11

### Breaking

- SeasonNamingConvention config option has been removed. Please update your configuration accordingly. Read below for more information.

### Added

- Standardized naming convention for image files. This is to help future proof users who want to migrate from Plex to Emby/Jellyfin. The new naming conventions are as follows:
  - Posters: Movie/poster.jpg or Show/poster.jpg
  - Backdrops: Movie/backdrop.jpg or Show/backdrop.jpg
  - Season Posters: Show/seasonXX-poster.jpg
  - Special Season Posters: Show/season-specials-poster.jpg
  - Titlecards: Show/Season #/episode.jpg

  Please note that if you were using SeasonNamingConvention before, this is no longer supported. SeasonNamingConvention was using to determine the Season number but is no longer needed for the new format since all images are saved in root folder. Episode naming conventions remain unchanged. Episode naming for those using "static" will now use the currrent episode numbering format (e.g. S01E01.jpg or S1E1.jpg).

### Fixed

- Fixed issue where running aura in Docker for Windows would cause errors with path names not matching.

---

## [0.9.63] - 2025-12-09

### Fixed

- Fixed issue where response is nil when MediUX returns an error, causing a panic.

---

## [0.9.62] - 2025-12-09

### Fixed

- Fixed issue where Special Season Posters were not being properly named.
- [#120](https://github.com/mediux-team/AURA/issues/120) - Fixed issue where aura would fail to start when external applications were unreachable during startup (e.g. MediUX, Media Server, Sonarr, Radarr).
- Clarify port number for Sonarr in Webhook documentation.

---

## [0.9.61] - 2025-11-18

### Added

- Added Saved Sets to search functionality.
- Added image to Saved Sets table for better visual identification of sets.

### Fixed

- Fixed the layout of the cards so that the badges are aligned properly in cards.
- [#116](https://github.com/mediux-team/AURA/issues/116) - Fixed issue where MediUX token validation would fail if the MediUX service was unreachable, now it properly handles connection errors.
- Improved date formatting to show hours and minutes for recent updates
- Fixed issue where search queries with multiple words would not return expected results.
- Fixed issue where search bar was not being updated when clicking on Database icon in Media Item Page.
- Fixed alignment of Uploader name and avatar in Download Modal for better visual consistency.

---

## [0.9.60] - 2025-11-17

### Added

- Added more breakpoints for larger screens (3xl, 4xl, 5xl, 6xl) to support ultra-wide and 4K/5K/8K monitors.

### Fixed

- Fixed issue with backdrops not working in Emby.
- Fixed issue where multi-set Saved Sets would not save correctly when one was marked for deletion.
- Fixed issue where Selected Types would throw error when undefined in Saved Sets Page.

---

## [0.9.59] - 2025-11-16

### Added

- Added ability to Redownload titlecards when Sonarr episode file is upgrade. This requires you to set up a custom webhook connection from Sonarr to aura. View the [documentation](https://mediux-team.github.io/AURA/sonarr-webhook-integration) for more details.

### Fixed

- Fixed issue where undefined possible action paths would cause error when determining main label for log entries.

---

## [0.9.58] - 2025-11-15

### Added

- Added Bulk Actions to the Saved Sets Page. You can now, force recheck of autodownload, apply tags/labels and delete sets.
- Added total number of sets for each MediUX User in the search results for better context.
- Changed default tab on User Page to "Show Sets" and "Movie Sets" when no Box Sets are available.
- Changed library filter option on User Page to not require unselecting of previous library selection.

### Fixed

- Fixed issue where using the back button would revert page number to 1 instead of the last visited page.

---

## [0.9.57] - 2025-11-14

### Breaking

- The LabelsAndTagsProvider config section has been updated. The field `AddLabelsForSelectedTypes` has been renamed to `AddLabelTagForSelectedTypes` to better reflect its purpose of adding labels in Plex and tags in Sonarr/Radarr. Please update your configuration accordingly. This is a hidden setting (not available in the UI) to add labels/tags for each selected image type during downloads to Plex/Sonarr/Radarr.

### Added

- Added new search bar component to allow searching for Media Items and MediUX Users from anywhere in the app.
- Added Avatar images to the MediUX Users for better visual identification.
- Added some color to the Edit button on the Settings Page for better visibility.
- Added option to add tags in Sonarr/Radarr for each selected image type during downloads. This is a hidden setting (not available in the UI) called `AddLabelTagForSelectedTypes` under the LabelsAndTagsProvider config section.
- Updated changelog format to show which version you are currently on and the changes since your last update.
- Updated Movie Boxsets to use the new Responsive Grid layout for better visibility of items.

### Fixed

- Fixed issue where saving the new config would create a new config.yaml file instead of overwriting existing config.yml.
- Fixed issue where clicking on a Media Item from the Collection Items Page would throw an error for Emby/Jellyfin media servers.
- Fixed issue where images were not being applied correctly for Emby/Jellyfin.
- Fixed issue where Media Item Title on Collection Items Page was causing misalignment in the Carousel.
- Fixed issue where Force Recheck for Auto-Download was not working correctly for large sets.
- Fixed Collections Download modal "Cancel" button to match Main Download modal.
- Cleaned up some inconsistent naming for MediUX in various places.

---

## [0.9.56] - 2025-11-11

### Added

- Added support for Collections in Emby and Jellyfin media servers.
- Changed the Carousel within Collections to be a Responsive Grid for viewing more items at once.

### Fixed

- Fixed issue with Emby/Jellyfin where downloading a Backdrop image would cause it to be uploaded but not selected.

---

## [0.9.55] - 2025-11-11

### Fixed

- Fixed issue where backend didn't pass back info about whether media item exists in database to frontend for download modal.
- Fixed panic error where Sonarr/Radarr didn't find the TMDB ID in the response.
- Fixed issue where login page would not redirect after successful login if user was already authenticated.

---

## [0.9.54] - 2025-11-10

### Added

- Added responsive grid for all pages to improve layout on different screen sizes.
- Added hidden option to add labels to Plex for each selected image type.
- Added support for TMDB Poster and Backdrop URLs in the download queue image selection logic.

### Fixed

- Fixed missing User-Agent header in requests to MediUX GraphQL API.
- Fixed issue where download queue was having panics when Poster for set was empty.
- Fixed issue where pagination was not being reset when number of items per page was changed on another page.

---

## [0.9.53] - 2025-11-09

### Added

- Added a view density slider to allow users to customize the size of Images in the carousel views. This setting is saved in User Preferences.

---

## [0.9.52] - 2025-11-09

### Added

- [#112](https://github.com/mediux-team/AURA/issues/112) - Collections Page will now respect User Preferences for Download Defaults when opening the Collections Download Modal.

### Fixed

- [#113](https://github.com/mediux-team/AURA/issues/113) - Fixed issue where Collections with no media items were being shown on the Collections Page.

---

## [0.9.51] - 2025-11-07

### Added

- Added new route and function to get status of last download in the download queue.

### Fixed

- Fixed issue where download modal was not selecting correct default image type based on user preferences.

---

## [0.9.50] - 2025-11-06

### Added

- Added Collections Page to handle applying Posters and Backdrops to a Collection within Plex. This is a Plex exclusive feature (for now).
- Added aura logo to blank images for better user experience when images are missing.

### Fixed

- Fixed issue with Autodownload not filtering out Seasons/Titlecards that are not present on Media Server.
- Fixed issue where trying to enable "Save Images Locally" during initial onboarding would cause error.
- Fixed issue where Media Server images were no longer being downscaled correctly.
- Fixed issue where Cache Images would cause smaller images to be used even when Mediux.DownloadQuality was set to "original".
- Fixed issue where fetching image from Media Server would not throw an error if the media item did not exist.
- Fixed Documentation to show how to manually edit Config file for MediaServer.Libraries Section.
- Fixed issue where Log Filter actions were not sorted within the Group.

---

## [0.9.49] - 2025-11-02

### Breaking

- If you use Plex as your Media Server, the config option for Season Naming Convention has been moved under the Images -> SaveImagesLocally section. Please update your configuration accordingly. View the [documentation](https://mediux-team.github.io/AURA/config#saveimageslocallyseasonnamingconvention) for more details.

### Added

- Added support for Episode Naming Convention under the Images -> SaveImagesLocally section for Plex Media Servers. This allows you to customize how episode files are named when saving images locally. View the [documentation](https://mediux-team.github.io/AURA/config#saveimageslocallyepisodenamingconvention) for more details.
- Added new Not Found (404) page for better user experience when navigating to invalid routes.
- Added new Error (500) page for better user experience when the application encounters server errors.
- Added Browser Source Mapping for easier debugging of frontend issues.

### Fixed

- Fixed issue where adding first Notification provider would throw an error due to undefined config state

---

## [0.9.48] - 2025-11-02

### Added

- [#108](https://github.com/mediux-team/AURA/issues/108) MediUX images that have a Blurhash will now show a Blurhash placeholder while loading for a better user experience.
- [#110](https://github.com/mediux-team/AURA/issues/110) Added support for custom Webhook notification provider

### Fixed

- [#106](https://github.com/mediux-team/AURA/issues/106) Fixed issue where images for Download Queue were not showing centered correctly on larger screens
- [#107](https://github.com/mediux-team/AURA/issues/107) Fixed issue where season posters were not being downloaded when in Download Queue
- [#109](https://github.com/mediux-team/AURA/issues/109) Fixed issue where large poster sets were not being added to the database.

---

## [0.9.47] - 2025-11-02

### Added

- Added a new Download Queue Page to help manage failed and in-progress downloads. You can redownload items with warnings or errors, and remove files from the queue.
- Added notifications for Download Queue events so you are informed of progress and issues as they happen.
- Added logging details to App Startup to help diagnose issues during initialization.

### Fixed

- Fixed issue with download queue not processing multiple downloads correctly when queued in quick succession.
- Fixed issue with Logs Page not showing up correctly on mobile devices.
- Fixed error handling when Plex doesn't return posters the first time.
- Changed Plex/Sonarr/Radarr label and tag handling to only occur when items are added to the database, instead of during every file download.
- Moved database queue route logic to a separate function for better organization.

---

## [0.9.46] - 2025-11-01

### Added

- Plex Only: Try to dynamically get the season path from episodes on file (falls back to Season 1 or Season 01 - based on Season Naming Conventions)

### Fixed

- Changed exported log file name for clarity

---

## [0.9.45] - 2025-11-01

### Added

- Added 🎉 emojis to changelog headings for better visibility
- Added border to dialog content for better visual distinction
- Updated all filters to use dialog for better visual consistency

---

## [0.9.44] - 2025-11-01

### Added

- Logs Page will now remember filter settings between visits using local storage
- Added backend pagination support for log entries to improve performance on large log files
- Added backend filtering support for log levels, statuses, and actions to enhance log retrieval

### Fixed

- Add image validation and loading state to DimmedBackground component
- Include timestamp in download queue JSON file names to handle multiple sets
- Removed frontend log filtering logic to rely solely on backend filtering for consistency and performance

---

## [0.9.43] - 2025-10-31

### Fixed

- Fixed issue with Plex token not being sent in headers for FetchLibrarySectionOptions and GetMediaServerStatus requests

---

## [0.9.42b] - 2025-10-31

### Added

- Standardized Filters on Home Screen to match Saved Sets and Logs page filters
- Combined Sort and Filter into one section for better UX

### Fixed

- Fixed Docker Pulls badge link in README
- Fixed issue on Saved Sets page DB Query where pagination was not working correctly when multiple poster sets exist for a media item
- Fixed issue on Saved Sets page where sort order was not being reset when changed
- Fixed text on Logs Filter drawer description

---

## [0.9.42] - 2025-10-30

### Fixed

- [#104](https://github.com/mediux-team/AURA/issues/104) - Fixed issue with padding on download modal causing misalignment

---

## [0.9.41] - 2025-10-29

### Added

- Added support for adding items to a download queue for better management of multiple downloads

---

## [0.9.40] - 2025-10-27

### Breaking

- Updated log storage format to JSONL, previous log files will not be compatible

### Added

- Revamped the logging system to use JSONL format for better structure and parsing
- Revamped the Logs page to display structured log entries with expandable details
- Added ability to export individual log entries as JSON files
- Logs will also now auto-rotate based on size and age to manage disk space better

---

## [0.9.30] - 2025-10-03

### Added

- Enhance Download Modal to include previous download history for better user tracking
- Can now cycle through poster and season posters on the media item page for series (using touch or mouse drag)
- Added --user support for docker images to run as non-root user
- Added UMASK environment variable support for setting file permissions on downloaded images
- Support for configuring Sonarr/Radarr instances
- Support for setting tags in Sonarr
- Support for setting tags in Radarr
- Added support for testing Sonarr and Radarr connections individually
- Added status indicators for Sonarr and Radarr in settings
- Added support for testing Notification providers individually
- Added status indicators for Notification providers in settings
- Added support for force checking Movies for Rating Key and Path changes on Saved Sets page
- Show already downloaded poster sets at the top of the poster selection carousels

### Fixed

- Updated error details to use structured maps instead of formatted strings for better clarity and consistency
- Reduce size of docker image by switching to a smaller base image and multi-stage builds
- Migrated Database to new structure. TMDB is the main key for Media Items. See breaking changes below.
- Plex labels are now added asynchronously to improve performance when downloading images
- Autodownload will now check for changes to Rating Key (which can change when Media Items are deleted/re-added). This uses TMDB ID as the unique identifier.
- Autodownload will now check for changes to Media Item path (which can change on upgrade or if the file is moved). This is for movies and episodes.
- Fixed issue with clutter on Settings/Onboarding page when item is changed or error on field validation.
- Return Media Item details, Posters and User/Follow Hides in one response to reduce number of API calls

### Breaking

- Database schema has changed. Previous database file should be backed up automatically. All previous entries should be migrated automatically. Any issues should be available in a file called
  `migration_warning_v1.txt` in the same directory as your database.

---

## [0.9.29] - 2025-10-02

### Added

- Change button variant to ghost and update styles for consistency in JumpToTop and RefreshButton components
- [#98](https://github.com/mediux-team/AURA/issues/98) Mask sensitive information in logging for Pushover notifications and media server configuration
- Change home page loading progress bar to be front and center for better visibility on larger libraries
- Changed home page loading sections to use a skeleton loader for better user experience

### Fixed

- [#87](https://github.com/mediux-team/AURA/issues/87) Fixed issue with poster update failing after Plex movie file is replaced
- Adjust footer padding and link text size for improved layout consistency

---

## [0.9.28] - 2025-10-01

### Added

- Added ability to save images locally with configurable path
- Added icons to changelog headings for better visibility
- Added breaking section to changelog for important changes

### Fixed

- Remove cache images from Media Server logic so that images are always fresh
- [#99](https://github.com/mediux-team/AURA/issues/99) - Fixed issue with aura logo in Home Screen icon not having enough padding
- Remove option for Tags from non-Plex media servers
- Remove option for SaveImagesLocally from non-Plex media servers

### Breaking

- If you use SaveImagesNextToContent, please change over to the new SaveImagesLocally option in your config file. The old option has been removed. View the [documentation](https://mediux-team.github.io/AURA/config#saveimageslocallyenabled) for more details.

---

## [0.9.27] - 2025-10-01

### Added

- Added tab navigation for settings and user preferences, enhancing UI organization
- Added release notes dialog to display changelog updates upon new version detection
- Enhance Media Server and MediUX connection with status indicators

### Fixed

- Update search bar to remove on-click animation for improved user experience
- Fixed issue with plex not returning posters
- Update tab triggers in UserSetPage for improved styling and user interaction
- Changed settings cog color to grey for better visual integration
- Update layout and metadata for improved web app manifest and icons
- Add 'select-none' class to SelectTrigger components for improved styling
- Fixed Media Item Page background so that it doesn't move when scrollbar is hidden/shown
- Stop flickering on test connection button by adding an artificial delay

---

## [0.9.26] - 2025-09-25

### Added

- Added GitHub icon next to issue links in changelog for better visibility

### Fixed

- Improve button active state on click for enhanced user experience
- Prevent text selection in badge component for improved usability
- Standardize Destructive button styles across the application
- Rename "Default Image Types" to "Download Defaults" in User Preferences for clarity

---

## [0.9.25] - 2025-09-25

### Added

- [#96](https://github.com/mediux-team/AURA/issues/96) - Added new popup confirmation for destructive actions

### Fixed

- Fixed issue with clearing app cache not working properly on User Preferences Store

---

## [0.9.24] - 2025-09-25

### Added

- Added new changelog page

### Fixed

- Fixed issue with auth token handling

---
