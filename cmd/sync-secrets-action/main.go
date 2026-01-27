package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

var (
	// Version of the application.
	Version string
	// Revision or Commit this binary was built from.
	Revision string
	// GoVersion running this binary.
	GoVersion = runtime.Version()
	// StartTime of the application.
	StartTime = time.Now()
)

// EnvArgs holds command-line arguments and environment variables for configuring the application.
type EnvArgs struct {
	TargetRepo  string `arg:"--target,env:TARGET"`
	GithubToken string `arg:"--github-token,env:GITHUB_TOKEN,required"`
	DryRun      bool   `arg:"--dry-run,env:DRY_RUN"`
	Secrets     string `arg:"--secrets,env:SECRETS"`
	Variables   string `arg:"--variables,env:VARIABLES"`
	RateLimit   bool   `arg:"--rate-limit,env:RATE_LIMIT"`
	MaxRetries  int    `arg:"--max-retries,env:MAX_RETRIES" default:"3"`
	Prune       bool   `arg:"--prune,env:PRUNE"`
	Environment string `arg:"--environment,env:ENVIRONMENT"`
	Type        string `arg:"--type,env:TYPE" default:"actions"`
	Query       string `arg:"--query,env:QUERY"`
}

// Version returns a formatted string with application version details.
func (EnvArgs) Version() string {
	return fmt.Sprintf("Version: %s %s\nBuildTime: %s\n%s\n", Revision, Version, StartTime.Format("2006-01-02"), GoVersion)
}

// TargetType defines the type of target for secret synchronization.
type TargetType string

const (
	Actions    TargetType = "actions"
	Dependabot TargetType = "dependabot"
	Codespaces TargetType = "codespaces"
)

// main is the entry point of the application. It parses input arguments and orchestrates the synchronization process.
func main() {
	var args EnvArgs
	arg.MustParse(&args)

	// Validate input arguments.
	if args.MaxRetries < 0 {
		log.Fatal("max-retries cannot be less than 0")
	}
	if (args.TargetRepo != "" && args.Query != "") || (args.TargetRepo == "" && args.Query == "") {
		log.Fatal("Either TargetRepo must be set or Query, not both")
	}

	ctx := context.Background()
	apiClient := NewGitHubAPI(ctx, args.GithubToken, args.MaxRetries, args.RateLimit, args.DryRun)

	// Parse secrets and variables from the provided strings.
	secretsMap, err := parseKeyValuePairs(args.Secrets)
	if err != nil {
		log.Fatalf("Error parsing secrets: %v", err)
	}

	variablesMap, err := parseKeyValuePairs(args.Variables)
	if err != nil {
		log.Fatalf("Error parsing variables: %v", err)
	}

	// Process repositories based on the provided target repository or query.
	if args.Query != "" {
		repos, err := apiClient.SearchRepositories(ctx, args.Query)
		if err != nil {
			log.Fatalf("Error searching for repositories: %v", err)
		}
		for _, repo := range repos {
			targetOwner := repo.GetOwner().GetLogin()
			targetRepoName := repo.GetName()
			processRepository(ctx, args, apiClient, targetOwner, targetRepoName, secretsMap, variablesMap)
		}
	} else {
		targetOwner, targetRepoName := parseRepoFullName(args.TargetRepo)
		processRepository(ctx, args, apiClient, targetOwner, targetRepoName, secretsMap, variablesMap)
	}
}

// processRepository handles the synchronization of secrets and variables for a single repository.
func processRepository(ctx context.Context, args EnvArgs, apiClient GitHubActionClient, owner, repoName string, secretsMap, variablesMap map[string]string) {
	log.Printf("Processing %s/%s\n", owner, repoName)
	switch TargetType(args.Type) {
	case Actions:
		if args.Environment == "" {
			handleRepoSecrets(ctx, args, apiClient, owner, repoName, secretsMap)
			handleRepoVariables(ctx, args, apiClient, owner, repoName, variablesMap)
		} else {
			handleEnvironmentSecrets(ctx, args, apiClient, owner, repoName, args.Environment, secretsMap)
			handleEnvironmentVariables(ctx, args, apiClient, owner, repoName, args.Environment, variablesMap)
		}
	case Dependabot:
		handleDependabotSecrets(ctx, args, apiClient, owner, repoName, secretsMap)
	case Codespaces:
		handleCodespacesSecrets(ctx, args, apiClient, owner, repoName, secretsMap)
	default:
		log.Fatalf("Unsupported target: %s", args.Type)
	}

	log.Printf("Successfully processed values for %s/%s\n", owner, repoName)
}

func handleRepoSecrets(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo string, secrets map[string]string) {
	if len(secrets) == 0 {
		return
	}
	if args.Prune {
		err := client.SyncRepoSecrets(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to sync repository secrets: %v", err)
		}
	} else {
		err := client.PutRepoSecrets(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to put repository secrets: %v", err)
		}
	}
	log.Println("Repository secrets processed successfully.")
}

func handleRepoVariables(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo string, variables map[string]string) {
	if len(variables) == 0 {
		return
	}
	if args.Prune {
		err := client.SyncRepoVariables(ctx, owner, repo, variables)
		if err != nil {
			log.Fatalf("Failed to sync repository secrets: %v", err)
		}
	} else {
		err := client.PutRepoVariables(ctx, owner, repo, variables)
		if err != nil {
			log.Fatalf("Failed to put repository secrets: %v", err)
		}
	}
	log.Println("Repository variables processed successfully.")
}

func handleEnvironmentSecrets(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo, environment string, secrets map[string]string) {
	if len(secrets) == 0 {
		return
	}
	if args.Prune {
		err := client.SyncEnvSecrets(ctx, owner, repo, environment, secrets)
		if err != nil {
			log.Fatalf("Failed to sync environment secrets: %v", err)
		}
	} else {
		err := client.PutEnvSecrets(ctx, owner, repo, environment, secrets)
		if err != nil {
			log.Fatalf("Failed to put environment secrets: %v", err)
		}
	}
	log.Println("Environment secrets processed successfully.")
}

func handleEnvironmentVariables(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo, environment string, variables map[string]string) {
	if len(variables) == 0 {
		return
	}
	if args.Prune {
		err := client.SyncEnvVariables(ctx, owner, repo, environment, variables)
		if err != nil {
			log.Fatalf("Failed to sync environment variables: %v", err)
		}
	} else {
		err := client.PutEnvVariables(ctx, owner, repo, environment, variables)
		if err != nil {
			log.Fatalf("Failed to put environment variables: %v", err)
		}
	}
	log.Println("Environment variables processed successfully.")
}

func handleDependabotSecrets(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo string, secrets map[string]string) {
	if len(secrets) == 0 {
		return
	}
	if args.Prune {
		err := client.SyncDependabotSecrets(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to sync Dependabot secrets: %v", err)
		}
	} else {
		err := client.PutDependabotSecrets(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to put Dependabot secrets: %v", err)
		}
	}
	log.Println("Dependabot secrets processed successfully.")
}

func handleCodespacesSecrets(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo string, secrets map[string]string) {
	if len(secrets) == 0 {
		return
	}
	if args.Prune {
		err := client.SyncCodespacesSecrets(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to sync Codespaces secrets: %v", err)
		}
	} else {
		err := client.PutCodespacesSecrets(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to put Codespaces secrets: %v", err)
		}
	}
	log.Println("Codespaces secrets processed successfully.")
}

func parseKeyValuePairs(secretsRaw string) (map[string]string, error) {
	secrets := make(map[string]string)

	if secretsRaw == "" {
		return secrets, nil
	}

	trimmed := strings.TrimSpace(secretsRaw)

	// Auto-detect JSON format: if input starts with '{' and ends with '}', parse as JSON.
	// This allows multi-line secrets since JSON handles newlines within string values.
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return parseJSONKeyValuePairs(trimmed)
	}

	// Standard key-value format: newline-separated KEY=VALUE pairs
	lines := strings.Split(secretsRaw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed secret, does not contain a key=value pair: %s (note: if you see '***', GitHub Actions may be masking the value - check your input format)", line)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("malformed secret, key or value is empty: %s", line)
		}
		secrets[strings.ToUpper(key)] = value
	}
	return secrets, nil
}

// parseJSONKeyValuePairs parses a JSON object where keys are secret names and values are secret values.
// This format supports multi-line secrets since JSON handles newlines within string values.
// Keys are converted to uppercase for consistency with the key-value format.
// Values are preserved exactly as provided in JSON (no trimming), allowing intentional whitespace.
func parseJSONKeyValuePairs(jsonStr string) (map[string]string, error) {
	var rawSecrets map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &rawSecrets); err != nil {
		return nil, fmt.Errorf("failed to parse JSON secrets: %w", err)
	}

	secrets := make(map[string]string)
	for key, value := range rawSecrets {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("malformed JSON secret: key is empty")
		}
		if value == "" {
			return nil, fmt.Errorf("malformed JSON secret: value is empty for key %s", key)
		}
		secrets[strings.ToUpper(key)] = value
	}
	return secrets, nil
}

func parseRepoFullName(fullName string) (owner, repo string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1]
	}
	log.Fatalf("Invalid repository format: %s", fullName)
	return "", ""
}
