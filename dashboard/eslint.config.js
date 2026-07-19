import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import globals from 'globals'

export default tseslint.config(
  // Ignore build artifacts and generated files
  {
    ignores: ['dist/**', '*.d.ts'],
  },

  // Base: JS + TS recommendations
  js.configs.recommended,
  ...tseslint.configs.recommended,

  // React plugin setup
  {
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.es2022,
        ...globals.node,
      },
    },
    rules: {
      // React Hooks — strict correctness
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',

      // React Refresh — catch accidental production JSX
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],

      // Reasonable TS defaults
      '@typescript-eslint/no-unused-vars': 'warn',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/consistent-type-imports': 'error',

      // General
      'no-console': 'warn',
    },
  },
)
