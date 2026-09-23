# Releasing InstrLint

1. Confirm CI is green on `main` and the intended changes are present.
2. Create an annotated semantic version tag, for example `git tag -a v0.2.1 -m "InstrLint v0.2.1"`, then push that tag with `git push origin v0.2.1`.
3. Wait for the [release workflow](../.github/workflows/release.yml). It builds six archives and prepares `checksums.txt` before creating a draft GitHub Release.
4. Inspect the draft's tag, title, six archives, and `checksums.txt`. Verify every checksum against its downloaded archive.
5. Edit the generated release notes, publish the draft manually, and confirm the intended release is marked Latest.

For a local packaging check before tagging, use `bash scripts/package-release.sh v0.2.1 <goos> <goarch> <output-dir>` from the repository root. It accepts the six targets listed in the release workflow. Keep the generated archives out of the repository.
