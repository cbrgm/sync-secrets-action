package main

import (
	"context"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-github/v61/github"
)

// GitHubRepositorySearch for searching GitHub repositories.
type GitHubRepositorySearch interface {
	SearchRepositories(ctx context.Context, query string) ([]*github.Repository, error)
	Ratelimits(ctx context.Context) (*github.RateLimits, *github.Response, error)
}

func (api *gitHubAPI) SearchRepositories(ctx context.Context, query string) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := api.client.Search.Repositories(ctx, query, opts)
		if err != nil {
			return nil, err
		}

		allRepos = append(allRepos, result.Repositories...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}

func (api *gitHubAPI) Ratelimits(ctx context.Context) (*github.RateLimits, *github.Response, error) {
	return api.client.RateLimit.Get(ctx)
}

// Ratelimits

func (r *rateLimitedGitHubAPI) SearchRepositories(ctx context.Context, query string) ([]*github.Repository, error) {
	r.ensureRatelimits(ctx)
	return r.client.SearchRepositories(ctx, query)
}

func (r *rateLimitedGitHubAPI) Ratelimits(ctx context.Context) (*github.RateLimits, *github.Response, error) {
	return r.client.Ratelimits(ctx)
}

// Retryable

func (r *retryableGitHubAPI) SearchRepositories(ctx context.Context, query string) ([]*github.Repository, error) {
	var repos []*github.Repository
	var err error

	retryFunc := func() error {
		repos, err = r.client.SearchRepositories(ctx, query)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return repos, err
}

func (r *retryableGitHubAPI) Ratelimits(ctx context.Context) (*github.RateLimits, *github.Response, error) {
	return r.client.Ratelimits(ctx)
}
