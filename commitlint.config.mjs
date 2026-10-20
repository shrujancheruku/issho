// Conventional Commits: https://www.conventionalcommits.org
// Scopes are optional; prefer the area of the repo, e.g. feat(backend): ..., fix(mobile): ...
export default {
  extends: ["@commitlint/config-conventional"],
  rules: {
    "scope-enum": [1, "always", ["backend", "mobile", "admin", "api", "db", "matching", "data", "infra", "ci", "deps", "release"]],
  },
};
