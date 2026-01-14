package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"go.sbr.pm/x/internal/cache"
	"go.sbr.pm/x/internal/config"
	"go.sbr.pm/x/internal/deps"
	"go.sbr.pm/x/internal/output"
	"go.sbr.pm/x/internal/pr"
)

func runWatch(out *output.Writer, flags watchFlags) error {
	// Initialize cache
	depsCache, err := cache.New(24*time.Hour, "nixpkgs-pr-watch")
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	prCache, err := cache.New(6*time.Hour, "nixpkgs-pr-watch")
	if err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	// Determine which hosts to analyze
	cfg, err := config.New(flags.flakePath)
	if err != nil {
		return fmt.Errorf("failed to load flake configuration: %w", err)
	}

	var hostsToAnalyze []string
	if flags.allHosts {
		hostsToAnalyze, err = cfg.AllHosts()
		if err != nil {
			return fmt.Errorf("failed to get all hosts: %w", err)
		}
	} else {
		hostname := flags.host
		if hostname == "" {
			hostname, err = cfg.CurrentHost()
			if err != nil {
				return fmt.Errorf("failed to determine current host: %w", err)
			}
		}
		hostsToAnalyze = []string{hostname}
	}

	out.Info("Analyzing hosts: %v", hostsToAnalyze)

	// Extract dependencies for each host
	allDeps := make(map[string]*deps.Dependencies)
	totalPackages := 0
	totalModules := 0

	for _, hostname := range hostsToAnalyze {
		cacheKey := fmt.Sprintf("%s-deps", hostname)
		var hostDeps deps.Dependencies

		// Try to load from cache
		if !flags.refreshDeps {
			if err := depsCache.Get(cacheKey, &hostDeps); err == nil && len(hostDeps.Packages) > 0 {
				out.Info("  %s: loaded from cache (%d packages, %d modules)", hostname, len(hostDeps.Packages), len(hostDeps.Modules))
				allDeps[hostname] = &hostDeps
				totalPackages += len(hostDeps.Packages)
				totalModules += len(hostDeps.Modules)
				continue
			}
		}

		// Extract dependencies
		out.Info("  %s: extracting dependencies...", hostname)
		extractor := deps.NewExtractor(flags.flakePath, hostname)
		hostDeps, err = extractor.Extract()
		if err != nil {
			out.Warning("  %s: failed to extract dependencies: %v", hostname, err)
			continue
		}

		// Cache the results
		if err := depsCache.Set(cacheKey, hostDeps); err != nil {
			out.Warning("  %s: failed to cache dependencies: %v", hostname, err)
		}

		out.Info("  %s: found %d packages, %d modules", hostname, len(hostDeps.Packages), len(hostDeps.Modules))
		allDeps[hostname] = &hostDeps
		totalPackages += len(hostDeps.Packages)
		totalModules += len(hostDeps.Modules)
	}

	if len(allDeps) == 0 {
		return fmt.Errorf("no dependencies extracted from any host")
	}

	// Merge dependencies from all hosts
	merged := deps.Merge(allDeps)
	out.Info("Total unique: %d packages, %d modules", len(merged.Packages), len(merged.Modules))

	// Fetch PRs using incremental cache with smart merging
	out.Info("Fetching nixpkgs PRs (limit: %d)...", flags.limit)
	var prs []pr.PullRequest

	// Check cache metadata to see if we have cached PRs
	type prCacheMetadata struct {
		MaxLimit  int       `json:"max_limit"`
		FetchedAt time.Time `json:"fetched_at"`
		Cursor    string    `json:"cursor"` // GraphQL cursor for pagination
	}

	var metadata prCacheMetadata
	var cachedPRs []pr.PullRequest
	metadataKey := "nixpkgs-prs-metadata"
	prsKey := "nixpkgs-prs-data"

	// Load existing cache
	hasCachedPRs := false
	if !flags.refreshPRs {
		if err := prCache.Get(metadataKey, &metadata); err == nil {
			if err := prCache.Get(prsKey, &cachedPRs); err == nil && len(cachedPRs) > 0 {
				hasCachedPRs = true
			}
		}
	}

	// Decide what to fetch
	if hasCachedPRs && metadata.MaxLimit >= flags.limit {
		// Cache has enough PRs, use them
		prs = cachedPRs[:flags.limit]
		out.Info("Loaded %d PRs from cache (cached: %d, age: %v)",
			len(prs), len(cachedPRs), time.Since(metadata.FetchedAt).Round(time.Minute))
	} else if hasCachedPRs && metadata.MaxLimit < flags.limit {
		// Cache has some PRs but not enough - fetch additional PRs using cursor
		deltaNeeded := flags.limit - metadata.MaxLimit
		out.Info("Cache has %d PRs, fetching %d more using cursor...", metadata.MaxLimit, deltaNeeded)

		fetcher := pr.NewFetcher()
		newPRs, newCursor, err := fetcher.FetchNixpkgsPRsWithCursor(deltaNeeded, metadata.Cursor, flags.baseBranch)

		// Merge cached PRs with any new PRs we got (even if there was an error)
		prs = append(cachedPRs, newPRs...)

		// Update cache with combined results if we got new data
		if len(newPRs) > 0 {
			metadata = prCacheMetadata{
				MaxLimit:  len(prs),
				FetchedAt: time.Now(),
				Cursor:    newCursor,
			}
			if cacheErr := prCache.Set(prsKey, prs); cacheErr != nil {
				out.Warning("Failed to cache PRs: %v", cacheErr)
			}
			if cacheErr := prCache.Set(metadataKey, metadata); cacheErr != nil {
				out.Warning("Failed to cache metadata: %v", cacheErr)
			}
		}

		if err != nil {
			out.Warning("Failed to fetch additional PRs: %v", err)
			if len(newPRs) > 0 {
				out.Info("Cached partial results: %d previous + %d new = %d total PRs", len(cachedPRs), len(newPRs), len(prs))
			} else {
				out.Info("Using cached %d PRs instead", len(cachedPRs))
			}
		} else {
			out.Info("Fetched %d additional PRs, total: %d", len(newPRs), len(prs))
		}
	} else {
		// No cache or refresh requested - fetch fresh data using cursor-based API
		fetcher := pr.NewFetcher()
		var cursor string
		prs, cursor, err = fetcher.FetchNixpkgsPRsWithCursor(flags.limit, "", flags.baseBranch)

		// Cache partial results even if there was an error
		if len(prs) > 0 {
			metadata = prCacheMetadata{
				MaxLimit:  len(prs),
				FetchedAt: time.Now(),
				Cursor:    cursor,
			}
			if cacheErr := prCache.Set(prsKey, prs); cacheErr != nil {
				out.Warning("Failed to cache PRs: %v", cacheErr)
			}
			if cacheErr := prCache.Set(metadataKey, metadata); cacheErr != nil {
				out.Warning("Failed to cache metadata: %v", cacheErr)
			}

			if err != nil {
				out.Warning("Fetch incomplete due to error: %v", err)
				out.Info("Using %d PRs fetched before error", len(prs))
			} else {
				out.Info("Fetched %d PRs", len(prs))
			}
		} else if err != nil {
			return fmt.Errorf("failed to fetch PRs: %w", err)
		}
	}

	// Filter PRs by user if requested
	if flags.user != "" {
		var filteredPRs []pr.PullRequest
		for _, p := range prs {
			if p.Author == flags.user {
				filteredPRs = append(filteredPRs, p)
			}
		}
		out.Info("Filtered to %d PRs by user @%s", len(filteredPRs), flags.user)
		prs = filteredPRs
	}

	// Match PRs to dependencies
	out.Info("Matching PRs to dependencies...")
	matcher := pr.NewMatcher(merged)
	results := matcher.MatchAll(prs)

	// Filter by confidence
	var filtered []pr.MatchResult
	for _, result := range results {
		if shouldIncludeByConfidence(result, flags.minConfidence) {
			filtered = append(filtered, result)
		}
	}

	out.Info("Found %d matching PRs", len(filtered))

	// Sort results
	sortResults(filtered, flags.sortBy)

	// Output results
	switch flags.outputFormat {
	case "json":
		return outputJSON(filtered, merged, hostsToAnalyze)
	default:
		return outputTerminal(out, filtered, merged, hostsToAnalyze, flags)
	}
}

func shouldIncludeByConfidence(result pr.MatchResult, minConfidence string) bool {
	switch minConfidence {
	case "high":
		return result.HighestConfidence() == "high"
	case "medium":
		conf := result.HighestConfidence()
		return conf == "high" || conf == "medium"
	case "low":
		return true
	default:
		return result.HighestConfidence() != "low"
	}
}

func outputJSON(results []pr.MatchResult, deps *deps.Dependencies, hosts []string) error {
	output := map[string]interface{}{
		"metadata": map[string]interface{}{
			"timestamp":          time.Now().Format(time.RFC3339),
			"hosts_analyzed":     hosts,
			"total_dependencies": len(deps.Packages),
			"total_modules":      len(deps.Modules),
			"total_prs_matched":  len(results),
		},
		"dependencies": map[string]interface{}{
			"packages": deps.Packages,
			"modules":  deps.Modules,
		},
		"matches": results,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputTerminal(out *output.Writer, results []pr.MatchResult, deps *deps.Dependencies, hosts []string, flags watchFlags) error {
	out.Println("")
	out.Println("┌─────────────────────────────────────────────────────────────────────────────┐")
	out.Println("│ NixOS/nixpkgs PRs matching your configuration                              │")
	out.Println("│ Analyzed: %s (%d packages, %d modules)%s│",
		formatHosts(hosts),
		len(deps.Packages),
		len(deps.Modules),
		pad(42-len(formatHosts(hosts))-countDigits(len(deps.Packages))-countDigits(len(deps.Modules))))
	out.Println("│ Found: %d relevant PRs%s│", len(results), pad(57-countDigits(len(results))))
	out.Println("└─────────────────────────────────────────────────────────────────────────────┘")
	out.Println("")

	// Group by confidence
	highConf := []pr.MatchResult{}
	medConf := []pr.MatchResult{}
	lowConf := []pr.MatchResult{}

	for _, r := range results {
		switch r.HighestConfidence() {
		case "high":
			highConf = append(highConf, r)
		case "medium":
			medConf = append(medConf, r)
		case "low":
			lowConf = append(lowConf, r)
		}
	}

	if len(highConf) > 0 {
		out.Success("HIGH CONFIDENCE MATCHES (%d)", len(highConf))
		out.Println("════════════════════════════════════════════════════════════════════════════════")
		out.Println("")
		for _, r := range highConf {
			printMatch(out, r, flags.compact)
		}
	}

	if len(medConf) > 0 {
		out.Info("MEDIUM CONFIDENCE MATCHES (%d)", len(medConf))
		out.Println("════════════════════════════════════════════════════════════════════════════════")
		out.Println("")
		for _, r := range medConf {
			printMatch(out, r, flags.compact)
		}
	}

	if len(lowConf) > 0 {
		out.Warning("LOW CONFIDENCE MATCHES (%d)", len(lowConf))
		out.Println("════════════════════════════════════════════════════════════════════════════════")
		out.Println("")
		for _, r := range lowConf {
			printMatch(out, r, flags.compact)
		}
	}

	return nil
}

func printMatch(out *output.Writer, r pr.MatchResult, compact bool) {
	if compact {
		// Compact mode: 2 lines
		out.Success("[#%d] %s (created: %s)", r.PR.Number, r.PR.Title, formatDate(r.PR.CreatedAt))
		out.Println("  %s by @%s - %s", formatMatches(r.Matches), r.PR.Author, r.PR.URL)
	} else {
		// Full mode: include date and all details
		out.Success("[#%d] %s", r.PR.Number, r.PR.Title)
		out.Println("  → Matches: %s", formatMatches(r.Matches))
		if len(r.PR.Files) > 0 {
			out.Println("  │ Files: %s", formatFiles(r.PR.Files))
		}
		if len(r.PR.Labels) > 0 {
			out.Println("  │ Labels: %s", formatLabels(r.PR.Labels))
		}
		out.Println("  │ Created: %s | Updated: %s", formatDate(r.PR.CreatedAt), formatDate(r.PR.UpdatedAt))
		out.Println("  │ Author: @%s", r.PR.Author)
		out.Println("  └ %s", r.PR.URL)
	}
	out.Println("")
}

func formatHosts(hosts []string) string {
	if len(hosts) == 1 {
		return hosts[0]
	}
	return fmt.Sprintf("%d hosts", len(hosts))
}

func formatMatches(matches []pr.Match) string {
	if len(matches) == 0 {
		return ""
	}
	if len(matches) == 1 {
		icon := "📦"
		if matches[0].Type == "module" {
			icon = "⚙️ "
		}
		return fmt.Sprintf("%s %s (%s)", icon, matches[0].Dependency, matches[0].Type)
	}

	// Group by type
	pkgs := 0
	mods := 0
	for _, m := range matches {
		if m.Type == "module" {
			mods++
		} else if m.Type == "package" {
			pkgs++
		}
	}

	parts := []string{}
	if pkgs > 0 {
		parts = append(parts, fmt.Sprintf("%d pkg%s", pkgs, pluralize(pkgs)))
	}
	if mods > 0 {
		parts = append(parts, fmt.Sprintf("%d mod%s", mods, pluralize(mods)))
	}

	return fmt.Sprintf("%s (%s)", matches[0].Dependency, strings.Join(parts, ", "))
}

func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func formatFiles(files []pr.File) string {
	if len(files) == 0 {
		return ""
	}
	if len(files) == 1 {
		return fmt.Sprintf("%s (+%d/-%d)", files[0].Path, files[0].Additions, files[0].Deletions)
	}
	return fmt.Sprintf("%s and %d more files", files[0].Path, len(files)-1)
}

func formatLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	if len(labels) <= 2 {
		return fmt.Sprintf("%v", labels)
	}
	return fmt.Sprintf("%s, %s, and %d more", labels[0], labels[1], len(labels)-2)
}

func formatDate(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	// Less than a day
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours < 1 {
			minutes := int(diff.Minutes())
			return fmt.Sprintf("%dm ago", minutes)
		}
		return fmt.Sprintf("%dh ago", hours)
	}

	// Less than a week
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}

	// Less than a month
	if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / 24 / 7)
		return fmt.Sprintf("%dw ago", weeks)
	}

	// Otherwise show the date
	return t.Format("2006-01-02")
}

func pad(n int) string {
	if n <= 0 {
		return ""
	}
	s := ""
	for i := 0; i < n; i++ {
		s += " "
	}
	return s
}

func countDigits(n int) int {
	if n == 0 {
		return 1
	}
	count := 0
	if n < 0 {
		count = 1
		n = -n
	}
	for n > 0 {
		n /= 10
		count++
	}
	return count
}

// sortByCreatedDesc sorts PRs by creation date descending (newest first)
func sortByCreatedDesc(prs []pr.PullRequest) {
	sort.Slice(prs, func(i, j int) bool {
		return prs[i].CreatedAt.After(prs[j].CreatedAt)
	})
}

// sortResults sorts match results by the specified field
func sortResults(results []pr.MatchResult, sortBy string) {
	switch sortBy {
	case "updated":
		sort.Slice(results, func(i, j int) bool {
			return results[i].PR.UpdatedAt.After(results[j].PR.UpdatedAt)
		})
	case "created":
		fallthrough
	default:
		sort.Slice(results, func(i, j int) bool {
			return results[i].PR.CreatedAt.After(results[j].PR.CreatedAt)
		})
	}
}
