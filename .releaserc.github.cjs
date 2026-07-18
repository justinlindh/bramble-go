// semantic-release config for the GitHub-hosted repo.
//
// Uses the GITHUB_TOKEN provided automatically by GitHub Actions; the release
// job grants it `contents: write` so tags and GitHub Releases can be created.
// The Gitea mirror keeps its own frozen config in .releaserc.cjs.
//
// Wraps the standard commit-analyzer / release-notes-generator pair in
// scripts/release/semantic-release-squash-expander so GitHub squash-merge
// commits get expanded into their per-bullet virtual commits before the
// release decision. Without this, the analyzer reads only the squash subject
// (the PR title); a PR titled with a non-releasing type (e.g. "docs: ...")
// whose body bullets contain a feat/fix would be missed.
const path = require('path');
const squashExpander = path.resolve(__dirname, 'scripts', 'release', 'semantic-release-squash-expander.cjs');
// The expander lives outside any project's node_modules and would fail to
// resolve these otherwise; load them here where they're installed and pass
// them through plugin options.
const wrapped = {
  commitAnalyzer: require('@semantic-release/commit-analyzer'),
  releaseNotesGenerator: require('@semantic-release/release-notes-generator'),
};

module.exports = {
  branches: ['main'],
  tagFormat: 'v${version}',
  plugins: [
    [squashExpander, { _wrapped: wrapped }],
    '@semantic-release/github',
  ],
};
