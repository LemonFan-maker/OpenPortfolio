package model

type GitHubRepo struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	FullName        string   `json:"fullName"`
	DefaultBranch   string   `json:"defaultBranch"`
	Description     *string  `json:"description"`
	HTMLURL         string   `json:"htmlUrl"`
	Homepage        *string  `json:"homepage"`
	Language        *string  `json:"language"`
	Topics          []string `json:"topics"`
	StargazersCount int      `json:"stargazersCount"`
	ForksCount      int      `json:"forksCount"`
	OpenIssuesCount int      `json:"openIssuesCount"`
	Fork            bool     `json:"fork"`
	Archived        bool     `json:"archived"`
	Private         bool     `json:"private"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
	PushedAt        string   `json:"pushedAt"`
}

type ProjectCard struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Language    string         `json:"language"`
	Languages   []LanguageStat `json:"languages"`
	Stars       int            `json:"stars"`
	Forks       int            `json:"forks"`
	Topics      []string       `json:"topics"`
	URL         string         `json:"url"`
	Homepage    string         `json:"homepage"`
	UpdatedAt   string         `json:"updatedAt"`
}

type LanguageStat struct {
	Name    string  `json:"name"`
	Bytes   int64   `json:"bytes"`
	Percent float64 `json:"percent"`
}

type LanguageStats struct {
	TotalBytes int64          `json:"totalBytes"`
	Languages  []LanguageStat `json:"languages"`
}

type ContributionDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
	Color string `json:"color"`
}

type ContributionWeek struct {
	FirstDay string            `json:"firstDay"`
	Days     []ContributionDay `json:"days"`
}

type ContributionMonth struct {
	Name       string `json:"name"`
	FirstDay   string `json:"firstDay"`
	TotalWeeks int    `json:"totalWeeks"`
	Year       int    `json:"year"`
}

type CommitActivity struct {
	TotalContributions int                 `json:"totalContributions"`
	MaxContributions   int                 `json:"maxContributions"`
	Weeks              []ContributionWeek  `json:"weeks"`
	Months             []ContributionMonth `json:"months"`
}

type CommitActivityDetail struct {
	Date         string         `json:"date"`
	TotalCommits int            `json:"totalCommits"`
	Commits      []CommitRecord `json:"commits"`
	Truncated    bool           `json:"truncated"`
}

type CommitRecord struct {
	SHA          string           `json:"sha"`
	Message      string           `json:"message"`
	Repository   string           `json:"repository"`
	URL          string           `json:"url"`
	CommittedAt  string           `json:"committedAt"`
	Additions    int              `json:"additions"`
	Deletions    int              `json:"deletions"`
	Files        []CommitFileDiff `json:"files"`
	FilesChanged int              `json:"filesChanged"`
}

type CommitFileDiff struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

type RepoDetail struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Readme      string         `json:"readme"`
	Language    string         `json:"language"`
	Languages   []LanguageStat `json:"languages"`
	Stars       int            `json:"stars"`
	Forks       int            `json:"forks"`
	OpenIssues  int            `json:"openIssues"`
	Topics      []string       `json:"topics"`
	URL         string         `json:"url"`
	Homepage    string         `json:"homepage"`
	UpdatedAt   string         `json:"updatedAt"`
}

type RepoTree struct {
	DefaultBranch string          `json:"defaultBranch"`
	Truncated     bool            `json:"truncated"`
	Entries       []RepoTreeEntry `json:"entries"`
}

type RepoTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
}

type RepoFile struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	Size      int64  `json:"size"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

type RepoSearchMatch struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Snippet string `json:"snippet"`
}
