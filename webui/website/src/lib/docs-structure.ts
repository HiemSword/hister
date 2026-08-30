interface DocsCategory {
  name: string;
  slugs: string[];
  color: string;
}

export const docsStructure: DocsCategory[] = [
  {
    name: 'Start Here',
    slugs: ['intro', 'quickstart', 'installing', 'query-language'],
    color: 'indigo',
  },
  {
    name: 'Collecting Content',
    slugs: ['browser-ingestion', 'browser-extension', 'import', 'crawler', 'file-types'],
    color: 'coral',
  },
  {
    name: 'Data and Access',
    slugs: ['user-handling', 'rules', 'data-lifecycle'],
    color: 'amber',
  },
  {
    name: 'Search and Integrations',
    slugs: ['terminal-client', 'extractors', 'mcp'],
    color: 'lime',
  },
  {
    name: 'Running Hister',
    slugs: ['configuration', 'server-setup', 'docker', 'troubleshooting'],
    color: 'teal',
  },
  {
    name: 'Development',
    slugs: ['developer'],
    color: 'indigo',
  },
];
