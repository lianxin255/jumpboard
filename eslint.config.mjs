// @ts-check
import antfu from '@antfu/eslint-config'

export default antfu({
  type: 'app',
  vue: true,
  typescript: true,
  formatters: false,
  jsonc: false,
  yaml: false,
  ignores: [
    '**/dist/**',
    '**/target/**',
    '**/node_modules/**',
    'build/**',
    'apps/desktop/gen/**',
    'apps/desktop/.stage/**',
    'resources/**',
    'data/**',
    'release/**',
    'out/**',
    'docs/**',
  ],
}, {
  rules: {
    'style/quotes': 'off',
    'style/semi': 'off',
    'no-console': 'off',
    'node/prefer-global/process': 'warn',
    'style/max-statements-per-line': 'warn',
    'style/indent': 'warn',
    'regexp/no-super-linear-backtracking': 'warn',
    'regexp/no-unused-capturing-group': 'warn',
    'node/prefer-global/buffer': 'warn',
    'no-use-before-define': 'warn',
    'ts/no-use-before-define': 'warn',
    'ts/no-require-imports': 'warn',
    'ts/no-var-requires': 'warn',
  },
})
