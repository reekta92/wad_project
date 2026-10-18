import js from '@eslint/js';
import accessibility from 'eslint-plugin-jsx-a11y';

const browser = Object.fromEntries(['window', 'document', 'fetch', 'Response', 'URL', 'URLSearchParams', 'FormData', 'localStorage', 'sessionStorage', 'AbortController', 'AbortSignal', 'TextDecoder', 'atob', 'setTimeout', 'clearTimeout', 'console'].map((name) => [name, 'readonly']));

export default [
  { ignores: ['dist/**', 'node_modules/**'] },
  {
    files: ['src/**/*.{js,jsx}'],
    ...js.configs.recommended,
    languageOptions: { ecmaVersion: 'latest', sourceType: 'module', parserOptions: { ecmaFeatures: { jsx: true } }, globals: browser },
    plugins: { 'jsx-a11y': accessibility },
    rules: {
      ...js.configs.recommended.rules,
      ...accessibility.configs.recommended.rules,
      'no-unused-vars': ['error', { varsIgnorePattern: '^[A-Z]', argsIgnorePattern: '^_' }],
      // Labeled scroll regions need keyboard focus even though they are not widgets.
      'jsx-a11y/no-noninteractive-tabindex': ['error', { roles: ['region', 'tabpanel'] }],
    },
  },
];
