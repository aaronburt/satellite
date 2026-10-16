# Security Policy

## Supported Versions

Security fixes are applied to the latest release of each app (Windows & Linux in `windows/`, Android in `android/`). Older releases are not patched.

## Reporting a Vulnerability

Please do not report security issues in public issues or pull requests.

Report them privately using GitHub's [private vulnerability reporting](https://github.com/aaronburt/satellite/security/advisories/new). Include:

- Which app is affected (Windows, Linux or Android) and its version
- A description of the issue and its impact
- Steps to reproduce, or a proof of concept if you have one

This is a personal project, so responses are best-effort. You can expect an initial reply within a few days, and a fix or mitigation as soon as practical for confirmed issues. Reporters are credited in the advisory unless they prefer otherwise.

## Scope

Both apps connect to an MQTT broker and Home Assistant that you run. Issues in the broker, Home Assistant or the operating system itself should be reported to those projects.
