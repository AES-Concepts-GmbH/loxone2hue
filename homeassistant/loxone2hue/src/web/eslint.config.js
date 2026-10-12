// ESLint flat config for the Web UI. `npm run lint` used to have no
// configuration at all and aborted immediately; this mirrors the Vite React
// TypeScript template so lint can run in CI.
import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['dist', 'node_modules'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      // Only the two classic hook rules. The React-Compiler rules that
      // eslint-plugin-react-hooks 6+ adds to "recommended" flag established
      // patterns in this code base (fetch in useEffect, reconnect closures)
      // and are a refactoring topic of their own, not a lint gate.
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
    },
  },
);
