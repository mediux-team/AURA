package ej

import "time"

type EmbyJellyLibrarySectionsResponse struct {
	Items []struct {
		Name     string `json:"Name"`
		ServerID string `json:"ServerId"`
		ID       string `json:"Id"`
		GUID     string `json:"Guid"`
		IsFolder bool   `json:"IsFolder"`
		Type     string `json:"Type"`
		UserData struct {
			PlaybackPositionTicks int  `json:"PlaybackPositionTicks"`
			IsFavorite            bool `json:"IsFavorite"`
			Played                bool `json:"Played"`
		} `json:"UserData"`
		CollectionType string `json:"CollectionType"`
		ImageTags      struct {
			Primary string `json:"Primary"`
		} `json:"ImageTags"`
		BackdropImageTags []any `json:"BackdropImageTags"`
	} `json:"Items"`
	TotalRecordCount int `json:"TotalRecordCount"`
}

type EmbyJellyUserIDResponse []struct {
	Name                      string    `json:"Name"`
	ServerID                  string    `json:"ServerId"`
	ID                        string    `json:"Id"`
	HasPassword               bool      `json:"HasPassword"`
	HasConfiguredPassword     bool      `json:"HasConfiguredPassword"`
	HasConfiguredEasyPassword bool      `json:"HasConfiguredEasyPassword"`
	EnableAutoLogin           bool      `json:"EnableAutoLogin"`
	LastLoginDate             time.Time `json:"LastLoginDate"`
	LastActivityDate          time.Time `json:"LastActivityDate"`
	Configuration             struct {
		PlayDefaultAudioTrack      bool   `json:"PlayDefaultAudioTrack"`
		SubtitleLanguagePreference string `json:"SubtitleLanguagePreference"`
		DisplayMissingEpisodes     bool   `json:"DisplayMissingEpisodes"`
		GroupedFolders             []any  `json:"GroupedFolders"`
		SubtitleMode               string `json:"SubtitleMode"`
		DisplayCollectionsView     bool   `json:"DisplayCollectionsView"`
		EnableLocalPassword        bool   `json:"EnableLocalPassword"`
		OrderedViews               []any  `json:"OrderedViews"`
		LatestItemsExcludes        []any  `json:"LatestItemsExcludes"`
		MyMediaExcludes            []any  `json:"MyMediaExcludes"`
		HidePlayedInLatest         bool   `json:"HidePlayedInLatest"`
		RememberAudioSelections    bool   `json:"RememberAudioSelections"`
		RememberSubtitleSelections bool   `json:"RememberSubtitleSelections"`
		EnableNextEpisodeAutoPlay  bool   `json:"EnableNextEpisodeAutoPlay"`
		CastReceiverID             string `json:"CastReceiverId"`
	} `json:"Configuration"`
	Policy struct {
		IsAdministrator                  bool   `json:"IsAdministrator"`
		IsHidden                         bool   `json:"IsHidden"`
		EnableCollectionManagement       bool   `json:"EnableCollectionManagement"`
		EnableSubtitleManagement         bool   `json:"EnableSubtitleManagement"`
		EnableLyricManagement            bool   `json:"EnableLyricManagement"`
		IsDisabled                       bool   `json:"IsDisabled"`
		BlockedTags                      []any  `json:"BlockedTags"`
		AllowedTags                      []any  `json:"AllowedTags"`
		EnableUserPreferenceAccess       bool   `json:"EnableUserPreferenceAccess"`
		AccessSchedules                  []any  `json:"AccessSchedules"`
		BlockUnratedItems                []any  `json:"BlockUnratedItems"`
		EnableRemoteControlOfOtherUsers  bool   `json:"EnableRemoteControlOfOtherUsers"`
		EnableSharedDeviceControl        bool   `json:"EnableSharedDeviceControl"`
		EnableRemoteAccess               bool   `json:"EnableRemoteAccess"`
		EnableLiveTvManagement           bool   `json:"EnableLiveTvManagement"`
		EnableLiveTvAccess               bool   `json:"EnableLiveTvAccess"`
		EnableMediaPlayback              bool   `json:"EnableMediaPlayback"`
		EnableAudioPlaybackTranscoding   bool   `json:"EnableAudioPlaybackTranscoding"`
		EnableVideoPlaybackTranscoding   bool   `json:"EnableVideoPlaybackTranscoding"`
		EnablePlaybackRemuxing           bool   `json:"EnablePlaybackRemuxing"`
		ForceRemoteSourceTranscoding     bool   `json:"ForceRemoteSourceTranscoding"`
		EnableContentDeletion            bool   `json:"EnableContentDeletion"`
		EnableContentDeletionFromFolders []any  `json:"EnableContentDeletionFromFolders"`
		EnableContentDownloading         bool   `json:"EnableContentDownloading"`
		EnableSyncTranscoding            bool   `json:"EnableSyncTranscoding"`
		EnableMediaConversion            bool   `json:"EnableMediaConversion"`
		EnabledDevices                   []any  `json:"EnabledDevices"`
		EnableAllDevices                 bool   `json:"EnableAllDevices"`
		EnabledChannels                  []any  `json:"EnabledChannels"`
		EnableAllChannels                bool   `json:"EnableAllChannels"`
		EnabledFolders                   []any  `json:"EnabledFolders"`
		EnableAllFolders                 bool   `json:"EnableAllFolders"`
		InvalidLoginAttemptCount         int    `json:"InvalidLoginAttemptCount"`
		LoginAttemptsBeforeLockout       int    `json:"LoginAttemptsBeforeLockout"`
		MaxActiveSessions                int    `json:"MaxActiveSessions"`
		EnablePublicSharing              bool   `json:"EnablePublicSharing"`
		BlockedMediaFolders              []any  `json:"BlockedMediaFolders"`
		BlockedChannels                  []any  `json:"BlockedChannels"`
		RemoteClientBitrateLimit         int    `json:"RemoteClientBitrateLimit"`
		AuthenticationProviderID         string `json:"AuthenticationProviderId"`
		PasswordResetProviderID          string `json:"PasswordResetProviderId"`
		SyncPlayAccess                   string `json:"SyncPlayAccess"`
	} `json:"Policy"`
}

type EmbyJellyLibraryItemsResponse struct {
	Items []struct {
		Name                 string    `json:"Name"`
		ServerID             string    `json:"ServerId"`
		ID                   string    `json:"Id"`
		CanDelete            bool      `json:"CanDelete"`
		CanDownload          bool      `json:"CanDownload"`
		SupportsSync         bool      `json:"SupportsSync"`
		RunTimeTicks         int64     `json:"RunTimeTicks"`
		ProductionYear       int       `json:"ProductionYear"`
		DateCreated          time.Time `json:"DateCreated"`
		PremiereDate         time.Time `json:"PremiereDate"`
		DateLastContentAdded time.Time `json:"DateLastContentAdded"`
		IsFolder             bool      `json:"IsFolder"`
		Path                 string    `json:"Path"`
		ProviderIds          struct {
			Tvdb            string `json:"Tvdb"`
			Imdb            string `json:"Imdb"`
			Tmdb            string `json:"Tmdb"`
			Eidr            string `json:"EIDR"`
			Facebook        string `json:"Facebook"`
			Instagram       string `json:"Instagram"`
			OfficialWebsite string `json:"Official Website"`
			Reddit          string `json:"Reddit"`
			TVMaze          string `json:"TV Maze"`
			XTwitter        string `json:"X (Twitter)"`
			Wikidata        string `json:"Wikidata"`
			Wikipedia       string `json:"Wikipedia"`
			Youtube         string `json:"Youtube"`
		} `json:"ProviderIds"`
		Type     string `json:"Type"`
		UserData struct {
			PlaybackPositionTicks int  `json:"PlaybackPositionTicks"`
			PlayCount             int  `json:"PlayCount"`
			IsFavorite            bool `json:"IsFavorite"`
			Played                bool `json:"Played"`
		} `json:"UserData"`
		PrimaryImageAspectRatio float64 `json:"PrimaryImageAspectRatio"`
		ImageTags               struct {
			Primary string `json:"Primary"`
			Logo    string `json:"Logo"`
			Thumb   string `json:"Thumb"`
		} `json:"ImageTags,omitempty"`
		BackdropImageTags []string `json:"BackdropImageTags"`
		MediaType         string   `json:"MediaType"`
	} `json:"Items"`
	TotalRecordCount int `json:"TotalRecordCount"`
}

type EmbyJellyItemContentResponse struct {
	Name                  string    `json:"Name"`
	OriginalTitle         string    `json:"OriginalTitle"`
	ServerID              string    `json:"ServerId"`
	ID                    string    `json:"Id"`
	Etag                  string    `json:"Etag"`
	DateCreated           time.Time `json:"DateCreated"`
	PremiereDate          time.Time `json:"PremiereDate"`
	CanDelete             bool      `json:"CanDelete"`
	CanDownload           bool      `json:"CanDownload"`
	PresentationUniqueKey string    `json:"PresentationUniqueKey"`
	SupportsSync          bool      `json:"SupportsSync"`
	Container             string    `json:"Container"`
	SortName              string    `json:"SortName"`
	ForcedSortName        string    `json:"ForcedSortName"`
	ExternalUrls          []struct {
		Name string `json:"Name"`
		URL  string `json:"Url"`
	} `json:"ExternalUrls"`
	MediaSources []struct {
		Protocol             string `json:"Protocol"`
		ID                   string `json:"Id"`
		Path                 string `json:"Path"`
		Type                 string `json:"Type"`
		Container            string `json:"Container"`
		Size                 int64  `json:"Size"`
		Name                 string `json:"Name"`
		IsRemote             bool   `json:"IsRemote"`
		HasMixedProtocols    bool   `json:"HasMixedProtocols"`
		RunTimeTicks         int64  `json:"RunTimeTicks"`
		SupportsTranscoding  bool   `json:"SupportsTranscoding"`
		SupportsDirectStream bool   `json:"SupportsDirectStream"`
		SupportsDirectPlay   bool   `json:"SupportsDirectPlay"`
		IsInfiniteStream     bool   `json:"IsInfiniteStream"`
		RequiresOpening      bool   `json:"RequiresOpening"`
		RequiresClosing      bool   `json:"RequiresClosing"`
		RequiresLooping      bool   `json:"RequiresLooping"`
		SupportsProbing      bool   `json:"SupportsProbing"`
		MediaStreams         []struct {
			Codec                           string  `json:"Codec"`
			ColorTransfer                   string  `json:"ColorTransfer,omitempty"`
			ColorPrimaries                  string  `json:"ColorPrimaries,omitempty"`
			ColorSpace                      string  `json:"ColorSpace,omitempty"`
			TimeBase                        string  `json:"TimeBase,omitempty"`
			VideoRange                      string  `json:"VideoRange,omitempty"`
			DisplayTitle                    string  `json:"DisplayTitle"`
			IsInterlaced                    bool    `json:"IsInterlaced"`
			BitRate                         int     `json:"BitRate,omitempty"`
			BitDepth                        int     `json:"BitDepth,omitempty"`
			RefFrames                       int     `json:"RefFrames,omitempty"`
			IsDefault                       bool    `json:"IsDefault"`
			IsForced                        bool    `json:"IsForced"`
			IsHearingImpaired               bool    `json:"IsHearingImpaired"`
			Height                          int     `json:"Height,omitempty"`
			Width                           int     `json:"Width,omitempty"`
			AverageFrameRate                float64 `json:"AverageFrameRate,omitempty"`
			RealFrameRate                   float64 `json:"RealFrameRate,omitempty"`
			Profile                         string  `json:"Profile,omitempty"`
			Type                            string  `json:"Type"`
			AspectRatio                     string  `json:"AspectRatio,omitempty"`
			Index                           int     `json:"Index"`
			IsExternal                      bool    `json:"IsExternal"`
			IsTextSubtitleStream            bool    `json:"IsTextSubtitleStream"`
			SupportsExternalStream          bool    `json:"SupportsExternalStream"`
			Protocol                        string  `json:"Protocol"`
			PixelFormat                     string  `json:"PixelFormat,omitempty"`
			Level                           int     `json:"Level,omitempty"`
			IsAnamorphic                    bool    `json:"IsAnamorphic,omitempty"`
			ExtendedVideoType               string  `json:"ExtendedVideoType"`
			ExtendedVideoSubType            string  `json:"ExtendedVideoSubType"`
			ExtendedVideoSubTypeDescription string  `json:"ExtendedVideoSubTypeDescription"`
			AttachmentSize                  int     `json:"AttachmentSize"`
			Language                        string  `json:"Language,omitempty"`
			Title                           string  `json:"Title,omitempty"`
			DisplayLanguage                 string  `json:"DisplayLanguage,omitempty"`
			ChannelLayout                   string  `json:"ChannelLayout,omitempty"`
			Channels                        int     `json:"Channels,omitempty"`
			SampleRate                      int     `json:"SampleRate,omitempty"`
			SubtitleLocationType            string  `json:"SubtitleLocationType,omitempty"`
			Path                            string  `json:"Path,omitempty"`
		} `json:"MediaStreams"`
		Formats             []any `json:"Formats"`
		Bitrate             int   `json:"Bitrate"`
		RequiredHTTPHeaders struct {
		} `json:"RequiredHttpHeaders"`
		AddAPIKeyToDirectStreamURL bool   `json:"AddApiKeyToDirectStreamUrl"`
		ReadAtNativeFramerate      bool   `json:"ReadAtNativeFramerate"`
		DefaultAudioStreamIndex    int    `json:"DefaultAudioStreamIndex"`
		ItemID                     string `json:"ItemId"`
	} `json:"MediaSources"`
	CriticRating        int      `json:"CriticRating"`
	ProductionLocations []string `json:"ProductionLocations"`
	Path                string   `json:"Path"`
	OfficialRating      string   `json:"OfficialRating"`
	Overview            string   `json:"Overview"`
	Taglines            []string `json:"Taglines"`
	Genres              []string `json:"Genres"`
	CommunityRating     float64  `json:"CommunityRating"`
	RunTimeTicks        int64    `json:"RunTimeTicks"`
	Size                int64    `json:"Size"`
	FileName            string   `json:"FileName"`
	Bitrate             int      `json:"Bitrate"`
	ProductionYear      int      `json:"ProductionYear"`
	RemoteTrailers      []struct {
		URL string `json:"Url"`
	} `json:"RemoteTrailers"`
	ProviderIds struct {
		Tvdb            string `json:"Tvdb"`
		Imdb            string `json:"Imdb"`
		Tmdb            string `json:"Tmdb"`
		Eidr            string `json:"EIDR"`
		Facebook        string `json:"Facebook"`
		Instagram       string `json:"Instagram"`
		OfficialWebsite string `json:"Official Website"`
		Reddit          string `json:"Reddit"`
		TVMaze          string `json:"TV Maze"`
		XTwitter        string `json:"X (Twitter)"`
		Wikidata        string `json:"Wikidata"`
		Wikipedia       string `json:"Wikipedia"`
		Youtube         string `json:"Youtube"`
	} `json:"ProviderIds"`
	IsFolder bool   `json:"IsFolder"`
	ParentID string `json:"ParentId"`
	Type     string `json:"Type"`
	People   []struct {
		Name            string `json:"Name"`
		ID              string `json:"Id"`
		Type            string `json:"Type"`
		PrimaryImageTag string `json:"PrimaryImageTag,omitempty"`
	} `json:"People"`
	Studios []struct {
		Name string `json:"Name"`
		ID   any    `json:"Id"`
	} `json:"Studios"`
	GenreItems []struct {
		Name string `json:"Name"`
		ID   any    `json:"Id"`
	} `json:"GenreItems"`
	TagItems []struct {
		Name string `json:"Name"`
		ID   any    `json:"Id"`
	} `json:"TagItems"`
	LocalTrailerCount int `json:"LocalTrailerCount"`
	UserData          struct {
		PlaybackPositionTicks int  `json:"PlaybackPositionTicks"`
		PlayCount             int  `json:"PlayCount"`
		IsFavorite            bool `json:"IsFavorite"`
		Played                bool `json:"Played"`
	} `json:"UserData"`
	DisplayPreferencesID    string  `json:"DisplayPreferencesId"`
	PrimaryImageAspectRatio float64 `json:"PrimaryImageAspectRatio"`
	MediaStreams            []struct {
		Codec                           string  `json:"Codec"`
		ColorTransfer                   string  `json:"ColorTransfer,omitempty"`
		ColorPrimaries                  string  `json:"ColorPrimaries,omitempty"`
		ColorSpace                      string  `json:"ColorSpace,omitempty"`
		TimeBase                        string  `json:"TimeBase,omitempty"`
		VideoRange                      string  `json:"VideoRange,omitempty"`
		DisplayTitle                    string  `json:"DisplayTitle"`
		IsInterlaced                    bool    `json:"IsInterlaced"`
		BitRate                         int     `json:"BitRate,omitempty"`
		BitDepth                        int     `json:"BitDepth,omitempty"`
		RefFrames                       int     `json:"RefFrames,omitempty"`
		IsDefault                       bool    `json:"IsDefault"`
		IsForced                        bool    `json:"IsForced"`
		IsHearingImpaired               bool    `json:"IsHearingImpaired"`
		Height                          int     `json:"Height,omitempty"`
		Width                           int     `json:"Width,omitempty"`
		AverageFrameRate                float64 `json:"AverageFrameRate,omitempty"`
		RealFrameRate                   float64 `json:"RealFrameRate,omitempty"`
		Profile                         string  `json:"Profile,omitempty"`
		Type                            string  `json:"Type"`
		AspectRatio                     string  `json:"AspectRatio,omitempty"`
		Index                           int     `json:"Index"`
		IsExternal                      bool    `json:"IsExternal"`
		IsTextSubtitleStream            bool    `json:"IsTextSubtitleStream"`
		SupportsExternalStream          bool    `json:"SupportsExternalStream"`
		Protocol                        string  `json:"Protocol"`
		PixelFormat                     string  `json:"PixelFormat,omitempty"`
		Level                           int     `json:"Level,omitempty"`
		IsAnamorphic                    bool    `json:"IsAnamorphic,omitempty"`
		ExtendedVideoType               string  `json:"ExtendedVideoType"`
		ExtendedVideoSubType            string  `json:"ExtendedVideoSubType"`
		ExtendedVideoSubTypeDescription string  `json:"ExtendedVideoSubTypeDescription"`
		AttachmentSize                  int     `json:"AttachmentSize"`
		Language                        string  `json:"Language,omitempty"`
		Title                           string  `json:"Title,omitempty"`
		DisplayLanguage                 string  `json:"DisplayLanguage,omitempty"`
		ChannelLayout                   string  `json:"ChannelLayout,omitempty"`
		Channels                        int     `json:"Channels,omitempty"`
		SampleRate                      int     `json:"SampleRate,omitempty"`
		SubtitleLocationType            string  `json:"SubtitleLocationType,omitempty"`
		Path                            string  `json:"Path,omitempty"`
	} `json:"MediaStreams"`
	PartCount int `json:"PartCount"`
	ImageTags struct {
		Primary string `json:"Primary"`
		Banner  string `json:"Banner"`
		Logo    string `json:"Logo"`
		Thumb   string `json:"Thumb"`
	} `json:"ImageTags"`
	BackdropImageTags []string `json:"BackdropImageTags"`
	Chapters          []struct {
		StartPositionTicks int    `json:"StartPositionTicks"`
		Name               string `json:"Name"`
		MarkerType         string `json:"MarkerType"`
		ChapterIndex       int    `json:"ChapterIndex"`
	} `json:"Chapters"`
	MediaType         string    `json:"MediaType"`
	LockedFields      []any     `json:"LockedFields"`
	LockData          bool      `json:"LockData"`
	Width             int       `json:"Width"`
	Height            int       `json:"Height"`
	ChildCount        int       `json:"ChildCount"`
	Status            string    `json:"Status"`
	AirDays           []any     `json:"AirDays"`
	DisplayOrder      string    `json:"DisplayOrder"`
	EndDate           time.Time `json:"EndDate"`
	IndexNumber       int       `json:"IndexNumber"`
	IndexNumberEnd    int       `json:"IndexNumberEnd"`
	ParentIndexNumber int       `json:"ParentIndexNumber"`
}

type EmbyJellyItemContentChildResponse struct {
	Items            []EmbyJellyItemContentResponse `json:"Items"`
	TotalRecordCount int                            `json:"TotalRecordCount"`
}

type EmbyJellyVirtualFoldersResponse struct {
	Name      string   `json:"Name"`
	Locations []string `json:"Locations"`
	PathInfos []struct {
		Path string `json:"Path"`
	} `json:"PathInfos"`
	LibraryOptions struct {
		PathInfos []struct {
			Path string `json:"Path"`
		} `json:"PathInfos"`
	} `json:"LibraryOptions"`
}
