package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"net/url"
	"path"
	"sort"
	"strconv"
)

func GetAllUsers(ctx context.Context) (users []models.MediuxUserInfo, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Getting All Users from MediUX", logging.LevelDebug)
	defer logAction.Complete()

	users = []models.MediuxUserInfo{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the MediUX API request
	u, err := url.Parse(MediuxApiURL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return users, *logAction.Error
	}
	u.Path = path.Join(u.Path, "items", "contributors")
	q := u.Query()
	q.Set("limit", "-1")
	u.RawQuery = q.Encode()
	URL := u.String()

	// Make the HTTP Request to MediUX
	resp, respBody, Err := makeRequest(ctx, URL, "GET", nil, "", false)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return users, *logAction.Error
	}
	defer resp.Body.Close()

	type MediuxUserInfoResponse struct {
		ID             string `json:"id"`
		DateUpdated    string `json:"date_updated"`
		ShowSets       int    `json:"show_sets"`
		MovieSets      int    `json:"movie_sets"`
		CollectionSets int    `json:"collection_sets"`
		Username       string `json:"username"`
		Avatar         string `json:"avatar"`
		TotalSets      string `json:"total_sets"`
	}

	type MediuxUsersResponse struct {
		Data []MediuxUserInfoResponse `json:"data"`
	}

	// Decode the Response
	var mediuxResp MediuxUsersResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &mediuxResp, "MediUX All Users Response")
	if Err.Message != "" {
		return users, Err
	}

	// Convert to MediuxUserInfo slice
	for _, user := range mediuxResp.Data {
		if user.Username == "" || (user.TotalSets == "0" && user.ShowSets == 0 && user.MovieSets == 0 && user.CollectionSets == 0) {
			continue
		}
		userInfo := models.MediuxUserInfo{
			ID:       user.ID,
			Username: user.Username,
			Avatar:   user.Avatar,
			TotalSets: func() int {
				if user.TotalSets != "" {
					val, err := strconv.Atoi(user.TotalSets)
					if err == nil {
						return val
					}
				}
				return user.ShowSets + user.MovieSets + user.CollectionSets
			}(),
		}
		users = append(users, userInfo)
	}

	// Sort the users by Total Sets descending
	sort.SliceStable(users, func(i, j int) bool {
		return users[i].TotalSets > users[j].TotalSets
	})

	// Load the list of users into the cache for faster access later
	cache.MediuxUsers.StoreMediuxUsers(users)
	logging.LOGGER.Info().Timestamp().Int("mediux_users", len(users)).Msg("Loaded MediUX users into cache")

	return users, Err
}
