# Changelog

## [1.4.0](https://github.com/et-do/no-pilot/compare/v1.3.0...v1.4.0) (2026-04-26)


### Features

* **execute:** expand runTests with language inference and runner support ([35ea937](https://github.com/et-do/no-pilot/commit/35ea9373030ebaacfbc8928ff8344934bd9e5578))
* **policy:** add strict self-protect validation and terminal/edit po… ([#32](https://github.com/et-do/no-pilot/issues/32)) ([35ea937](https://github.com/et-do/no-pilot/commit/35ea9373030ebaacfbc8928ff8344934bd9e5578))

## [1.3.0](https://github.com/et-do/no-pilot/compare/v1.2.0...v1.3.0) (2026-04-26)


### Features

* add web_fetch tool with DOM extraction, caching, and selectors ([#31](https://github.com/et-do/no-pilot/issues/31)) ([fbe3e68](https://github.com/et-do/no-pilot/commit/fbe3e68438dbec9bbeabf50de34fe1850975f942))
* **execute:** add per-session cwd/env, terminal listing, and ranged output reads ([#26](https://github.com/et-do/no-pilot/issues/26)) ([ab9537a](https://github.com/et-do/no-pilot/commit/ab9537acd759c1d966f8713e566d07d717a91a6e))
* **execute:** add standalone notebook execution, test tooling, and t… ([#29](https://github.com/et-do/no-pilot/issues/29)) ([ff864a6](https://github.com/et-do/no-pilot/commit/ff864a6ed7a6e90c67da11db8434ba221bd5472a))
* expand standalone MCP tool coverage and harden policy enforcement ([#28](https://github.com/et-do/no-pilot/issues/28)) ([1cb604f](https://github.com/et-do/no-pilot/commit/1cb604fb4f48f8ac2c54c0fbbb256512ffe694e9))
* **web:** add web_fetch with guarded static extraction, selectors, and cache revalidation ([#30](https://github.com/et-do/no-pilot/issues/30)) ([3e84233](https://github.com/et-do/no-pilot/commit/3e842337c362c1fa1dcb1479d15898b3567df43c))

## [1.2.0](https://github.com/et-do/no-pilot/compare/v1.1.0...v1.2.0) (2026-04-25)


### Features

* add edit/createFile tool with policy enforcement and no-overwrite semantics ([#24](https://github.com/et-do/no-pilot/issues/24)) ([0ad7aa5](https://github.com/et-do/no-pilot/commit/0ad7aa56fe089624fb2ab3c09e7c6c59c85a7b21))
* add live policy hot-reload and provider-based enforcement ([#20](https://github.com/et-do/no-pilot/issues/20)) ([97f4858](https://github.com/et-do/no-pilot/commit/97f4858501f89bf1b5e34f2b185a3e18f38c65ad))
* add search/fileSearch tool with glob and maxResults support ([#22](https://github.com/et-do/no-pilot/issues/22)) ([2f8924d](https://github.com/et-do/no-pilot/commit/2f8924d96c69308f964f2af645b6dfa977aed2b5))


### Bug Fixes

* enforce read_problems deny_paths using filePath arg ([2f8924d](https://github.com/et-do/no-pilot/commit/2f8924d96c69308f964f2af645b6dfa977aed2b5))
* enforce read_problems deny_paths using filePath arg ([7c86361](https://github.com/et-do/no-pilot/commit/7c86361356c4fbd78c9baf89266c2e00c5e6a666))
* enforce read_problems deny_paths using filePath arg ([97f4858](https://github.com/et-do/no-pilot/commit/97f4858501f89bf1b5e34f2b185a3e18f38c65ad))
* fail closed on invalid deny_commands and deny_urls patterns  ([#21](https://github.com/et-do/no-pilot/issues/21)) ([7c86361](https://github.com/et-do/no-pilot/commit/7c86361356c4fbd78c9baf89266c2e00c5e6a666))

## [1.1.0](https://github.com/et-do/no-pilot/compare/v1.0.3...v1.1.0) (2026-04-24)


### Features

* add command-intersection, improve security policies  ([#16](https://github.com/et-do/no-pilot/issues/16)) ([c2d5934](https://github.com/et-do/no-pilot/commit/c2d5934aedb0af118be3e170b6138915d50845e0))


### Bug Fixes

* mcp setup ([#15](https://github.com/et-do/no-pilot/issues/15)) ([7dabb9b](https://github.com/et-do/no-pilot/commit/7dabb9bbf29c9be1da1f0b537501ffa0a08186f6))

## [1.0.3](https://github.com/et-do/no-pilot/compare/v1.0.2...v1.0.3) (2026-04-24)


### Bug Fixes

* invalid tool names ([#13](https://github.com/et-do/no-pilot/issues/13)) ([404e1d8](https://github.com/et-do/no-pilot/commit/404e1d8b5767a2ccc9c6fc7508cd668681406d3d))

## [1.0.2](https://github.com/et-do/no-pilot/compare/v1.0.1...v1.0.2) (2026-04-24)


### Bug Fixes

* add pr title action ([#9](https://github.com/et-do/no-pilot/issues/9)) ([cca2c59](https://github.com/et-do/no-pilot/commit/cca2c594136ca570493ecd0ca27fb60e03716ad5))

## [1.0.1](https://github.com/et-do/no-pilot/compare/v1.0.0...v1.0.1) (2026-04-24)


### Bug Fixes

* trigger release ([#5](https://github.com/et-do/no-pilot/issues/5)) ([3b8e47d](https://github.com/et-do/no-pilot/commit/3b8e47daa1f2230f50b45c5a68a994c5cf391374))

## 1.0.0 (2026-04-24)


### Features

* add dev env ([ecce206](https://github.com/et-do/no-pilot/commit/ecce2064b5faf187386abc9c0d013158431a860f))
* add initial execute tools ([a0d8359](https://github.com/et-do/no-pilot/commit/a0d8359035dbb0d4261e852926a6420f3153887d))
* add initial read tools ([7777432](https://github.com/et-do/no-pilot/commit/7777432937d529e565c50c9b89f0fff98283d583))
* add initial search tools ([3d5a4c0](https://github.com/et-do/no-pilot/commit/3d5a4c0aa81aea1f17f1e93ba0372e114d898d0c))
* add server, policy, and config ([009969f](https://github.com/et-do/no-pilot/commit/009969f876ba69c90799985f77eb7b21647c58e3))
* add stubs for browser, edit, and web tools ([86f4411](https://github.com/et-do/no-pilot/commit/86f441128307d0c3416042b9fd155751e052b91b))
* build workflows ([#2](https://github.com/et-do/no-pilot/issues/2)) ([7b8d20f](https://github.com/et-do/no-pilot/commit/7b8d20fd316f50ceb3dc997ac4ab7846bab45d6d))
* github workflows for build, release, and ci ([#1](https://github.com/et-do/no-pilot/issues/1)) ([8bbabb4](https://github.com/et-do/no-pilot/commit/8bbabb44183fa64a80982d71bd13a7c4de524c68))
* update build files ([5631a0d](https://github.com/et-do/no-pilot/commit/5631a0dd49c6c4c94198ee2f6582737c37d5f4da))
