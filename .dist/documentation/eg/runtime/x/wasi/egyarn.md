<!-- Code generated by gomarkdoc. DO NOT EDIT -->

## In Development Package, API may change between versions.

```go
import "github.com/egdaemon/eg/runtime/x/wasi/egyarn"
```

Package egyarn has supporting functions for configuring the environment for running yarn berry for caching. in the future we may support previous versions.

<a name="CacheDirectory"></a>
## func [CacheDirectory](<https://github.com/egdaemon/eg/blob/main/runtime/x/wasi/egyarn/egyarn.go#L15>)

```go
func CacheDirectory(dirs ...string) string
```



<a name="Env"></a>
## func [Env](<https://github.com/egdaemon/eg/blob/main/runtime/x/wasi/egyarn/egyarn.go#L20>)

```go
func Env() ([]string, error)
```

attempt to build the yarn environment that properly

<a name="Runtime"></a>
## func [Runtime](<https://github.com/egdaemon/eg/blob/main/runtime/x/wasi/egyarn/egyarn.go#L29>)

```go
func Runtime() shell.Command
```

Create a shell runtime that properly sets up the yarn environment for caching.

