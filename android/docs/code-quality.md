# Kotlin code quality

Android Lint still runs in CI. On top of it, every Kotlin source and Gradle
script is checked by two pinned command-line tools:

| Tool | Version | Purpose |
| --- | --- | --- |
| [ktlint](https://pinterest.github.io/ktlint/) | 1.7.1 | Formatting |
| [detekt](https://detekt.dev/) | 1.23.8 | Static analysis |

They run as standalone CLIs rather than Gradle plugins, so they don't depend on
the Kotlin/AGP versions in the build and don't need the Android SDK.

## Commands

From `android/` (needs Java 17+ and `bash`; on Windows use Git Bash or WSL):

```sh
scripts/kotlin-checks.sh            # check formatting and run detekt
scripts/kotlin-checks.sh --format   # fix formatting, then run detekt
```

The script downloads the pinned tools once into `android/.tools/` (git-ignored).
CI runs the same script in the `Kotlin format and static analysis` job.
`build/` directories are excluded.

## Rule set and rationale

**ktlint** uses the `android_studio` code style from
[`.editorconfig`](../.editorconfig), matching Android Studio's formatter, with
120-character lines. Two adjustments:

- `@Composable` functions are allowed to be PascalCase.
- Call-site trailing commas are not enforced. The code used them
  inconsistently and enforcing them would add hundreds of lines of churn.

The existing code was reformatted once in a separate formatting-only commit, so
there is no ktlint baseline.

**detekt** uses its default rules plus the overrides in
[`config/detekt/detekt.yml`](../config/detekt/detekt.yml): PascalCase allowed
for `@Composable` functions and top-level constants, magic numbers allowed in
property declarations and Compose code, and guard-clause early returns not
counted against `ReturnCount`.

Existing findings (magic numbers, long methods and parameter lists, and similar)
are recorded in [`config/detekt/baseline.xml`](../config/detekt/baseline.xml)
so this change makes no behaviour changes. New code must pass without being added to
the baseline. When you fix a baselined finding, remove its entry. To regenerate
it after a deliberate decision, run detekt with `--create-baseline` using the
same arguments as the script.
