# cliphist-cleanup

Clean up your clipboard history by deleting entries matching specific patterns.

## Usage

```bash
cliphist-cleanup <pattern> [pattern2] [pattern3] ...
```

Patterns are treated as case-insensitive regular expressions.

## Examples

Delete all git sign-off entries:
```bash
cliphist-cleanup 'Signed-off-by:'
```

Delete multiple types of git commit messages:
```bash
cliphist-cleanup '# This is a combination' 'Co-Authored-By:'
```

Delete entries starting with specific text:
```bash
cliphist-cleanup '^password:' '^token:'
```

## How It Works

1. Lists all clipboard history entries using `cliphist list`
2. Matches each entry against the provided patterns
3. Deletes matching entries using `cliphist delete-query`
4. Reports the number of entries checked and deleted

## Building

```bash
go build -o cliphist-cleanup
```

## Installing

```bash
go install go.sbr.pm/x/cmd/cliphist-cleanup@latest
```
