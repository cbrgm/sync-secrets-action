package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/google/go-github/v58/github"
	"golang.org/x/oauth2"
)

var args struct {
	TargetRepo  string `arg:"--target,env:TARGET,required"`
	GithubToken string `arg:"--github-token,env:GITHUB_TOKEN,required"`
	DryRun      bool   `arg:"--dry-run,env:DRY_RUN"`
	Secrets     string `arg:"--secrets,env:SECRETS"`
	Variables   string `arg:"--variables,env:VARIABLES"`
	RateLimit   bool   `arg:"--rate-limit,env:RATE_LIMIT"`
	MaxRetries  bool   `arg:"--max-retries,env:MAX_RETRIES"`
	Prune       bool   `arg:"--prune,env:PRUNE"`
	Environment string `arg:"--environment,env:ENVIRONMENT"`
	Type        string `arg:"--type,env:TYPE" default:"actions"`
}

type TargetType string

const (
	Actions    TargetType = "actions"
	Dependabot TargetType = "dependabot"
	Codespaces TargetType = "codespaces"
)

func main() {
	arg.MustParse(&args)
	ctx := context.Background()
	client := newGitHubClient(ctx, args.GithubToken)
	apiClient := NewGitHubAPI(client, args.RateLimit, args.DryRun)

	secretsMap, err := parseKeyValuePairs(args.Secrets)
	if err != nil {
		log.Fatalf("Error parsing secrets: %v", err)
	}

	variablesMap, err := parseKeyValuePairs(args.Variables)
	if err != nil {
		log.Fatalf("Error parsing variables: %v", err)
	}

	targetOwner, targetRepoName := parseRepoFullName(args.TargetRepo)

	if args.DryRun {
		log.Printf("Dry run enabled. No changes will be made to %s\n", args.TargetRepo)
	}

	switch TargetType(args.Type) {
	case Actions:
		if args.Environment == "" {
			handleRepoSecrets(ctx, apiClient, targetOwner, targetRepoName, secretsMap)
		} else {
			handleEnvironmentSecrets(ctx, apiClient, targetOwner, targetRepoName, args.Environment, secretsMap)
		}
		handleRepoVariables(ctx, apiClient, targetOwner, targetRepoName, variablesMap)
	case Dependabot:
		handleDependabotSecrets(ctx, apiClient, targetOwner, targetRepoName, secretsMap)
	case Codespaces:
		handleCodespacesSecrets(ctx, apiClient, targetOwner, targetRepoName, secretsMap)
	default:
		log.Fatalf("Unsupported target: %s", args.Type)
	}

	log.Printf("Successfully processed secrets for %s/%s\n", targetOwner, targetRepoName)
}

func handleRepoSecrets(ctx context.Context, client GitHubActionClient, owner, repo string, secrets map[string]string) {
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

func handleRepoVariables(ctx context.Context, client GitHubActionClient, owner, repo string, secrets map[string]string) {
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

func handleEnvironmentSecrets(ctx context.Context, client GitHubActionClient, owner, repo, environment string, secrets map[string]string) {
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

func handleDependabotSecrets(ctx context.Context, client GitHubActionClient, owner, repo string, secrets map[string]string) {
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

func handleCodespacesSecrets(ctx context.Context, client GitHubActionClient, owner, repo string, secrets map[string]string) {
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

func newGitHubClient(ctx context.Context, token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
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
