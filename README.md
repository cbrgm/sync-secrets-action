# Sync Secrets Action

**Sync Repository, Dependabot and Codespaces secrets + variables between GitHub repositories**

[![GitHub release](https://img.shields.io/github/release/cbrgm/sync-secrets-action.svg)](https://github.com/cbrgm/sync-secrets-action)
[![Go Report Card](https://goreportcard.com/badge/github.com/cbrgm/sync-secrets-action)](https://goreportcard.com/report/github.com/cbrgm/sync-secrets-action)
[![go-lint-test](https://github.com/cbrgm/sync-secrets-action/actions/workflows/go-lint-test.yml/badge.svg)](https://github.com/cbrgm/sync-secrets-action/actions/workflows/go-lint-test.yml)
[![go-binaries](https://github.com/cbrgm/sync-secrets-action/actions/workflows/go-binaries.yml/badge.svg)](https://github.com/cbrgm/sync-secrets-action/actions/workflows/go-binaries.yml)
[![container](https://github.com/cbrgm/sync-secrets-action/actions/workflows/container.yml/badge.svg)](https://github.com/cbrgm/sync-secrets-action/actions/workflows/container.yml)

## Inputs

- `github-token`: **Required** - The GitHub token to use. Use GitHub secrets for security.
- `target`: Optional - The repository to sync secrets and variables to. Either `target` or `query` must be set, but not both.
- `secrets`: Optional - Secrets to sync. Formatted as a string of newline-separated `KEY=VALUE` pairs.
- `variables`: Optional - Variables to sync. Formatted as a string of newline-separated `KEY=VALUE` pairs.
- `rate-limit`: Optional - Enables rate limit checking. Set to `true` to enable. Default is `false`.
- `max-retries`: Optional - Maximum number of retries for operations. Must not be smaller than zero. Default is `3`.
- `dry-run`: Optional - Dry run mode. If true, no changes will be made. Useful for testing. Default is `false`.
- `prune`: Optional - Prunes all existing secrets and variables not in the subset of those defined. Default is `false`.
- `environment`: Optional - The GitHub environment to sync variables or secrets to. Use when targeting environment-specific secrets or variables.
- `type`: Optional - Type of the secrets to manage: `actions`, `dependabot`, or `codespaces`. Default is `actions`.
- `query`: Optional - GitHub search query to find repositories for batch processing. Either `query` or `target` must be set, but not both.

## Container Usage

This action can be executed independently from workflows within a container. To do so, use the following command:

```
podman run --rm -it ghcr.io/cbrgm/sync-secrets-action:v1 --help
```

## Usage Examples

Here are some usage examples to help you getting started! Feel free to contribute more.

### Syncing Repository Secrets and Variables

```yaml
name: Sync Repository Secrets and Variables

on:
  workflow_dispatch:

jobs:
  sync-repo-secrets-and-vars:
    runs-on: ubuntu-latest
    steps:
      - name: Sync Secrets and Variables
        uses: cbrgm/sync-secrets-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          target: 'user/repository'
          secrets: |
            SECRET_KEY=${{ secrets.SOME_SECRET }}
            ANOTHER_SECRET=${{ secrets.ANOTHER_SECRET }}
          variables: |
            VAR_KEY=varvalue
            ANOTHER_VAR=${{ secrets.A_VARIABLE }}

```

### Matrix Build Example - Syncing Across Multiple Repositories

```yaml
name: Sync Secrets Across Repositories

on:
  workflow_dispatch:

jobs:
  sync-secrets-across-repos:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        target: ['user/repo1', 'user/repo2', 'user/repo3']
    steps:
      - name: Sync Secrets to ${{ matrix.target }}
        uses: cbrgm/sync-secrets-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          target: ${{ matrix.target }}
          secrets: |
            GLOBAL_SECRET=${{ secrets.GLOBAL_SECRET }}
          variables: |
            GLOBAL_VAR=globalvarvalue
```

### Query Example - Syncing to Repositories by Search Query

```yaml
name: Sync Secrets to Repositories by Query

on:
  workflow_dispatch:

jobs:
  sync-secrets-by-query:
    runs-on: ubuntu-latest
    steps:
      - name: Sync Secrets to Repositories Matching Query
        uses: cbrgm/sync-secrets-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          query: 'org:myorganization topic:mytopic'
          secrets: |
            GLOBAL_SECRET=${{ secrets.GLOBAL_SECRET }}
          variables: |
            GLOBAL_VAR=globalvarvalue
```

> This workflow uses the query argument to target repositories within `myorganization` that are tagged with the topic `mytopic`. It syncs the specified secrets and variables to all matching repositories.

See [GitHub Queries](https://docs.github.com/en/graphql/reference/queries).

### Advanced Usage - Syncing Environment Secrets

> Tip: Make sure these environments exist before distributing secrets!

```yaml
name: Sync Environment Secrets

on:
  push:
    branches:
      - main

jobs:
  sync-env-secrets:
    runs-on: ubuntu-latest
    steps:
      - name: Sync Environment Secrets
        uses: cbrgm/sync-secrets-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          target: 'user/repository'
          environment: 'production'
          secrets: |
            PROD_DB_PASSWORD=${{ secrets.PROD_DB_PASSWORD }}
          dry-run: 'false'
          prune: 'true'

```

### Sync Secrets Across Multiple Repositories and Environments

```yaml
name: Sync Secrets Across Repositories and Environments

on:
  workflow_dispatch:

jobs:
  sync-secrets:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        repo: ['org/repo1', 'org/repo2', 'org/repo3'] # Target repositories
        environment: ['development', 'staging', 'production'] # Target environments
    steps:
      - name: Sync Secrets to ${{ matrix.repo }} for ${{ matrix.environment }} Environment
        uses: cbrgm/sync-secrets-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          target: ${{ matrix.repo }}
          environment: ${{ matrix.environment }}
          secrets: |
            DATABASE_URL=${{ secrets['DB_URL_' + matrix.environment] }}
            API_KEY=${{ secrets['API_KEY_' + matrix.environment] }}
          type: 'actions'
          dry-run: 'false'
          prune: 'true'
```

>  The secrets input dynamically references secrets based on the environment. For example, `DB_URL_development`, `DB_URL_staging`, and `DB_URL_production` should be defined in your repository's secrets. This approach allows each job to use environment-specific secret values.

### Syncing Codespaces Secrets

```yaml
name: Sync Codespaces Secrets

on:
  workflow_dispatch:

jobs:
  sync-codespaces-secrets:
    runs-on: ubuntu-latest
    steps:
      - name: Sync Codespaces Secrets
        uses: cbrgm/sync-secrets-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          target: 'user/repository'
          secrets: |
            CODESPACE_SECRET=${{ secrets.CODESPACE_SECRET }}
          type: 'codespaces'

```

### Syncing Dependabot Secrets

```yaml
name: Sync Dependabot Secrets

on:
  workflow_dispatch:

jobs:
  sync-dependabot-secrets:
    runs-on: ubuntu-latest
    steps:
      - name: Sync Dependabot Secrets
        uses: cbrgm/sync-secrets-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          target: 'user/repository'
          secrets: |
            DEPENDABOT_SECRET=${{ secrets.DEPENDABOT_SECRET }}
          type: 'dependabot'
```

### Local Development

You can build this action from source using `Go`:

```bash
make build
```

## Contributing & License

* **Contributions Welcome!**: Interested in improving or adding features? Check our [Contributing Guide](https://github.com/cbrgm/pr-size-labeler-action/blob/main/CONTRIBUTING.md) for instructions on submitting changes and setting up development environment.
* **Open-Source & Free**: Developed in my spare time, available for free under [Apache 2.0 License](https://github.com/cbrgm/pr-size-labeler-action/blob/main/LICENSE). License details your rights and obligations.
* **Your Involvement Matters**: Code contributions, suggestions, feedback crucial for improvement and success. Let's maintain it as a useful resource for all 🌍.

