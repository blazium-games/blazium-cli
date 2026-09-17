package games

// Result is agent-usable output from games build / addfiles.
type Result struct {
	Status  string `json:"status"`
	BuildID string `json:"build_id,omitempty"`
	AppID   string `json:"app_id,omitempty"`
	FileUID string `json:"file_uid,omitempty"`
}

func resultFromData(data map[string]interface{}) *Result {
	r := &Result{Status: "ok"}
	if data == nil {
		return r
	}
	r.BuildID = responseBuildID(data)
	if id, _ := data["app_id"].(string); id != "" {
		r.AppID = id
	}
	if uid, _ := data["file_uid"].(string); uid != "" {
		r.FileUID = uid
	}
	return r
}
