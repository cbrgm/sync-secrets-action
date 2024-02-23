package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	crypto_rand "crypto/rand"

	"github.com/google/go-github/v58/github"
	"golang.org/x/crypto/nacl/box"
)

// GitHubActionClient describes the GitHub Actions client for handling repository secrets.
type GitHubActionClient interface {
	// todo(cbrgm): break this up into smaller interfaces
	// Repository secrets management
	PutRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
	GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateRepoSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error)
	DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
	ListRepoSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	SyncRepoSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error

	// Repository variables management
	PutRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error
	CreateOrUpdateRepoVariable(ctx context.Context, owner, repo string, variable *github.ActionsVariable) (*github.Response, error)
	DeleteRepoVariable(ctx context.Context, owner, repo string, variableName string) (*github.Response, error)
	ListRepoVariables(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error)
	SyncRepoVariables(ctx context.Context, owner, repo string, mappings map[string]string) error

	// Environment secrets management
	PutEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error
	GetEnvPublicKey(ctx context.Context, repoID int, envName string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateEnvSecret(ctx context.Context, repoID int, envName string, eSecret *github.EncryptedSecret) (*github.Response, error)
	DeleteEnvSecret(ctx context.Context, repoID int, envName, name string) (*github.Response, error)
	ListEnvSecrets(ctx context.Context, repoID int, envName string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	SyncEnvSecrets(ctx context.Context, owner, repo, envName string, mappings map[string]string) error

	// Dependabot secrets management
	PutDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
	GetDependabotPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateDependabotSecret(ctx context.Context, owner, repo string, eSecret *github.DependabotEncryptedSecret) (*github.Response, error)
	DeleteDependabotSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
	ListDependabotSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	SyncDependabotSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error

	// Codespaces secrets management
	PutCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
	GetCodespacesPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateCodespacesSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error)
	DeleteCodespacesSecret(ctx context.Context, owner, repo, name string) (*github.Response, error)
	ListCodespacesSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error)
	SyncCodespacesSecrets(ctx context.Context, owner, repo string, mappings map[string]string) error
}

const rateLimitedMessage = "GitHub API rate limit close to being exceeded. Stopping execution"

type gitHubAPI struct {
	client                *github.Client
	rateLimitCheckEnabled bool
	dryRunEnabled         bool
}

func NewGitHubAPI(client *github.Client, rateLimitCheckEnabled, dryRunEnabled bool) *gitHubAPI {
	return &gitHubAPI{
		client:                client,
		rateLimitCheckEnabled: rateLimitCheckEnabled,
		dryRunEnabled:         dryRunEnabled,
	}
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

func (api *gitHubAPI) GetRepoPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.GetRepoPublicKey(ctx, owner, repo)
}

func (api *gitHubAPI) GetEnvPublicKey(ctx context.Context, repoID int, envName string) (*github.PublicKey, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.GetEnvPublicKey(ctx, repoID, envName)
}

func (api *gitHubAPI) CreateOrUpdateRepoSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
}

func (api *gitHubAPI) CreateOrUpdateEnvSecret(ctx context.Context, repoID int, envName string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.CreateOrUpdateEnvSecret(ctx, repoID, envName, eSecret)
}

func (api *gitHubAPI) DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}

	return api.client.Actions.DeleteRepoSecret(ctx, owner, repo, name)
}

func (api *gitHubAPI) DeleteEnvSecret(ctx context.Context, repoID int, envName, name string) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.DeleteEnvSecret(ctx, int(repoID), envName, name)
}

func (api *gitHubAPI) ListEnvSecrets(ctx context.Context, repoID int, envName string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.ListEnvSecrets(ctx, repoID, envName, opts)
}

func (api *gitHubAPI) ListRepoSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.ListRepoSecrets(ctx, owner, repo, opts)
}

func (api *gitHubAPI) CreateOrUpdateRepoVariable(ctx context.Context, owner, repo string, variable *github.ActionsVariable) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.CreateRepoVariable(ctx, owner, repo, variable)
}

func (api *gitHubAPI) DeleteRepoVariable(ctx context.Context, owner, repo string, variableName string) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.DeleteRepoVariable(ctx, owner, repo, variableName)
}

func (api *gitHubAPI) ListRepoVariables(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.ActionsVariables, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Actions.ListRepoVariables(ctx, owner, repo, opts)
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

func (api *gitHubAPI) GetDependabotPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Dependabot.GetRepoPublicKey(ctx, owner, repo)
}

func (api *gitHubAPI) CreateOrUpdateDependabotSecret(ctx context.Context, owner, repo string, eSecret *github.DependabotEncryptedSecret) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Dependabot.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
}

func (api *gitHubAPI) DeleteDependabotSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Dependabot.DeleteRepoSecret(ctx, owner, repo, name)
}

func (api *gitHubAPI) ListDependabotSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Dependabot.ListRepoSecrets(ctx, owner, repo, opts)
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

func (api *gitHubAPI) GetCodespacesPublicKey(ctx context.Context, owner, repo string) (*github.PublicKey, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Codespaces.GetRepoPublicKey(ctx, owner, repo)
}

func (api *gitHubAPI) CreateOrUpdateCodespacesSecret(ctx context.Context, owner, repo string, eSecret *github.EncryptedSecret) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Codespaces.CreateOrUpdateRepoSecret(ctx, owner, repo, eSecret)
}

func (api *gitHubAPI) DeleteCodespacesSecret(ctx context.Context, owner, repo, name string) (*github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Codespaces.DeleteRepoSecret(ctx, owner, repo, name)
}

func (api *gitHubAPI) ListCodespacesSecrets(ctx context.Context, owner, repo string, opts *github.ListOptions) (*github.Secrets, *github.Response, error) {
	if api.isRateLimitExceeded(ctx) {
		log.Println(rateLimitedMessage)
		os.Exit(0)
	}
	return api.client.Codespaces.ListRepoSecrets(ctx, owner, repo, opts)
}

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

func (g *gitHubAPI) isRateLimitExceeded(ctx context.Context) bool {
	rateLimitStatus, _, err := g.client.RateLimit.Get(ctx)
	if err != nil {
		log.Printf("Error fetching rate limit status: %v\n", err)
		return false
	}

	limit := rateLimitStatus.Core.Limit
	remaining := rateLimitStatus.Core.Remaining

	return float64(remaining)/float64(limit) <= 0.05
}

func encryptSecretWithPublicKey(publicKey *github.PublicKey, secretName, secretValue string) (*github.EncryptedSecret, error) {
	decodedPublicKey, err := base64.StdEncoding.DecodeString(publicKey.GetKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	var boxKey [32]byte
	copy(boxKey[:], decodedPublicKey)
	secretBytes := []byte(secretValue)
	encryptedBytes, err := box.SealAnonymous([]byte{}, secretBytes, &boxKey, crypto_rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %v", err)
	}

	encryptedString := base64.StdEncoding.EncodeToString(encryptedBytes)

	keyID := publicKey.GetKeyID()
	encryptedSecret := &github.EncryptedSecret{
		Name:           secretName,
		KeyID:          keyID,
		EncryptedValue: encryptedString,
	}
	return encryptedSecret, nil
}

func encryptDependabotWithPublicKey(publicKey *github.PublicKey, secretName, secretValue string) (*github.DependabotEncryptedSecret, error) {
	decodedPublicKey, err := base64.StdEncoding.DecodeString(publicKey.GetKey())
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	var boxKey [32]byte
	copy(boxKey[:], decodedPublicKey)
	secretBytes := []byte(secretValue)
	encryptedBytes, err := box.SealAnonymous([]byte{}, secretBytes, &boxKey, crypto_rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt secret: %v", err)
	}

	encryptedString := base64.StdEncoding.EncodeToString(encryptedBytes)

	keyID := publicKey.GetKeyID()
	encryptedSecret := &github.DependabotEncryptedSecret{
		Name:           secretName,
		KeyID:          keyID,
		EncryptedValue: encryptedString,
	}
	return encryptedSecret, nil
}
