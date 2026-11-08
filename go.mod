module ttnmapper-services

go 1.24.0

replace github.com/throttled/throttled/v2 => github.com/TheThingsIndustries/throttled/v2 v2.7.1-noredis

replace github.com/ebarkie/aprs => github.com/ttnmapper/aprs v1.0.4-0.20210814150543-b944bc851c7d

require (
	github.com/766b/chi-prometheus v0.0.0-20180509160047-46ac2b31aa30
	github.com/artyom/csvstruct v1.1.0
	github.com/auth0/go-jwt-middleware v1.0.1
	github.com/bserdar/jsonstream v0.0.0-20190428032403-9f1769267072
	github.com/caarlos0/env/v11 v11.3.1
	github.com/chirpstack/chirpstack/api/go/v4 v4.6.0
	github.com/deepmap/oapi-codegen v1.9.0
	github.com/ebarkie/aprs v1.0.3
	github.com/fogleman/gg v1.3.0
	github.com/form3tech-oss/jwt-go v3.2.5+incompatible
	github.com/getkin/kin-openapi v0.131.0
	github.com/go-chi/chi v3.3.3+incompatible
	github.com/go-chi/chi/v5 v5.2.4
	github.com/go-chi/cors v1.2.0
	github.com/go-chi/render v1.0.1
	github.com/golang/protobuf v1.5.3
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.0
	github.com/j4/gosm v0.0.0-20141123101329-8f3e37d8629e
	github.com/jackc/pgx/v5 v5.8.0
	github.com/kellydunn/golang-geo v0.7.0
	github.com/labstack/echo/v4 v4.6.1
	github.com/ormembaar/angry-purple-tiger v0.1.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/paulmach/go.geojson v1.4.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.23.2
	github.com/streadway/amqp v1.0.0
	github.com/ulule/deepcopier v0.0.0-20200430083143-45decc6639b6
	github.com/umahmood/haversine v0.0.0-20151105152445-808ab04add26
	go.packetbroker.org/api/routing v1.8.18
	go.packetbroker.org/api/v3 v3.12.4
	go.packetbroker.org/pb v0.0.0-20220124200758-fd08c86e9140
	go.thethings.network/lorawan-stack/v3 v3.25.1
	go.uber.org/zap v1.24.0
	golang.org/x/image v0.5.0
	golang.org/x/net v0.43.0
	google.golang.org/protobuf v1.36.8
	gorm.io/datatypes v1.0.6
	gorm.io/driver/mysql v1.3.2
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.1
)

require (
	github.com/TheThingsIndustries/protoc-gen-go-flags v1.0.6 // indirect
	github.com/TheThingsIndustries/protoc-gen-go-json v1.4.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/ebarkie/weatherlink v1.0.2 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/erikstmartin/go-testdb v0.0.0-20160219214506-8d10e4a1bae5 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/gotnospirit/makeplural v0.0.0-20180622080156-a5f48d94d976 // indirect
	github.com/gotnospirit/messageformat v0.0.0-20221001023931-dfe49f1eb092 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.15.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kylelemons/go-gypsy v1.0.0 // indirect
	github.com/labstack/gommon v0.3.0 // indirect
	github.com/lib/pq v1.10.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.1 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/ziutek/mymysql v1.5.4 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto v0.0.0-20231030173426-d783a09b4405 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231030173426-d783a09b4405 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231030173426-d783a09b4405 // indirect
	google.golang.org/grpc v1.59.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
