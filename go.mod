module github.com/stellar/go-stellar-sdk

go 1.24.0

require (
	cloud.google.com/go/storage v1.42.0
	github.com/BurntSushi/toml v1.3.2
	github.com/Masterminds/squirrel v1.5.4
	github.com/Microsoft/go-winio v0.6.1
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2
	github.com/aws/aws-sdk-go v1.45.27
	github.com/djherbis/fscache v0.10.1
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-errors/errors v1.5.1
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/schema v1.4.1
	github.com/guregu/null v4.0.0+incompatible
	github.com/holiman/uint256 v1.2.3
	github.com/howeyc/gopass v0.0.0-20170109162249-bf9dde6d0d2c
	github.com/jarcoal/httpmock v0.0.0-20161210151336-4442edb3db31
	github.com/jmoiron/sqlx v1.3.5
	github.com/lib/pq v1.10.9
	github.com/manucorporat/sse v0.0.0-20160126180136-ee05b128a739
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.27.10
	github.com/pelletier/go-toml v1.9.5
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.17.0
	github.com/prometheus/client_model v0.5.0
	github.com/rs/cors v1.11.0
	github.com/rubenv/sql-migrate v1.5.2
	github.com/segmentio/go-loggly v0.5.1-0.20171222203950-eb91657e62b2
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.17.0
	github.com/stellar/go-xdr v0.0.0-20231122183749-b53fb00bcac2
	github.com/stretchr/testify v1.10.0
	github.com/tyler-smith/go-bip39 v0.0.0-20180618194314-52158e4697b8
	github.com/xdrpp/goxdr v0.1.1
	google.golang.org/api v0.183.0
	gopkg.in/gavv/httpexpect.v1 v1.0.0-20170111145843-40724cf1e4a0
	gopkg.in/tylerb/graceful.v1 v1.2.15
)

require (
	github.com/aws/aws-sdk-go-v2 v1.36.5
	github.com/aws/aws-sdk-go-v2/config v1.29.17
	github.com/aws/aws-sdk-go-v2/credentials v1.17.70
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.83
	github.com/aws/aws-sdk-go-v2/service/s3 v1.83.0
	github.com/aws/smithy-go v1.22.4
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/creachadair/jrpc2 v1.2.0
	github.com/fsouza/fake-gcs-server v1.49.2
	go.opentelemetry.io/otel v1.34.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.28.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.28.0
	go.opentelemetry.io/otel/sdk v1.34.0
	go.opentelemetry.io/otel/trace v1.34.0
)

require (
	cloud.google.com/go/auth v0.5.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.2 // indirect
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	cloud.google.com/go/iam v1.1.8 // indirect
	cloud.google.com/go/pubsub v1.38.0 // indirect
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.11 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.32 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.34.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/creachadair/mds v0.13.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-gorp/gorp/v3 v3.1.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gobuffalo/packd v1.0.2 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
	github.com/google/renameio/v2 v2.0.0 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/gorilla/handlers v1.5.2 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/mattn/go-sqlite3 v1.14.17 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/pkg/xattr v0.4.9 // indirect
	github.com/sagikazarmark/locafero v0.3.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/yudai/pp v2.0.1+incompatible // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240701130421-f6361c86f094 // indirect
	gopkg.in/djherbis/atime.v1 v1.0.0 // indirect
	gopkg.in/djherbis/stream.v1 v1.3.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)

require (
	cloud.google.com/go v0.114.0 // indirect
	github.com/ajg/form v0.0.0-20160822230020-523a5da1a92f // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/buger/goreplay v1.3.2
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fatih/structs v1.0.0 // indirect
	github.com/gavv/monotime v0.0.0-20161010190848-47d58efa6955 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.7.0
	github.com/google/go-querystring v0.0.0-20160401233042-9235644dd9e5 // indirect
	github.com/googleapis/gax-go/v2 v2.12.4 // indirect
	github.com/hashicorp/golang-lru v1.0.2
	github.com/imkira/go-interpol v1.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.17.6
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moul/http2curl v0.0.0-20161031194548-4e24498b31db // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.34.0 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/yalp/jsonpath v0.0.0-20150812003900-31a79c7593bb // indirect
	github.com/yudai/gojsondiff v0.0.0-20170107030110-7b1b7adf999d // indirect
	github.com/yudai/golcs v0.0.0-20150405163532-d1c525dea8ce // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.27.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/genproto v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/grpc v1.64.1 // indirect
	google.golang.org/protobuf v1.34.2
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
