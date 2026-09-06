# Quickstart: Portable project + machine-level stores

## Step 1 — Declare your stores once, per machine

```bash
mkdir -p ~/.config/dsops
cp examples/user-config.yaml ~/.config/dsops/config.yaml
$EDITOR ~/.config/dsops/config.yaml     # set real addresses / emails
```

Or, with home-manager, use the snippet in
[contracts/config-schema.md](./contracts/config-schema.md).

## Step 2 — Keep the project file portable

`examples/portable-project.yaml` declares **no** `secretStores:`; it only
references `store://work-vault/...`, `store://bw-work/...`,
`store://aws-work/...`.

## Step 3 — See where each store comes from

```bash
dsops --config examples/portable-project.yaml providers
dsops --config examples/portable-project.yaml doctor
```

Both print a `Configuration sources:` block and a `SOURCE` column
(`project` / `user`).

## Step 4 — The contributor experience without a user file

```bash
DSOPS_USER_CONFIG=none dsops --config examples/portable-project.yaml plan --env dev
```

The error names the store, the project file, and the exact user-config path to
create.

## Step 5 — Pin or disable in CI

```bash
DSOPS_USER_CONFIG=none dsops plan --env dev            # never read the runner's home
DSOPS_USER_CONFIG=/ci/stores.yaml dsops plan --env dev # explicit; missing → error
```

## Step 6 — Negative tests

- Add `envs:` to the user file → `dsops doctor` fails: "section 'envs' not
  allowed in user configuration".
- `chmod 666 ~/.config/dsops/config.yaml` → doctor prints a "writable by
  other users" warning.
- Declare `work-vault` in the project too → doctor reports the user entry as
  shadowed; the project definition is used.
