package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/go-github/v59/github"
)

// GitHubRepoSecrets for GitHub repository secrets management.
type GitHubRepoSecrets interface {
	CreateOrUpdateRepoSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error)
	DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
	GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	ListRepoSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	PutRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
	SyncRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
}

// GitHubRepoVariables for GitHub repository variables management.
type GitHubRepoVariables interface {
	CreateOrUpdateRepoVariable(ctx context.Context, owner, repo string, variable *github.ActionsVariable) (*github.Response, error)
	DeleteRepoVariable(ctx context.Context, owner, repo, variableName string) (*github.Response, error)
	ListRepoVariables(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error)
	PutRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error
	SyncRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error
}

func (api *gitHubAPI) GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	return api.client.Actions.GetRepoPublicKey(ctx, owner, repo)
}

func (api *gitHubAPI) CreateOrUpdateRepoSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	return api.client.Actions.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
}

func (api *gitHubAPI) DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	return api.client.Actions.DeleteRepoSecret(ctx, owner, repo, name)
}

func (api *gitHubAPI) ListRepoSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	return api.client.Actions.ListRepoSecrets(ctx, owner, repo, opts)
}

func (api *gitHubAPI) CreateOrUpdateRepoVariable(ctx context.Context, owner, repo string, variable *github.ActionsVariable) (*github.Response, error) {
	return api.client.Actions.CreateRepoVariable(ctx, owner, repo, variable)
}

func (api *gitHubAPI) DeleteRepoVariable(ctx context.Context, owner, repo, variableName string) (*github.Response, error) {
	return api.client.Actions.DeleteRepoVariable(ctx, owner, repo, variableName)
}

func (api *gitHubAPI) ListRepoVariables(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error) {
	return api.client.Actions.ListRepoVariables(ctx, owner, repo, opts)
}

func (api *gitHubAPI) SyncRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Syncing repository secrets for repo %s/%s\n", owner, repo)
		opts := &github.ListOptions{PerPage: 100}
		for {
			secrets, resp, err := api.ListRepoSecrets(ctx, owner, repo, opts)
			if err != nil {
				return fmt.Errorf("dry run: failed to list existing secrets: %v", err)
			}

			for _, secret := range secrets.Secrets {
				if _, exists := mappings[secret.Name]; !exists {
					log.Printf("Dry run: Would delete secret '%s' from repo %s/%s\n", secret.Name, owner, repo)
				}
			}
			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		for secretName := range mappings {
			log.Printf("Dry run: Would add/update secret '%s' in repo %s/%s\n", secretName, owner, repo)
		}

		return nil
	}

	existingMap := make(map[string]bool)

	opts := &github.ListOptions{PerPage: 100}
	for {
		secrets, resp, err := api.ListRepoSecrets(ctx, owner, repo, opts)
		if err != nil {
			return fmt.Errorf("failed to list existing secrets: %v", err)
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
			_, err := api.DeleteRepoSecret(ctx, owner, repo, secretName)
			if err != nil {
				return fmt.Errorf("failed to delete secret %s: %v", secretName, err)
			}
		}
	}

	return api.PutRepoSecrets(ctx, owner, repo, mappings)
}

func (api *gitHubAPI) PutRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Putting repository secrets for repo %s/%s\n", owner, repo)
		for secretName := range mappings {
			log.Printf("Dry run: Would put secret '%s' in repo %s/%s\n", secretName, owner, repo)
		}
		return nil
	}

	publicKey, _, err := api.GetRepoPublicKey(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get public key for repo %s/%s: %v", owner, repo, err)
	}

	for secretName, secretValue := range mappings {
		secret, err := encryptSecretWithPublicKey(publicKey, secretName, secretValue)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret %s: %v", secretName, err)
		}
		_, err = api.CreateOrUpdateRepoSecret(ctx, owner, repo, secret)
		if err != nil {
			return fmt.Errorf("failed to update secret %s in repo %s/%s: %v", secretName, owner, repo, err)
		}
	}
	return nil
}

func (api *gitHubAPI) SyncRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Syncing repository variables for repo %s/%s", owner, repo)
		opts := &github.ListOptions{PerPage: 100}
		for {
			variables, resp, err := api.ListRepoVariables(ctx, owner, repo, opts)
			if err != nil {
				return fmt.Errorf("dry run: failed to list existing variables: %v", err)
			}

			for _, variable := range variables.Variables {
				if _, exists := mappings[variable.Name]; !exists {
					log.Printf("Dry run: Would delete variable '%s' from repo %s/%s", variable.Name, owner, repo)
				}
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}

		for variableName := range mappings {
			log.Printf("Dry run: Would add/update variable '%s' in repo %s/%s", variableName, owner, repo)
		}

		return nil
	}

	existingMap := make(map[string]bool)

	opts := &github.ListOptions{PerPage: 100}
	for {
		variables, resp, err := api.ListRepoVariables(ctx, owner, repo, opts)
		if err != nil {
			return fmt.Errorf("failed to list existing variables: %v", err)
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
			_, err := api.DeleteRepoVariable(ctx, owner, repo, variableName)
			if err != nil {
				return fmt.Errorf("failed to delete variable %s: %v", variableName, err)
			}
		}
	}

	// Add or update variables from mappings
	return api.PutRepoVariables(ctx, owner, repo, mappings)
}

func (api *gitHubAPI) PutRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error {
	if api.dryRunEnabled {
		log.Printf("Dry run: Putting repository variables for repo %s/%s", owner, repo)
		for variableName, variableValue := range mappings {
			log.Printf("Dry run: Would put variable '%s' with value '%s' in repo %s/%s", variableName, variableValue, owner, repo)
		}
		return nil
	}

	for secretName, secretValue := range mappings {
		_, err := api.CreateOrUpdateRepoVariable(ctx, owner, repo, &github.ActionsVariable{
			Name:  secretName,
			Value: secretValue,
		})
		if err != nil {
			return fmt.Errorf("failed to update secret %s in repo %s/%s: %v", secretName, owner, repo, err)
		}
	}
	return nil
}

func (r *rateLimitedGitHubAPI) PutRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.PutRepoSecrets(ctx, owner, repo, mappings)
}

func (r *rateLimitedGitHubAPI) GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.GetRepoPublicKey(ctx, owner, repo)
}

func (r *rateLimitedGitHubAPI) CreateOrUpdateRepoSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
}

func (r *rateLimitedGitHubAPI) DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.DeleteRepoSecret(ctx, owner, repo, name)
}

func (r *rateLimitedGitHubAPI) ListRepoSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.ListRepoSecrets(ctx, owner, repo, opts)
}

func (r *rateLimitedGitHubAPI) SyncRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.SyncRepoSecrets(ctx, owner, repo, mappings)
}

func (r *rateLimitedGitHubAPI) PutRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.PutRepoVariables(ctx, owner, repo, mappings)
}

func (r *rateLimitedGitHubAPI) CreateOrUpdateRepoVariable(ctx context.Context, owner, repo string, variable *github.ActionsVariable) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.CreateOrUpdateRepoVariable(ctx, owner, repo, variable)
}

func (r *rateLimitedGitHubAPI) DeleteRepoVariable(ctx context.Context, owner, repo, variableName string) (*github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.DeleteRepoVariable(ctx, owner, repo, variableName)
}

func (r *rateLimitedGitHubAPI) ListRepoVariables(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error) {
	r.ensureRatelimits(ctx)
	return r.client.ListRepoVariables(ctx, owner, repo, opts)
}

func (r *rateLimitedGitHubAPI) SyncRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error {
	r.ensureRatelimits(ctx)
	return r.client.SyncRepoVariables(ctx, owner, repo, mappings)
}

// Retryable

// GitHubRepoSecrets implementations.
func (r *retryableGitHubAPI) CreateOrUpdateRepoSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() error {
		resp, err = r.client.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return resp, err
}

func (r *retryableGitHubAPI) DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() error {
		resp, err = r.client.DeleteRepoSecret(ctx, owner, repo, name)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return resp, err
}

func (r *retryableGitHubAPI) GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	var publicKey *github.PublicKey
	var resp *github.Response
	var err error

	retryFunc := func() error {
		publicKey, resp, err = r.client.GetRepoPublicKey(ctx, owner, repo)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return publicKey, resp, err
}

func (r *retryableGitHubAPI) ListRepoSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	var secrets *github.Secrets
	var resp *github.Response
	var err error

	retryFunc := func() error {
		secrets, resp, err = r.client.ListRepoSecrets(ctx, owner, repo, opts)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return secrets, resp, err
}

func (r *retryableGitHubAPI) PutRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() error {
		return r.client.PutRepoSecrets(ctx, owner, repo, mappings)
	}
	return backoff.Retry(retryFunc, r.backoffOptions)
}

func (r *retryableGitHubAPI) SyncRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() error {
		return r.client.SyncRepoSecrets(ctx, owner, repo, mappings)
	}
	return backoff.Retry(retryFunc, r.backoffOptions)
}

func (r *retryableGitHubAPI) CreateOrUpdateRepoVariable(ctx context.Context, owner, repo string, variable *github.ActionsVariable) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() error {
		resp, err = r.client.CreateOrUpdateRepoVariable(ctx, owner, repo, variable)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return resp, err
}

func (r *retryableGitHubAPI) DeleteRepoVariable(ctx context.Context, owner, repo, variableName string) (*github.Response, error) {
	var resp *github.Response
	var err error

	retryFunc := func() error {
		resp, err = r.client.DeleteRepoVariable(ctx, owner, repo, variableName)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return resp, err
}

func (r *retryableGitHubAPI) ListRepoVariables(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error) {
	var variables *github.ActionsVariables
	var resp *github.Response
	var err error

	retryFunc := func() error {
		variables, resp, err = r.client.ListRepoVariables(ctx, owner, repo, opts)
		return err
	}

	err = backoff.Retry(retryFunc, r.backoffOptions)
	return variables, resp, err
}

func (r *retryableGitHubAPI) PutRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() error {
		return r.client.PutRepoVariables(ctx, owner, repo, mappings)
	}
	return backoff.Retry(retryFunc, r.backoffOptions)
}

func (r *retryableGitHubAPI) SyncRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error {
	retryFunc := func() error {
		return r.client.SyncRepoVariables(ctx, owner, repo, mappings)
	}
	return backoff.Retry(retryFunc, r.backoffOptions)
}
