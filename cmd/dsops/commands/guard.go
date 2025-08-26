package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
)

func NewGuardCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guard",
		Short: "Security guardrails and leak detection",
		Long: `Security guardrails to prevent secret leaks and enforce policies.

Subcommands:
  repo       Scan repository for committed secrets
  gitignore  Check .gitignore patterns for secret files

Examples:
  dsops guard repo                    # Scan current repository
  dsops guard repo --path /some/repo  # Scan specific repository
  dsops guard gitignore              # Check .gitignore patterns`,
	}

	cmd.AddCommand(
		NewGuardRepoCommand(cfg),
		NewGuardGitignoreCommand(cfg),
	)

	return cmd
}

func NewGuardRepoCommand(cfg *config.Config) *cobra.Command {
	var (
		path       string
		all        bool
		verbose    bool
		exitCode   bool
		patterns   []string
	)

	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Scan repository for committed secrets",
		Long: `Scan git repository history for potential secret leaks.

This command searches through git history looking for patterns that might
indicate committed secrets like API keys, passwords, tokens, etc.

Examples:
  dsops guard repo                           # Scan current repository
  dsops guard repo --path /some/repo         # Scan specific repository  
  dsops guard repo --all                     # Scan all history (slower)
  dsops guard repo --pattern "api.*key"      # Custom regex patterns
  dsops guard repo --exit-code               # Exit 1 if secrets found

Security Note:
This tool helps identify potential leaks but is not foolproof. Use proper
secret management practices and never commit secrets to version control.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return scanRepository(path, all, verbose, exitCode, patterns)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Repository path to scan")
	cmd.Flags().BoolVar(&all, "all", false, "Scan all history (default: recent commits only)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed output")
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "Exit with code 1 if secrets found")
	cmd.Flags().StringArrayVar(&patterns, "pattern", nil, "Additional regex patterns to search for")

	return cmd
}

func NewGuardGitignoreCommand(cfg *config.Config) *cobra.Command {
	var (
		path    string
		verbose bool
	)

	cmd := &cobra.Command{
		Use:   "gitignore",
		Short: "Check .gitignore patterns for secret files",
		Long: `Check .gitignore file for patterns that help prevent secret leaks.

This command analyzes your .gitignore file and suggests patterns to add
for common secret file types and formats.

Examples:
  dsops guard gitignore                # Check current directory
  dsops guard gitignore --path /repo   # Check specific repository`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkGitignore(path, verbose)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Repository path to check")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed analysis")

	return cmd
}

func scanRepository(repoPath string, scanAll, verbose, exitCode bool, customPatterns []string) error {
	// Verify this is a git repository
	if !isGitRepository(repoPath) {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Not a git repository: %s", repoPath),
			Suggestion: "Navigate to a git repository or specify --path to one",
		}
	}

	fmt.Printf("🔍 Scanning repository: %s\n", repoPath)
	if scanAll {
		fmt.Println("📚 Scanning all history (this may take a while)")
	} else {
		fmt.Println("📝 Scanning recent commits (use --all for full history)")
	}
	fmt.Println()

	// Define secret patterns
	patterns := getSecretPatterns()
	patterns = append(patterns, customPatterns...)

	if verbose {
		fmt.Printf("Using %d detection patterns\n", len(patterns))
		fmt.Println()
	}

	// Get commit range - handle repositories with few commits
	var commitRange string
	if scanAll {
		commitRange = "--all"
	} else {
		// Try to get last 10 commits, fallback to all if repository has fewer
		cmd := exec.Command("git", "rev-list", "--count", "HEAD")
		cmd.Dir = repoPath
		if output, err := cmd.Output(); err == nil {
			commitCount := strings.TrimSpace(string(output))
			if commitCount == "0" {
				return dserrors.UserError{
					Message:    "Repository has no commits",
					Suggestion: "Make at least one commit before scanning",
				}
			}
		}
		
		// Use a range that works with any number of commits
		commitRange = "--max-count=10"
	}

	// Scan for secrets
	findings, err := scanCommitsForSecrets(repoPath, commitRange, patterns, verbose)
	if err != nil {
		return err
	}

	// Report results
	if len(findings) == 0 {
		fmt.Println("✅ No potential secrets found in repository")
		return nil
	}

	fmt.Printf("⚠️  Found %d potential secret leak(s):\n\n", len(findings))
	for i, finding := range findings {
		fmt.Printf("%d. %s\n", i+1, finding.Description)
		fmt.Printf("   Commit: %s\n", finding.Commit)
		fmt.Printf("   File: %s:%d\n", finding.File, finding.Line)
		if verbose {
			fmt.Printf("   Match: %s\n", finding.Match)
		}
		fmt.Println()
	}

	fmt.Println("🛡️  Recommendations:")
	fmt.Println("  1. Review these findings and remove any real secrets")
	fmt.Println("  2. Use 'git filter-branch' or 'BFG Repo-Cleaner' to purge secrets")
	fmt.Println("  3. Rotate any exposed credentials immediately")
	fmt.Println("  4. Use dsops for proper secret management going forward")

	if exitCode {
		os.Exit(1)
	}

	return nil
}

func checkGitignore(repoPath string, verbose bool) error {
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	
	fmt.Printf("🔍 Checking .gitignore patterns: %s\n", gitignorePath)
	fmt.Println()

	// Read existing .gitignore
	var existingPatterns []string
	if content, err := os.ReadFile(gitignorePath); err == nil {
		existingPatterns = strings.Split(string(content), "\n")
	}

	// Recommended patterns for secret files
	recommendedPatterns := []string{
		"# Secret and environment files",
		"*.env",
		"*.env.*",
		".env.local",
		".env.*.local",
		"secrets/",
		"secret.*",
		"*.key",
		"*.pem",
		"*.p12",
		"*.pfx",
		"id_rsa",
		"id_dsa",
		"id_ecdsa",
		"id_ed25519",
		"# dsops files",
		"dsops.yaml",
		"*.dsops.yaml",
		"# Cloud credentials",
		".aws/credentials",
		".gcp/",
		"gcloud/",
		"# Application secrets",
		"config/database.yml",
		"config/secrets.yml",
	}

	// Check which patterns are missing
	var missingPatterns []string
	for _, pattern := range recommendedPatterns {
		if strings.HasPrefix(pattern, "#") {
			continue // Skip comments
		}
		
		found := false
		for _, existing := range existingPatterns {
			if strings.TrimSpace(existing) == pattern {
				found = true
				break
			}
		}
		
		if !found {
			missingPatterns = append(missingPatterns, pattern)
		}
	}

	// Report results
	if len(missingPatterns) == 0 {
		fmt.Println("✅ .gitignore contains good secret file patterns")
		return nil
	}

	fmt.Printf("⚠️  Consider adding these patterns to .gitignore:\n\n")
	for _, pattern := range missingPatterns {
		fmt.Printf("  %s\n", pattern)
	}

	fmt.Println("\n🛡️  Recommendations:")
	fmt.Println("  1. Add these patterns to prevent accidental secret commits")
	fmt.Println("  2. Use 'dsops install-hook' to add pre-commit protection")
	fmt.Println("  3. Keep .gitignore patterns up to date with your project needs")

	return nil
}

type SecretFinding struct {
	Description string
	Commit      string
	File        string
	Line        int
	Match       string
}

func getSecretPatterns() []string {
	return []string{
		`(?i)(api[_-]?key|apikey)\s*[:=]\s*['""]?[a-zA-Z0-9]{16,}['""]?`,
		`(?i)(secret[_-]?key|secretkey)\s*[:=]\s*['""]?[a-zA-Z0-9]{16,}['""]?`,
		`(?i)(access[_-]?token|accesstoken)\s*[:=]\s*['""]?[a-zA-Z0-9]{16,}['""]?`,
		`(?i)(auth[_-]?token|authtoken)\s*[:=]\s*['""]?[a-zA-Z0-9]{16,}['""]?`,
		`(?i)password\s*[:=]\s*['""]?[a-zA-Z0-9!@#$%^&*]{8,}['""]?`,
		`(?i)passwd\s*[:=]\s*['""]?[a-zA-Z0-9!@#$%^&*]{8,}['""]?`,
		`(?i)private[_-]?key\s*[:=]`,
		`-----BEGIN [A-Z ]+PRIVATE KEY-----`,
		`(?i)aws[_-]?access[_-]?key[_-]?id\s*[:=]\s*['""]?AKIA[A-Z0-9]{16}['""]?`,
		`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*['""]?[A-Za-z0-9/+=]{40}['""]?`,
		`(?i)github[_-]?token\s*[:=]\s*['""]?gh[ps]_[A-Za-z0-9]{36}['""]?`,
		`(?i)slack[_-]?token\s*[:=]\s*['""]?xox[baprs]-[A-Za-z0-9-]{10,}['""]?`,
		`(?i)discord[_-]?token\s*[:=]\s*['""]?[MN][A-Za-z\d]{23}\.[A-Za-z\d-_]{6}\.[A-Za-z\d-_]{27}['""]?`,
		`(?i)google[_-]?api[_-]?key\s*[:=]\s*['""]?AIza[A-Za-z0-9-_]{35}['""]?`,
		`(?i)stripe[_-]?key\s*[:=]\s*['""]?sk_live_[A-Za-z0-9]{24}['""]?`,
		`(?i)twilio[_-]?auth[_-]?token\s*[:=]\s*['""]?[A-Za-z0-9]{32}['""]?`,
		`(?i)mailgun[_-]?api[_-]?key\s*[:=]\s*['""]?key-[A-Za-z0-9]{32}['""]?`,
		`(?i)sendgrid[_-]?api[_-]?key\s*[:=]\s*['""]?SG\.[A-Za-z0-9-_]{22}\.[A-Za-z0-9-_]{43}['""]?`,
	}
}

func scanCommitsForSecrets(repoPath, commitRange string, patterns []string, verbose bool) ([]SecretFinding, error) {
	var findings []SecretFinding

	// Compile regex patterns
	var regexes []*regexp.Regexp
	for _, pattern := range patterns {
		if regex, err := regexp.Compile(pattern); err == nil {
			regexes = append(regexes, regex)
		} else if verbose {
			fmt.Printf("Warning: Invalid regex pattern: %s\n", pattern)
		}
	}

	// Get git log with file changes
	var cmd *exec.Cmd
	if commitRange == "--all" {
		cmd = exec.Command("git", "log", "--oneline", "--name-only", "--all")
	} else {
		cmd = exec.Command("git", "log", "--oneline", "--name-only", commitRange)
	}
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, dserrors.CommandError{
			Command:    "git log",
			Message:    err.Error(),
			Suggestion: "Ensure you're in a git repository with commit history",
		}
	}

	// Parse commits and scan files
	lines := strings.Split(string(output), "\n")
	var currentCommit string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a commit line (starts with hash)
		if strings.Contains(line, " ") && len(strings.Fields(line)[0]) >= 7 {
			currentCommit = strings.Fields(line)[0]
			continue
		}

		// This is a file name, scan it for secrets
		if currentCommit != "" {
			fileFindings := scanFileInCommit(repoPath, currentCommit, line, regexes, verbose)
			findings = append(findings, fileFindings...)
		}
	}

	return findings, nil
}

func scanFileInCommit(repoPath, commit, filename string, regexes []*regexp.Regexp, verbose bool) []SecretFinding {
	var findings []SecretFinding

	// Get file content at specific commit
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", commit, filename))
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return findings // File might be deleted or binary
	}

	content := string(output)
	lines := strings.Split(content, "\n")

	// Scan each line with each regex
	for lineNum, line := range lines {
		for _, regex := range regexes {
			if matches := regex.FindAllString(line, -1); len(matches) > 0 {
				for _, match := range matches {
					finding := SecretFinding{
						Description: "Potential secret detected",
						Commit:      commit,
						File:        filename,
						Line:        lineNum + 1,
						Match:       match,
					}
					findings = append(findings, finding)
				}
			}
		}
	}

	return findings
}

func isGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		return true
	}
	return false
}