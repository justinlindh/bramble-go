const giteaUrl = process.env.GITEA_URL;

if (!process.env.GITEA_TOKEN) {
  throw new Error('GITEA_TOKEN is required for semantic-release');
}

if (!giteaUrl) {
  throw new Error('GITEA_URL is required for the Gitea semantic-release config');
}

module.exports = {
  branches: ['main'],
  tagFormat: 'v${version}',
  plugins: [
    '@semantic-release/commit-analyzer',
    '@semantic-release/release-notes-generator',
    [
      '@saithodev/semantic-release-gitea',
      {
        giteaUrl,
      },
    ],
  ],
};
