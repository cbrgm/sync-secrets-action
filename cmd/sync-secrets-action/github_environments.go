package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cenkalti/backoff/v5"
	"github.com/google/go-github/v74/github"
)

// GitHubEnvSecrets for GitHub environment secrets management.
type GitHubEnvSecrets interface {
	CreateOrUpdateEnvSecret(ctx context.Context, repoID int, envName string, eSecret *github.EncryptedSecret) (*github.Response, error)
	DeleteEnvSecret(ctx context.Context, repoID int, envName, name string) (*github.Response, error)
	GetEnvPublicKey(ctx context.Context, repoID int, envName string) (*github.PublicKey, *github.Response, error)
	ListEnvSecrets(ctx context.Context, repoID int, envName string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	PutEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error
	SyncEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error

	CreateOrUpdateEnvVariable(ctx context.Context, owner, repo, envName string, eSecret *github.ActionsVariable) (*github.Response, error)
	DeleteEnvVariable(ctx context.Context, owner, repo, envName, name string) (*github.Response, error)
	ListEnvVariables(ctx context.Context, owner, repo, envName string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error)
	PutEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error
	SyncEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error
}

func (api *gitHubAPI) DeleteEnvSecret(ctx context.Context, repoID int, envName, name string) (*github.Response, error) {
	return api.client.Actions.DeleteEnvSecret(ctx, int(repoID), envName, name)
}

func (api *gitHubAPI) ListEnvSecrets(ctx context.Context, repoID int, envName string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	return api.client.Actions.ListEnvSecrets(ctx, repoID, envName, opts)
}

func (api *gitHubAPI) GetEnvPublicKey(ctx context.Context, repoID int, envName string) (*github.PublicKey, *github.Response, error) {
	return api.client.Actions.GetEnvPublicKey(ctx, repoID, envName)
}

func (api *gitHubAPI) CreateOrUpdateEnvSecret(ctx context.Context, repoID int, envName string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	return api.client.Actions.CreateOrUpdateEnvSecret(ctx, repoID, envName, eSecret)
}

func (api *gitHubAPI) DeleteEnvVariable(ctx context.Context, owner, repo, envName, name string) (*github.Response, error) {
	return api.client.Actions.DeleteEnvVariable(ctx, owner, repo, envName, name)
}

func (api *gitHubAPI) ListEnvVariables(ctx context.Context, owner, repo, envName string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error) {
	return api.client.Actions.ListEnvVariables(ctx, owner, repo, envName, opts)
}

func (api *gitHubAPI) CreateOrUpdateEnvVariable(ctx context.Context, owner, repo, envName string, eVariable *github.ActionsVariable) (*github.Response, error) {
	// Try to update the variable first
	resp, err := api.client.Actions.UpdateEnvVariable(ctx, owner, repo, envName, eVariable)
	if err != nil {
		// If update fails (e.g., variable doesn't exist), try to create it
		createResp, createErr := api.client.Actions.CreateEnvVariable(ctx, owner, repo, envName, eVariable)
		if createErr != nil {
			return nil, fmt.Errorf("failed to update environment variable %s in environment %s: %v; failed to create: %v", eVariable.Name, envName, err, createErr)
		}
		return createResp, nil
	}
	return resp, err
}

func (api *gitHubAPI) SyncEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	r, _, err := api.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to list repo %s/%s: %v", owner, repo, err)
	}

	if api.dryRunEnabled {
		log.Printf("Dry run: Syncing environment secrets for '%s' in repo %s/%s", envName, owner, repo)
		opts := &github.ListOptions{PerPage: 100}
		for {
			secrets, resp, err := api.ListEnvSecrets(ctx, int(r.GetID()), envName, opts)
			if err != nil {
				return fmt.Errorf("dry run: failed to fetch existing environment secrets for %s in repo %s/%s: %v", envName, owner, repo, err)
			}

			for _, secret := range secrets.Secrets {
				if _, ok := mappings[secret.Name]; !ok {
					log.Printf("Dry run: Would delete environment secret '%s' in '%s' for repo %s/%s\n", secret.Name, envName, owner, repo)
				}
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		for secretName := range mappings {
			log.Printf("Dry run: Would add/update environment secret '%s' in '%s' for repo %s/%s\n", secretName, envName, owner, repo)
		}

		return nil
	}

	existingMap := make(map[string]bool)

	// Pagination setup
	opts := &github.ListOptions{PerPage: 100}
	for {
		secrets, resp, err := api.ListEnvSecrets(ctx, int(r.GetID()), envName, opts)
		if err != nil {
			return fmt.Errorf("failed to list existing environment secrets for %s: %v", envName, err)
		}

		for _, secret := range secrets.Secrets {
			existingMap[secret.Name] = true
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Delete secrets not in mappings
	for secretName := range existingMap {
		if _, exists := mappings[secretName]; !exists {
			_, err := api.DeleteEnvSecret(ctx, int(r.GetID()), envName, secretName)
			if err != nil {
				return fmt.Errorf("failed to delete environment secret %s in %s for repo %s/%s: %v", secretName, envName, owner, repo, err)
			}
		}
	}

	// Add or update secrets from mappings
	return api.PutEnvSecrets(ctx, owner, repo, envName, mappings)
}

func (api *gitHubAPI) PutEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Putting environment secrets for '%s' in repo %s/%s\n", envName, owner, repo)
		for secretName := range mappings {
			log.Printf("Dry run: Would put environment secret '%s' in '%s' for repo %s/%s\n", secretName, envName, owner, repo)
		}
		return nil
	}

	r, _, err := api.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to list repo %s/%s: %v", owner, repo, err)
	}

	publicKey, _, err := api.GetEnvPublicKey(ctx, int(r.GetID()), envName)
	if err != nil {
		return fmt.Errorf("failed to get public key for environment %s in repo %s/%s: %v", envName, owner, repo, err)
	}

	for secretName, secretValue := range mappings {
		secret, err := encryptSecretWithPublicKey(publicKey, secretName, secretValue)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret %s: %v", secretName, err)
		}
		_, err = api.CreateOrUpdateEnvSecret(ctx, int(r.GetID()), envName, secret)
		if err != nil {
			return fmt.Errorf("failed to update secret %s in environment %s for repo %s/%s: %v", secretName, envName, owner, repo, err)
		}
	}
	return nil
}

func (api *gitHubAPI) SyncEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	r, _, err := api.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to list repo %s/%s: %v", owner, repo, err)
	}

	if api.dryRunEnabled {
		log.Printf("Dry run: Syncing environment variables for '%s' in repo %s/%s", envName, owner, repo)
		opts := &github.ListOptions{PerPage: 100}
		for {
			variables, resp, err := api.ListEnvVariables(ctx, r.GetOwner().GetName(), r.GetName(), envName, opts)
			if err != nil {
				return fmt.Errorf("dry run: failed to fetch existing environment variables for %s in repo %s/%s: %v", envName, owner, repo, err)
			}

			for _, variable := range variables.Variables {
				if _, ok := mappings[variable.Name]; !ok {
					log.Printf("Dry run: Would delete environment variable '%s' in '%s' for repo %s/%s\n", variable.Name, envName, owner, repo)
				}
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		for variableName := range mappings {
			log.Printf("Dry run: Would add/update environment variable '%s' in '%s' for repo %s/%s\n", variableName, envName, owner, repo)
		}

		return nil
	}

	existingMap := make(map[string]bool)

	// Pagination setup
	opts := &github.ListOptions{PerPage: 100}
	for {
		variables, resp, err := api.ListEnvVariables(ctx, r.GetOwner().GetName(), r.GetName(), envName, opts)
		if err != nil {
			return fmt.Errorf("failed to list existing environment variables for %s: %v", envName, err)
		}

		for _, variable := range variables.Variables {
			existingMap[variable.Name] = true
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Delete variables not in mappings
	for variableName := range existingMap {
		if _, exists := mappings[variableName]; !exists {
			_, err := api.DeleteEnvVariable(ctx, r.GetOwner().GetName(), r.GetName(), envName, variableName)
			if err != nil {
				return fmt.Errorf("failed to delete environment variable %s in %s for repo %s/%s: %v", variableName, envName, owner, repo, err)
			}
		}
	}

	// Add or update variables from mappings
	return api.PutEnvVariables(ctx, owner, repo, envName, mappings)
}

func (api *gitHubAPI) PutEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Putting environment variables for '%s' in repo %s/%s\n", envName, owner, repo)
		for variableName := range mappings {
			log.Printf("Dry run: Would put environment variable '%s' in '%s' for repo %s/%s\n", variableName, envName, owner, repo)
		}
		return nil
	}

	r, _, err := api.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to list repo %s/%s: %v", owner, repo, err)
	}

	for variableName, variableValue := range mappings {
		_, err = api.CreateOrUpdateEnvVariable(ctx, r.GetOwner().GetName(), r.GetName(), envName, &github.ActionsVariable{
			Name:  variableName,
			Value: variableValue,
		})
		if err != nil {
			return fmt.Errorf("failed to update variable %s in environment %s for repo %s/%s: %v", variableName, envName, owner, repo, err)
		}
	}
	return nil
}

func (r *rateLimitedGitHubAPI) PutEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.PutEnvSecrets(ctx, owner, repo, envName, mappings)
}

func (r *rateLimitedGitHubAPI) GetEnvPublicKey(ctx context.Context, repoID int, envName string) (*github.PublicKey, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.GetEnvPublicKey(ctx, repoID, envName)
}

func (r *rateLimitedGitHubAPI) CreateOrUpdateEnvSecret(ctx context.Context, repoID int, envName string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.CreateOrUpdateEnvSecret(ctx, repoID, envName, eSecret)
}

func (r *rateLimitedGitHubAPI) DeleteEnvSecret(ctx context.Context, repoID int, envName, name string) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.DeleteEnvSecret(ctx, repoID, envName, name)
}

func (r *rateLimitedGitHubAPI) ListEnvSecrets(ctx context.Context, repoID int, envName string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.ListEnvSecrets(ctx, repoID, envName, opts)
}

func (r *rateLimitedGitHubAPI) SyncEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.SyncEnvSecrets(ctx, owner, repo, envName, mappings)
}

func (r *rateLimitedGitHubAPI) PutEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.PutEnvVariables(ctx, owner, repo, envName, mappings)
}

func (r *rateLimitedGitHubAPI) CreateOrUpdateEnvVariable(ctx context.Context, owner, repo, envName string, eVariable *github.ActionsVariable) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.CreateOrUpdateEnvVariable(ctx, owner, repo, envName, eVariable)
}

func (r *rateLimitedGitHubAPI) DeleteEnvVariable(ctx context.Context, owner, repo, envName, name string) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.DeleteEnvVariable(ctx, owner, repo, envName, name)
}

func (r *rateLimitedGitHubAPI) ListEnvVariables(ctx context.Context, owner, repo, envName string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.ListEnvVariables(ctx, owner, repo, envName, opts)
}

func (r *rateLimitedGitHubAPI) SyncEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.SyncEnvVariables(ctx, owner, repo, envName, mappings)
}

// Retry

func (r *retryableGitHubAPI) CreateOrUpdateEnvSecret(ctx context.Context, repoID int, envName string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.CreateOrUpdateEnvSecret(ctx, repoID, envName, eSecret)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) DeleteEnvSecret(ctx context.Context, repoID int, envName, name string) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.DeleteEnvSecret(ctx, repoID, envName, name)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) GetEnvPublicKey(ctx context.Context, repoID int, envName string) (*github.PublicKey, *github.Response, error) {
	var publicKey *github.PublicKey
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		publicKey, resp, err = r.client.GetEnvPublicKey(ctx, repoID, envName)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return publicKey, resp, err
}

func (r *retryableGitHubAPI) ListEnvSecrets(ctx context.Context, repoID int, envName string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	var secrets *github.Secrets
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		secrets, resp, err = r.client.ListEnvSecrets(ctx, repoID, envName, opts)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return secrets, resp, err
}

func (r *retryableGitHubAPI) PutEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.PutEnvSecrets(ctx, owner, repo, envName, mappings)
	}
	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}

func (r *retryableGitHubAPI) SyncEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.SyncEnvSecrets(ctx, owner, repo, envName, mappings)
	}
	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}

func (r *retryableGitHubAPI) CreateOrUpdateEnvVariable(ctx context.Context, owner, repo, envName string, eVariable *github.ActionsVariable) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.CreateOrUpdateEnvVariable(ctx, owner, repo, envName, eVariable)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) DeleteEnvVariable(ctx context.Context, owner, repo, envName, name string) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		resp, err = r.client.DeleteEnvVariable(ctx, owner, repo, envName, name)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return resp, err
}

func (r *retryableGitHubAPI) ListEnvVariables(ctx context.Context, owner, repo, envName string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error) {
	var secrets *github.ActionsVariables
	var resp *github.Response
	var err error

	retryFunc := func() (bool, error) {
		secrets, resp, err = r.client.ListEnvVariables(ctx, owner, repo, envName, opts)
		return true, err
	}

	_, err = backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return secrets, resp, err
}

func (r *retryableGitHubAPI) PutEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.PutEnvVariables(ctx, owner, repo, envName, mappings)
	}
	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}

func (r *retryableGitHubAPI) SyncEnvVariables(ctx context.Context, owner, repo, envName string, mappings map[string]string) error {
	retryFunc := func() (bool, error) {
		return true, r.client.SyncEnvVariables(ctx, owner, repo, envName, mappings)
	}
	_, err := backoff.Retry(ctx, retryFunc, r.backoffOptions...)
	return err
}
