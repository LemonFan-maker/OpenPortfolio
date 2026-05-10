package model

type ProjectSummaryRequest struct {
	RepoName     string `json:"repoName" binding:"required"`
	ForceRefresh bool   `json:"forceRefresh"`
}

type SiteSummaryRequest struct {
	ForceRefresh bool `json:"forceRefresh"`
}

type ProjectSummary struct {
	ShortSummary string   `json:"shortSummary"`
	LongSummary  string   `json:"longSummary"`
	Highlights   []string `json:"highlights"`
	TechTags     []string `json:"techTags"`
}

type SiteSummarySection struct {
	Title string   `json:"title"`
	Body  string   `json:"body,omitempty"`
	Items []string `json:"items,omitempty"`
}

type SiteSummary struct {
	Title        string               `json:"title"`
	Subtitle     string               `json:"subtitle"`
	ShortSummary string               `json:"shortSummary"`
	LongSummary  string               `json:"longSummary"`
	Sections     []SiteSummarySection `json:"sections"`
	TechTags     []string             `json:"techTags"`
}

type SiteSummaryStatus struct {
	Source    string      `json:"source"`
	Error     string      `json:"error,omitempty"`
	Cached    bool        `json:"cached"`
	Summary   SiteSummary `json:"summary"`
	Generated string      `json:"generatedAt"`
}

type ProjectSummaryStatus struct {
	RepoName  string         `json:"repoName"`
	Source    string         `json:"source"`
	Error     string         `json:"error,omitempty"`
	Cached    bool           `json:"cached"`
	Summary   ProjectSummary `json:"summary"`
	Generated string         `json:"generatedAt"`
}

type ProjectSummaryDebug struct {
	RepoName  string                  `json:"repoName"`
	Source    string                  `json:"source"`
	Error     string                  `json:"error,omitempty"`
	Cached    bool                    `json:"cached"`
	Summary   ProjectSummary          `json:"summary"`
	Detail    RepoDetail              `json:"detail"`
	Tree      RepoTree                `json:"tree"`
	Generated string                  `json:"generatedAt"`
	Turns     []ProjectSummaryTurnLog `json:"turns"`
}

type ProjectSummaryTurnLog struct {
	Turn       int    `json:"turn"`
	Prompt     string `json:"prompt"`
	Response   string `json:"response"`
	Action     string `json:"action"`
	Path       string `json:"path,omitempty"`
	Query      string `json:"query,omitempty"`
	Reason     string `json:"reason,omitempty"`
	ToolResult string `json:"toolResult,omitempty"`
	Error      string `json:"error,omitempty"`
}
