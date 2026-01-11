# Quickstart: psst Provider

Get started with dsops + psst in 5 minutes.

## Prerequisites

1. **psst installed**: `pip install psst` or see [psst GitHub](https://github.com/Michaelliv/psst)
2. **psst initialized**: Run `psst init` to create your vault
3. **Secrets added**: Run `psst set API_KEY` to add a secret

## Step 1: Verify psst Setup

```bash
# Check psst is working
psst list

# You should see your secrets listed
```

## Step 2: Configure dsops

Create or update your `dsops.yaml`:

```yaml
version: 1

secretStores:
  local:
    type: psst

envs:
  development:
    API_KEY:
      from:
        store: store://local/API_KEY
```

## Step 3: Test with dsops plan

```bash
dsops plan --env development
```

Expected output:
```
Planning environment: development

Secret Stores:
  ✓ local (psst)

Variables:
  API_KEY: store://local/API_KEY ✓

All secrets resolvable.
```

## Step 4: Run with dsops exec

```bash
dsops exec --env development -- printenv API_KEY
```

Your secret is injected into the command's environment!

## Using Multiple Environments

psst supports named environments. Configure them in dsops:

```yaml
version: 1

secretStores:
  dev-secrets:
    type: psst
    env: development

  staging-secrets:
    type: psst
    env: staging

envs:
  development:
    DATABASE_URL:
      from:
        store: store://dev-secrets/DATABASE_URL

  staging:
    DATABASE_URL:
      from:
        store: store://staging-secrets/DATABASE_URL
```

## Multi-Provider Setup

Use psst for local development, 1Password for production:

```yaml
version: 1

secretStores:
  local:
    type: psst

  production-vault:
    type: onepassword
    account: my-team

envs:
  development:
    API_KEY:
      from:
        store: store://local/API_KEY

  production:
    API_KEY:
      from:
        store: store://production-vault/prod/API_KEY
```

## Troubleshooting

### "psst CLI not found"

Install psst:
```bash
pip install psst
# or
pipx install psst
```

### "Vault not accessible"

Initialize psst:
```bash
psst init
```

### "Secret not found"

Check if secret exists:
```bash
psst list
psst get SECRET_NAME
```

### Authentication errors

If your vault is locked:
```bash
psst unlock
```

For CI/headless environments, set:
```bash
export PSST_PASSWORD="your-vault-password"
```

## How It Works

1. dsops calls `psst get <secret-name>` for each referenced secret
2. psst checks: local vault → global vault → environment variables
3. dsops injects resolved values into your command's environment
4. Secrets never touch disk, only exist in memory

## Next Steps

- Run `dsops doctor` to validate your configuration
- Check [Provider Documentation](/reference/providers/psst) for advanced options
- See [Multi-Provider Guide](/guides/multi-provider) for complex setups
