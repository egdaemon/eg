---
created: 2024-02-28T05:00:00.000Z
updated: 2024-02-28T05:00:00.000Z
title: 'Getting Started'
index: 1
---
## Getting Started

This guide covers writing a basic module and running it locally and remotely on egdaemon's
platform.

## implementing your first module

initialize the eg directory, the workspace command will initialize a hello world module to run.
```bash
eg actl bootstrap module
ls -lha .eg
drwxr-xr-x  3 user user 4.0K Jan 1 00:00 .
drwxr-xr-x 24 user user 4.0K Jan 1 00:00 ..
-rw-r--r--  1 user user   18 Jan 1 00:00 .gitignore
-rw-r--r--  1 user user   22 Jan 1 00:00 go.mod
-rw-r--r--  1 user user   22 Jan 1 00:00 go.sum
-rw-r--r--  1 user user 2.9K Jan 1 00:00 main.go
```

run the module locally, you can use eg compute local to integrate with any CI/CD platform that supports golang and a linux shell.
```bash
eg compute local
```

run the module remotely using egdaemon, you'll need to register and either pay for a plan or setup a self hosted runner.
```bash
eg register # follow the link and register your account and setup billing.
eg compute upload
```

## running a container compute task (coming soon, its in alpha)

```bash
eg compute c8s upload ./Containerfile
```

## running a wasi payload (coming soon, its in alpha)

```bash
eg compute wasi upload ./myprog.wasi
```