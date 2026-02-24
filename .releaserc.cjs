const giteaUrl = process.env.GITEA_URL || 'https://github.com';

if (!process.env.GITEA_TOKEN) {
  throw new Error('GITEA_TOKEN is required for semantic-release');
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
