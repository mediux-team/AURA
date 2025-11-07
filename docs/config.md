---
layout: default
title: "Configuration"
nav_order: 2
description: "Configuration instructions for aura."
permalink: /config
---

# Configuration

aura uses a `config.yaml` file for configuration. You can setup the configuration file during the onboarding process. However, if you would like, below are the instructions for creating and modifying the `config.yaml` file.

1. **Create the `config.yaml` File**:

   - You can create a new file named `config.yaml` in the root directory of your aura installation.

2. **Edit the `config.yaml` File**:
   - Open the `config.yaml` file in a text editor of your choice.
   - Modify the configuration settings according to your needs.
3. **Place the `config.yaml` File**:
   - Place your configuration file in the `/config` directory on your Docker container.

---

# Configuration Options

## Authentication

- **Example**:

```yaml
Auth:
  Enable: true
  Password: $argon2id$v=19$m=16,t=2,p=1$Wlp5RGd4dTNkdmVGVDRkMg$2QEi6FDa4BWDxuGrzhjuVw
```

While this password authentication method is effective, it is important to keep your password secure and not share it with others. For enhanced security, consider using solutions like [Authentik](https://goauthentik.io/) or [Authelia](https://www.authelia.com/).  
**I am not a security expert** 😅

### Enable

- **Default**: `false`
- **Options**: `true` or `false`
- **Description**: Whether to enable authentication.
- **Details**: If set to `true`, you will be required to authenticate before accessing the application.

### Password

- **Default**: `null`
- **Options**: Any valid Argon2id hash
- **Description**: The password hash used to authenticate user.
- **Details**: This password is used to authenticate user when they log in to the application. It is recommended to use a strong, unique password for this purpose. You can generate a new Argon2id hash using tools like [Argon2 Online](https://argon2.online/). You can use the default settings.
  ![Argon2 Online](assets/argon2-online.png)

---

## Logging

- **Example**:

```yaml
Logging:
  Level: DEBUG
```

### Level

- **Default**: `TRACE`
- **Options**: `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`
- **Description**: The logging level for aura.
- **Details**:
  - `TRACE`: Most detailed logging, useful for debugging.
  - `DEBUG`: Less detailed than TRACE, but still provides useful information for debugging.
  - `INFO`: General information about the application's operation.
  - `WARN`: Indicates potential issues that are not necessarily errors.
  - `ERROR`: Indicates errors that occur during the application's operation.
- **Note**: The logging level can be adjusted based on your needs. For production environments, it is recommended to use `INFO` or `WARN` to reduce log verbosity. If you run into issues, you can temporarily set it to `DEBUG` or `TRACE` for more detailed logs.

---

## MediaServer

- **Example for Plex**:

```yaml
MediaServer:
  Type: Plex
  URL: http://localhost:32400
  Token: YOUR_PLEX_API_TOKEN_HERE
  Libraries:
    - Name: "Movies"
    - Name: "TV Shows"
```

-- **Example for Emby**:

```yaml
MediaServer:
  Type: Emby
  URL: http://localhost:8096
  Token: YOUR_EMBY_API_TOKEN_HERE
  Libraries:
    - Name: "Movies"
    - Name: "TV Shows"
```

-- **Example for Jellyfin**:

```yaml
MediaServer:
  Type: Jellyfin
  URL: http://localhost:8096
  Token: YOUR_JELLYFIN_API_TOKEN_HERE
  Libraries:
    - Name: "Movies"
    - Name: "TV Shows"
```

### Type

- **Options**: `Plex`, `Emby`, `Jellyfin`
- **Description**: The type of media server you are using.
- **Details**: This option specifies the type of media server that aura will interact with. Depending on your choice, aura will use the appropriate API and methods to manage images and metadata.

### URL

- **Description**: The URL of the media server.
- **Details**: This option specifies the URL of the media server that aura will interact with.
- **Note**: Make sure to include the protocol (e.g., `http://` or `https://`) in the URL.
- **Example**: `http://localhost:32400`, `https://my-emby-server.com`, or `http://jellyfin.example.com`.

### Token

- **Description**: The authentication token for the media server.
- **Details**: This option specifies the authentication token required to access the media server's API. You can obtain this token from your media server's settings or API documentation.
- **Note**: The token is necessary for aura to authenticate and perform actions on your media server. Make sure to keep this token secure and do not share it publicly.

### Libraries

- **Description**: The name of the media server library to use.
- **Details**: This option specifies the name of the library on your media server that aura will interact with. aura will use this library to manage images and metadata.
- **Note**: Ensure that the library name matches exactly with the name on your media server, including case sensitivity. Only show and movies libraries are supported.

---

## Mediux

- **Example**:

```yaml
Mediux:
  APIKey: YOUR_MEDIUX_API_KEY_HERE
  DownloadQuality: optimized
```

### APIKey

- **Description**: The API key for Mediux.
- **Details**: This option specifies the API key required to access Mediux's API. This can be obtained by creating an account on [Mediux](https://mediux.io/) and generating an API key in your account settings.
- **Note**: This is not yet available to the public, but will be in the future.
  If you would like to test out aura, please contact us on [Discord](https://discord.gg/YAKzwKPwyw) to get access to the API key.

### DownloadQuality

- **Default**: `optimized`
- **Options**: `optimized`, `original`
- **Description**: The quality of images to download from Mediux.
- **Details**: This option specifies the quality of images to download from Mediux.
  - `optimized`: Downloads images that are optimized for space savings and performance.
  - `original`: Downloads the original images without any optimization.

---

## AutoDownload

- **Example**:

```yaml
AutoDownload:
  Enabled: true
  Cron: "0 0 * * *"
```

### Enabled

- **Default**: `false`
- **Options**: `true` or `false`
- **Description**: Whether to automatically download images from updated sets.
- **Details**: When downloading images, you have the option to saved sets for "Automatic Downloads". If this option is enabled, aura will automatically download images from sets that have been updated. This is useful for keeping your media library up-to-date with the latest images without manual intervention.
- **Note**: Enabling this option may result in increased network usage as aura will periodically check for updates and download new images.

### Cron

- **Default**: `0 0 * * *`
- **Options**: Cron expression
- **Description**: The cron expression for scheduling automatic downloads.
- **Details**: This cron expression determines how often aura checks for updates and downloads images. The default value `0 0 * * *` means that aura will check for updates every day at midnight. You can modify this expression to change the frequency of automatic downloads according to your needs.
  **Note**: Make sure to use a valid cron expression. You can use online tools like [crontab.guru](https://crontab.guru/) to help you create and validate cron expressions.

---

## Images

- **Example**:

```yaml
Images:
  CacheImages:
    Enabled: false
  SaveImagesLocally:
    Enabled: false
    Path: ""
    SeasonNamingConvention: "2"
    EpisodeNamingConvention: "match"
```

## CacheImages.Enabled

- **Default**: `false`
- **Options**: `true` or `false`
- **Description**: Whether to cache images locally.
- **Details**: If set to `true`, aura will cache images to reduce load times and improve performance. This is particularly useful for frequently accessed images.Keep in mind that enabling this option will increase disk space usage as images will be stored locally.

## SaveImagesLocally.Enabled

- **Default:** `false`
- **Options:** `true` or `false`
- **Description:** Whether to save images locally.
- **Details:**
  - If `true`, images are saved in the same directory as the Media Server content.
  - If `false`, images are updated on the Media Server but not saved next to the content.
  - For **Emby** or **Jellyfin**, this option is ignored (handled by the server).
  - For **Plex**, this option determines if images are saved next to content.

## SaveImagesLocally.Path

- **Default:** `""` (empty string)
- **Options:** Any valid file path
- **Description:** The path where images should be saved if `SaveImagesLocally.Enabled` is `true`.
- **Details:**
  - If set to a valid path, images will be saved to that directory.
  - If left empty, images will be saved next to the media content.
  - Ensure the specified path is added to your docker container.
  - Ensure the specified path is writable by the application.

## SaveImagesLocally.SeasonNamingConvention

- **Default:** `"2"`
- **Options:** `"1"` or `"2"`
- **Description:** The naming convention for season images when saving locally.
- **Details:**
  - `"1"`: Uses the format `Season 1`, `Season 2`,
  - `"2"`: Uses the format `Season 01`, `Season 02`.
- **Note:** This option is only applicable when using Plex as the Media Server.

## SaveImagesLocally.EpisodeNamingConvention

- **Default:** `"match"`
- **Options:** `"match"` or `"static"`
- **Description:** The naming convention for episode images when saving locally.
- **Details:**
  - `"match"`: Episode images will match the episode file name.
  - `"static"`: Episode images will use a static naming format like `S01E01.jpg`.
- **Note:** This option is only applicable when using Plex as the Media Server. Choosing static will use the Season Naming Convention for level of zero padding.

---

## Labels and Tags

Aura supports adding and removing labels (tags) on Plex items after processing. This is useful for organizing your media library, marking items for automation, or integrating with other tools.

- **Example**:

```yaml
LabelsAndTags:
  Applications:
    - Application: Plex
      Enabled: true
      Add:
        - "Overlay"
        - "4K"
      Remove:
        - "OldLabel"
```

### Applications

- **Description**:  
  An array of label/tag configuration blocks, one per supported application (currently only Plex is supported).
- **Fields**:
  - `Application`: The name of the application (e.g., `Plex`).
  - `Enabled`: Set to `true` to enable label/tag management for this application.
  - `Add`: A list of labels/tags to add to items after processing.
  - `Remove`: A list of labels/tags to remove from items after processing.

#### Example Use Case

If you want Aura to add the labels `Overlay` and `4K` to your Plex items, and remove the label `OldLabel`, your config would look like:

```yaml
LabelsAndTags:
  Applications:
    - Application: Plex
      Enabled: true
      Add:
        - "Overlay"
        - "4K"
      Remove:
        - "OldLabel"
```

#### Notes

- You can leave `Add` or `Remove` empty if you only want to add or only want to remove labels.
- Only applications with `Enabled: true` will be processed.
- This structure is extensible for future support of other applications (such as Sonarr or Radarr).

---

## Notifications

Configure one or more providers. Notifications can be disabled globally or per provider.

Example:

```yaml
Notifications:
  Enabled: true # Master switch (false = ignore all providers)
  Providers:
    - Provider: "Discord"
      Enabled: true
      Discord:
        Webhook: "https://discord.com/api/webhooks/123456789/abcdefghijklmnopqrstuvwxyz"
    - Provider: "Pushover"
      Enabled: true
      Pushover:
        Token: YOUR_PUSHOVER_APP_TOKEN
        UserKey: YOUR_PUSHOVER_USER_KEY
    - Provider: "Gotify"
      Enabled: true
      Gotify:
        URL: YOUR_GOTIFY_SERVER_URL
        Token: YOUR_GOTIFY_APP_TOKEN
    - Provider: "Webhook"
      Enabled: true
      Webhook:
        URL: "https://your-webhook-url.com/endpoint"
        Headers:
          Some-Header: "HeaderValue"
          Another-Header: "AnotherValue"
```

### Structure

- Notifications.Enabled  
  Global on/off. If false, Providers are loaded but not used.
- Notifications.Providers[]  
  Array of provider entries.

### Provider Entry Fields

| Field            | Required                               | Notes                                        |
| ---------------- | -------------------------------------- | -------------------------------------------- |
| Provider         | yes                                    | Case-sensitive. Supported: Discord, Pushover |
| Enabled          | yes                                    | If false, entry kept but skipped             |
| Discord.Webhook  | yes (when Provider=Discord & Enabled)  | Full Discord webhook URL                     |
| Pushover.Token   | yes (when Provider=Pushover & Enabled) | Your app token                               |
| Pushover.UserKey | yes (when Provider=Pushover & Enabled) | Your user key                                |
| Gotify.URL       | yes (when Provider=Gotify & Enabled)   | Base URL for your Gotify server              |
| Gotify.Token     | yes (when Provider=Gotify & Enabled)   | Your Gotify app token                        |

---

## Sonarr / Radarr

Configure support for the Sonarr and Radarr applications.

Example:

```yaml
SonarrRadarr:
  Applications:
    - Type: Sonarr
      Library: Shows
      URL: http://<sonarr-url:port>
      APIKey: YOUR_SONARR_API_TOKEN
    - Type: Radarr
      Library: Movies
      URL: http://<radarr-url:port>
      APIKey: YOUR_RADARR_API_TOKEN
```

---

