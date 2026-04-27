# Changelog

## [0.2.0](https://github.com/RUCICS/LabKit/compare/labkit-v0.1.2...labkit-v0.2.0) (2026-04-27)


### Features

* **api,cli:** add versioned API routes and CLI release pipeline ([29931f8](https://github.com/RUCICS/LabKit/commit/29931f88e466ffe53fb8e0414bbb9b49c7e0ea33))
* **cli:** add duplicate submit confirmation and quota summaries ([8ca6c2e](https://github.com/RUCICS/LabKit/commit/8ca6c2e54a9f0b3af7ee45a97f6dfd54fbe10ccd))
* **cli:** add self-update command and periodic reminder ([817062e](https://github.com/RUCICS/LabKit/commit/817062e445f1bea53c135433ac8d6a36d49b8ac9))
* **cli:** polish leaderboard and loading feedback ([70dc692](https://github.com/RUCICS/LabKit/commit/70dc6922e2ff97e77f4bd5a6708cfbb6ca3d4042))
* **cli:** polish UX and preserve auth device names ([0ef93f1](https://github.com/RUCICS/LabKit/commit/0ef93f1a0dccbe34e8825f2d53031ebe07d9c0b0))
* **cli:** refine leaderboard badges and sync evaluator deps ([9ea4548](https://github.com/RUCICS/LabKit/commit/9ea4548182d77cabae8c39e3cacffd9844dc35d5))
* **cli:** ship polished labkit CLI with web auth and project config ([f952650](https://github.com/RUCICS/LabKit/commit/f9526503e58a9c913a55969a8f641f390f258fea))
* **cli:** show spinner feedback during submission request ([935a9e9](https://github.com/RUCICS/LabKit/commit/935a9e9642a507b1294d4c2e644d5c1467289f5c))
* **deploy:** cut over to golang-migrate runner ([7476b35](https://github.com/RUCICS/LabKit/commit/7476b3592f56e43787dbbe0860e3b4e117fedd66))
* **deploy:** prepare production oauth and https ([af65d07](https://github.com/RUCICS/LabKit/commit/af65d07edea4296814c469b3a8adf98e04a35bfb))
* **platform:** build labkit backend, worker, and deploy stack ([0efd430](https://github.com/RUCICS/LabKit/commit/0efd4301ee714996b96ba542bd5e3a415021061c))
* **profile:** add global nickname storage ([fd65130](https://github.com/RUCICS/LabKit/commit/fd65130a19138291743314e6504a10674e230d68))
* **profile:** ship profile hub and stable nav ([c63a778](https://github.com/RUCICS/LabKit/commit/c63a778f097dfdc318364019e615a8de609c20df))
* **submissions:** enforce quota state and expose precheck/quota APIs ([53c8f93](https://github.com/RUCICS/LabKit/commit/53c8f9337296c05bbaafcc408d70ddd7d8ccee31))
* **ui:** add shared console primitives ([dab8d43](https://github.com/RUCICS/LabKit/commit/dab8d437a27ba1d0c204da1afb0a8c35eec3f5fa))
* **web:** add board profile and history flow ([e4574ce](https://github.com/RUCICS/LabKit/commit/e4574cefb753900726933142577ed2e0b0d7822b))
* **web:** add leaderboard context bar ([d96fb99](https://github.com/RUCICS/LabKit/commit/d96fb999b827e199fb68c7c662172a17ceb4ee97))
* **web:** dim rows outside selected track ([57f34fe](https://github.com/RUCICS/LabKit/commit/57f34fe775b667c8e0d721824c569534bb8c7fdd))
* **web:** rebuild device auth surfaces ([f5b298d](https://github.com/RUCICS/LabKit/commit/f5b298d33a5b3a733ceb4912fbcf252790c84185))
* **web:** ship leaderboard, profile, and admin app ([4bed66e](https://github.com/RUCICS/LabKit/commit/4bed66ecef5fb1ee0e9b27de95687db320df5862))
* **worker:** mark submission as running on evaluation start ([aea0846](https://github.com/RUCICS/LabKit/commit/aea0846eaa39638f0b66af4aa0c59a10aaacb09c))


### Bug Fixes

* **deploy:** install tzdata in api image ([2892ff2](https://github.com/RUCICS/LabKit/commit/2892ff265ffb4229a5fd8f7ace2d3fe4e71f9c99))
* **deploy:** run fingerprint migration during bootstrap ([594d2b3](https://github.com/RUCICS/LabKit/commit/594d2b36ac2494ff12dcc4a73310ba0be4f84e0c))
* **deploy:** run quota state migration in compose ([2fa65c8](https://github.com/RUCICS/LabKit/commit/2fa65c884f6ceac34a06ad1a88c29e9380938b6c))
* **release:** allow release-please to use PAT token ([a4eb47d](https://github.com/RUCICS/LabKit/commit/a4eb47d93b55b53d49f32582a89fe94ab42b10bf))
* **release:** disable go.work during CLI release builds ([0eb3a38](https://github.com/RUCICS/LabKit/commit/0eb3a38fbbb1032ed594717d3b5bce1a7ccb4630))
* **release:** use v* tags and GoReleaser v2 config ([bea978b](https://github.com/RUCICS/LabKit/commit/bea978bdf665edc6bb408de24263192ca4aeb79a))
* support postgres passwords with reserved chars ([42ffd58](https://github.com/RUCICS/LabKit/commit/42ffd5867ce3721a75a68cba6c8f2d7599e8be17))
* **web:** align UI with design spec and unify visual system ([d068d64](https://github.com/RUCICS/LabKit/commit/d068d642adb2c74b5a43ce8aec6deb796d8abfce))
* **web:** recompute ranks within selected track ([da3a85b](https://github.com/RUCICS/LabKit/commit/da3a85be791f122fdd26648dd69dec472abf365d))
* **web:** support Go-style manifest keys in schedule ([683df02](https://github.com/RUCICS/LabKit/commit/683df02d327a45f3f76076a313fe0f81dddf48b7))
* **worker:** track worker entrypoint sources ([8da2fee](https://github.com/RUCICS/LabKit/commit/8da2fee01f8c0ec780c8cedef1e481a8adbcff3b))
