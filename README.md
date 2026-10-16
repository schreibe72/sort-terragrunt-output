# sort-terragrunt-output

Terragrunt's `run-all` interleaves the output of all modules, which makes a
plan hard to read. `sort-terragrunt-output` reads that log, groups it per
module and sorts the modules into categories:

| Section | Modules |
|---|---|
| ⚠️ Changes with destroy | plan contains changes and at least one resource is destroyed |
| 🚀 Changes | plan contains changes, nothing is destroyed |
| 🕒 Deprecations | output contains `Warning: Argument is deprecated` |
| 🎉 No Changes | no changes (a module with deprecations and no changes is listed here as well) |

Every module is printed as a collapsible group with its plan summary
`Module[add/change/destroy]: <path>`. Modules are sorted alphabetically within
a section. Lines that do not belong to a module (init output, errors, the unit
tree, ...) are printed before the report, see the formats below.

## Installation

Download the binary for your architecture (`linux_amd64` or `linux_arm64`)
from the [releases](../../releases), together with `checksums.txt`:

```sh
sha256sum -c --ignore-missing checksums.txt
chmod +x sort-terragrunt-output_linux_amd64
mv sort-terragrunt-output_linux_amd64 /usr/local/bin/sort-terragrunt-output
```

The file names do not contain the version, so the latest release can be
fetched with a stable URL, e.g.
`https://github.com/schreibe72/sort-terragrunt-output/releases/latest/download/sort-terragrunt-output_linux_amd64`.

Or build it yourself (requires Go, see `go.mod`):

```sh
go build -o sort-terragrunt-output .
```

A local build reports the version `dev`; release binaries report their release
version.

## Usage

The input is the log of `terragrunt run-all plan`, in which every line looks
like this:

```
12:00:00.000 STDOUT [/path/to/module] tofu: Plan: 1 to add, 0 to change, 0 to destroy.
```

`tofu:` and `terraform:` are both recognized. ANSI color codes in the plan are
handled; they are kept in the output for the `github`, `azure` and `gitlab`
formats and removed for `markdown` and `plain`.

```sh
# from a file
sort-terragrunt-output -f run-all.log

# from stdin
terragrunt run-all plan 2>&1 | sort-terragrunt-output -p "$PWD/"
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-f` | stdin | Input file |
| `-p` | | Prefix removed from the module paths |
| `-format` | `github` | Output format, see below |
| `-hide-empty` | off | Omit sections without modules |
| `-hide-nochanges` | off | Omit the "No Changes" section |
| `-version` | | Print the version and exit |

Errors (unreadable input, unknown format) are printed to stderr and the exit
code is 1.

By default all four sections are printed, also when they are empty. The two
`-hide-*` flags work with every output format and can be combined: with both
set, only sections that contain modules are printed, and modules without
changes are left out (modules with deprecations are still listed under
"Deprecations").

### Output formats

| Format | Grouping |
|---|---|
| `github` | GitHub Actions: `::group::` / `::endgroup::` |
| `azure` | Azure DevOps: `##[group]` / `##[endgroup]` |
| `gitlab` | GitLab CI: collapsed `section_start` / `section_end` |
| `markdown` | Headings and `<details>` blocks, e.g. for PR comments or job summaries |
| `plain` | Plain text without color codes, e.g. for local use |

`github`, `azure` and `gitlab` print lines outside of modules unchanged. In
`markdown` they are collected in a collapsible block "Terragrunt output" at the
top, so errors and tree drawings cannot break the formatting. All log text is
put into code fences that are always longer than any backticks inside the text,
and module titles are HTML-escaped.
`plain` removes color codes from those lines as well (only ANSI color/style
sequences are recognized).

In `github`, `azure` and `gitlab` the log text is sanitized so that it is not
interpreted as a command of the CI system: carriage returns are replaced by
spaces, and a line that starts with `::` (GitHub) or `##` (Azure), after
optional whitespace or color codes, or that contains a GitLab `section_start:` /
`section_end:` marker gets a space inserted, e.g. `::add-mask::x` is printed as
`: :add-mask::x`. This also applies to module titles. All other text is printed
unchanged.

Examples in a GitHub Actions job:

```yaml
# collapsible groups in the job log (default format)
- run: sort-terragrunt-output -f run-all.log -p "$GITHUB_WORKSPACE/"

# job summary
- run: sort-terragrunt-output -f run-all.log -p "$GITHUB_WORKSPACE/" -format markdown >> "$GITHUB_STEP_SUMMARY"
```

## Development

```sh
go vet ./...
go test ./...
```

The output of every format is covered by golden files in `testdata/`. After an
intended change of the output, refresh them and review the diff:

```sh
go test -update ./...
git diff testdata
```

## Releases

Every push to `main` or to a branch named `rc/**` (and a manual run on one of
these branches) runs `.github/workflows/release.yml`: tests, then the Linux
binaries for `amd64` and `arm64` are built and published as a GitHub release
with checksums. Pull requests only run the tests.

- `main` publishes a final release `vX.Y.Z`.
- `rc/**` publishes a pre-release `vX.Y.Z-rc.N` (N counts up for every push),
  so a change can be tried out before it is merged.

The version is derived from the commits since the latest `v*` tag
(see `.github/scripts/next-version.sh`):

| Commit | Bump |
|---|---|
| `type!:` in the subject or `BREAKING CHANGE` in the body | major |
| `feat:` / `feat(scope):` | minor |
| anything else | patch |

Release candidates do not count as a base for the next version. The first
release is `v0.1.0`. Without new commits since the last final tag, no release
is created.
