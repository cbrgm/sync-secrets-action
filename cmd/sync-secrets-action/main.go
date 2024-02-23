package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
)

// Global variables for application metadata.
var (
	Version   string              // Version of the application.
	Revision  string              // Revision or Commit this binary was built from.
	GoVersion = runtime.Version() // GoVersion running this binary.
	StartTime = time.Now()        // StartTime of the application.
)

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

type TargetType string

const (
	Actions    TargetType = "actions"
	Dependabot TargetType = "dependabot"
	Codespaces TargetType = "codespaces"
)

func main() {
	var args EnvArgs
	arg.MustParse(&args)

	if args.MaxRetries < 0 {
		log.Fatal("max-retries cannot be less than 0")
	}

	if (args.TargetRepo != "" && args.Query != "") || (args.TargetRepo == "" && args.Query == "") {
		log.Fatal("Either TargetRepo must be set or Query, not both")
	}

	ctx := context.Background()
	apiClient := NewGitHubAPI(ctx, args.GithubToken, args.MaxRetries, args.RateLimit, args.DryRun)

	secretsMap, err := parseKeyValuePairs(args.Secrets)
	if err != nil {
		log.Fatalf("Error parsing secrets: %v", err)
	}

	variablesMap, err := parseKeyValuePairs(args.Variables)
	if err != nil {
		log.Fatalf("Error parsing variables: %v", err)
	}

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

func processRepository(ctx context.Context, args EnvArgs, apiClient GitHubActionClient, owner, repoName string, secretsMap, variablesMap map[string]string) {
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

	log.Printf("Successfully processed secrets for %s/%s\n", owner, repoName)
}

func handleRepoSecrets(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo string, secrets map[string]string) {
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

func handleRepoVariables(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo string, secrets map[string]string) {
	if args.Prune {
		err := client.SyncRepoVariables(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to sync repository secrets: %v", err)
		}
	} else {
		err := client.PutRepoVariables(ctx, owner, repo, secrets)
		if err != nil {
			log.Fatalf("Failed to put repository secrets: %v", err)
		}
	}
	log.Println("Repository variables processed successfully.")
}

func handleEnvironmentSecrets(ctx context.Context, args EnvArgs, client GitHubActionClient, owner, repo, environment string, secrets map[string]string) {
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
	lines := strings.Split(secretsRaw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed secret, does not contain a key=value pair: %s", line)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return nil, fmt.Errorf("malformed secret, key or value is empty: %s", line)
		}
		secrets[key] = value
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
