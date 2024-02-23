package main

import (
	"context"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-github/v59/github"
	"golang.org/x/oauth2"
)

// GitHubActionClient combines all GitHub-specific interfaces for comprehensive functionality
type GitHubActionClient interface {
	GitHubRepositorySearch
	GitHubRepoSecrets
	GitHubRepoVariables
	GitHubEnvSecrets
	GitHubDependabotSecrets
	GitHubCodespacesSecrets
}

// NewGitHubAPI creates a new instance of gitHubAPI with a GitHub client initialized using the provided token.
// It also sets up rate limit checking and dry run capabilities based on the provided flags.
func NewGitHubAPI(ctx context.Context, token string, maxRetries int, rateLimitCheckEnabled, dryRunEnabled bool) GitHubActionClient {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	var apiClient GitHubActionClient

	apiClient = &gitHubAPI{
		client:        client,
		dryRunEnabled: dryRunEnabled,
	}

	apiClient = newRetryableGitHubAPI(apiClient, uint64(maxRetries))

	if rateLimitCheckEnabled {
		apiClient = newRateLimitedGitHubAPI(apiClient)
	}

	return apiClient
}

type gitHubAPI struct {
	client                *github.Client
	rateLimitCheckEnabled bool
	dryRunEnabled         bool
}

// RateLimitedGitHubAPI wraps a GitHubActionClient and adds rate limiting functionality.
type rateLimitedGitHubAPI struct {
	client GitHubActionClient
}

func newRateLimitedGitHubAPI(client GitHubActionClient) GitHubActionClient {
	var api GitHubActionClient = &rateLimitedGitHubAPI{
		client: client,
	}
	return api
}

func (g *rateLimitedGitHubAPI) waitForRateLimitReset(ctx context.Context) {
	const rateLimitedMessage = "GitHub API rate limit close to being exceeded. Waiting for reset.."
	for {
		rateLimits, _, err := g.client.Ratelimits(ctx)
		if err != nil {
			log.Printf("Error fetching rate limits: %v\n", err)
			return
		}

		coreRate := rateLimits.GetCore()
		resetTime := coreRate.Reset.Time
		timeToWait := time.Until(resetTime)

		if timeToWait > 0 {
			log.Printf("%s Waiting for %v\n", rateLimitedMessage, timeToWait)
			time.Sleep(timeToWait + time.Second) // Adding a buffer of 1 second to ensure reset has occurred
		} else {
			return // Exit the function once the rate limit has reset or if there's no need to wait
		}
	}
}

func (g *rateLimitedGitHubAPI) ensureRatelimits(ctx context.Context) bool {
	rateLimitStatus, _, err := g.client.Ratelimits(ctx)
	if err != nil {
		log.Printf("Error fetching rate limit status: %v\n", err)
		return false // Assume not exceeded on error to avoid false positives
	}

	coreRate := rateLimitStatus.Core
	limit := coreRate.Limit
	remaining := coreRate.Remaining

	if float64(remaining)/float64(limit) <= 0.05 {
		g.waitForRateLimitReset(ctx) // Wait for rate limit reset if we're close to exceeding it
	}

	return false // Return false because we've handled the waiting logic within the check
}

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
