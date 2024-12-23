package main

import (
	"context"
	"log"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

// GitHubActionClient defines an interface that combines all GitHub-specific interfaces
// for comprehensive functionality, including repository search, secrets, and variables management.
type GitHubActionClient interface {
	GitHubRepositorySearch
	GitHubRepoSecrets
	GitHubRepoVariables
	GitHubEnvSecrets
	GitHubDependabotSecrets
	GitHubCodespacesSecrets
}

// NewGitHubAPI initializes a new GitHub API client with optional features like rate limit checking and dry run capabilities.
// It returns an instance of GitHubActionClient, which aggregates various GitHub API functionalities.
func NewGitHubAPI(ctx context.Context, token string, maxRetries int, rateLimitCheckEnabled, dryRunEnabled bool) GitHubActionClient {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	apiClient := newGitHubAPI(client, dryRunEnabled)
	apiClient = newRetryableGitHubAPI(apiClient, uint64(maxRetries))

	if rateLimitCheckEnabled {
		apiClient = newRateLimitedGitHubAPI(apiClient)
	}

	return apiClient
}

// gitHubAPI is an internal implementation of GitHubActionClient that holds a GitHub client and a flag indicating if dry run is enabled.
type gitHubAPI struct {
	client        *github.Client
	dryRunEnabled bool
}

// newGitHubAPI creates a new instance of gitHubAPI with the specified GitHub client and dry run flag.
func newGitHubAPI(client *github.Client, dryRunEnabled bool) GitHubActionClient {
	return &gitHubAPI{
		client:        client,
		dryRunEnabled: dryRunEnabled,
	}
}

// rateLimitedGitHubAPI is a decorator for GitHubActionClient that adds rate limiting functionality.
type rateLimitedGitHubAPI struct {
	client GitHubActionClient
}

// newRateLimitedGitHubAPI wraps a given GitHubActionClient with rate limiting functionality.
func newRateLimitedGitHubAPI(client GitHubActionClient) GitHubActionClient {
	return &rateLimitedGitHubAPI{client: client}
}

// waitForRateLimitReset blocks until the GitHub API rate limit resets or an error occurs.
// It logs the waiting time and periodically checks the rate limit status.
func (g *rateLimitedGitHubAPI) waitForRateLimitReset(ctx context.Context) {
	const rateLimitedMessage = "GitHub API rate limit close to being exceeded. Waiting for reset..."
	for {
		rateLimits, _, err := g.client.Ratelimits(ctx)
		if err != nil {
			log.Printf("Error fetching rate limits: %v", err)
			return
		}

		coreRate := rateLimits.GetCore()
		resetTime := coreRate.Reset.Time
		timeToWait := time.Until(resetTime)

		if timeToWait > 0 {
			log.Printf("%s Waiting for %v", rateLimitedMessage, timeToWait)
			time.Sleep(timeToWait + time.Second) // Adding a buffer to ensure reset has occurred
		} else {
			return // Exit the function once the rate limit has reset
		}
	}
}

// ensureRatelimits checks the current rate limit status and waits for a reset if limits are close to being exceeded.
func (g *rateLimitedGitHubAPI) ensureRatelimits(ctx context.Context) {
	rateLimitStatus, _, err := g.client.Ratelimits(ctx)
	if err != nil {
		log.Printf("Error fetching rate limit status: %v", err)
		return
	}

	coreRate := rateLimitStatus.Core
	if float64(coreRate.Remaining)/float64(coreRate.Limit) <= 0.05 {
		g.waitForRateLimitReset(ctx)
	}
}

// retryableGitHubAPI is a decorator for GitHubActionClient that adds retry functionality using exponential backoff.
type retryableGitHubAPI struct {
	client         GitHubActionClient
	backoffOptions backoff.BackOff
}

func newRetryableGitHubAPI(client GitHubActionClient, maxRetries uint64) GitHubActionClient {
	backoffOptions := backoff.NewExponentialBackOff()
	backoffOptions.InitialInterval = 2 * time.Second
	backoffOptions.RandomizationFactor = backoff.DefaultRandomizationFactor
	backoffOptions.Multiplier = backoff.DefaultMultiplier
	backoffOptions.MaxInterval = backoff.DefaultMaxInterval
	backoffOptions.MaxElapsedTime = backoff.DefaultMaxElapsedTime
	withMaxTries := backoff.WithMaxRetries(backoffOptions, maxRetries)

	var api GitHubActionClient = &retryableGitHubAPI{
		client:         client,
		backoffOptions: withMaxTries,
	}
	return api
}
