# Changelog

## [0.3.0](https://github.com/shrujancheruku/issho/compare/v0.2.0...v0.3.0) (2026-10-20)


### Features

* M3 walk/drive travel times, venue discovery, on-demand translation and an Austin pack ([#5](https://github.com/shrujancheruku/issho/issues/5)) ([877265c](https://github.com/shrujancheruku/issho/commit/877265cf4f4f1ce45d09ed8bd5070be057ce579c))

## [0.2.0](https://github.com/shrujancheruku/issho/compare/v0.1.0...v0.2.0) (2026-10-20)


### Features

* **api:** add IdentityService.DeleteAccount ([13b884f](https://github.com/shrujancheruku/issho/commit/13b884f432eefd991edad45b80f4b4514c69999b))
* **brand:** add Issho logo and mark ([e24adce](https://github.com/shrujancheruku/issho/commit/e24adce4b40e56b539d07ba385047bd2d0e17ac2))
* **brand:** layer outlined parts and switch the wordmark font ([50e870c](https://github.com/shrujancheruku/issho/commit/50e870c14ef0fa9f1c9578ef3745e6a5edb45a15))
* **brand:** layer outlined parts and switch the wordmark font ([#4](https://github.com/shrujancheruku/issho/issues/4)) ([d39d464](https://github.com/shrujancheruku/issho/commit/d39d46478a27c2b41e41d7b1d14e77d03e64b344))
* **brand:** use a single outlined logo on every background ([0a37f43](https://github.com/shrujancheruku/issho/commit/0a37f437b06593986b885ce87a9a1d8cb8566597))
* **infra:** configure issho-dev email via Resend SMTP ([221031d](https://github.com/shrujancheruku/issho/commit/221031d073eb8bffaad6e8d003a7c70f85329d8f))
* **infra:** send 6-digit sign-in codes instead of links ([fcd9193](https://github.com/shrujancheruku/issho/commit/fcd919367b0f90962ab6d6715d0b8342ab5985cc))
* M2 email sign-in, hosted Supabase, secure sessions and account deletion ([#3](https://github.com/shrujancheruku/issho/issues/3)) ([e0e5f2d](https://github.com/shrujancheruku/issho/commit/e0e5f2da5825b34726a8e129a891e3db008449ef))
* **mobile:** sign in with email codes and gate navigation on auth ([bf0afa6](https://github.com/shrujancheruku/issho/commit/bf0afa69aaa27de1062d15de7e94de8b163fb2d3))
* **mobile:** use Dela Gothic One for standout text ([2d92f8b](https://github.com/shrujancheruku/issho/commit/2d92f8be2541189ef337f4f1b02c59a0eb375de8))


### Bug Fixes

* **backend:** stop leaking internal error details to clients ([f561bcb](https://github.com/shrujancheruku/issho/commit/f561bcb3c6237362570f1b38d81c2af71656d10e))
* **db:** keep migration bookkeeping out of the public schema ([974f1ae](https://github.com/shrujancheruku/issho/commit/974f1aea6d7bfc111601b546b9206975a2b91898))
* **infra:** disable the Supabase Data API locally ([6427414](https://github.com/shrujancheruku/issho/commit/6427414c3a4fd8fdd6ef99a42561f2ae021da5ea))
* **mobile:** keep the sign-in code field above the keyboard on Android ([b807646](https://github.com/shrujancheruku/issho/commit/b807646883ca7785418904f9d3100ee4cc6db00c))

## 0.1.0 (2026-10-20)


### Features

* scaffold Issho monorepo with Go API, Expo app and local Supabase ([75ce44c](https://github.com/shrujancheruku/issho/commit/75ce44cd8b24ded9ac7f359ae5cd17e0ad4532ec))


### Bug Fixes

* **backend:** deny cross-origin requests when no origins are configured ([6d64b3d](https://github.com/shrujancheruku/issho/commit/6d64b3d7151ec2d1cf22f1fc4b60bf658b33b6ee))
* **ci:** make CI pass from a clean checkout ([75f7a20](https://github.com/shrujancheruku/issho/commit/75f7a203a86136574af2e834debf329393f4ceff))
