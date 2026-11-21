// Conventional Commits: https://www.conventionalcommits.org
// Scopes are optional; prefer the area of the repo, e.g. feat(backend): ..., fix(app): ...
export default {
  extends: ["@commitlint/config-conventional"],
  rules: {
    "scope-enum": [1, "always", ["backend", "app", "admin", "api", "db", "matching", "data", "infra", "ci", "deps", "release"]],
  },
};
