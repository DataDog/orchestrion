module github.com/DataDog/orchestrion/_samples

go 1.23.0

replace (
	github.com/DataDog/orchestrion => ..
	github.com/DataDog/orchestrion/instrument => ../instrument
)

require (
	github.com/99designs/gqlgen v0.17.76
	github.com/DataDog/orchestrion v1.5.0
	github.com/DataDog/orchestrion/instrument v1.4.0
	github.com/IBM/sarama v1.45.2
	github.com/Shopify/sarama v1.38.1
	github.com/aws/aws-sdk-go v1.55.7
	github.com/aws/aws-sdk-go-v2 v1.36.5
	github.com/aws/aws-sdk-go-v2/service/s3 v1.83.0
	github.com/elastic/go-elasticsearch/v6 v6.8.10
	github.com/elastic/go-elasticsearch/v7 v7.17.10
	github.com/elastic/go-elasticsearch/v8 v8.18.1
	github.com/gin-gonic/gin v1.10.1
	github.com/go-chi/chi/v5 v5.2.2
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-redis/redis/v7 v7.4.1
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gocql/gocql v1.7.0
	github.com/gofiber/fiber/v2 v2.52.8
	github.com/gomodule/redigo v1.9.2
	github.com/hashicorp/vault/api v1.20.0
	github.com/jackc/pgx/v5 v5.7.5
	github.com/labstack/echo/v4 v4.13.4
	github.com/mattn/go-sqlite3 v1.14.28
	github.com/redis/go-redis/v9 v9.11.0
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.10.0
	github.com/twitchtv/twirp v8.1.3+incompatible
	github.com/vektah/gqlparser/v2 v2.5.30
	go.mongodb.org/mongo-driver v1.17.4
	golang.org/x/tools v0.35.0
	google.golang.org/grpc v1.73.0
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.30.0
)

require (
	cloud.google.com/go v0.121.3 // indirect
	cloud.google.com/go/auth v0.16.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	cloud.google.com/go/iam v1.5.2 // indirect
	cloud.google.com/go/pubsub v1.49.0 // indirect
	github.com/DataDog/appsec-internal-go v1.13.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.68.0 // indirect
	github.com/DataDog/datadog-go/v5 v5.6.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/99designs/gqlgen/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/IBM/sarama/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/Shopify/sarama/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/aws/aws-sdk-go-v2/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/aws/aws-sdk-go/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/cloud.google.com/go/pubsub.v1/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/confluentinc/confluent-kafka-go/kafka.v2/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/confluentinc/confluent-kafka-go/kafka/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/database/sql/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/elastic/go-elasticsearch.v6/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gin-gonic/gin/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-chi/chi.v5/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-chi/chi/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-redis/redis.v7/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-redis/redis.v8/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-redis/redis/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go.mongodb.org/mongo-driver/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gocql/gocql/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gofiber/fiber.v2/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gomodule/redigo/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/google.golang.org/grpc/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gorilla/mux/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gorm.io/gorm.v1/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/graph-gophers/graphql-go/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/graphql-go/graphql/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/hashicorp/vault/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/jackc/pgx.v5/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/julienschmidt/httprouter/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/k8s.io/client-go/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/labstack/echo.v4/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/log/slog/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/net/http/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/redis/go-redis.v9/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/redis/rueidis/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/segmentio/kafka-go/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/sirupsen/logrus/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/twitchtv/twirp/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/valkey-io/valkey-go/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/orchestrion/all/v2 v2.1.0 // indirect
	github.com/DataDog/dd-trace-go/v2 v2.1.0 // indirect
	github.com/DataDog/go-libddwaf/v4 v4.3.0 // indirect
	github.com/DataDog/go-runtime-metrics-internal v0.0.4-0.20250710075948-d3ee4f430e22 // indirect
	github.com/DataDog/go-sqllexer v0.1.6 // indirect
	github.com/DataDog/go-tuf v1.1.0-0.5.2 // indirect
	github.com/DataDog/gostackparse v0.7.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.29.1 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/agnivade/levenshtein v1.2.1 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.11 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.45 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.19 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.44.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.40.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.10.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.35.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sfn v1.35.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.34.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.38.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.0 // indirect
	github.com/aws/smithy-go v1.22.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb // indirect
	github.com/bytedance/sonic v1.13.3 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.3.1 // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.9.3 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/cloudwego/base64x v0.1.5 // indirect
	github.com/confluentinc/confluent-kafka-go v1.9.2 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.11.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/dave/dst v0.27.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/eapache/queue/v2 v2.0.0-20230407133247-75960ed334e4 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.7.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.9 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-chi/chi v1.5.5 // indirect
	github.com/go-jose/go-jose/v4 v4.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.27.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.18.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/pprof v0.0.0-20250630185457-6e76a2b096b5 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/graph-gophers/graphql-go v1.6.0 // indirect
	github.com/graphql-go/graphql v0.8.1 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.2.0 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.7 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/hashicorp/vault/sdk v0.14.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20250317134145-8bc96cf8fc35 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/nats-io/jwt/v2 v2.7.4 // indirect
	github.com/nats-io/nats-server/v2 v2.11.6 // indirect
	github.com/nats-io/nats.go v1.43.0 // indirect
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9 // indirect
	github.com/redis/rueidis v1.0.62 // indirect
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/segmentio/kafka-go v0.4.48 // indirect
	github.com/shirou/gopsutil/v4 v4.25.6 // indirect
	github.com/sosodev/duration v1.3.1 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/tinylib/msgp v1.3.0 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	github.com/urfave/cli/v2 v2.27.7 // indirect
	github.com/valkey-io/valkey-go v1.0.62 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.64.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/xrash/smetrics v0.0.0-20250705151800-55b8f293f342 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/component v1.36.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.36.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.130.0 // indirect
	go.opentelemetry.io/collector/pdata v1.36.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.130.0 // indirect
	go.opentelemetry.io/collector/semconv v0.125.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.62.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/log v0.13.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/arch v0.19.0 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/exp v0.0.0-20250711185948-6ae5c78190dc // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/term v0.33.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/api v0.241.0 // indirect
	google.golang.org/genproto v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.33.0-alpha.2 // indirect
	k8s.io/client-go v0.33.0-alpha.2 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
)
