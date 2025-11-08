package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cenkalti/backoff/v5"
	"github.com/google/go-github/v78/github"
)

// GitHubDependabotSecrets for GitHub Dependabot secrets management.
type GitHubDependabotSecrets interface {
	PutDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
	GetDependabotPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateDependabotSecret(ctx context.Context, owner, repo string, eSecret *github.DependabotEncryptedSecret) (*github.Response, error)
	DeleteDependabotSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
	ListDependabotSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	SyncDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
}

func (api *gitHubAPI) GetDependabotPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	return api.client.Dependabot.GetRepoPublicKey(ctx, owner, repo)
}

func (api *gitHubAPI) CreateOrUpdateDependabotSecret(ctx context.Context, owner, repo string, eSecret *github.DependabotEncryptedSecret) (*github.Response, error) {
	return api.client.Dependabot.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
}

func (api *gitHubAPI) DeleteDependabotSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	return api.client.Dependabot.DeleteRepoSecret(ctx, owner, repo, name)
}

func (api *gitHubAPI) ListDependabotSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	return api.client.Dependabot.ListRepoSecrets(ctx, owner, repo, opts)
}

func (api *gitHubAPI) PutDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Putting Dependabot secrets for repo %s/%s", owner, repo)
		for secretName := range mappings {
			log.Printf("Dry run: Would put Dependabot secret '%s' in repo %s/%s", secretName, owner, repo)
		}
		return nil
	}

	publicKey, _, err := api.GetDependabotPublicKey(ctx, owner, repo)
	if err != nil {
		return err
	}

	for secretName, secretValue := range mappings {
		encryptedSecret, err := encryptDependabotWithPublicKey(publicKey, secretName, secretValue)
		if err != nil {
			return err
		}

		_, err = api.CreateOrUpdateDependabotSecret(ctx, owner, repo, encryptedSecret)
		if err != nil {
			return err
		}
	}
	return nil
}

func (api *gitHubAPI) SyncDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Syncing Dependabot secrets for repo %s/%s", owner, repo)
		opts := &github.ListOptions{PerPage: 100}
		for {
			secrets, resp, err := api.ListDependabotSecrets(ctx, owner, repo, opts)
			if err != nil {
				return fmt.Errorf("dry run: failed to list existing Dependabot secrets: %v", err)
			}

			for _, secret := range secrets.Secrets {
				if _, exists := mappings[secret.Name]; !exists {
					log.Printf("Dry run: Would delete Dependabot secret '%s' from repo %s/%s", secret.Name, owner, repo)
				}
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		for secretName := range mappings {
			log.Printf("Dry run: Would add/update Dependabot secret '%s' in repo %s/%s", secretName, owner, repo)
		}

		return nil
	}

	existingMap := make(map[string]bool)

	opts := &github.ListOptions{PerPage: 100}
	for {
		secrets, resp, err := api.ListDependabotSecrets(ctx, owner, repo, opts)
		if err != nil {
			return err
		}

		for _, secret := range secrets.Secrets {
			existingMap[secret.Name] = true
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	for secretName := range existingMap {
		if _, exists := mappings[secretName]; !exists {
			_, err := api.DeleteDependabotSecret(ctx, owner, repo, secretName)
			if err != nil {
				return err
			}
		}
	}

	return api.PutDependabotSecrets(ctx, owner, repo, mappings)
}

// Ratelimiting

func (r *rateLimitedGitHubAPI) PutDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.PutDependabotSecrets(ctx, owner, repo, mappings)
}

func (r *rateLimitedGitHubAPI) GetDependabotPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.GetDependabotPublicKey(ctx, owner, repo)
}

func (r *rateLimitedGitHubAPI) CreateOrUpdateDependabotSecret(ctx context.Context, owner, repo string, eSecret *github.DependabotEncryptedSecret) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.CreateOrUpdateDependabotSecret(ctx, owner, repo, eSecret)
}

func (r *rateLimitedGitHubAPI) DeleteDependabotSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.DeleteDependabotSecret(ctx, owner, repo, name)
}

func (r *rateLimitedGitHubAPI) ListDependabotSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.ListDependabotSecrets(ctx, owner, repo, opts)
}

func (r *rateLimitedGitHubAPI) SyncDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.SyncDependabotSecrets(ctx, owner, repo, mappings)
}

// Retry

func (r *retryableGitHubAPI) GetDependabotPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	var publicKey *github.PublicKey
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		publicKey, resp, err = r.client.GetDependabotPublicKey(ctx, owner, repo)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return publicKey, resp, err
}

func (r *retryableGitHubAPI) CreateOrUpdateDependabotSecret(ctx context.Context, owner, repo string, eSecret *github.DependabotEncryptedSecret) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.CreateOrUpdateDependabotSecret(ctx, owner, repo, eSecret)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) DeleteDependabotSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.DeleteDependabotSecret(ctx, owner, repo, name)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) ListDependabotSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	var secrets *github.Secrets
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		secrets, resp, err = r.client.ListDependabotSecrets(ctx, owner, repo, opts)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return secrets, resp, err
}

func (r *retryableGitHubAPI) SyncDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.SyncDependabotSecrets(ctx, owner, repo, mappings)
	}
	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}

func (r *retryableGitHubAPI) PutDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.PutDependabotSecrets(ctx, owner, repo, mappings)
	}

	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}
