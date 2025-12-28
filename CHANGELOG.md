# Changelog

## [0.2.2](https://github.com/systmms/dsops/compare/v0.2.1...v0.2.2) (2025-12-28)


### Bug Fixes

* **release:** migrate to homebrew_casks and dockers_v2 ([d58dac9](https://github.com/systmms/dsops/commit/d58dac9066fc7dac0d5891f00043585a77360892))

## [0.2.1](https://github.com/systmms/dsops/compare/v0.2.0...v0.2.1) (2025-12-28)


### Bug Fixes

* **release:** move completion generation to before hooks ([d6706df](https://github.com/systmms/dsops/commit/d6706dfc9f3b54758032f10a4ca95807c6ad73e3))

## [0.2.0](https://github.com/systmms/dsops/compare/v0.1.0...v0.2.0) (2025-12-28)


### ⚠ BREAKING CHANGES

* **specs:** Spec file paths changed from categorized files to numbered directories. Update any tooling that references specs/features/ or specs/providers/ to use flat specs/ directory with numbered subdirectories.
* Retire VISION*.md documents in favor of spec-kit workflow

### Features

* **cli:** add shell completion command ([f2741dd](https://github.com/systmms/dsops/commit/f2741ddb8a890f3d7fc1b68cc58d54e3632dcea8))
* **health:** add custom script health checker ([aa4bd10](https://github.com/systmms/dsops/commit/aa4bd1000d7687a51a0fea70bbe01d200b75b792))
* **health:** add health monitoring system with SQL and HTTP checkers ([cc8adcf](https://github.com/systmms/dsops/commit/cc8adcf5c0bec2335b0891a37f27c7f22bc37449))
* **health:** add Prometheus metrics with HTTP server ([c17f782](https://github.com/systmms/dsops/commit/c17f782a08a327b16cf41a9e8988e6a989b7f0db))
* implement dsops core with 14 providers and rotation engine ([8cc5c47](https://github.com/systmms/dsops/commit/8cc5c47e77e7c13e55a1f106ab8060c9fd6a8139))
* integrate GitHub Spec-Kit for specification-driven development ([e64bab0](https://github.com/systmms/dsops/commit/e64bab0dc83c0f2bb2ba07a0f8b307b9479f5bb7))
* Integrate GitHub Spec-Kit for specification-driven development ([284e071](https://github.com/systmms/dsops/commit/284e071392f7b497ef03c616f4576db56019c234))
* **release:** add automated release & distribution infrastructure ([7c4f62e](https://github.com/systmms/dsops/commit/7c4f62e65ca03f81fc5a9092a7fdb58025baaaba))
* **release:** add automated release infrastructure ([719137c](https://github.com/systmms/dsops/commit/719137c88f93ac05adfc06023d869a5c89dca02e))
* **release:** add release-please for automated versioning ([1f32bdf](https://github.com/systmms/dsops/commit/1f32bdf18ff2bea4ae735f2e1294e068e889081d))
* **rotation:** add canary rollout strategy with discovery providers ([aff14ae](https://github.com/systmms/dsops/commit/aff14ae88df850cc28d547104d96703891013446))
* **rotation:** add email, pagerduty, and webhook notification providers ([91ea9f1](https://github.com/systmms/dsops/commit/91ea9f199283cb4f4136a44b5e9e330251f18e8b))
* **rotation:** add instance discovery providers ([35343f2](https://github.com/systmms/dsops/commit/35343f2d884fa34817e318123333f7bb4aeefe9b))
* **rotation:** add manual rollback CLI command ([9b3037e](https://github.com/systmms/dsops/commit/9b3037ed3918872e7583fefff17354d551733d78))
* **rotation:** add percentage rollout strategy ([7d79ca0](https://github.com/systmms/dsops/commit/7d79ca0f3e0cb8503af689caf1d4cafd9d787e3e))
* **rotation:** add rollback notification templates and enhanced metadata ([e6b5f90](https://github.com/systmms/dsops/commit/e6b5f904fb728a7b102b6c4ed5928c9c3e8960e3))
* **rotation:** add service group rotation strategy ([d55c122](https://github.com/systmms/dsops/commit/d55c122049c490000d72bbe11201babab02694da))
* **rotation:** implement notification system and automatic rollback ([1ee0f03](https://github.com/systmms/dsops/commit/1ee0f0378a72d33eca65fe977ea5b84ecb976494))
* **specs:** add SPEC-020 release & distribution infrastructure ([cc96b08](https://github.com/systmms/dsops/commit/cc96b08ca7b0e49ad4432e9ab23e813601efc326))


### Bug Fixes

* address PR review feedback ([eb36ddb](https://github.com/systmms/dsops/commit/eb36ddb38262d17a8cff75d44fe1cbc0172aa271))
* **ci:** add golangci-lint action and fix test issues ([32f4de3](https://github.com/systmms/dsops/commit/32f4de32240923c174ec1e93dfa7cce290736bcb))
* **ci:** add gosec exclusions for false positives and intentional patterns ([94f35b7](https://github.com/systmms/dsops/commit/94f35b73bac031b662cc117a2b25a782bd919613))
* **ci:** convert gosec config to JSON and fix CI workflows ([914679f](https://github.com/systmms/dsops/commit/914679f16f359d1b8b2724f665c1e0c4c8f0d876))
* **ci:** optimize security scanning and add dependency checks ([52d4d62](https://github.com/systmms/dsops/commit/52d4d628dd0322b4a1189f8ac063e040748abae2))
* **ci:** resolve goreleaser and gosec CI failures ([77b7b67](https://github.com/systmms/dsops/commit/77b7b675176a5e448736877870a2758dcfd72945))
* **ci:** use .gosec.json as single source for security exclusions ([ce06437](https://github.com/systmms/dsops/commit/ce064371ade583044dfbee364120ed03b568b80b))
* **dev:** install golangci-lint and govulncheck via go install ([6dc10e6](https://github.com/systmms/dsops/commit/6dc10e6a0f9efba127bbc93b02f478686bf76537))
* **lint:** fix remaining lint errors (errcheck, staticcheck, unused) ([6916a5b](https://github.com/systmms/dsops/commit/6916a5b628805f088642d0c36338ac6447c1b242))
* **notifications:** improve email header injection prevention ([c1f52ee](https://github.com/systmms/dsops/commit/c1f52ee27496d461e9ec9ecf797ca70f2e0fa8be))
* **providers:** resolve data race in Vault client and enable LocalStack testing ([e3b8a70](https://github.com/systmms/dsops/commit/e3b8a70f69f346763d815623a9549c8c715a8a7f))
* **release:** address PR review feedback from Gemini and Copilot ([7e88b4d](https://github.com/systmms/dsops/commit/7e88b4d9809739d92c9ee95d5edd85b5ced49395))
* resolve golangci-lint errcheck and staticcheck issues ([3196f48](https://github.com/systmms/dsops/commit/3196f48f256a5d7ba9212bb16bd2cfcd979b623a))
* **security:** update Go to 1.25.5 to address crypto/x509 vulnerabilities ([4e80153](https://github.com/systmms/dsops/commit/4e80153fba111f6b450d208437859e749026efd3))
* **specs:** correct malformed summary sentences in provider specs ([730f7f1](https://github.com/systmms/dsops/commit/730f7f1ab05ab6da738c83fb8223bcc18506eda6))
* **test:** enable AWS integration tests to run locally with LocalStack ([0c45df4](https://github.com/systmms/dsops/commit/0c45df44591fd11fdc9227adbd16876a86120193))
* **test:** prevent Docker network conflicts in parallel integration tests ([ba1e78e](https://github.com/systmms/dsops/commit/ba1e78e8aaf942643d8676b03cbcb18a1d702836))
* **test:** prevent premature context cancellation in PostgresTestClient ([8ce6ab3](https://github.com/systmms/dsops/commit/8ce6ab33a296387ebb5c68b306b42d30f6d3e663))
* **test:** resolve QueryRow context canceled errors in PostgreSQL tests ([b973dba](https://github.com/systmms/dsops/commit/b973dba76d95b6ca2cda99b8fe5fbe77c56cde92))
* **tests:** replace localhost with 127.0.0.1 to fix IPv6 resolution issues ([40af982](https://github.com/systmms/dsops/commit/40af982c95b0b7408590e00781642a652e0c208e))
* **tests:** resolve integration test hangs and improve CI coverage reporting ([213c6eb](https://github.com/systmms/dsops/commit/213c6ebdf3164c19395f2c54898034f7b7639cec))


### Documentation

* migrate from VISION tracking to spec-kit specifications ([9aed0e4](https://github.com/systmms/dsops/commit/9aed0e41ffc9d6e7e090067171ddf3974ca26faa))


### Code Refactoring

* **specs:** migrate to standard spec-kit directory structure ([df092e7](https://github.com/systmms/dsops/commit/df092e74ba3529838aaf2e22fc97ad126dfbe812))
