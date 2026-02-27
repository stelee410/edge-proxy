package builtin

import (
	"encoding/json"
	"fmt"
	"io"
	"linkyun-edge-proxy/internal/commands"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// TrendingCommand 趋势命令
type TrendingCommand struct {
	chatClient interface{}
}

// NewTrendingCommand 创建趋势命令
func NewTrendingCommand(chatClient interface{}) *TrendingCommand {
	return &TrendingCommand{chatClient: chatClient}
}

func (c *TrendingCommand) Name() string              { return "trending" }
func (c *TrendingCommand) Description() string       { return "Get trending topics and popular repositories" }
func (c *TrendingCommand) Usage() string            { return "/trending [options]" }
func (c *TrendingCommand) Aliases() []string        { return []string{"trend", "hot"} }
func (c *TrendingCommand) Category() string        { return "Info" }

// TrendingOptions 趋势查询选项
type TrendingOptions struct {
	Source   string // github, hackernews, all
	Language string // 编程语言 (github)
	Period   string // daily, weekly, monthly (github)
	Category string // 分类
	Query    string // 搜索关键词
	Limit    int    // 结果数量
	Format   string // output format
}

func (c *TrendingCommand) Execute(ctx *commands.Context, args []string) (string, error) {
	opts := c.parseOptions(args)
	opts.setDefaultValues()

	var results []string

	switch opts.Source {
	case "github":
		result, err := c.getGithubTrending(opts)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	case "hackernews":
		result, err := c.getHackerNewsTrending(opts)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	case "all":
		github, err := c.getGithubTrending(opts)
		if err == nil {
			results = append(results, github)
		}
		hn, err := c.getHackerNewsTrending(opts)
		if err == nil {
			results = append(results, hn)
		}
	default:
		// 默认获取 GitHub 趋势
		result, err := c.getGithubTrending(opts)
		if err != nil {
			return "", err
		}
		results = append(results, result)
	}

	return strings.Join(results, "\n\n"), nil
}

func (c *TrendingCommand) Validate(args []string) error {
	return nil
}

func (c *TrendingCommand) parseOptions(args []string) *TrendingOptions {
	opts := &TrendingOptions{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source", "-s":
			if i+1 < len(args) {
				opts.Source = args[i+1]
				i++
			}
		case "--lang", "-l":
			if i+1 < len(args) {
				opts.Language = args[i+1]
				i++
			}
		case "--period", "-p":
			if i+1 < len(args) {
				opts.Period = args[i+1]
				i++
			}
		case "--category", "-c":
			if i+1 < len(args) {
				opts.Category = args[i+1]
				i++
			}
		case "--query", "-q":
			if i+1 < len(args) {
				opts.Query = args[i+1]
				i++
			}
		case "--limit", "-n":
			if i+1 < len(args) {
				var n int
				_, err := fmt.Sscanf(args[i+1], "%d", &n)
				if err == nil {
					opts.Limit = n
				}
				i++
			}
		case "--format", "-f":
			if i+1 < len(args) {
				opts.Format = args[i+1]
				i++
			}
		default:
			// 如果不是标志，则作为查询关键词
			if !strings.HasPrefix(args[i], "-") {
				if opts.Query == "" {
					opts.Query = args[i]
				} else {
					opts.Query += " " + args[i]
				}
			}
		}
	}

	return opts
}

func (o *TrendingOptions) setDefaultValues() {
	if o.Source == "" {
		o.Source = "github"
	}
	if o.Period == "" {
		o.Period = "daily"
	}
	if o.Limit == 0 {
		o.Limit = 10
	}
	if o.Format == "" {
		o.Format = "summary"
	}
}

// GitHub API 响应结构
type GithubRepo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
	Stars       int    `json:"stargazers_count"`
	Forks       int    `json:"forks_count"`
	Language    string `json:"language"`
	Topics      []string `json:"topics"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type GithubSearchResponse struct {
	Items []GithubRepo `json:"items"`
	TotalCount int      `json:"total_count"`
}

func (c *TrendingCommand) getGithubTrending(opts *TrendingOptions) (string, error) {
	// 构建查询 URL
	baseURL := "https://api.github.com/search/repositories"
	query := "is:public"

	if opts.Query != "" {
		query += " " + opts.Query
	}
	if opts.Language != "" {
		query += fmt.Sprintf(" language:%s", opts.Language)
	}

	// 根据时间段构建日期范围
	var dateFilter string
	now := time.Now()
	switch opts.Period {
	case "daily":
		dateFilter = now.AddDate(0, 0, -1).Format("2006-01-02")
	case "weekly":
		dateFilter = now.AddDate(0, 0, -7).Format("2006-01-02")
	case "monthly":
		dateFilter = now.AddDate(0, -1, 0).Format("2006-01-02")
	}
	if dateFilter != "" {
		query += fmt.Sprintf(" pushed:>%s", dateFilter)
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("sort", "stars")
	params.Set("order", "desc")
	params.Set("per_page", fmt.Sprintf("%d", opts.Limit))

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 发起请求
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Linkyun-Edge-Proxy")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch GitHub trending: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result GithubSearchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// 格式化输出
	return c.formatGithubResult(result, opts), nil
}

func (c *TrendingCommand) formatGithubResult(result GithubSearchResponse, opts *TrendingOptions) string {
	var sb strings.Builder

	period := opts.Period
	if period == "daily" {
		period = "today"
	} else if period == "weekly" {
		period = "this week"
	} else if period == "monthly" {
		period = "this month"
	}

	language := ""
	if opts.Language != "" {
		language = fmt.Sprintf(" (%s)", opts.Language)
	}
	query := ""
	if opts.Query != "" {
		query = fmt.Sprintf(" matching '%s'", opts.Query)
	}

	sb.WriteString(fmt.Sprintf("🔥 GitHub Trending %s%s%s\n", period, language, query))
	sb.WriteString(fmt.Sprintf("Found %d repositories\n\n", result.TotalCount))

	for i, repo := range result.Items {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, repo.FullName))

		if opts.Format == "detailed" {
			if repo.Description != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", truncateString(repo.Description, 80)))
			}
			sb.WriteString(fmt.Sprintf("   ⭐ %d stars  |  🍴 %d forks", repo.Stars, repo.Forks))
			if repo.Language != "" {
				sb.WriteString(fmt.Sprintf("  |  📦 %s", repo.Language))
			}
			sb.WriteString(fmt.Sprintf("  |  📅 %s\n", formatDate(repo.UpdatedAt)))
			sb.WriteString(fmt.Sprintf("   🔗 %s\n", repo.HTMLURL))

			if len(repo.Topics) > 0 {
				sb.WriteString(fmt.Sprintf("   Topics: %s\n", strings.Join(repo.Topics, ", ")))
			}
		} else {
			// summary format
			sb.WriteString(fmt.Sprintf("   ⭐ %d  ", repo.Stars))
			if repo.Language != "" {
				sb.WriteString(fmt.Sprintf("📦 %s  ", repo.Language))
			}
			if repo.Description != "" {
				sb.WriteString(fmt.Sprintf("- %s", truncateString(repo.Description, 60)))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Hacker News API 响应结构
type HNStory struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	Time        int64  `json:"time"`
	By          string `json:"by"`
}

func (c *TrendingCommand) getHackerNewsTrending(opts *TrendingOptions) (string, error) {
	// 获取 Hacker News Top Stories IDs
	url := "https://hacker-news.firebaseio.com/v0/topstories.json"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Linkyun-Edge-Proxy")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch HN stories: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var storyIDs []int
	if err := json.Unmarshal(body, &storyIDs); err != nil {
		return "", fmt.Errorf("failed to parse story IDs: %w", err)
	}

	// 限制数量
	limit := opts.Limit
	if limit > len(storyIDs) {
		limit = len(storyIDs)
	}

	// 获取故事详情
	var stories []HNStory
	for i := 0; i < limit; i++ {
		storyURL := fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", storyIDs[i])

		resp, err := client.Get(storyURL)
		if err != nil {
			continue
		}

		storyBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		var story HNStory
		if err := json.Unmarshal(storyBody, &story); err != nil {
			continue
		}

		stories = append(stories, story)
	}

	// 分类过滤
	if opts.Category != "" {
		stories = filterStories(stories, opts.Category)
	}

	// 排序
	sort.Slice(stories, func(i, j int) bool {
		return stories[i].Score > stories[j].Score
	})

	return c.formatHNResult(stories, opts), nil
}

func (c *TrendingCommand) formatHNResult(stories []HNStory, opts *TrendingOptions) string {
	var sb strings.Builder

	category := ""
	if opts.Category != "" {
		category = fmt.Sprintf(" [%s]", opts.Category)
	}

	sb.WriteString(fmt.Sprintf("📰 Hacker News Top Stories%s\n\n", category))

	for i, story := range stories {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, story.Title))

		if opts.Format == "detailed" {
			if story.URL != "" {
				sb.WriteString(fmt.Sprintf("   🔗 %s\n", story.URL))
			}
			sb.WriteString(fmt.Sprintf("   👍 %d points  |  💬 %d comments", story.Score, story.Descendants))
			sb.WriteString(fmt.Sprintf("  |  👤 by %s", story.By))
			sb.WriteString(fmt.Sprintf("  |  📅 %s\n", formatTimestamp(story.Time)))
		} else {
			// summary format
			sb.WriteString(fmt.Sprintf("   👍 %d", story.Score))
			if story.Descendants > 0 {
				sb.WriteString(fmt.Sprintf("  |  💬 %d", story.Descendants))
			}
			if story.URL != "" {
				sb.WriteString(fmt.Sprintf("  |  🔗 %s", truncateURL(story.URL)))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// 辅助函数

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func truncateURL(s string) string {
	if len(s) <= 50 {
		return s
	}
	// 尝试提取域名
	u, err := url.Parse(s)
	if err != nil {
		return s[:47] + "..."
	}
	if len(u.Host) > 50 {
		return u.Host[:47] + "..."
	}
	return u.Host
}

func formatDate(dateStr string) string {
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("2006-01-02")
}

func formatTimestamp(ts int64) string {
	t := time.Unix(ts, 0)
	return t.Format("2006-01-02")
}

func filterStories(stories []HNStory, category string) []HNStory {
	// 简单的关键词匹配
	var filtered []HNStory
	categoryLower := strings.ToLower(category)

	for _, story := range stories {
		titleLower := strings.ToLower(story.Title)
		urlLower := strings.ToLower(story.URL)

		if strings.Contains(titleLower, categoryLower) || strings.Contains(urlLower, categoryLower) {
			filtered = append(filtered, story)
		}
	}

	return filtered
}
