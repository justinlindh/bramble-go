// semantic-release config for the GitHub-hosted mirror.
// Uses the GITHUB_TOKEN provided automatically by GitHub Actions; the release
// job grants it `contents: write` so tags and GitHub Releases can be created.
// The Gitea mirror keeps its own frozen config in .releaserc.cjs.
module.exports = {
  branches: ['main'],
  tagFormat: 'v${version}',
  plugins: [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
    '@semantic-release/github',
  ],
};
