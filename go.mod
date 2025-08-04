module github.com/egdaemon/eg

go 1.24.0

toolchain go1.24.5

require (
	github.com/KimMachineGun/automemlimit v0.7.4
	github.com/alecthomas/kong v1.12.1
	github.com/alitto/pond/v2 v2.5.0
	github.com/containers/podman/v5 v5.5.2
	github.com/coreos/go-systemd/v22 v22.5.1-0.20250731085716-6b6ee9e09932
	github.com/dave/jennifer v1.7.1
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/docker/docker v28.3.3+incompatible
	github.com/dominikbraun/graph v0.23.0
	github.com/dustin/go-humanize v1.0.1
	github.com/egdaemon/wasinet/wasinet v0.0.0-20250731121320-3287a738a635
	github.com/egdaemon/wasinet/wazeronet v0.0.0-20250731121320-3287a738a635
	github.com/fsnotify/fsnotify v1.9.0
	github.com/go-git/go-git/v5 v5.16.2
	github.com/gofrs/uuid v4.4.0+incompatible
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/ipfs/go-datastore v0.8.2
	github.com/ipfs/go-log/v2 v2.8.0
	github.com/justinas/alice v1.2.0
	github.com/libp2p/go-libp2p v0.41.1
	github.com/libp2p/go-libp2p-kad-dht v0.33.1
	github.com/logrusorgru/aurora v2.0.3+incompatible
	github.com/mailru/easyjson v0.9.0
	github.com/marcboeker/go-duckdb/v2 v2.3.4
	github.com/mattn/go-isatty v0.0.20
	github.com/multiformats/go-multiaddr v0.16.0
	github.com/onsi/ginkgo/v2 v2.23.4
	github.com/onsi/gomega v1.38.0
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/pkg/errors v0.9.1
	github.com/posener/complete v1.2.3
	github.com/shirou/gopsutil/v4 v4.25.6
	github.com/stretchr/testify v1.10.0
	github.com/tetratelabs/wazero v1.9.0
	github.com/willabides/kongplete v0.4.0
	go.uber.org/automaxprocs v1.6.0
	golang.org/x/crypto v0.40.0
	golang.org/x/exp v0.0.0-20250718183923-645b1fa84792
	golang.org/x/oauth2 v0.30.0
	golang.org/x/term v0.33.0
	golang.org/x/time v0.12.0
	golang.org/x/tools v0.35.0
	google.golang.org/grpc v1.74.2
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/elastic/gosigar v0.14.3 // indirect
	github.com/filecoin-project/go-clock v0.1.0 // indirect
	github.com/raulk/go-watchdog v1.3.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/BurntSushi/toml v1.5.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hcsshim v0.12.9 // indirect
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/apache/arrow-go/v18 v18.4.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chzyer/readline v1.5.1 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/containerd/cgroups/v3 v3.0.5 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.1 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/containers/buildah v1.40.1 // indirect
	github.com/containers/common v0.63.1
	github.com/containers/image/v5 v5.35.0 // indirect
	github.com/containers/libtrust v0.0.0-20230121012942-c1716e8a8d01 // indirect
	github.com/containers/ocicrypt v1.2.1 // indirect
	github.com/containers/psgo v1.9.0 // indirect
	github.com/containers/storage v1.58.0 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/disiqueira/gotree/v3 v3.0.2 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/duckdb/duckdb-go-bindings v0.1.17 // indirect
	github.com/duckdb/duckdb-go-bindings/darwin-amd64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/darwin-arm64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/linux-amd64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/linux-arm64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/windows-amd64 v0.1.12 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/noise v1.1.0 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.1 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/runtime v0.28.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.3.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/godbus/dbus/v5 v5.1.1-0.20241109141217-c266b19b28e9 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.20.3 // indirect
	github.com/google/go-intervals v0.0.2 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/pprof v0.0.0-20250630185457-6e76a2b096b5 // indirect
	github.com/gorilla/schema v1.4.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/huin/goupnp v1.3.0 // indirect
	github.com/ipfs/boxo v0.30.0 // indirect
	github.com/ipfs/go-cid v0.5.0 // indirect
	github.com/ipld/go-ipld-prime v0.21.0 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/koron/go-ssdp v0.0.6 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/letsencrypt/boulder v0.0.0-20240620165639-de9c06129bec // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/libp2p/go-cidranger v1.1.0 // indirect
	github.com/libp2p/go-flow-metrics v0.3.0 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.4.1 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.7.0 // indirect
	github.com/libp2p/go-libp2p-record v0.3.1 // indirect
	github.com/libp2p/go-libp2p-routing-helpers v0.7.5 // indirect
	github.com/libp2p/go-msgio v0.3.0 // indirect
	github.com/libp2p/go-netroute v0.2.2 // indirect
	github.com/libp2p/go-reuseport v0.4.0 // indirect
	github.com/libp2p/go-yamux/v5 v5.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/marcboeker/go-duckdb/arrowmapping v0.0.10 // indirect
	github.com/marcboeker/go-duckdb/mapping v0.0.11 // indirect
	github.com/marten-seemann/tcp v0.0.0-20210406111302-dfbc87cc63fd // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-sqlite3 v1.14.28 // indirect
	github.com/miekg/dns v1.1.68 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mikioh/tcpinfo v0.0.0-20190314235526-30a79bb1804b // indirect
	github.com/mikioh/tcpopt v0.0.0-20190314235656-172688c1accc // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mistifyio/go-zfs/v3 v3.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/capability v0.4.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr-dns v0.4.1 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.9.2 // indirect
	github.com/multiformats/go-multihash v0.2.3 // indirect
	github.com/multiformats/go-multistream v0.6.1 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/opencontainers/cgroups v0.0.3 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runc v1.2.6 // indirect
	github.com/opencontainers/runtime-spec v1.2.1 // indirect
	github.com/opencontainers/runtime-tools v0.9.1-0.20250303011046-260e151b8552 // indirect
	github.com/opencontainers/selinux v1.12.0 // indirect
	github.com/ostreedev/ostree-go v0.0.0-20210805093236-719684c64e4f // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pion/datachannel v1.5.10 // indirect
	github.com/pion/dtls/v2 v2.2.12 // indirect
	github.com/pion/dtls/v3 v3.0.6 // indirect
	github.com/pion/ice/v4 v4.0.10 // indirect
	github.com/pion/interceptor v0.1.40 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/mdns/v2 v2.0.7 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.15 // indirect
	github.com/pion/rtp v1.8.21 // indirect
	github.com/pion/sctp v1.8.39 // indirect
	github.com/pion/sdp/v3 v3.0.15 // indirect
	github.com/pion/srtp/v3 v3.0.6 // indirect
	github.com/pion/stun v0.6.1 // indirect
	github.com/pion/stun/v3 v3.0.0 // indirect
	github.com/pion/transport/v2 v2.2.10 // indirect
	github.com/pion/transport/v3 v3.0.7 // indirect
	github.com/pion/turn/v4 v4.0.2 // indirect
	github.com/pion/webrtc/v4 v4.1.3 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/pkg/sftp v1.13.9 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/polydawn/refmt v0.89.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/proglottis/gpgme v0.1.4 // indirect
	github.com/prometheus/client_golang v1.23.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/quic-go/quic-go v0.50.1 // indirect
	github.com/quic-go/webtransport-go v0.8.1-0.20241018022711-4ac2c9250e66 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/riywo/loginshell v0.0.0-20200815045211-7d26008be1ab // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/sigstore/fulcio v1.6.6 // indirect
	github.com/sigstore/protobuf-specs v0.4.3 // indirect
	github.com/sigstore/rekor v1.3.10 // indirect
	github.com/sigstore/sigstore v1.9.3 // indirect
	github.com/sirupsen/logrus v1.9.3
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/smallstep/pkcs7 v0.1.1 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20230803200340-78284954bff6 // indirect
	github.com/sylabs/sif/v2 v2.21.1 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tchap/go-patricia/v2 v2.3.2 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	github.com/vbatts/tar-split v0.12.1 // indirect
	github.com/vbauerster/mpb/v8 v8.9.3 // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/fx v1.24.0 // indirect
	go.uber.org/mock v0.5.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	lukechampine.com/blake3 v1.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
	tags.cncf.io/container-device-interface v1.0.1 // indirect
)
