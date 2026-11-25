# Build make files

The files in `makelib/` are copied unchanged from
[crossplane/build](https://github.com/crossplane/build) at commit
`6a16dad359dfc2a2509e61d1c2a604b837363f33`. They used to be included through a
git submodule. They are licensed under the Apache License 2.0, see `LICENSE`.

Only the files the provider's Makefile includes are kept. The commit above
applies to all of them, and they depend on each other, so to pick up an upstream
change copy all eight files from one upstream commit and then update the commit
above.
