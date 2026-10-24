package sonarr_radarr

import (
	"encoding/json"
)

// SR_Item is a Sonarr series or Radarr movie. Every field is kept as Sonarr/Radarr sent it,
// so updating the tags doesn't drop or change any other field.
type SR_Item struct {
	ID     int64
	Tags   []int64
	fields map[string]json.RawMessage
}

// parseSRItem reads the ID and tags of a Sonarr series or Radarr movie, and keeps the rest as it is
func parseSRItem(body []byte) (item SR_Item, err error) {
	if err := json.Unmarshal(body, &item.fields); err != nil {
		return item, err
	}
	var known struct {
		ID   int64   `json:"id"`
		Tags []int64 `json:"tags"`
	}
	if err := json.Unmarshal(body, &known); err != nil {
		return item, err
	}
	item.ID, item.Tags = known.ID, known.Tags
	return item, nil
}

// MarshalJSON is the item as Sonarr/Radarr sent it, with its tags
func (item SR_Item) MarshalJSON() ([]byte, error) {
	fields := make(map[string]json.RawMessage, len(item.fields)+1)
	for key, value := range item.fields {
		fields[key] = value
	}
	// An item without tags still sends an empty list, so its last tag can be removed
	tags := item.Tags
	if tags == nil {
		tags = []int64{}
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}
	fields["tags"] = encoded
	return json.Marshal(fields)
}
