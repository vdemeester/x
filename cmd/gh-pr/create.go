package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"go.sbr.pm/x/internal/output"
	"go.sbr.pm/x/internal/templates"
)

func createCmd(out *output.Writer) *cobra.Command {
	var (
		title         string
		body          string
		template      string
		draft         bool
		base          string
		head          string
		web           bool
		reviewers     []string
		assignees     []string
		labels        []string
		refresh       bool
		allowMain     bool
		noPush        bool
		requireSigned bool
		branch        string
		force         bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		Long: `Create a pull request with optional template support.

Templates are automatically discovered from:
  - .github/PULL_REQUEST_TEMPLATE.md
  - .github/PULL_REQUEST_TEMPLATE/
  - docs/PULL_REQUEST_TEMPLATE.md

Use --template to specify a template file, or list available templates
with 'gh-pr list-templates'.

By default, the command:
  - Prevents creating PRs from main/master branches
  - Pushes the current branch to remote before creating the PR
  - Use --allow-main to override the branch check
  - Use --no-push to skip pushing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(out, createOpts{
				title:         title,
				body:          body,
				template:      template,
				draft:         draft,
				base:          base,
				head:          head,
				web:           web,
				reviewers:     reviewers,
				assignees:     assignees,
				labels:        labels,
				refresh:       refresh,
				allowMain:     allowMain,
				noPush:        noPush,
				requireSigned: requireSigned,
				branch:        branch,
				force:         force,
			})
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Pull request title")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Pull request body")
	cmd.Flags().StringVar(&template, "template", "", "Use a specific template file")
	cmd.Flags().BoolVarP(&draft, "draft", "d", false, "Create as draft pull request")
	cmd.Flags().StringVar(&base, "base", "", "Base branch (default: main/master)")
	cmd.Flags().StringVar(&head, "head", "", "Head branch (default: current branch)")
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in web browser")
	cmd.Flags().StringSliceVarP(&reviewers, "reviewer", "r", nil, "Request reviewers (comma-separated)")
	cmd.Flags().StringSliceVarP(&assignees, "assignee", "a", nil, "Assign users (comma-separated)")
	cmd.Flags().StringSliceVarP(&labels, "label", "l", nil, "Add labels (comma-separated)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh template cache")
	cmd.Flags().BoolVar(&allowMain, "allow-main", false, "Allow creating PR from main/master branch")
	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing branch to remote")
	cmd.Flags().BoolVar(&requireSigned, "require-signed", false, "Require commits to be GPG signed")
	cmd.Flags().StringVar(&branch, "branch", "", "Create and switch to a new branch before creating PR")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force push (use with caution)")

	return cmd
}

type createOpts struct {
	title         string
	body          string
	template      string
	draft         bool
	base          string
	head          string
	web           bool
	reviewers     []string
	assignees     []string
	labels        []string
	refresh       bool
	allowMain     bool
	noPush        bool
	requireSigned bool
	branch        string
	force         bool
}

func runCreate(out *output.Writer, opts createOpts) error {
	// Get current branch
	currentBranch, err := getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if on main/master branch
	if isMainBranch(currentBranch) && !opts.allowMain {
		if opts.branch == "" {
			return fmt.Errorf("cannot create PR from %s branch; use --branch to create a new branch or --allow-main to override", currentBranch)
		}
	}

	// Create new branch if specified
	if opts.branch != "" {
		out.Info("Creating branch: %s", opts.branch)
		if err := createBranch(opts.branch); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
		currentBranch = opts.branch
		out.Success("Switched to branch: %s", opts.branch)
	}

	// Check for uncommitted changes
	hasChanges, err := hasUncommittedChanges(".")
	if err != nil {
		out.Warning("Could not check for uncommitted changes: %v", err)
	} else if hasChanges {
		return fmt.Errorf("you have uncommitted changes; please commit or stash them first")
	}

	// Check if there are commits to push
	hasCommits, err := hasUnpushedCommits()
	if err != nil {
		out.Warning("Could not check for unpushed commits: %v", err)
	}

	// Validate commit signatures if required
	if opts.requireSigned {
		out.Info("Checking commit signatures...")
		if err := validateCommitSignatures(); err != nil {
			return err
		}
		out.Success("All commits are signed")
	}

	// Push branch to remote (unless --no-push)
	if !opts.noPush {
		if hasCommits {
			out.Info("Pushing branch to remote...")
			if err := pushBranch(currentBranch, opts.force); err != nil {
				return fmt.Errorf("failed to push branch: %w", err)
			}
			out.Success("Pushed to remote")
		} else {
			out.Info("Branch is up to date with remote")
		}
	}

	// If template is specified, load it
	if opts.template != "" {
		content, err := loadTemplate(out, opts.template, opts.refresh)
		if err != nil {
			return err
		}

		// Use template content if body is empty
		if opts.body == "" {
			opts.body = content
		}
	}

	// Build gh pr create command
	ghArgs := []string{"pr", "create"}

	if opts.title != "" {
		ghArgs = append(ghArgs, "--title", opts.title)
	}

	if opts.body != "" {
		ghArgs = append(ghArgs, "--body", opts.body)
	}

	if opts.draft {
		ghArgs = append(ghArgs, "--draft")
	}

	if opts.base != "" {
		ghArgs = append(ghArgs, "--base", opts.base)
	}

	if opts.head != "" {
		ghArgs = append(ghArgs, "--head", opts.head)
	}

	if opts.web {
		ghArgs = append(ghArgs, "--web")
	}

	for _, reviewer := range opts.reviewers {
		ghArgs = append(ghArgs, "--reviewer", reviewer)
	}

	for _, assignee := range opts.assignees {
		ghArgs = append(ghArgs, "--assignee", assignee)
	}

	for _, label := range opts.labels {
		ghArgs = append(ghArgs, "--label", label)
	}

	out.Info("Creating pull request...")

	// Execute gh command
	cmd := exec.Command("gh", ghArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh pr create failed: %w", err)
	}

	return nil
}

// getCurrentBranch returns the current git branch name
func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// isMainBranch checks if the branch is main or master
func isMainBranch(branch string) bool {
	return branch == "main" || branch == "master"
}

// createBranch creates and switches to a new branch
func createBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// hasUnpushedCommits checks if there are local commits not pushed to remote
func hasUnpushedCommits() (bool, error) {
	// Get the upstream branch
	upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "@{upstream}")
	var stderr bytes.Buffer
	upstreamCmd.Stderr = &stderr
	upstreamOut, err := upstreamCmd.Output()
	if err != nil {
		// No upstream set, so there are unpushed commits (entire branch)
		return true, nil
	}
	upstream := strings.TrimSpace(string(upstreamOut))

	// Check for commits ahead of upstream
	cmd := exec.Command("git", "rev-list", "--count", upstream+"..HEAD")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}

	count := strings.TrimSpace(string(out))
	return count != "0", nil
}

// pushBranch pushes the branch to the remote
func pushBranch(branch string, force bool) error {
	// Detect the remote (prefer origin, but check for user fork)
	remote := detectRemote()

	args := []string{"push", "-u", remote, branch}
	if force {
		args = append(args, "--force-with-lease")
	}

	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// detectRemote returns the appropriate remote to push to
func detectRemote() string {
	// Check if user has a personal fork remote (common pattern: username as remote)
	// For now, default to origin
	// Could be enhanced to check for remotes like "chmouel" or username
	cmd := exec.Command("git", "remote")
	out, err := cmd.Output()
	if err != nil {
		return "origin"
	}

	remotes := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, r := range remotes {
		// Skip origin and upstream, prefer personal remote if it exists
		if r != "origin" && r != "upstream" && r != "" {
			// Verify it's a valid remote
			checkCmd := exec.Command("git", "remote", "get-url", r)
			if checkCmd.Run() == nil {
				return r
			}
		}
	}

	return "origin"
}

// validateCommitSignatures checks that all unpushed commits are GPG signed
func validateCommitSignatures() error {
	// Get commits that haven't been pushed
	upstreamCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "@{upstream}")
	upstreamOut, err := upstreamCmd.Output()
	var commitRange string
	if err != nil {
		// No upstream, check all commits on this branch against main/master
		mainBranch := detectMainBranch()
		commitRange = mainBranch + "..HEAD"
	} else {
		upstream := strings.TrimSpace(string(upstreamOut))
		commitRange = upstream + "..HEAD"
	}

	// Check each commit for signature
	cmd := exec.Command("git", "log", "--pretty=format:%H %G?", commitRange)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check commit signatures: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return nil // No commits to check
	}

	var unsigned []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		commitHash := parts[0][:8] // Short hash
		signatureStatus := parts[1]

		// G = good signature, U = good signature with unknown validity
		// N = no signature, B = bad signature, E = expired, X = expired signature
		if signatureStatus != "G" && signatureStatus != "U" {
			unsigned = append(unsigned, commitHash)
		}
	}

	if len(unsigned) > 0 {
		return fmt.Errorf("unsigned commits found: %s; use 'git commit --amend -S' to sign or remove --require-signed", strings.Join(unsigned, ", "))
	}

	return nil
}

// detectMainBranch detects whether the repo uses main or master
func detectMainBranch() string {
	// Check for main first
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	if cmd.Run() == nil {
		return "main"
	}

	// Fall back to master
	cmd = exec.Command("git", "rev-parse", "--verify", "master")
	if cmd.Run() == nil {
		return "master"
	}

	// Default to main
	return "main"
}

func loadTemplate(out *output.Writer, templatePath string, refresh bool) (string, error) {
	finder, err := templates.NewFinder()
	if err != nil {
		return "", err
	}

	// If template path is just a name, try to find it
	if !strings.Contains(templatePath, "/") {
		out.Info("Searching for template: %s", templatePath)

		tmplList, err := finder.Find(refresh)
		if err != nil {
			return "", fmt.Errorf("failed to find templates: %w", err)
		}

		for _, tmpl := range tmplList {
			if tmpl.Name == templatePath || tmpl.Path == templatePath {
				out.Success("Found template: %s", tmpl.Path)
				return tmpl.Content, nil
			}
		}

		return "", fmt.Errorf("template not found: %s", templatePath)
	}

	// Direct file path
	return templates.ReadTemplate(templatePath)
}
