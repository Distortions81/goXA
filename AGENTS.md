# AGENTS Instructions for goXA

These instructions apply to all files in this repository.

## Required checks
- Run `go vet ./...` for static analysis.
- Run unit tests with `go test ./...`.
- For full end‑to‑end testing, run the `./test-goxa.sh` script.

## Formatting
- Ensure all `.go` files are formatted with `gofmt -w`.

## Documentation
- Update the in-app help in main.go, keep it similar to other open-source cli utility programs.
- Update `README.md` when you add new flags or change usage examples. The readme should be concise but friendly 'golang style'.
- Update goxa.1 man when you add new flags or change usage examples. The man file should be verbose, detailed and technical.
- Update FILE-FORMAT.md if the archive format is modified. It should be absurdly detailed technical and verbose. Even tangently related information should be included. This is for developers designing software compatible with our standard.
- Update JSON-LIST-FORMAT.md if you edit the json file listing output format.
- Update ATTRIBUTION.md if you add or remove imports (check go.mod)

## Commits and PRs
- Use concise commit messages (first line under 72 characters).
- Reference relevant files or lines in your pull request summary.
