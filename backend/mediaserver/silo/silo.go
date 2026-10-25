package silo

import "time"

type SiloProblemResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

type SiloSystemInfoResponse struct {
	ServerVersion  string `json:"server_version"`
	APIMajor       int    `json:"api_major"`
	ContractDigest string `json:"contract_digest"`
}

type SiloBrandingResponse struct {
	ServerName string `json:"server_name"`
}

type SiloAccountResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type SiloLibrariesResponse struct {
	Items []struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Type    string   `json:"type"`
		Paths   []string `json:"paths"`
		Enabled bool     `json:"enabled"`
	} `json:"items"`
}

type SiloProfilesResponse struct {
	Items []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsPrimary bool   `json:"is_primary"`
	} `json:"items"`
}

type SiloCatalogResponse struct {
	Items []SiloCatalogItem `json:"items"`
	Page  struct {
		NextCursor string `json:"next_cursor"`
		HasMore    bool   `json:"has_more"`
	} `json:"page"`
	Total int `json:"total"`
}

type SiloCatalogItem struct {
	ContentID     string    `json:"content_id"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
	Year          int       `json:"year"`
	ContentRating string    `json:"content_rating"`
	Overview      string    `json:"overview"`
	ReleaseDate   string    `json:"release_date"`
	AddedAt       time.Time `json:"added_at"`
}

type SiloCatalogItemDetailResponse struct {
	ContentID     string   `json:"content_id"`
	Type          string   `json:"type"`
	Title         string   `json:"title"`
	Year          int      `json:"year"`
	ContentRating string   `json:"content_rating"`
	Overview      string   `json:"overview"`
	TMDB_ID       string   `json:"tmdb_id"`
	IMDB_ID       string   `json:"imdb_id"`
	TVDB_ID       string   `json:"tvdb_id"`
	ReleaseDate   string   `json:"release_date"`
	FirstAirDate  string   `json:"first_air_date"`
	SeasonCount   int      `json:"season_count"`
	EpisodeCount  int      `json:"episode_count"`
	PosterURL     string   `json:"poster_url"`
	BackdropURL   string   `json:"backdrop_url"`
	FolderPaths   []string `json:"folder_paths"`
	UserRating    int      `json:"user_rating"`
	Ratings       []struct {
		Source string  `json:"source"`
		Score  float64 `json:"score"`
	} `json:"ratings"`
	Versions []struct {
		FileID   string    `json:"file_id"`
		FilePath string    `json:"file_path"`
		FileSize int64     `json:"file_size"`
		Duration int64     `json:"duration"`
		AddedAt  time.Time `json:"added_at"`
	} `json:"versions"`
}

type SiloSeasonsResponse struct {
	Items []struct {
		ContentID    string `json:"content_id"`
		SeasonNumber int    `json:"season_number"`
		Title        string `json:"title"`
	} `json:"items"`
}

type SiloEpisodesResponse struct {
	Items []struct {
		ContentID     string `json:"content_id"`
		SeasonNumber  int    `json:"season_number"`
		EpisodeNumber int    `json:"episode_number"`
		Title         string `json:"title"`
		Runtime       int64  `json:"runtime"`
		Files         []struct {
			FileID   string `json:"file_id"`
			FileSize int64  `json:"file_size"`
		} `json:"files"`
	} `json:"items"`
}

type SiloItemFilesResponse struct {
	Items []struct {
		ID       string `json:"id"`
		FilePath string `json:"file_path"`
	} `json:"items"`
	Page struct {
		NextCursor string `json:"next_cursor"`
		HasMore    bool   `json:"has_more"`
	} `json:"page"`
}

type SiloImageApplyRequest struct {
	Type        string `json:"type"`
	OriginalURL string `json:"original_url"`
}

type SiloImageApplyResponse struct {
	ContentID  string `json:"content_id"`
	StoredPath string `json:"stored_path"`
}

type SiloRefreshRequest struct {
	Mode string `json:"mode"`
}

type SiloJobResponse struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	State    string `json:"state"`
	Terminal bool   `json:"terminal"`
}

type SiloRatingRequest struct {
	Rating int `json:"rating"`
}
