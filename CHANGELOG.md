# Changelog

## [v0.1.0](https://github.com/sivchari/dendrite/commits/v0.1.0) - 2026-04-28
- Fix golangci-lint issues by @sivchari in https://github.com/sivchari/dendrite/pull/2
- Fix gocognit lint for TestParseBytes by @sivchari in https://github.com/sivchari/dendrite/pull/3
- Support raw binary assets without archive extension by @sivchari in https://github.com/sivchari/dendrite/pull/4
- Add bin_map and tar.bz2 support by @sivchari in https://github.com/sivchari/dendrite/pull/5
- Add format field to override archive format detection by @sivchari in https://github.com/sivchari/dendrite/pull/6
- Use owner/repo directory structure for generated packages by @sivchari in https://github.com/sivchari/dendrite/pull/7
- Add strip_components for tar archives with subdirectories by @sivchari in https://github.com/sivchari/dendrite/pull/8
- Support strip_components for zip archives by @sivchari in https://github.com/sivchari/dendrite/pull/9
- Add dontStrip to generated default.nix by @sivchari in https://github.com/sivchari/dendrite/pull/10
- feat: add Renovate preset for automatic tool version updates by @sivchari in https://github.com/sivchari/dendrite/pull/11
- release v0.1.0 by @sivchari in https://github.com/sivchari/dendrite/pull/12
- feat!: change asset field from string to per-OS map by @sivchari in https://github.com/sivchari/dendrite/pull/13
- feat!: use platform keys (os/arch) for asset map, remove os_map and arch_map by @sivchari in https://github.com/sivchari/dendrite/pull/14
- feat: generate index default.nix for all packages by @sivchari in https://github.com/sivchari/dendrite/pull/15
