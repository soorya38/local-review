## Requirements
- A [Groq API key]

---

## Installation

```bash
git clone https://github.com/soorya38/local-review.git
cd local-review
make install
```

`make install` builds the binary and copies it to `/opt/homebrew/bin/lr`.

To rebuild and reinstall after any code change:

```bash
make install
```

---

## Configuration

Settings can be persisted so you do not have to pass flags on every command.

```bash
lr config set groq-key <your-groq-api-key>
lr config set standards /path/to/CODING_STANDARDS.md
```

Settings are stored in `~/.config/lr/config`.

To inspect what is currently configured:

```bash
lr config verify
```

This shows which value will be used for each setting and where it comes from (flag, environment variable, config file, or default).

### Priority order

For each setting, the value is resolved in this order (highest to lowest):

```
CLI flag  >  environment variable  >  config file  >  default
```

The environment variable for the API key is `GROQ_API_KEY`. If it is set in your shell, it takes precedence over the config file. Use `lr config verify` to confirm which value is active.

---

## Running a Review

```bash
lr -r -b <base-branch> <target-branch>
```

### Flags

| Flag | Short | Required | Description |
|---|---|---|---|
| `--branch` | `-b` | Yes | Base branch to compare from |
| `--review` | `-r` | Yes | Triggers the review pipeline |
| `--groq-key` | `-k` | No | API key, overrides env var and config file |
| `--standards` | `-s` | No | Path to coding standards file, overrides config file |

### Examples

```bash
# Basic review — after config is set
lr -r -b main feature/auth

# Supply key and standards inline without saving to config
lr -r -k gsk_xxx -s ./CODING_STANDARDS.md -b main feature/auth

# API key via environment variable
GROQ_API_KEY=gsk_xxx lr -r -b main feature/auth
```

---

## Coding Standards

The file at `CODING_STANDARDS.md` in the project root is sent verbatim to the LLM as the review context. Edit it to match your team's standards.

To use a different file:

```bash
lr config set standards /path/to/your/standards.md
```

---

## Config Subcommands

```bash
lr config set <key> <value>   # Save a setting
lr config get <key>           # Print the stored value of a setting
lr config list                # List all stored settings
lr config unset <key>         # Remove a setting
lr config verify              # Show which value and source will be used at runtime
```

Supported keys: `groq-key`, `standards`

---

## Makefile Targets

```bash
make build    # Compile the binary to ./lr
make install  # Build and install to /opt/homebrew/bin/lr
```

---

## Troubleshooting

**401 Invalid API Key**

Run `lr config verify` to see which key is being used and from which source. If the env var `GROQ_API_KEY` is set in your shell, it overrides the config file:

```bash
unset GROQ_API_KEY
lr config verify
```

**Branch does not exist**

Both branches must exist locally. Run `git fetch` if the branch was created remotely.

**No standards file found**

If the standards file path is wrong, the review still runs but without coding standards context. A warning is printed. Use `lr config verify` to check the configured path.
