package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cenkalti/backoff/v5"
	"github.com/google/go-github/v79/github"
)

// GitHubCodespacesSecrets defines the interface for managing GitHub Codespaces secrets.
type GitHubCodespacesSecrets interface {
	CreateOrUpdateCodespacesSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error)
	DeleteCodespacesSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
	GetCodespacesPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	ListCodespacesSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	PutCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
	SyncCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
}

// GetCodespacesPublicKey retrieves the public key for a repository, used for encrypting Codespaces secrets.
func (api *gitHubAPI) GetCodespacesPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	return api.client.Codespaces.GetRepoPublicKey(ctx, owner, repo)
}

// CreateOrUpdateCodespacesSecret adds or updates a secret in a repository's Codespaces environment.
func (api *gitHubAPI) CreateOrUpdateCodespacesSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	return api.client.Codespaces.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
}

// DeleteCodespacesSecret removes a secret from a repository's Codespaces environment.
func (api *gitHubAPI) DeleteCodespacesSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	return api.client.Codespaces.DeleteRepoSecret(ctx, owner, repo, name)
}

// ListCodespacesSecrets lists all secrets available in a repository's Codespaces environment.
func (api *gitHubAPI) ListCodespacesSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	return api.client.Codespaces.ListRepoSecrets(ctx, owner, repo, opts)
}

func (api *gitHubAPI) PutCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Putting codespaces secrets for repo %s/%s\n", owner, repo)
		for secretName := range mappings {
			log.Printf("Dry run: Would put codespaces secret '%s' in repo %s/%s\n", secretName, owner, repo)
		}
		return nil
	}

	publicKey, _, err := api.GetCodespacesPublicKey(ctx, owner, repo)
	if err != nil {
		return err
	}

	for secretName, secretValue := range mappings {
		encryptedSecret, err := encryptSecretWithPublicKey(publicKey, secretName, secretValue)
		if err != nil {
			return err
		}

		_, err = api.CreateOrUpdateCodespacesSecret(ctx, owner, repo, encryptedSecret)
		if err != nil {
			return err
		}
	}
	return nil
}

// PutCodespacesSecrets creates or updates multiple Codespaces secrets for a repository.
func (api *gitHubAPI) SyncCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Syncing Codespaces secrets for repo %s/%s", owner, repo)
		opts := &github.ListOptions{PerPage: 100}
		for {
			secrets, resp, err := api.ListCodespacesSecrets(ctx, owner, repo, opts)
			if err != nil {
				return fmt.Errorf("dry run: failed to list existing Codespaces secrets: %v", err)
			}

			for _, secret := range secrets.Secrets {
				if _, exists := mappings[secret.Name]; !exists {
					log.Printf("Dry run: Would delete Codespaces secret '%s' from repo %s/%s", secret.Name, owner, repo)
				}
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		for secretName := range mappings {
			log.Printf("Dry run: Would add/update Codespaces secret '%s' in repo %s/%s", secretName, owner, repo)
		}

		return nil
	}

	existingMap := make(map[string]bool)

	opts := &github.ListOptions{PerPage: 100}
	for {
		secrets, resp, err := api.ListCodespacesSecrets(ctx, owner, repo, opts)
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
			_, err := api.DeleteCodespacesSecret(ctx, owner, repo, secretName)
			if err != nil {
				return err
			}
		}
	}

	return api.PutCodespacesSecrets(ctx, owner, repo, mappings)
}

// Below are rate limited and retryable implementations of the GitHubCodespacesSecrets interface methods.
// These wrap the basic implementations with additional functionality like waiting for rate limit resets or retrying on failure.

// Ratelimiting

func (r *rateLimitedGitHubAPI) PutCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.PutCodespacesSecrets(ctx, owner, repo, mappings)
}

func (r *rateLimitedGitHubAPI) GetCodespacesPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.GetCodespacesPublicKey(ctx, owner, repo)
}

func (r *rateLimitedGitHubAPI) CreateOrUpdateCodespacesSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.CreateOrUpdateCodespacesSecret(ctx, owner, repo, eSecret)
}

func (r *rateLimitedGitHubAPI) DeleteCodespacesSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.DeleteCodespacesSecret(ctx, owner, repo, name)
}

func (r *rateLimitedGitHubAPI) ListCodespacesSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.ListCodespacesSecrets(ctx, owner, repo, opts)
}

func (r *rateLimitedGitHubAPI) SyncCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.SyncCodespacesSecrets(ctx, owner, repo, mappings)
}

// Retryable

func (r *retryableGitHubAPI) CreateOrUpdateCodespacesSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.CreateOrUpdateCodespacesSecret(ctx, owner, repo, eSecret)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) DeleteCodespacesSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.DeleteCodespacesSecret(ctx, owner, repo, name)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) GetCodespacesPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	var publicKey *github.PublicKey
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		publicKey, resp, err = r.client.GetCodespacesPublicKey(ctx, owner, repo)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return publicKey, resp, err
}

func (r *retryableGitHubAPI) ListCodespacesSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	var secrets *github.Secrets
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		secrets, resp, err = r.client.ListCodespacesSecrets(ctx, owner, repo, opts)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return secrets, resp, err
}

func (r *retryableGitHubAPI) SyncCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.SyncCodespacesSecrets(ctx, owner, repo, mappings)
	}

	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}

func (r *retryableGitHubAPI) PutCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.PutCodespacesSecrets(ctx, owner, repo, mappings)
	}

	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}
