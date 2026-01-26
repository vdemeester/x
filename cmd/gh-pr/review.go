package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go.sbr.pm/x/internal/output"
)

func reviewCmd(out *output.Writer) *cobra.Command {
	var (
		includeDiff     bool
		includeComments bool
		jsonOutput      bool
		llmFormat       bool
	)

	cmd := &cobra.Command{
		Use:   "review [<number> | <url> | <branch>]",
		Short: "Gather all context needed to review a pull request",
		Long: `Fetch comprehensive information about a pull request for review.

This command gathers all the context you need to review a PR in one call:
  - PR metadata (title, body, author, state, URL)
  - Files changed with diff statistics
  - CI/CD check status (passing/failing/pending)
  - Review comments with resolution status
  - Reviews (approved/changes requested/pending)
  - Commit list

Use --diff to include the full diff output.
Use --llm for a format optimized for AI/LLM consumption.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prRef := ""
			if len(args) > 0 {
				prRef = args[0]
			}
			return runReview(out, reviewOpts{
				prRef:           prRef,
				includeDiff:     includeDiff,
				includeComments: includeComments,
				jsonOutput:      jsonOutput,
				llmFormat:       llmFormat,
			})
		},
	}

	cmd.Flags().BoolVar(&includeDiff, "diff", false, "Include full diff output")
	cmd.Flags().BoolVarP(&includeComments, "comments", "c", true, "Include review comments")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&llmFormat, "llm", false, "Format output for LLM consumption (markdown)")

	return cmd
}

type reviewOpts struct {
	prRef           string
	includeDiff     bool
	includeComments bool
	jsonOutput      bool
	llmFormat       bool
}

// PRContext holds all the gathered PR information
type PRContext struct {
	Number          int             `json:"number"`
	Title           string          `json:"title"`
	Body            string          `json:"body"`
	Author          string          `json:"author"`
	State           string          `json:"state"`
	IsDraft         bool            `json:"isDraft"`
	URL             string          `json:"url"`
	BaseRef         string          `json:"baseRef"`
	HeadRef         string          `json:"headRef"`
	Mergeable       string          `json:"mergeable"`
	ReviewDecision  string          `json:"reviewDecision"`
	Additions       int             `json:"additions"`
	Deletions       int             `json:"deletions"`
	ChangedFiles    int             `json:"changedFiles"`
	Files           []FileChange    `json:"files"`
	Checks          []CheckStatus   `json:"checks"`
	Reviews         []Review        `json:"reviews"`
	Comments        []ReviewComment `json:"comments,omitempty"`
	Commits         []Commit        `json:"commits"`
	Diff            string          `json:"diff,omitempty"`
	ChecksSummary   ChecksSummary   `json:"checksSummary"`
	CommentsSummary CommentsSummary `json:"commentsSummary"`
}

type FileChange struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"` // added, modified, removed, renamed
}

type CheckStatus struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // completed, in_progress, queued
	Conclusion string `json:"conclusion"` // success, failure, skipped, etc
	URL        string `json:"url,omitempty"`
}

type ChecksSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
	Skipped int `json:"skipped"`
}

type Review struct {
	Author    string `json:"author"`
	State     string `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING
	Body      string `json:"body,omitempty"`
	SubmittedAt string `json:"submittedAt"`
}

type ReviewComment struct {
	ID         int    `json:"id"`
	Author     string `json:"author"`
	Body       string `json:"body"`
	Path       string `json:"path"`
	Line       int    `json:"line,omitempty"`
	IsResolved bool   `json:"isResolved"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	InReplyTo  int    `json:"inReplyTo,omitempty"`
}

type CommentsSummary struct {
	Total      int `json:"total"`
	Resolved   int `json:"resolved"`
	Unresolved int `json:"unresolved"`
}

type Commit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
}

func runReview(out *output.Writer, opts reviewOpts) error {
	// Fetch PR data using gh CLI
	ctx, err := fetchPRContext(opts)
	if err != nil {
		return err
	}

	// Output based on format
	if opts.jsonOutput {
		return outputJSON(ctx)
	}

	if opts.llmFormat {
		return outputLLM(out, ctx)
	}

	return outputHuman(out, ctx)
}

func fetchPRContext(opts reviewOpts) (*PRContext, error) {
	ctx := &PRContext{}

	// Parse repo from URL if provided
	repoFromURL := ""
	if strings.Contains(opts.prRef, "github.com") {
		repo, _ := parseGitHubURL(opts.prRef)
		if repo != "" {
			repoFromURL = repo
		}
	}

	// Build the gh pr view command with JSON output
	args := []string{"pr", "view"}
	if opts.prRef != "" {
		args = append(args, opts.prRef)
	}
	args = append(args, "--json",
		"number,title,body,author,state,isDraft,url,baseRefName,headRefName,"+
			"mergeable,reviewDecision,additions,deletions,changedFiles,files,"+
			"statusCheckRollup,reviews,commits")

	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR: %w", err)
	}

	// Parse the JSON response
	var prData map[string]interface{}
	if err := json.Unmarshal(output, &prData); err != nil {
		return nil, fmt.Errorf("failed to parse PR data: %w", err)
	}

	// Extract fields
	ctx.Number = int(prData["number"].(float64))
	ctx.Title = prData["title"].(string)
	ctx.Body = getString(prData, "body")
	ctx.State = prData["state"].(string)
	ctx.IsDraft = getBool(prData, "isDraft")
	ctx.URL = prData["url"].(string)
	ctx.BaseRef = prData["baseRefName"].(string)
	ctx.HeadRef = prData["headRefName"].(string)
	ctx.Mergeable = getString(prData, "mergeable")
	ctx.ReviewDecision = getString(prData, "reviewDecision")
	ctx.Additions = int(prData["additions"].(float64))
	ctx.Deletions = int(prData["deletions"].(float64))
	ctx.ChangedFiles = int(prData["changedFiles"].(float64))

	// Extract author
	if author, ok := prData["author"].(map[string]interface{}); ok {
		ctx.Author = getString(author, "login")
	}

	// Extract files
	if files, ok := prData["files"].([]interface{}); ok {
		for _, f := range files {
			file := f.(map[string]interface{})
			ctx.Files = append(ctx.Files, FileChange{
				Path:      getString(file, "path"),
				Additions: int(getFloat(file, "additions")),
				Deletions: int(getFloat(file, "deletions")),
				Status:    getString(file, "status"),
			})
		}
	}

	// Extract checks
	if checks, ok := prData["statusCheckRollup"].([]interface{}); ok {
		for _, c := range checks {
			check := c.(map[string]interface{})
			status := CheckStatus{
				Name:       getString(check, "name"),
				Status:     getString(check, "status"),
				Conclusion: getString(check, "conclusion"),
			}
			if detailsURL, ok := check["detailsUrl"].(string); ok {
				status.URL = detailsURL
			}
			ctx.Checks = append(ctx.Checks, status)
		}
	}

	// Calculate checks summary
	for _, check := range ctx.Checks {
		ctx.ChecksSummary.Total++
		switch strings.ToLower(check.Conclusion) {
		case "success":
			ctx.ChecksSummary.Passed++
		case "failure", "timed_out", "action_required":
			ctx.ChecksSummary.Failed++
		case "skipped", "neutral":
			ctx.ChecksSummary.Skipped++
		default:
			if check.Status != "completed" {
				ctx.ChecksSummary.Pending++
			}
		}
	}

	// Extract reviews
	if reviews, ok := prData["reviews"].([]interface{}); ok {
		for _, r := range reviews {
			review := r.(map[string]interface{})
			rev := Review{
				State:       getString(review, "state"),
				Body:        getString(review, "body"),
				SubmittedAt: getString(review, "submittedAt"),
			}
			if author, ok := review["author"].(map[string]interface{}); ok {
				rev.Author = getString(author, "login")
			}
			ctx.Reviews = append(ctx.Reviews, rev)
		}
	}

	// Extract commits
	if commits, ok := prData["commits"].([]interface{}); ok {
		for _, c := range commits {
			commit := c.(map[string]interface{})
			cm := Commit{
				SHA: getString(commit, "oid"),
			}
			if msgLines, ok := commit["messageHeadline"].(string); ok {
				cm.Message = msgLines
			}
			if authors, ok := commit["authors"].([]interface{}); ok && len(authors) > 0 {
				if author, ok := authors[0].(map[string]interface{}); ok {
					cm.Author = getString(author, "name")
				}
			}
			ctx.Commits = append(ctx.Commits, cm)
		}
	}

	// Fetch review comments if requested
	if opts.includeComments {
		comments, err := fetchReviewComments(opts.prRef, ctx.Number, repoFromURL)
		if err != nil {
			// Non-fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: could not fetch comments: %v\n", err)
		} else {
			ctx.Comments = comments
			for _, c := range comments {
				ctx.CommentsSummary.Total++
				if c.IsResolved {
					ctx.CommentsSummary.Resolved++
				} else {
					ctx.CommentsSummary.Unresolved++
				}
			}
		}
	}

	// Fetch diff if requested
	if opts.includeDiff {
		diff, err := fetchDiff(opts.prRef, repoFromURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch diff: %v\n", err)
		} else {
			ctx.Diff = diff
		}
	}

	return ctx, nil
}

func fetchReviewComments(prRef string, prNumber int, repoFromURL string) ([]ReviewComment, error) {
	// Use gh api to fetch review comments with thread info
	var comments []ReviewComment

	// Determine repo path for API
	repoPath := "{owner}/{repo}"
	if repoFromURL != "" {
		repoPath = repoFromURL
	}

	// Fetch review comments (on diff)
	args := []string{"api", fmt.Sprintf("repos/%s/pulls/%d/comments", repoPath, prNumber)}
	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var rawComments []map[string]interface{}
	if err := json.Unmarshal(output, &rawComments); err != nil {
		return nil, err
	}

	for _, c := range rawComments {
		comment := ReviewComment{
			ID:        int(getFloat(c, "id")),
			Body:      getString(c, "body"),
			Path:      getString(c, "path"),
			URL:       getString(c, "html_url"),
			CreatedAt: getString(c, "created_at"),
		}

		if user, ok := c["user"].(map[string]interface{}); ok {
			comment.Author = getString(user, "login")
		}

		if line, ok := c["line"].(float64); ok {
			comment.Line = int(line)
		}

		if inReplyTo, ok := c["in_reply_to_id"].(float64); ok {
			comment.InReplyTo = int(inReplyTo)
		}

		comments = append(comments, comment)
	}

	// Fetch review threads to get resolution status (GraphQL)
	threadsQuery := `
query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100) {
        nodes {
          isResolved
          comments(first: 1) {
            nodes {
              databaseId
            }
          }
        }
      }
    }
  }
}`

	// Get owner/repo - prefer from URL if available
	var owner, repo string
	if repoFromURL != "" {
		parts := strings.Split(repoFromURL, "/")
		if len(parts) == 2 {
			owner = parts[0]
			repo = parts[1]
		}
	}

	// Fall back to current repo context
	if owner == "" || repo == "" {
		repoCmd := exec.Command("gh", "repo", "view", "--json", "owner,name")
		repoOutput, err := repoCmd.Output()
		if err == nil {
			var repoInfo map[string]interface{}
			if json.Unmarshal(repoOutput, &repoInfo) == nil {
				owner = getString(repoInfo, "owner")
				repo = getString(repoInfo, "name")
			}
		}
	}

	if owner != "" && repo != "" {
		gqlArgs := []string{"api", "graphql",
			"-f", fmt.Sprintf("query=%s", threadsQuery),
			"-F", fmt.Sprintf("owner=%s", owner),
			"-F", fmt.Sprintf("repo=%s", repo),
			"-F", fmt.Sprintf("number=%d", prNumber),
		}
		gqlCmd := exec.Command("gh", gqlArgs...)
		gqlOutput, err := gqlCmd.Output()
		if err == nil {
			var gqlResult map[string]interface{}
			if json.Unmarshal(gqlOutput, &gqlResult) == nil {
				// Build map of comment ID -> isResolved
				resolvedMap := make(map[int]bool)
				if data, ok := gqlResult["data"].(map[string]interface{}); ok {
					if repository, ok := data["repository"].(map[string]interface{}); ok {
						if pr, ok := repository["pullRequest"].(map[string]interface{}); ok {
							if threads, ok := pr["reviewThreads"].(map[string]interface{}); ok {
								if nodes, ok := threads["nodes"].([]interface{}); ok {
									for _, n := range nodes {
										node := n.(map[string]interface{})
										isResolved := getBool(node, "isResolved")
										if nodeComments, ok := node["comments"].(map[string]interface{}); ok {
											if commentNodes, ok := nodeComments["nodes"].([]interface{}); ok {
												for _, cn := range commentNodes {
													cNode := cn.(map[string]interface{})
													if dbID, ok := cNode["databaseId"].(float64); ok {
														resolvedMap[int(dbID)] = isResolved
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}

				// Update comments with resolution status
				for i := range comments {
					if resolved, ok := resolvedMap[comments[i].ID]; ok {
						comments[i].IsResolved = resolved
					}
				}
			}
		}
	}

	return comments, nil
}

func fetchDiff(prRef string, repoFromURL string) (string, error) {
	args := []string{"pr", "diff"}
	if prRef != "" {
		args = append(args, prRef)
	}

	// Add repo flag if we have it from URL
	if repoFromURL != "" {
		args = append(args, "-R", repoFromURL)
	}

	args = append(args, "--color=never")

	cmd := exec.Command("gh", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// parseGitHubURL extracts owner/repo from a GitHub PR URL
func parseGitHubURL(url string) (repo string, prNumber int) {
	// Handle URLs like:
	// https://github.com/owner/repo/pull/123
	// github.com/owner/repo/pull/123

	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "github.com/")

	parts := strings.Split(url, "/")
	if len(parts) >= 4 && parts[2] == "pull" {
		repo = parts[0] + "/" + parts[1]
		if num, err := strconv.Atoi(parts[3]); err == nil {
			prNumber = num
		}
	}
	return
}

func outputJSON(ctx *PRContext) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(ctx)
}

func outputLLM(out *output.Writer, ctx *PRContext) error {
	var sb strings.Builder

	// PR metadata section
	sb.WriteString("=== PULL REQUEST ===\n")
	sb.WriteString(fmt.Sprintf("NUMBER: %d\n", ctx.Number))
	sb.WriteString(fmt.Sprintf("TITLE: %s\n", ctx.Title))
	sb.WriteString(fmt.Sprintf("AUTHOR: %s\n", ctx.Author))
	sb.WriteString(fmt.Sprintf("STATE: %s\n", ctx.State))
	if ctx.IsDraft {
		sb.WriteString("DRAFT: true\n")
	}
	sb.WriteString(fmt.Sprintf("BRANCH: %s -> %s\n", ctx.HeadRef, ctx.BaseRef))
	sb.WriteString(fmt.Sprintf("URL: %s\n", ctx.URL))
	sb.WriteString(fmt.Sprintf("REVIEW_DECISION: %s\n", ctx.ReviewDecision))
	sb.WriteString(fmt.Sprintf("MERGEABLE: %s\n", ctx.Mergeable))

	// Description
	if ctx.Body != "" {
		sb.WriteString("DESCRIPTION:\n")
		sb.WriteString(ctx.Body)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Files changed
	sb.WriteString("=== FILES ===\n")
	sb.WriteString(fmt.Sprintf("TOTAL: %d (+%d/-%d)\n", ctx.ChangedFiles, ctx.Additions, ctx.Deletions))
	for _, f := range ctx.Files {
		sb.WriteString(fmt.Sprintf("FILE: %s (+%d/-%d)\n", f.Path, f.Additions, f.Deletions))
	}
	sb.WriteString("\n")

	// CI Status
	sb.WriteString("=== CI STATUS ===\n")
	sb.WriteString(fmt.Sprintf("PASSED: %d/%d\n", ctx.ChecksSummary.Passed, ctx.ChecksSummary.Total))
	if ctx.ChecksSummary.Failed > 0 {
		sb.WriteString(fmt.Sprintf("FAILED: %d\n", ctx.ChecksSummary.Failed))
		for _, check := range ctx.Checks {
			if strings.ToLower(check.Conclusion) == "failure" ||
				strings.ToLower(check.Conclusion) == "timed_out" {
				sb.WriteString(fmt.Sprintf("FAILED_CHECK: %s\n", check.Name))
			}
		}
	}
	if ctx.ChecksSummary.Pending > 0 {
		sb.WriteString(fmt.Sprintf("PENDING: %d\n", ctx.ChecksSummary.Pending))
	}
	sb.WriteString("\n")

	// Reviews
	sb.WriteString("=== REVIEWS ===\n")
	if len(ctx.Reviews) == 0 {
		sb.WriteString("NONE\n")
	} else {
		for _, r := range ctx.Reviews {
			sb.WriteString(fmt.Sprintf("REVIEW: %s by @%s\n", r.State, r.Author))
			if r.Body != "" {
				sb.WriteString(fmt.Sprintf("REVIEW_BODY:\n%s\n", r.Body))
			}
		}
	}
	sb.WriteString("\n")

	// Review Comments - show unresolved first
	if len(ctx.Comments) > 0 {
		sb.WriteString("=== REVIEW COMMENTS ===\n")
		sb.WriteString(fmt.Sprintf("TOTAL: %d\n", ctx.CommentsSummary.Total))
		sb.WriteString(fmt.Sprintf("UNRESOLVED: %d\n", ctx.CommentsSummary.Unresolved))
		sb.WriteString(fmt.Sprintf("RESOLVED: %d\n", ctx.CommentsSummary.Resolved))
		sb.WriteString("\n")

		// Build reply map for threading
		replyMap := make(map[int][]ReviewComment)
		for _, c := range ctx.Comments {
			if c.InReplyTo != 0 {
				replyMap[c.InReplyTo] = append(replyMap[c.InReplyTo], c)
			}
		}

		// Output unresolved comments first
		for _, c := range ctx.Comments {
			if c.InReplyTo == 0 && !c.IsResolved {
				outputLLMComment(&sb, c, replyMap)
			}
		}

		// Then resolved comments
		for _, c := range ctx.Comments {
			if c.InReplyTo == 0 && c.IsResolved {
				outputLLMComment(&sb, c, replyMap)
			}
		}
	}

	// Commits
	sb.WriteString("=== COMMITS ===\n")
	for _, c := range ctx.Commits {
		sha := c.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		sb.WriteString(fmt.Sprintf("COMMIT: %s %s\n", sha, c.Message))
	}
	sb.WriteString("\n")

	// Diff
	if ctx.Diff != "" {
		sb.WriteString("=== DIFF ===\n")
		sb.WriteString(ctx.Diff)
		sb.WriteString("\n")
	}

	fmt.Print(sb.String())
	return nil
}

func outputLLMComment(sb *strings.Builder, c ReviewComment, replyMap map[int][]ReviewComment) {
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("FILE: %s", c.Path))
	if c.Line > 0 {
		sb.WriteString(fmt.Sprintf(":%d", c.Line))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("COMMENT_ID: %d\n", c.ID))
	sb.WriteString(fmt.Sprintf("AUTHOR: %s\n", c.Author))
	sb.WriteString(fmt.Sprintf("URL: %s\n", c.URL))
	if c.IsResolved {
		sb.WriteString("STATUS: resolved\n")
	} else {
		sb.WriteString("STATUS: unresolved\n")
	}

	// Check for suggestion blocks in the body
	body := c.Body
	if strings.Contains(body, "```suggestion") {
		// Split suggestion from comment
		parts := strings.SplitN(body, "```suggestion", 2)
		if len(parts) == 2 {
			commentPart := strings.TrimSpace(parts[0])
			suggestionPart := parts[1]
			// Find end of suggestion block
			if endIdx := strings.Index(suggestionPart, "```"); endIdx != -1 {
				suggestion := strings.TrimSpace(suggestionPart[:endIdx])
				afterSuggestion := strings.TrimSpace(suggestionPart[endIdx+3:])

				if commentPart != "" {
					sb.WriteString(fmt.Sprintf("COMMENT:\n%s\n", commentPart))
				}
				if suggestion != "" {
					sb.WriteString(fmt.Sprintf("SUGGESTION:\n%s\n", suggestion))
				}
				if afterSuggestion != "" {
					sb.WriteString(fmt.Sprintf("COMMENT:\n%s\n", afterSuggestion))
				}
			} else {
				sb.WriteString(fmt.Sprintf("COMMENT:\n%s\n", body))
			}
		} else {
			sb.WriteString(fmt.Sprintf("COMMENT:\n%s\n", body))
		}
	} else {
		sb.WriteString(fmt.Sprintf("COMMENT:\n%s\n", body))
	}

	// Show replies
	if replies, ok := replyMap[c.ID]; ok && len(replies) > 0 {
		sb.WriteString("REPLIES:\n")
		for i, reply := range replies {
			sb.WriteString(fmt.Sprintf("  [%d] @%s: %s\n", i+1, reply.Author, strings.ReplaceAll(reply.Body, "\n", "\n      ")))
		}
	}
}

func outputHuman(out *output.Writer, ctx *PRContext) error {
	// Title and basic info
	out.Println("")
	out.Success("PR #%d: %s", ctx.Number, ctx.Title)
	out.Println("")

	// Metadata
	state := ctx.State
	if ctx.IsDraft {
		state += " (Draft)"
	}
	out.Println("  Author:     %s", ctx.Author)
	out.Println("  State:      %s", state)
	out.Println("  Branch:     %s -> %s", ctx.HeadRef, ctx.BaseRef)
	out.Println("  URL:        %s", ctx.URL)
	out.Println("  Mergeable:  %s", ctx.Mergeable)
	out.Println("  Decision:   %s", ctx.ReviewDecision)
	out.Println("")

	// Files summary
	out.Info("Files Changed: %d (+%d/-%d)", ctx.ChangedFiles, ctx.Additions, ctx.Deletions)
	for _, f := range ctx.Files {
		out.Println("  %s %s (+%d/-%d)", statusIcon(f.Status), f.Path, f.Additions, f.Deletions)
	}
	out.Println("")

	// Checks summary
	checksStatus := fmt.Sprintf("%d/%d passed", ctx.ChecksSummary.Passed, ctx.ChecksSummary.Total)
	if ctx.ChecksSummary.Failed > 0 {
		out.Error("CI Status: %s (%d failed)", checksStatus, ctx.ChecksSummary.Failed)
	} else if ctx.ChecksSummary.Pending > 0 {
		out.Warning("CI Status: %s (%d pending)", checksStatus, ctx.ChecksSummary.Pending)
	} else {
		out.Success("CI Status: %s", checksStatus)
	}

	// Show failed checks
	for _, check := range ctx.Checks {
		if strings.ToLower(check.Conclusion) == "failure" {
			out.Error("  X %s", check.Name)
		}
	}
	out.Println("")

	// Reviews
	out.Info("Reviews:")
	if len(ctx.Reviews) == 0 {
		out.Println("  No reviews yet")
	} else {
		for _, r := range ctx.Reviews {
			switch r.State {
			case "APPROVED":
				out.Success("  + APPROVED by @%s", r.Author)
			case "CHANGES_REQUESTED":
				out.Error("  ! CHANGES REQUESTED by @%s", r.Author)
			default:
				out.Println("  - %s by @%s", r.State, r.Author)
			}
		}
	}
	out.Println("")

	// Comments summary
	if len(ctx.Comments) > 0 {
		out.Info("Comments: %d total (%d unresolved)",
			ctx.CommentsSummary.Total, ctx.CommentsSummary.Unresolved)

		// Show unresolved comments
		for _, c := range ctx.Comments {
			if !c.IsResolved && c.InReplyTo == 0 {
				out.Warning("  [UNRESOLVED] %s:%d - @%s", c.Path, c.Line, c.Author)
				// Truncate long comments
				body := c.Body
				if len(body) > 100 {
					body = body[:100] + "..."
				}
				out.Println("    %s", strings.ReplaceAll(body, "\n", " "))
			}
		}
		out.Println("")
	}

	// Commits
	out.Info("Commits: %d", len(ctx.Commits))
	for _, c := range ctx.Commits {
		out.Println("  %s %s", c.SHA[:7], c.Message)
	}

	return nil
}

func statusIcon(status string) string {
	switch status {
	case "added":
		return "A"
	case "modified":
		return "M"
	case "removed":
		return "D"
	case "renamed":
		return "R"
	default:
		return "?"
	}
}

// Helper functions for safe type assertions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getOwnerRepo extracts owner and repo from current directory
func getOwnerRepo() (string, string, error) {
	cmd := exec.Command("gh", "repo", "view", "--json", "owner,name")
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	var info struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(output, &info); err != nil {
		return "", "", err
	}

	return info.Owner, info.Name, nil
}

// Unused but kept for reference
var _ = strconv.Itoa
