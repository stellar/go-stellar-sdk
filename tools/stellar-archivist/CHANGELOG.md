# Changelog

All notable changes to this project will be documented in this
file.  This project adheres to [Semantic Versioning](http://semver.org/).

As this project is pre 1.0, breaking changes may happen for minor version
bumps.  A breaking change will get clearly notified in this log.

## ???

* Fix race condition in `mirror` command
* Dropped support for Go 1.10, 1.11, 1.12.
* Add `log` command
* Add `--recent` flag for `mirror` command
* Improve logging to use structured logging and color, add `--trace`
* Add `--skip-optional` flag to skip optional (SCP) checkpoint files

## [v0.1.0] - 2016-08-17

Initial release after import from https://github.com/stellar/archivist

[Unreleased]: https://github.com/stellar/go-stellar-sdk/compare/stellar-archivist-v0.1.0...master
