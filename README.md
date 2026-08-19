> ## This repository has been archived pending migration into kairos-io/kairos
>
> The Kairos SDK is being absorbed into the
> [kairos-io/kairos](https://github.com/kairos-io/kairos) monorepo as
> part of the plan tracked in
> [kairos-io/kairos#4301](https://github.com/kairos-io/kairos/issues/4301).
> Every existing tag remains resolvable via the Go module proxy and
> installable with the same
> `go get github.com/kairos-io/kairos-sdk@vX.Y.Z` as before, so anything
> already published keeps working.
>
> New development happens at
> [github.com/kairos-io/kairos/sdk](https://github.com/kairos-io/kairos/tree/master/sdk).
> If that link returns 404, the subtree import is not yet complete; see
> the tracking issue above for the current state and timeline.
>
> **To pick up a newer SDK,** update your imports:
>
> ```
> find . -type f -name '*.go' -exec sed -i \
>   's|github.com/kairos-io/kairos-sdk|github.com/kairos-io/kairos/sdk|g' {} +
> go mod tidy
> go get github.com/kairos-io/kairos@<latest>
> ```
>
> **On backports to older lines.** The default answer is: consume the
> latest release. We will only unarchive this repository for fixes we
> judge important enough (typically security fixes or breakage with no
> reasonable workaround). Convenience backports and feature backports
> do not qualify. To request one, open an issue on
> [kairos-io/kairos](https://github.com/kairos-io/kairos/issues)
> describing the SDK version, the fix, and why moving to the latest
> release is not viable. If we agree the fix meets the bar, we will
> unarchive temporarily to land it and cut a new patch tag.

---

<h1 align="center">
  <br>
     <img width="184" alt="kairos-white-column 5bc2fe34" src="https://user-images.githubusercontent.com/2420543/193010398-72d4ba6e-7efe-4c2e-b7ba-d3a826a55b7d.png">
    <br>
<br>
</h1>

<h3 align="center">Kairos - Kubernetes-focused, Cloud Native Linux meta-distribution</h3>
<p align="center">
  <a href="https://github.com/kairos-io/kairos/issues"><img src="https://img.shields.io/github/issues/kairos-io/kairos"></a>
  <a href="https://github.com/kairos-io/kairos/actions/workflows/image.yaml"> <img src="https://github.com/kairos-io/kairos/actions/workflows/image.yaml/badge.svg"></a>
</p>

<p align="center">
     <br>
    The immutable Linux meta-distribution for edge Kubernetes.
</p>

<hr>

With Kairos you can build immutable, bootable Kubernetes and OS images for your edge devices as easily as writing a Dockerfile. Optional P2P mesh with distributed ledger automates node bootstrapping and coordination. Updating nodes is as easy as CI/CD: push a new image to your container registry and let secure, risk-free A/B atomic upgrades do the rest. 

Kairos (formerly `c3os`) is an open-source project which brings Edge, cloud, and bare metal lifecycle OS management into the same design principles with a unified Cloud Native API.


This repo provides the SDK for kairos



## Community

You can find us at:

- [#kairos-io at matrix.org](https://matrix.to/#/#kairos-io:matrix.org)
- [IRC #kairos in libera.chat](https://web.libera.chat/#kairos)
- [GitHub Discussions](https://github.com/kairos-io/kairos/discussions)

### Project Office Hours

Project Office Hours is an opportunity for attendees to meet the maintainers of the project, learn more about the project, ask questions, and learn about new features and upcoming updates.

[Add to Google Calendar](https://calendar.google.com/calendar/embed?src=c_6d65f26502a5a67c9570bb4c16b622e38d609430bce6ce7fc1d8064f2df09c11%40group.calendar.google.com&ctz=Europe%2FRome)
