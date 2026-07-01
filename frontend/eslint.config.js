import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';
import tseslint from 'typescript-eslint';

/** @type {import('eslint').Linter.Config[]} */
export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...svelte.configs['flat/recommended'],
  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node
      }
    }
  },
  {
    files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser
      }
    }
  },
  {
    rules: {
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      // Static-adapter SPA with no base path: resolve() adds churn without value.
      'svelte/no-navigation-without-resolve': 'warn',
      // Reactivity is driven by $state reassignment; the rule fires on non-reactive
      // local/infra collections (timers, URL builders, throwaway copies).
      'svelte/prefer-svelte-reactivity': 'warn'
    }
  },
  {
    // Bare reads in $effect (e.g. `foo; bar;`) are the Svelte reactive-dependency
    // idiom; the rule misreads them as dead expressions. Keep it on for plain .ts.
    files: ['**/*.svelte'],
    rules: {
      '@typescript-eslint/no-unused-expressions': 'off'
    }
  },
  {
    // Vendored (noVNC), generated (shadcn ui, ambient .d.ts) — not our source.
    ignores: [
      '.svelte-kit/',
      'build/',
      'node_modules/',
      'static/',
      'src/lib/components/ui/',
      '**/*.d.ts'
    ]
  }
];
