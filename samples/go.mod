module github.com/DataDog/orchestrion/_samples

go 1.25.0

replace (
	github.com/DataDog/orchestrion => ..
	github.com/DataDog/orchestrion/instrument => ../instrument
)

require (
	github.com/99designs/gqlgen v0.17.94
	github.com/DataDog/orchestrion v1.12.0
	github.com/DataDog/orchestrion/instrument v1.12.1
	github.com/IBM/sarama v1.60.2
	github.com/Shopify/sarama v1.38.1
	github.com/aws/aws-sdk-go v1.55.8
	github.com/aws/aws-sdk-go-v2 v1.45.1
	github.com/aws/aws-sdk-go-v2/service/s3 v1.109.1
	github.com/elastic/go-elasticsearch/v6 v6.8.10
	github.com/elastic/go-elasticsearch/v7 v7.17.10
	github.com/elastic/go-elasticsearch/v8 v8.19.7
	github.com/gin-gonic/gin v1.12.0
	github.com/go-chi/chi/v5 v5.3.2
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-redis/redis/v7 v7.4.1
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gocql/gocql v1.7.0
	github.com/gofiber/fiber/v2 v2.52.15
	github.com/gomodule/redigo v1.9.3
	github.com/hashicorp/vault/api v1.23.0
	github.com/jackc/pgx/v5 v5.10.0
	github.com/labstack/echo/v4 v4.15.4
	github.com/mattn/go-sqlite3 v1.14.50
	github.com/redis/go-redis/v9 v9.22.0
	github.com/sirupsen/logrus v1.10.2
	github.com/stretchr/testify v1.12.1
	github.com/twitchtv/twirp v8.1.3+incompatible
	github.com/vektah/gqlparser/v2 v2.5.36
	go.mongodb.org/mongo-driver v1.17.9
	golang.org/x/tools v0.49.0
	google.golang.org/grpc v1.83.2
	gorm.io/driver/postgres v1.6.2
	gorm.io/gorm v1.31.2
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.23.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.13.0 // indirect
	cloud.google.com/go/pubsub v1.51.1 // indirect
	cloud.google.com/go/pubsub/v2 v2.7.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/stats v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.82.3 // indirect
	github.com/DataDog/datadog-go/v5 v5.9.1 // indirect
	github.com/DataDog/dd-trace-go/contrib/99designs/gqlgen/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/IBM/sarama/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/Shopify/sarama/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/aerospike/aerospike-client-go.v7/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/aws/aws-sdk-go-v2/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/aws/aws-sdk-go/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/cloud.google.com/go/pubsub.v1/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/cloud.google.com/go/pubsub.v2/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/confluentinc/confluent-kafka-go/kafka.v2/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/confluentinc/confluent-kafka-go/kafka/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/database/sql/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/elastic/go-elasticsearch.v6/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gin-gonic/gin/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-chi/chi.v5/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-chi/chi/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-redis/redis.v7/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-redis/redis.v8/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go-redis/redis/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go.mongodb.org/mongo-driver.v2/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/go.mongodb.org/mongo-driver/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gocql/gocql/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gofiber/fiber.v2/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gomodule/redigo/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/google.golang.org/grpc/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gorilla/mux/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/gorm.io/gorm.v1/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/graph-gophers/graphql-go/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/graphql-go/graphql/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/hashicorp/vault/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/jackc/pgx.v5/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/julienschmidt/httprouter/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/k8s.io/client-go/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/labstack/echo.v4/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/labstack/echo.v5/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/log/slog/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/net/http/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/redis/go-redis.v9/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/redis/rueidis/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/rs/zerolog/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/segmentio/kafka-go/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/sirupsen/logrus/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/twitchtv/twirp/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/twmb/franz-go/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/valkey-io/valkey-go/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/contrib/valyala/fasthttp/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/orchestrion/all/v2 v2.10.0 // indirect
	github.com/DataDog/dd-trace-go/v2 v2.10.0 // indirect
	github.com/DataDog/go-libddwaf/v5 v5.0.1 // indirect
	github.com/DataDog/go-runtime-metrics-internal v0.0.4-0.20260217080614-b0f4edc38a6d // indirect
	github.com/DataDog/go-sqllexer v0.2.4 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/DataDog/sketches-go v1.4.8 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/aerospike/aerospike-client-go/v7 v7.10.2 // indirect
	github.com/agnivade/levenshtein v1.2.1 // indirect
	github.com/andybalholm/brotli v1.2.3 // indirect
	github.com/antithesishq/antithesis-sdk-go v0.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.65.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.51.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.11.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.49.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sfn v1.47.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sns v1.44.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.48.1 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic v1.15.3 // indirect
	github.com/bytedance/sonic/loader v0.5.2 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/lipgloss v1.1.0 // indirect
	github.com/charmbracelet/x/ansi v0.11.8 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/confluentinc/confluent-kafka-go v1.9.2 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.15.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/dave/dst v0.27.4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/ebitengine/purego v0.10.2 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.11.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.15 // indirect
	github.com/gin-contrib/sse v1.1.1 // indirect
	github.com/go-chi/chi v1.5.5 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.3 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/pprof v0.0.0-20260830191439-4932ad3515ea // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.21 // indirect
	github.com/googleapis/gax-go/v2 v2.24.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/graph-gophers/graphql-go v1.10.2 // indirect
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
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/hashicorp/vault/sdk v0.25.0 // indirect
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
	github.com/klauspost/compress v1.19.2 // indirect
	github.com/klauspost/cpuid/v2 v2.4.0 // indirect
	github.com/labstack/echo/v5 v5.3.1 // indirect
	github.com/labstack/gommon v0.5.0 // indirect
	github.com/leodido/go-urn v1.5.0 // indirect
	github.com/linkdata/deadlock v0.5.5 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20260802145828-341c2f0c90b5 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-runewidth v0.0.28 // indirect
	github.com/minio/highwayhash v1.0.4 // indirect
	github.com/minio/simdjson-go v0.4.5 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/montanaflynn/stats v0.12.5 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/nats-io/jwt/v2 v2.8.2 // indirect
	github.com/nats-io/nats-server/v2 v2.14.5 // indirect
	github.com/nats-io/nats.go v1.53.1 // indirect
	github.com/nats-io/nkeys v0.4.16 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
	github.com/petermattis/goid v0.0.0-20260820044319-269ab09b5261 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.29 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/polyfloyd/go-errorlint v1.8.1-0.20250906200200-9b25878c4dea // indirect
	github.com/power-devops/perfstat v0.0.0-20260805114148-88456608a4f6 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.61.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9 // indirect
	github.com/redis/rueidis v1.0.77 // indirect
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.1 // indirect
	github.com/segmentio/kafka-go v0.4.51 // indirect
	github.com/shirou/gopsutil/v4 v4.26.7 // indirect
	github.com/sosodev/duration v1.4.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/trailofbits/go-mutexasserts v0.0.0-20250514102930-c1f3d2e37561 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/twmb/franz-go v1.21.6 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.13.1 // indirect
	github.com/ugorji/go/codec v1.3.2 // indirect
	github.com/urfave/cli/v2 v2.27.7 // indirect
	github.com/valkey-io/valkey-go v1.0.77 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.73.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xo/terminfo v1.0.0 // indirect
	github.com/xrash/smetrics v0.0.0-20250705151800-55b8f293f342 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yuin/gopher-lua v1.1.2 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.mongodb.org/mongo-driver/v2 v2.8.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/component v1.65.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.65.0 // indirect
	go.opentelemetry.io/collector/pdata v1.65.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.159.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.71.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/mock v0.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/arch v0.30.0 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/exp v0.0.0-20260813180055-c1d0aacb2297 // indirect
	golang.org/x/mod v0.40.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/api v0.295.0 // indirect
	google.golang.org/genproto v0.0.0-20260825221802-da73d73af1c5 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260825221802-da73d73af1c5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260825221802-da73d73af1c5 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apimachinery v0.36.0-alpha.2 // indirect
	k8s.io/client-go v0.36.0-alpha.2 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3 // indirect
)
