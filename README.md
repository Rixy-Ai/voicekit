<!--BEGIN_BANNER_IMAGE-->

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="/.github/banner_dark.png">
  <source media="(prefers-color-scheme: light)" srcset="/.github/banner_light.png">
  <img style="width:100%;" alt="The VoiceKit icon, the name of the repository and some sample code in the background." src="https://raw.githubusercontent.com/voicekit/voicekit/main/.github/banner_light.png">
</picture>

<!--END_BANNER_IMAGE-->

# VoiceKit: Real-time video, audio and data for developers

[VoiceKit](https://voicekit.rixy.ai) is an open source project that provides scalable, multi-user conferencing based on WebRTC.
It's designed to provide everything you need to build real-time video audio data capabilities in your applications.

VoiceKit's server is written in Go, using the awesome [Pion WebRTC](https://github.com/pion/webrtc) implementation.

[![GitHub stars](https://img.shields.io/github/stars/voicekit/voicekit?style=social&label=Star&maxAge=2592000)](https://github.com/voicekit/voicekit/stargazers/)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/voicekit/voicekit)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/voicekit/voicekit)](https://github.com/voicekit/voicekit/releases/latest)
[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/voicekit/voicekit/buildtest.yaml?branch=master)](https://github.com/voicekit/voicekit/actions/workflows/buildtest.yaml)
[![License](https://img.shields.io/github/license/voicekit/voicekit)](https://github.com/voicekit/voicekit/blob/master/LICENSE)

## Features

-   Scalable, distributed WebRTC SFU (Selective Forwarding Unit)
-   Modern, full-featured client SDKs
-   Built for production, supports JWT authentication
-   Robust networking and connectivity, UDP/TCP/TURN
-   Easy to deploy: single binary, Docker or Kubernetes
-   Advanced features including:
    -   [speaker detection](https://docs.voicekit.rixy.ai/home/client/tracks/subscribe/#speaker-detection)
    -   [simulcast](https://docs.voicekit.rixy.ai/home/client/tracks/publish/#video-simulcast)
    -   [end-to-end optimizations](https://blog.voicekit.rixy.ai/voicekit-one-dot-zero/)
    -   [selective subscription](https://docs.voicekit.rixy.ai/home/client/tracks/subscribe/#selective-subscription)
    -   [moderation APIs](https://docs.voicekit.rixy.ai/home/server/managing-participants/)
    -   end-to-end encryption
    -   SVC codecs (VP9, AV1)
    -   [webhooks](https://docs.voicekit.rixy.ai/home/server/webhooks/)
    -   [distributed and multi-region](https://docs.voicekit.rixy.ai/home/self-hosting/distributed/)

## Documentation & Guides

https://docs.voicekit.rixy.ai

## Live Demos

-   [VoiceKit Meet](https://meet.voicekit.rixy.ai) ([source](https://github.com/voicekit-examples/meet))
-   [Spatial Audio](https://spatial-audio-demo.voicekit.rixy.ai/) ([source](https://github.com/voicekit-examples/spatial-audio))
-   Livestreaming from OBS Studio ([source](https://github.com/voicekit-examples/livestream))
-   [AI voice assistant using ChatGPT](https://voicekit.rixy.ai/kitt) ([source](https://github.com/voicekit-examples/kitt))

## Ecosystem

-   [Agents](https://github.com/voicekit/agents): build real-time multimodal AI applications with programmable backend participants
-   [Egress](https://github.com/voicekit/egress): record or multi-stream rooms and export individual tracks
-   [Ingress](https://github.com/voicekit/ingress): ingest streams from external sources like RTMP, WHIP, HLS, or OBS Studio

## SDKs & Tools

### Client SDKs

Client SDKs enable your frontend to include interactive, multi-user experiences.

<table>
  <tr>
    <th>Language</th>
    <th>Repo</th>
    <th>
        <a href="https://docs.voicekit.rixy.ai/home/client/events/#declarative-ui" target="_blank" rel="noopener noreferrer">Declarative UI</a>
    </th>
    <th>Links</th>
  </tr>
  <!-- BEGIN Template
  <tr>
    <td>Language</td>
    <td>
      <a href="" target="_blank" rel="noopener noreferrer"></a>
    </td>
    <td></td>
    <td></td>
  </tr>
  END -->
  <!-- JavaScript -->
  <tr>
    <td>JavaScript (TypeScript)</td>
    <td>
      <a href="https://github.com/voicekit/client-sdk-js" target="_blank" rel="noopener noreferrer">client-sdk-js</a>
    </td>
    <td>
      <a href="https://github.com/voicekit/voicekit-react" target="_blank" rel="noopener noreferrer">React</a>
    </td>
    <td>
      <a href="https://docs.voicekit.rixy.ai/client-sdk-js/" target="_blank" rel="noopener noreferrer">docs</a>
      |
      <a href="https://github.com/voicekit/client-sdk-js/tree/main/example" target="_blank" rel="noopener noreferrer">JS example</a>
      |
      <a href="https://github.com/voicekit/client-sdk-js/tree/main/example" target="_blank" rel="noopener noreferrer">React example</a>
    </td>
  </tr>
  <!-- Swift -->
  <tr>
    <td>Swift (iOS / MacOS)</td>
    <td>
      <a href="https://github.com/voicekit/client-sdk-swift" target="_blank" rel="noopener noreferrer">client-sdk-swift</a>
    </td>
    <td>Swift UI</td>
    <td>
      <a href="https://docs.voicekit.rixy.ai/client-sdk-swift/" target="_blank" rel="noopener noreferrer">docs</a>
      |
      <a href="https://github.com/voicekit/client-example-swift" target="_blank" rel="noopener noreferrer">example</a>
    </td>
  </tr>
  <!-- Kotlin -->
  <tr>
    <td>Kotlin (Android)</td>
    <td>
      <a href="https://github.com/voicekit/client-sdk-android" target="_blank" rel="noopener noreferrer">client-sdk-android</a>
    </td>
    <td>Compose</td>
    <td>
      <a href="https://docs.voicekit.rixy.ai/client-sdk-android/index.html" target="_blank" rel="noopener noreferrer">docs</a>
      |
      <a href="https://github.com/voicekit/client-sdk-android/tree/main/sample-app/src/main/java/io/voicekit/android/sample" target="_blank" rel="noopener noreferrer">example</a>
      |
      <a href="https://github.com/voicekit/client-sdk-android/tree/main/sample-app-compose/src/main/java/io/voicekit/android/composesample" target="_blank" rel="noopener noreferrer">Compose example</a>
    </td>
  </tr>
<!-- Flutter -->
  <tr>
    <td>Flutter (all platforms)</td>
    <td>
      <a href="https://github.com/voicekit/client-sdk-flutter" target="_blank" rel="noopener noreferrer">client-sdk-flutter</a>
    </td>
    <td>native</td>
    <td>
      <a href="https://docs.voicekit.rixy.ai/client-sdk-flutter/" target="_blank" rel="noopener noreferrer">docs</a>
      |
      <a href="https://github.com/voicekit/client-sdk-flutter/tree/main/example" target="_blank" rel="noopener noreferrer">example</a>
    </td>
  </tr>
  <!-- Unity -->
  <tr>
    <td>Unity WebGL</td>
    <td>
      <a href="https://github.com/voicekit/client-sdk-unity-web" target="_blank" rel="noopener noreferrer">client-sdk-unity-web</a>
    </td>
    <td></td>
    <td>
      <a href="https://voicekit.github.io/client-sdk-unity-web/" target="_blank" rel="noopener noreferrer">docs</a>
    </td>
  </tr>
  <!-- React Native -->
  <tr>
    <td>React Native (beta)</td>
    <td>
      <a href="https://github.com/voicekit/client-sdk-react-native" target="_blank" rel="noopener noreferrer">client-sdk-react-native</a>
    </td>
    <td>native</td>
    <td></td>
  </tr>
  <!-- Rust -->
  <tr>
    <td>Rust</td>
    <td>
      <a href="https://github.com/voicekit/client-sdk-rust" target="_blank" rel="noopener noreferrer">client-sdk-rust</a>
    </td>
    <td></td>
    <td></td>
  </tr>
</table>

### Server SDKs

Server SDKs enable your backend to generate [access tokens](https://docs.voicekit.rixy.ai/home/get-started/authentication/),
call [server APIs](https://docs.voicekit.rixy.ai/reference/server/server-apis/), and
receive [webhooks](https://docs.voicekit.rixy.ai/home/server/webhooks/). In addition, the Go SDK includes client capabilities,
enabling you to build automations that behave like end-users.

| Language                | Repo                                                                                    | Docs                                                        |
| :---------------------- | :-------------------------------------------------------------------------------------- | :---------------------------------------------------------- |
| Go                      | [server-sdk-go](https://github.com/voicekit/server-sdk-go)                               | [docs](https://pkg.go.dev/github.com/voicekit/server-sdk-go) |
| JavaScript (TypeScript) | [server-sdk-js](https://github.com/voicekit/server-sdk-js)                               | [docs](https://docs.voicekit.rixy.ai/server-sdk-js/)              |
| Ruby                    | [server-sdk-ruby](https://github.com/voicekit/server-sdk-ruby)                           |                                                             |
| Java (Kotlin)           | [server-sdk-kotlin](https://github.com/voicekit/server-sdk-kotlin)                       |                                                             |
| Python (community)      | [python-sdks](https://github.com/voicekit/python-sdks)                                   |                                                             |
| PHP (community)         | [agence104/voicekit-server-sdk-php](https://github.com/agence104/voicekit-server-sdk-php) |                                                             |

### Tools

-   [CLI](https://github.com/voicekit/voicekit-cli) - command line interface & load tester
-   [Docker image](https://hub.docker.com/r/voicekit/voicekit-server)
-   [Helm charts](https://github.com/voicekit/voicekit-helm)

## Install

> [!TIP]
> We recommend installing [VoiceKit CLI](https://github.com/voicekit/voicekit-cli) along with the server. It lets you access
> server APIs, create tokens, and generate test traffic.

The following will install VoiceKit's media server:

### MacOS

```shell
brew install voicekit
```

### Linux

```shell
curl -sSL https://get.voicekit.rixy.ai | bash
```

### Windows

Download the [latest release here](https://github.com/voicekit/voicekit/releases/latest)

## Getting Started

### Starting VoiceKit

Start VoiceKit in development mode by running `voicekit-server --dev`. It'll use a placeholder API key/secret pair.

```
API Key: devkey
API Secret: secret
```

To customize your setup for production, refer to our [deployment docs](https://docs.voicekit.rixy.ai/deploy/)

### Creating access token

A user connecting to a VoiceKit room requires an [access token](https://docs.voicekit.rixy.ai/home/get-started/authentication/#creating-a-token). Access
tokens (JWT) encode the user's identity and the room permissions they've been granted. You can generate a token with our
CLI:

```shell
lk token create \
    --api-key devkey --api-secret secret \
    --join --room my-first-room --identity user1 \
    --valid-for 24h
```

### Test with example app

Head over to our [example app](https://example.voicekit.rixy.ai) and enter a generated token to connect to your VoiceKit
server. This app is built with our [React SDK](https://github.com/voicekit/voicekit-react).

Once connected, your video and audio are now being published to your new VoiceKit instance!

### Simulating a test publisher

```shell
lk room join \
    --url ws://localhost:7880 \
    --api-key devkey --api-secret secret \
    --identity bot-user1 \
    --publish-demo \
    my-first-room
```

This command publishes a looped demo video to a room. Due to how the video clip was encoded (keyframes every 3s),
there's a slight delay before the browser has sufficient data to begin rendering frames. This is an artifact of the
simulation.

## Deployment

### Use VoiceKit Cloud

VoiceKit Cloud is the fastest and most reliable way to run VoiceKit. Every project gets free monthly bandwidth and
transcoding credits.

Sign up for [VoiceKit Cloud](https://cloud.voicekit.rixy.ai/).

### Self-host

Read our [deployment docs](https://docs.voicekit.rixy.ai/deploy/) for more information.

## Building from source

Pre-requisites:

-   Go 1.23+ is installed
-   GOPATH/bin is in your PATH

Then run

```shell
git clone https://github.com/voicekit/voicekit
cd voicekit
./bootstrap.sh
mage
```

## Contributing

We welcome your contributions toward improving VoiceKit! Please join us
[on Slack](http://voicekit.rixy.ai/join-slack) to discuss your ideas and/or PRs.

## License

VoiceKit server is licensed under Apache License v2.0.

<!--BEGIN_REPO_NAV-->
<br/><table>
<thead><tr><th colspan="2">VoiceKit Ecosystem</th></tr></thead>
<tbody>
<tr><td>VoiceKit SDKs</td><td><a href="https://github.com/voicekit/client-sdk-js">Browser</a> · <a href="https://github.com/voicekit/client-sdk-swift">iOS/macOS/visionOS</a> · <a href="https://github.com/voicekit/client-sdk-android">Android</a> · <a href="https://github.com/voicekit/client-sdk-flutter">Flutter</a> · <a href="https://github.com/voicekit/client-sdk-react-native">React Native</a> · <a href="https://github.com/voicekit/rust-sdks">Rust</a> · <a href="https://github.com/voicekit/node-sdks">Node.js</a> · <a href="https://github.com/voicekit/python-sdks">Python</a> · <a href="https://github.com/voicekit/client-sdk-unity">Unity</a> · <a href="https://github.com/voicekit/client-sdk-unity-web">Unity (WebGL)</a></td></tr><tr></tr>
<tr><td>Server APIs</td><td><a href="https://github.com/voicekit/node-sdks">Node.js</a> · <a href="https://github.com/voicekit/server-sdk-go">Golang</a> · <a href="https://github.com/voicekit/server-sdk-ruby">Ruby</a> · <a href="https://github.com/voicekit/server-sdk-kotlin">Java/Kotlin</a> · <a href="https://github.com/voicekit/python-sdks">Python</a> · <a href="https://github.com/voicekit/rust-sdks">Rust</a> · <a href="https://github.com/agence104/voicekit-server-sdk-php">PHP (community)</a> · <a href="https://github.com/pabloFuente/voicekit-server-sdk-dotnet">.NET (community)</a></td></tr><tr></tr>
<tr><td>UI Components</td><td><a href="https://github.com/voicekit/components-js">React</a> · <a href="https://github.com/voicekit/components-android">Android Compose</a> · <a href="https://github.com/voicekit/components-swift">SwiftUI</a></td></tr><tr></tr>
<tr><td>Agents Frameworks</td><td><a href="https://github.com/voicekit/agents">Python</a> · <a href="https://github.com/voicekit/agents-js">Node.js</a> · <a href="https://github.com/voicekit/agent-playground">Playground</a></td></tr><tr></tr>
<tr><td>Services</td><td><b>VoiceKit server</b> · <a href="https://github.com/voicekit/egress">Egress</a> · <a href="https://github.com/voicekit/ingress">Ingress</a> · <a href="https://github.com/voicekit/sip">SIP</a></td></tr><tr></tr>
<tr><td>Resources</td><td><a href="https://docs.voicekit.rixy.ai">Docs</a> · <a href="https://github.com/voicekit-examples">Example apps</a> · <a href="https://voicekit.rixy.ai/cloud">Cloud</a> · <a href="https://docs.voicekit.rixy.ai/home/self-hosting/deployment">Self-hosting</a> · <a href="https://github.com/voicekit/voicekit-cli">CLI</a></td></tr>
</tbody>
</table>
<!--END_REPO_NAV-->
