---
created: 2026-11-15T05:00:00.000Z
updated: 2026-11-15T05:00:00.000Z
title: "IDE Integration"
index: 6
---

## IDE Integration

the eg extension for vs code lists the workloads in your workspace (found with `eg workloads detect`),
runs them locally, on the host, on a cluster or as a dev environment you can connect to, and lets you
keep, merge or discard what a workload changed. it drives the `eg` cli, so `eg` must be installed and on
your `PATH`, see installation. the editor must be version 1.90 or newer.

the extension id is `egdaemon.eg-vscode`. the open source builds of vs code, such as vscodium, can't use
the microsoft marketplace and install from open vsx instead.

| editor | source | install |
| --- | --- | --- |
| vs code | [visual studio marketplace](https://marketplace.visualstudio.com/items?itemName=egdaemon.eg-vscode) | `code --install-extension egdaemon.eg-vscode`, or search for `EG` in the extensions view. |
| vscodium, other open source builds | [open vsx](https://open-vsx.org/extension/egdaemon/eg-vscode) | `codium --install-extension egdaemon.eg-vscode`, or search for `EG` in the extensions view. |
| offline | the `.vsix` downloaded from either site | `code --install-extension ./eg-vscode.vsix`, or `codium` for the open source build. |
